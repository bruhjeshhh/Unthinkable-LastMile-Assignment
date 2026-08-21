package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/bruhjeshhh/delivery-tracker/internal/config"
	"github.com/bruhjeshhh/delivery-tracker/internal/db"
	"github.com/bruhjeshhh/delivery-tracker/internal/httpapi"
	"github.com/bruhjeshhh/delivery-tracker/internal/notifications"
)

func main() {
	cfg := config.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Background notification worker (outbox pattern: polls PENDING rows).
	emailSender := notifications.NewSMTPEmailSender(cfg)
	smsSender := notifications.NewMockSMSSender()
	worker := notifications.NewWorker(pool, emailSender, smsSender)
	go worker.Run(ctx)

	server := httpapi.NewServer(pool, cfg)

	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: server.Routes(),
	}

	go func() {
		log.Printf("delivery-tracker API listening on :%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10_000_000_000)
	defer cancel()
	httpServer.Shutdown(shutdownCtx)
}
