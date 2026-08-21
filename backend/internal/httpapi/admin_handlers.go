package httpapi

import (
	"net/http"

	"github.com/bruhjeshhh/delivery-tracker/internal/assignment"
	"github.com/bruhjeshhh/delivery-tracker/internal/auth"
	"github.com/bruhjeshhh/delivery-tracker/internal/models"
	"github.com/bruhjeshhh/delivery-tracker/internal/notifications"
)

// --- Zones & rate cards -----------------------------------------------

func (s *Server) handleListZones(w http.ResponseWriter, r *http.Request) {
	zonesList, err := s.zones.ListZones(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, zonesList)
}

type createZoneRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateZone(w http.ResponseWriter, r *http.Request) {
	var req createZoneRequest
	if err := decodeJSON(r, &req); err != nil || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	z, err := s.zones.CreateZone(r.Context(), req.Name)
	if err != nil {
		writeError(w, http.StatusConflict, "zone already exists or invalid")
		return
	}
	writeJSON(w, http.StatusCreated, z)
}

type mapPincodeRequest struct {
	ZoneID  int64  `json:"zone_id"`
	Pincode string `json:"pincode"`
	Label   string `json:"label,omitempty"`
}

func (s *Server) handleMapPincode(w http.ResponseWriter, r *http.Request) {
	var req mapPincodeRequest
	if err := decodeJSON(r, &req); err != nil || req.ZoneID == 0 || req.Pincode == "" {
		writeError(w, http.StatusBadRequest, "zone_id and pincode are required")
		return
	}
	za, err := s.zones.MapPincode(r.Context(), req.ZoneID, req.Pincode, req.Label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not map pincode")
		return
	}
	writeJSON(w, http.StatusOK, za)
}

func (s *Server) handleUpsertRateCard(w http.ResponseWriter, r *http.Request) {
	var card models.RateCard
	if err := decodeJSON(r, &card); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if card.OrderType == "" || card.FromZoneID == 0 || card.ToZoneID == 0 {
		writeError(w, http.StatusBadRequest, "order_type, from_zone_id, to_zone_id are required")
		return
	}
	result, err := s.zones.UpsertRateCard(r.Context(), card)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save rate card")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleListRateCards(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(),
		`SELECT id, order_type, from_zone_id, to_zone_id, base_fee, rate_per_kg, cod_surcharge FROM rate_cards ORDER BY order_type, from_zone_id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	defer rows.Close()
	var out []models.RateCard
	for rows.Next() {
		var c models.RateCard
		if err := rows.Scan(&c.ID, &c.OrderType, &c.FromZoneID, &c.ToZoneID, &c.BaseFee, &c.RatePerKg, &c.CODSurcharge); err != nil {
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, out)
}

// --- Agents --------------------------------------------------------------

func (s *Server) handleGetAgentProfile(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromContext(r.Context())
	var agent models.Agent
	var zoneName *string
	err := s.db.QueryRow(r.Context(),
		`SELECT a.user_id, u.name, a.zone_id, a.status, a.last_assigned_at,
		        z.name
		 FROM agents a
		 JOIN users u ON u.id = a.user_id
		 LEFT JOIN zones z ON z.id = a.zone_id
		 WHERE a.user_id = $1`, claims.UserID,
	).Scan(&agent.UserID, &agent.Name, &agent.ZoneID, &agent.Status, &agent.LastAssignedAt, &zoneName)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent profile not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user_id":   agent.UserID,
		"name":      agent.Name,
		"zone_id":   agent.ZoneID,
		"zone_name": zoneName,
		"status":    agent.Status,
	})
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(r.Context(), `
		SELECT a.user_id, u.name, a.zone_id, a.status, a.last_assigned_at
		FROM agents a JOIN users u ON u.id = a.user_id
		ORDER BY u.name`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	defer rows.Close()
	var out []models.Agent
	for rows.Next() {
		var a models.Agent
		if err := rows.Scan(&a.UserID, &a.Name, &a.ZoneID, &a.Status, &a.LastAssignedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
		out = append(out, a)
	}
	writeJSON(w, http.StatusOK, out)
}

type setAgentStatusRequest struct {
	Status models.AgentStatus `json:"status"`
	ZoneID *int64             `json:"zone_id,omitempty"`
}

// handleSetAgentAvailability lets an agent toggle their own availability
// (e.g. going online/offline for the day), or an admin reassign an agent's
// home zone.
func (s *Server) handleSetAgentAvailability(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromContext(r.Context())
	agentID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid agent id")
		return
	}
	if claims.Role == models.RoleAgent && claims.UserID != agentID {
		writeError(w, http.StatusForbidden, "cannot modify another agent")
		return
	}
	var req setAgentStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	_, err = s.db.Exec(r.Context(),
		`UPDATE agents SET status = COALESCE(NULLIF($1,''), status), zone_id = COALESCE($2, zone_id), updated_at = now() WHERE user_id = $3`,
		string(req.Status), req.ZoneID, agentID,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// --- Order assignment & override ------------------------------------------

type assignRequest struct {
	AgentID *int64 `json:"agent_id,omitempty"` // nil => trigger auto-assignment
}

func (s *Server) handleAssignOrder(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromContext(r.Context())
	orderID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	var req assignRequest
	decodeJSON(r, &req) // body optional for pure auto-assign

	ctx := r.Context()
	order, err := s.fetchOrder(ctx, orderID)
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	defer tx.Rollback(ctx)

	var agentID int64
	if req.AgentID != nil {
		agentID = *req.AgentID
		if _, err := tx.Exec(ctx, `UPDATE agents SET status = 'busy', last_assigned_at = now(), updated_at = now() WHERE user_id = $1`, agentID); err != nil {
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
	} else {
		assignSvc := assignment.NewService(s.db)
		agentID, err = assignSvc.AutoAssign(ctx, tx, order.FromZoneID)
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE orders SET agent_id = $1, status = 'ASSIGNED', updated_at = now() WHERE id = $2`, agentID, orderID); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if err := logHistory(ctx, tx, orderID, models.StatusAssigned, &claims.UserID, claims.Role, "assigned by admin"); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	subject, body := notifications.StatusMessage(orderID, models.StatusAssigned)
	if err := notifications.Enqueue(ctx, tx, orderID, order.CustomerID, "EMAIL", subject, body); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"agent_id": agentID})
}

// handleAdminOverrideStatus lets an admin force any status change,
// bypassing ValidTransitions — used to correct mistakes or handle edge
// cases the normal agent-driven flow doesn't cover.
func (s *Server) handleAdminOverrideStatus(w http.ResponseWriter, r *http.Request) {
	claims := auth.FromContext(r.Context())
	orderID, err := parseIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid order id")
		return
	}
	var req statusUpdateRequest
	if err := decodeJSON(r, &req); err != nil || req.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}

	ctx := r.Context()
	order, err := s.fetchOrder(ctx, orderID)
	if err != nil {
		writeError(w, http.StatusNotFound, "order not found")
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
	note := "admin override"
	if req.Note != "" {
		note = "admin override: " + req.Note
	}
	if err := logHistory(ctx, tx, orderID, req.Status, &claims.UserID, claims.Role, note); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if (req.Status == models.StatusDelivered || req.Status == models.StatusFailed) && order.AgentID != nil {
		assignSvc := assignment.NewService(s.db)
		if err := assignSvc.ReleaseAgent(ctx, tx, *order.AgentID); err != nil {
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
