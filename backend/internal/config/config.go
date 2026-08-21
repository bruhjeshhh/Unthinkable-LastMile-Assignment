package config

import (
	"os"
	"time"
)

// Config holds all environment-driven settings. No hardcoded values live
// outside this file: everything the rate engine, auth, and notification
// workers need is loaded here once at startup.
type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	TokenTTL    time.Duration

	SMTPHost  string
	SMTPPort  string
	SMTPUser  string
	SMTPPass  string
	FromEmail string

	// If true, SMS is only logged, never actually sent (default: true, since
	// SMS providers need paid/verified numbers on free tiers).
	SMSMock bool
}

func Load() Config {
	return Config{
		Port:        getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/delivery?sslmode=disable"),
		JWTSecret:   getEnv("JWT_SECRET", "dev-secret-change-me"),
		TokenTTL:    24 * time.Hour,

		SMTPHost:  getEnv("SMTP_HOST", ""),
		SMTPPort:  getEnv("SMTP_PORT", "587"),
		SMTPUser:  getEnv("SMTP_USER", ""),
		SMTPPass:  getEnv("SMTP_PASS", ""),
		FromEmail: getEnv("FROM_EMAIL", "no-reply@delivery-tracker.local"),

		SMSMock: getEnv("SMS_MOCK", "true") == "true",
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
