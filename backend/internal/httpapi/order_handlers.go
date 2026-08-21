package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/bruhjeshhh/delivery-tracker/internal/assignment"
	"github.com/bruhjeshhh/delivery-tracker/internal/auth"
	"github.com/bruhjeshhh/delivery-tracker/internal/models"
	"github.com/bruhjeshhh/delivery-tracker/internal/notifications"
	"github.com/bruhjeshhh/delivery-tracker/internal/rateengine"
	"github.com/bruhjeshhh/delivery-tracker/internal/zones"
)

type orderInput struct {
	CustomerID     *int64             `json:"customer_id,omitempty"` // set by admin when creating on behalf of a customer
	PickupAddress  string             `json:"pickup_address"`
	PickupPincode  string             `json:"pickup_pincode"`
	DropAddress    string             `json:"drop_address"`
	DropPincode    string             `json:"drop_pincode"`
	LengthCM       float64            `json:"length_cm"`
	BreadthCM      float64            `json:"breadth_cm"`
	HeightCM       float64            `json:"height_cm"`
	ActualWeightKg float64            `json:"actual_weight_kg"`
	OrderType      models.OrderType   `json:"order_type"`
	PaymentType    models.PaymentType `json:"payment_type"`
}

type chargePreview struct {
	FromZoneID         int64   `json:"from_zone_id"`
	ToZoneID           int64   `json:"to_zone_id"`
	VolumetricWeightKg float64 `json:"volumetric_weight_kg"`
	BillableWeightKg   float64 `json:"billable_weight_kg"`
	Charge             float64 `json:"charge"`
}

// resolveCharge is shared by the preview endpoint and order creation, so the
// number shown to the customer before confirmation is guaranteed to be the
// exact number the order is created with — no drift between preview and
// final charge.
func (s *Server) resolveCharge(ctx context.Context, in orderInput) (chargePreview, models.RateCard, error) {
	fromZone, err := s.zones.DetectZone(ctx, in.PickupPincode)
	if err != nil {
		return chargePreview{}, models.RateCard{}, err
	}
	toZone, err := s.zones.DetectZone(ctx, in.DropPincode)
	if err != nil {
		return chargePreview{}, models.RateCard{}, err
	}
	card, err := s.zones.GetRateCard(ctx, in.OrderType, fromZone, toZone)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return chargePreview{}, models.RateCard{}, rateengine.ErrNoRateCard
		}
		return chargePreview{}, models.RateCard{}, err
	}
	res, err := rateengine.Calculate(rateengine.ChargeInput{
		LengthCM: in.LengthCM, BreadthCM: in.BreadthCM, HeightCM: in.HeightCM,
		ActualWeightKg: in.ActualWeightKg, OrderType: in.OrderType, PaymentType: in.PaymentType,
	}, card)
	if err != nil {
		return chargePreview{}, models.RateCard{}, err
	}
	return chargePreview{
		FromZoneID: fromZone, ToZoneID: toZone,
		VolumetricWeightKg: res.VolumetricWeightKg, BillableWeightKg: res.BillableWeightKg,
		Charge: res.Charge,
	}, card, nil
}

func (s *Server) handlePreviewCharge(w http.ResponseWriter, r *http.Request) {
	var in orderInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	preview, _, err := s.resolveCharge(r.Context(), in)
	if err != nil {
		writeChargeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func writeChargeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, zones.ErrPincodeNotMapped):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	case errors.Is(err, rateengine.ErrNoRateCard):
		writeError(w, http.StatusUnprocessableEntity, "no rate card configured for this route and order type; contact admin")
	default:
		writeError(w, http.StatusInternalServerError, "could not calculate charge")
	}
}

func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromContext(r.Context())
	var in orderInput
	if err := decodeJSON(r, &in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	customerID := claims.UserID
	if claims.Role == models.RoleAdmin && in.CustomerID != nil {
		customerID = *in.CustomerID // admin creating on behalf of a customer
	} else if claims.Role != models.RoleCustomer && claims.Role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "only customers or admins can create orders")
		return
	}

	ctx := r.Context()
	preview, card, err := s.resolveCharge(ctx, in)
	if err != nil {
		writeChargeError(w, err)
		return
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	defer tx.Rollback(ctx)

	var order models.Order
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (
			customer_id, created_by, pickup_address, pickup_pincode, drop_address, drop_pincode,
			length_cm, breadth_cm, height_cm, actual_weight_kg, volumetric_weight_kg, billable_weight_kg,
			order_type, payment_type, from_zone_id, to_zone_id, rate_card_id, charge, status
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,'CREATED')
		RETURNING id, customer_id, created_by, pickup_address, pickup_pincode, drop_address, drop_pincode,
			length_cm, breadth_cm, height_cm, actual_weight_kg, volumetric_weight_kg, billable_weight_kg,
			order_type, payment_type, from_zone_id, to_zone_id, rate_card_id, charge, status, created_at, updated_at`,
		customerID, claims.UserID, in.PickupAddress, in.PickupPincode, in.DropAddress, in.DropPincode,
		in.LengthCM, in.BreadthCM, in.HeightCM, in.ActualWeightKg, preview.VolumetricWeightKg, preview.BillableWeightKg,
		in.OrderType, in.PaymentType, preview.FromZoneID, preview.ToZoneID, card.ID, preview.Charge,
	).Scan(&order.ID, &order.CustomerID, &order.CreatedBy, &order.PickupAddress, &order.PickupPincode,
		&order.DropAddress, &order.DropPincode, &order.LengthCM, &order.BreadthCM, &order.HeightCM,
		&order.ActualWeightKg, &order.VolumetricWeightKg, &order.BillableWeightKg, &order.OrderType,
		&order.PaymentType, &order.FromZoneID, &order.ToZoneID, &order.RateCardID, &order.Charge,
		&order.Status, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create order")
		return
	}

	if err := logHistory(ctx, tx, order.ID, models.StatusCreated, &claims.UserID, claims.Role, "order created"); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	subject, body := notifications.StatusMessage(order.ID, models.StatusCreated)
	if err := notifications.Enqueue(ctx, tx, order.ID, customerID, "EMAIL", subject, body); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func logHistory(ctx context.Context, tx pgx.Tx, orderID int64, status models.OrderStatus, actorID *int64, actorRole models.UserRole, note string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO order_status_history (order_id, status, actor_id, actor_role, note) VALUES ($1,$2,$3,$4,$5)`,
		orderID, status, actorID, actorRole, note,
	)
	return err
}

func (s *Server) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromContext(r.Context())
	id, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	order, err := s.fetchOrder(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if !canViewOrder(claims, order) {
		writeError(w, http.StatusForbidden, "not authorized to view this order")
		return
	}
	history, err := s.fetchHistory(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"order": order, "history": history})
}

func canViewOrder(claims *auth.Claims, order models.Order) bool {
	switch claims.Role {
	case models.RoleAdmin:
		return true
	case models.RoleCustomer:
		return order.CustomerID == claims.UserID
	case models.RoleAgent:
		return order.AgentID != nil && *order.AgentID == claims.UserID
	}
	return false
}

func (s *Server) fetchOrder(ctx context.Context, id int64) (models.Order, error) {
	var o models.Order
	err := s.db.QueryRow(ctx, `
		SELECT id, customer_id, created_by, pickup_address, pickup_pincode, drop_address, drop_pincode,
			length_cm, breadth_cm, height_cm, actual_weight_kg, volumetric_weight_kg, billable_weight_kg,
			order_type, payment_type, from_zone_id, to_zone_id, rate_card_id, charge, status, agent_id,
			scheduled_date, created_at, updated_at
		FROM orders WHERE id = $1`, id,
	).Scan(&o.ID, &o.CustomerID, &o.CreatedBy, &o.PickupAddress, &o.PickupPincode, &o.DropAddress, &o.DropPincode,
		&o.LengthCM, &o.BreadthCM, &o.HeightCM, &o.ActualWeightKg, &o.VolumetricWeightKg, &o.BillableWeightKg,
		&o.OrderType, &o.PaymentType, &o.FromZoneID, &o.ToZoneID, &o.RateCardID, &o.Charge, &o.Status, &o.AgentID,
		&o.ScheduledDate, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

func (s *Server) fetchHistory(ctx context.Context, orderID int64) ([]models.OrderStatusHistory, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, order_id, status, actor_id, actor_role, COALESCE(note,''), created_at
		 FROM order_status_history WHERE order_id = $1 ORDER BY created_at ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.OrderStatusHistory
	for rows.Next() {
		var h models.OrderStatusHistory
		if err := rows.Scan(&h.ID, &h.OrderID, &h.Status, &h.ActorID, &h.ActorRole, &h.Note, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

func (s *Server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromContext(r.Context())
	q := r.URL.Query()

	query := `SELECT id, customer_id, created_by, pickup_address, pickup_pincode, drop_address, drop_pincode,
		length_cm, breadth_cm, height_cm, actual_weight_kg, volumetric_weight_kg, billable_weight_kg,
		order_type, payment_type, from_zone_id, to_zone_id, rate_card_id, charge, status, agent_id,
		scheduled_date, created_at, updated_at FROM orders WHERE 1=1`
	args := []interface{}{}
	arg := func(v interface{}) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	switch claims.Role {
	case models.RoleCustomer:
		query += " AND customer_id = " + arg(claims.UserID)
	case models.RoleAgent:
		query += " AND agent_id = " + arg(claims.UserID)
	case models.RoleAdmin:
		if status := q.Get("status"); status != "" {
			query += " AND status = " + arg(status)
		}
		if zoneStr := q.Get("zone_id"); zoneStr != "" {
			if zoneID, err := strconv.ParseInt(zoneStr, 10, 64); err == nil {
				query += " AND (from_zone_id = " + arg(zoneID) + " OR to_zone_id = " + arg(zoneID) + ")"
			}
		}
		if agentStr := q.Get("agent_id"); agentStr != "" {
			if agentID, err := strconv.ParseInt(agentStr, 10, 64); err == nil {
				query += " AND agent_id = " + arg(agentID)
			}
		}
	}
	query += " ORDER BY created_at DESC LIMIT 200"

	rows, err := s.db.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	defer rows.Close()
	var out []models.Order
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(&o.ID, &o.CustomerID, &o.CreatedBy, &o.PickupAddress, &o.PickupPincode, &o.DropAddress,
			&o.DropPincode, &o.LengthCM, &o.BreadthCM, &o.HeightCM, &o.ActualWeightKg, &o.VolumetricWeightKg,
			&o.BillableWeightKg, &o.OrderType, &o.PaymentType, &o.FromZoneID, &o.ToZoneID, &o.RateCardID,
			&o.Charge, &o.Status, &o.AgentID, &o.ScheduledDate, &o.CreatedAt, &o.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		out = append(out, o)
	}
	writeJSON(w, http.StatusOK, out)
}

type statusUpdateRequest struct {
	Status models.OrderStatus `json:"status"`
	Note   string             `json:"note,omitempty"`
}

// handleAgentUpdateStatus lets the assigned agent move an order forward
// through the lifecycle. Transitions are validated against ValidTransitions;
// reaching a terminal state (Delivered/Failed) releases the agent back to
// 'available'. Every change is logged to order_status_history and enqueues
// a customer notification.
func (s *Server) handleAgentUpdateStatus(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromContext(r.Context())
	orderID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	var req statusUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	order, err := s.fetchOrder(ctx, orderID)
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if order.AgentID == nil || *order.AgentID != claims.UserID {
		writeError(w, http.StatusForbidden, "order is not assigned to you")
		return
	}
	if !isValidTransition(order.Status, req.Status) {
		writeError(w, http.StatusUnprocessableEntity, "illegal status transition: "+string(order.Status)+" -> "+string(req.Status))
		return
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `UPDATE orders SET status = $1, updated_at = now() WHERE id = $2`, req.Status, orderID); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if err := logHistory(ctx, tx, orderID, req.Status, &claims.UserID, claims.Role, req.Note); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	if req.Status == models.StatusDelivered || req.Status == models.StatusFailed {
		assignSvc := assignment.NewService(s.db)
		if err := assignSvc.ReleaseAgent(ctx, tx, claims.UserID); err != nil {
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
	}

	subject, body := notifications.StatusMessage(orderID, req.Status)
	if err := notifications.Enqueue(ctx, tx, orderID, order.CustomerID, "EMAIL", subject, body); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": string(req.Status)})
}

type rescheduleRequest struct {
	NewDate string `json:"new_date"` // YYYY-MM-DD
	Reason  string `json:"reason,omitempty"`
}

// handleReschedule implements the failed-delivery flow: only valid from a
// FAILED order, owned by the requesting customer (or an admin). Logs the
// reschedule, moves status to RESCHEDULED, then immediately attempts
// auto-assignment for the new attempt (falling back to unassigned/pending
// admin action if no agent is free in the pickup zone).
func (s *Server) handleReschedule(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromContext(r.Context())
	orderID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	var req rescheduleRequest
	if err := decodeJSON(r, &req); err != nil || req.NewDate == "" {
		writeError(w, http.StatusBadRequest, "new_date is required (YYYY-MM-DD)")
		return
	}

	ctx := r.Context()
	order, err := s.fetchOrder(ctx, orderID)
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if claims.Role == models.RoleCustomer && order.CustomerID != claims.UserID {
		writeError(w, http.StatusForbidden, "not your order")
		return
	}
	if order.Status != models.StatusFailed {
		writeError(w, http.StatusUnprocessableEntity, "can only reschedule an order with FAILED status")
		return
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`INSERT INTO reschedules (order_id, old_date, new_date, reason, requested_by) VALUES ($1,$2,$3,$4,$5)`,
		orderID, order.ScheduledDate, req.NewDate, req.Reason, claims.UserID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if _, err := tx.Exec(ctx,
		`UPDATE orders SET status = 'RESCHEDULED', scheduled_date = $1, agent_id = NULL, updated_at = now() WHERE id = $2`,
		req.NewDate, orderID,
	); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if err := logHistory(ctx, tx, orderID, models.StatusRescheduled, &claims.UserID, claims.Role, req.Reason); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	// Re-run auto-assignment for the new attempt.
	assignSvc := assignment.NewService(s.db)
	agentID, assignErr := assignSvc.AutoAssign(ctx, tx, order.FromZoneID)
	finalStatus := models.StatusRescheduled
	if assignErr == nil {
		if _, err := tx.Exec(ctx, `UPDATE orders SET status = 'ASSIGNED', agent_id = $1, updated_at = now() WHERE id = $2`, agentID, orderID); err != nil {
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		if err := logHistory(ctx, tx, orderID, models.StatusAssigned, &claims.UserID, claims.Role, "auto-assigned on reschedule"); err != nil {
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		finalStatus = models.StatusAssigned
	}

	subject, body := notifications.StatusMessage(orderID, models.StatusRescheduled)
	if err := notifications.Enqueue(ctx, tx, orderID, order.CustomerID, "EMAIL", subject, body); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": string(finalStatus)})
}

func parseIDParam(r *http.Request, name string) (int64, error) {
	v := r.PathValue(name)
	return strconv.ParseInt(v, 10, 64)
}
