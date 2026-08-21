package httpapi

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	appauth "github.com/bruhjeshhh/delivery-tracker/internal/auth"
	"github.com/bruhjeshhh/delivery-tracker/internal/config"
	"github.com/bruhjeshhh/delivery-tracker/internal/models"
	"github.com/bruhjeshhh/delivery-tracker/internal/zones"
)

type Server struct {
	db    *pgxpool.Pool
	cfg   config.Config
	zones *zones.Repository
}

func NewServer(db *pgxpool.Pool, cfg config.Config) *Server {
	return &Server{db: db, cfg: cfg, zones: zones.NewRepository(db)}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Public
	mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	authMw := appauth.Middleware(s.cfg.JWTSecret)
	adminOnly := appauth.RequireRole(models.RoleAdmin)
	customerOrAdmin := appauth.RequireRole(models.RoleCustomer, models.RoleAdmin)
	agentOnly := appauth.RequireRole(models.RoleAgent)
	agentOrAdmin := appauth.RequireRole(models.RoleAgent, models.RoleAdmin)
	anyAuthed := appauth.RequireRole(models.RoleCustomer, models.RoleAgent, models.RoleAdmin)

	// Orders
	mux.Handle("POST /api/orders/preview", authMw(customerOrAdmin(http.HandlerFunc(s.handlePreviewCharge))))
	mux.Handle("POST /api/orders", authMw(customerOrAdmin(http.HandlerFunc(s.handleCreateOrder))))
	mux.Handle("GET /api/orders", authMw(anyAuthed(http.HandlerFunc(s.handleListOrders))))
	mux.Handle("GET /api/orders/{id}", authMw(anyAuthed(http.HandlerFunc(s.handleGetOrder))))
	mux.Handle("PATCH /api/orders/{id}/status", authMw(agentOnly(http.HandlerFunc(s.handleAgentUpdateStatus))))
	mux.Handle("POST /api/orders/{id}/reschedule", authMw(anyAuthed(http.HandlerFunc(s.handleReschedule))))

	// Admin: zones, rate cards, agents, assignment, overrides
	mux.Handle("GET /api/zones", authMw(anyAuthed(http.HandlerFunc(s.handleListZones))))
	mux.Handle("POST /api/admin/zones", authMw(adminOnly(http.HandlerFunc(s.handleCreateZone))))
	mux.Handle("POST /api/admin/zones/map-pincode", authMw(adminOnly(http.HandlerFunc(s.handleMapPincode))))
	mux.Handle("GET /api/admin/rate-cards", authMw(adminOnly(http.HandlerFunc(s.handleListRateCards))))
	mux.Handle("POST /api/admin/rate-cards", authMw(adminOnly(http.HandlerFunc(s.handleUpsertRateCard))))
	mux.Handle("GET /api/admin/agents", authMw(adminOnly(http.HandlerFunc(s.handleListAgents))))
	mux.Handle("GET /api/agents/me", authMw(agentOnly(http.HandlerFunc(s.handleGetAgentProfile))))
	mux.Handle("PATCH /api/agents/{id}/availability", authMw(agentOrAdmin(http.HandlerFunc(s.handleSetAgentAvailability))))
	mux.Handle("POST /api/admin/orders/{id}/assign", authMw(adminOnly(http.HandlerFunc(s.handleAssignOrder))))
	mux.Handle("PATCH /api/admin/orders/{id}/override", authMw(adminOnly(http.HandlerFunc(s.handleAdminOverrideStatus))))

	return withCORS(mux)
}
