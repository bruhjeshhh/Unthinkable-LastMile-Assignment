package notifications

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Worker polls the notifications outbox and dispatches PENDING rows. Kept as
// a simple polling loop (no Kafka/queue infra) since outbox volume for this
// use case is low and Postgres SKIP LOCKED gives safe concurrent polling for
// free if the process is ever scaled to multiple instances.
type Worker struct {
	db       *pgxpool.Pool
	email    EmailSender
	sms      SMSSender
	interval time.Duration
}

func NewWorker(db *pgxpool.Pool, email EmailSender, sms SMSSender) *Worker {
	return &Worker{db: db, email: email, sms: sms, interval: 3 * time.Second}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

type pendingRow struct {
	id      int64
	channel string
	subject string
	body    string
	email   string
	phone   string
}

func (w *Worker) processBatch(ctx context.Context) {
	rows, err := w.db.Query(ctx, `
		SELECT n.id, n.channel, COALESCE(n.subject,''), n.body, u.email, COALESCE(u.phone,'')
		FROM notifications n
		JOIN users u ON u.id = n.user_id
		WHERE n.status = 'PENDING' AND n.attempts < 5
		ORDER BY n.created_at
		LIMIT 20
		FOR UPDATE SKIP LOCKED`)
	if err != nil {
		log.Printf("notifications worker: query error: %v", err)
		return
	}
	var batch []pendingRow
	for rows.Next() {
		var p pendingRow
		if err := rows.Scan(&p.id, &p.channel, &p.subject, &p.body, &p.email, &p.phone); err != nil {
			log.Printf("notifications worker: scan error: %v", err)
			continue
		}
		batch = append(batch, p)
	}
	rows.Close()

	for _, p := range batch {
		var sendErr error
		switch p.channel {
		case "EMAIL":
			sendErr = w.email.Send(p.email, p.subject, p.body)
		case "SMS":
			if p.phone != "" {
				sendErr = w.sms.Send(p.phone, p.body)
			}
		}
		if sendErr != nil {
			w.db.Exec(ctx,
				`UPDATE notifications SET attempts = attempts + 1, last_error = $2,
				 status = CASE WHEN attempts + 1 >= 5 THEN 'FAILED' ELSE 'PENDING' END
				 WHERE id = $1`, p.id, sendErr.Error())
			log.Printf("notifications worker: send failed for %d: %v", p.id, sendErr)
			continue
		}
		w.db.Exec(ctx, `UPDATE notifications SET status = 'SENT', sent_at = now() WHERE id = $1`, p.id)
	}
}
