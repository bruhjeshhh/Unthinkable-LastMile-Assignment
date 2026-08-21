package assignment

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNoAgentAvailable = errors.New("no available agent in this zone")

type Service struct {
	db *pgxpool.Pool
}

func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// AutoAssign picks the "nearest available agent" for a pickup zone. Nearest
// is modelled as zone-membership (agents are assigned a home zone) rather
// than live GPS distance — this reuses the same zone graph the rate engine
// already relies on, and needs no location-tracking infra. Among agents in
// the zone who are currently 'available', it picks the one who was assigned
// least recently (round robin), so load spreads evenly rather than always
// hitting the same agent.
//
// Runs inside the same transaction as the order update so the agent's status
// flip to 'busy' and the order's agent_id assignment are atomic.
func (s *Service) AutoAssign(ctx context.Context, tx pgx.Tx, zoneID int64) (int64, error) {
	var agentID int64
	err := tx.QueryRow(ctx,
		`SELECT user_id FROM agents
		 WHERE zone_id = $1 AND status = 'available'
		 ORDER BY last_assigned_at ASC NULLS FIRST
		 LIMIT 1
		 FOR UPDATE SKIP LOCKED`,
		zoneID,
	).Scan(&agentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoAgentAvailable
	}
	if err != nil {
		return 0, err
	}

	_, err = tx.Exec(ctx,
		`UPDATE agents SET status = 'busy', last_assigned_at = now(), updated_at = now() WHERE user_id = $1`,
		agentID,
	)
	if err != nil {
		return 0, err
	}
	return agentID, nil
}

// ReleaseAgent frees an agent back to 'available' — called when an order
// reaches a terminal state for that delivery attempt (Delivered or Failed).
func (s *Service) ReleaseAgent(ctx context.Context, tx pgx.Tx, agentID int64) error {
	_, err := tx.Exec(ctx,
		`UPDATE agents SET status = 'available', updated_at = now() WHERE user_id = $1`,
		agentID,
	)
	return err
}
