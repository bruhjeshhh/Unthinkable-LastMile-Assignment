package httpapi

import (
	"net/http"

	"github.com/bruhjeshhh/delivery-tracker/internal/auth"
	"github.com/bruhjeshhh/delivery-tracker/internal/models"
)

type registerRequest struct {
	Name     string          `json:"name"`
	Email    string          `json:"email"`
	Phone    string          `json:"phone"`
	Password string          `json:"password"`
	Role     models.UserRole `json:"role"`              // customer or agent (admin created manually / seeded)
	ZoneID   *int64          `json:"zone_id,omitempty"` // required if role = agent
}

type authResponse struct {
	Token string      `json:"token"`
	User  models.User `json:"user"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "name, email, and password are required")
		return
	}
	if req.Role != models.RoleCustomer && req.Role != models.RoleAgent {
		req.Role = models.RoleCustomer
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not process password")
		return
	}

	ctx := r.Context()
	tx, err := s.db.Begin(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	defer tx.Rollback(ctx)

	var user models.User
	err = tx.QueryRow(ctx,
		`INSERT INTO users (role, name, email, phone, password_hash) VALUES ($1,$2,$3,$4,$5)
		 RETURNING id, role, name, email, COALESCE(phone,''), created_at`,
		req.Role, req.Name, req.Email, req.Phone, hash,
	).Scan(&user.ID, &user.Role, &user.Name, &user.Email, &user.Phone, &user.CreatedAt)
	if err != nil {
		writeError(w, http.StatusConflict, "email already registered or invalid data")
		return
	}

	if req.Role == models.RoleAgent {
		_, err = tx.Exec(ctx,
			`INSERT INTO agents (user_id, zone_id, status) VALUES ($1, $2, 'offline')`,
			user.ID, req.ZoneID,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not create agent profile")
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}

	token, err := auth.GenerateToken(s.cfg.JWTSecret, user.ID, user.Role, s.cfg.TokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate token")
		return
	}
	writeJSON(w, http.StatusCreated, authResponse{Token: token, User: user})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var user models.User
	var hash string
	err := s.db.QueryRow(r.Context(),
		`SELECT id, role, name, email, COALESCE(phone,''), password_hash, created_at FROM users WHERE email = $1`,
		req.Email,
	).Scan(&user.ID, &user.Role, &user.Name, &user.Email, &user.Phone, &hash, &user.CreatedAt)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if !auth.CheckPassword(hash, req.Password) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}

	token, err := auth.GenerateToken(s.cfg.JWTSecret, user.ID, user.Role, s.cfg.TokenTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not generate token")
		return
	}
	writeJSON(w, http.StatusOK, authResponse{Token: token, User: user})
}
