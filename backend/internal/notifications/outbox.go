package notifications

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/bruhjeshhh/delivery-tracker/internal/models"
)

// Enqueue writes a PENDING notification row inside the caller's transaction.
// This is the outbox pattern: the notification is committed atomically with
// the status change that triggered it, so a crash between "update order" and
// "send email" can never silently drop a notification — the worker will pick
// up any PENDING row on its next poll regardless of when the process died.
func Enqueue(ctx context.Context, tx pgx.Tx, orderID, userID int64, channel string, subject, body string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO notifications (order_id, user_id, channel, subject, body) VALUES ($1,$2,$3,$4,$5)`,
		orderID, userID, channel, subject, body,
	)
	return err
}

// StatusMessage builds the customer-facing copy for a given order status.
func StatusMessage(orderID int64, status models.OrderStatus) (subject, body string) {
	switch status {
	case models.StatusAssigned:
		return fmt.Sprintf("Order #%d: agent assigned", orderID),
			fmt.Sprintf("A delivery agent has been assigned to your order #%d.", orderID)
	case models.StatusPickedUp:
		return fmt.Sprintf("Order #%d: picked up", orderID),
			fmt.Sprintf("Your order #%d has been picked up and is on its way.", orderID)
	case models.StatusInTransit:
		return fmt.Sprintf("Order #%d: in transit", orderID),
			fmt.Sprintf("Your order #%d is in transit.", orderID)
	case models.StatusOutForDelivery:
		return fmt.Sprintf("Order #%d: out for delivery", orderID),
			fmt.Sprintf("Your order #%d is out for delivery today.", orderID)
	case models.StatusDelivered:
		return fmt.Sprintf("Order #%d: delivered", orderID),
			fmt.Sprintf("Your order #%d has been delivered. Thank you!", orderID)
	case models.StatusFailed:
		return fmt.Sprintf("Order #%d: delivery failed", orderID),
			fmt.Sprintf("We were unable to deliver your order #%d. Please reschedule a new delivery date.", orderID)
	case models.StatusRescheduled:
		return fmt.Sprintf("Order #%d: rescheduled", orderID),
			fmt.Sprintf("Your order #%d has been rescheduled and a new agent will be assigned.", orderID)
	case models.StatusCancelled:
		return fmt.Sprintf("Order #%d: cancelled", orderID),
			fmt.Sprintf("Your order #%d has been cancelled.", orderID)
	default:
		return fmt.Sprintf("Order #%d: status update", orderID),
			fmt.Sprintf("Your order #%d status is now %s.", orderID, status)
	}
}
