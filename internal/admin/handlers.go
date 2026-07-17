// Package admin implements the JWT-protected admin API: login, and
// viewing/overriding per-client rate limit configuration.
package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ClientStore is the minimal persistence interface the admin API needs
// against the Redis-backed client config hash (config:client:{id}).
type ClientStore interface {
	GetClient(ctx context.Context, clientID string) (tier string, usage, limit int64, found bool)
	SetClientLimit(ctx context.Context, clientID string, limit int64) error
}

type Handlers struct {
	store     ClientStore
	jwtSecret []byte
	// adminUser/adminPassHash would normally be looked up from a real user
	// store; kept simple and injectable here for a portfolio-scale project.
	adminUser     string
	adminPassword string
}

func NewHandlers(store ClientStore, jwtSecret []byte, adminUser, adminPassword string) *Handlers {
	return &Handlers{store: store, jwtSecret: jwtSecret, adminUser: adminUser, adminPassword: adminPassword}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

// Login issues a short-lived JWT on valid credentials.
// POST /admin/login
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// NOTE: plaintext comparison here for simplicity; production code should
	// store bcrypt hashes and use bcrypt.CompareHashAndPassword.
	if req.Username != h.adminUser || req.Password != h.adminPassword {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	expiresIn := 3600
	claims := jwt.RegisteredClaims{
		Subject:   req.Username,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expiresIn) * time.Second)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(h.jwtSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to sign token")
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{Token: signed, ExpiresIn: expiresIn})
}

type limitsResponse struct {
	ClientID     string `json:"client_id"`
	Tier         string `json:"tier"`
	CurrentUsage int64  `json:"current_usage"`
	Limit        int64  `json:"limit"`
}

// GetLimits returns a client's current tier, usage, and limit.
// GET /admin/limits/:client_id
func (h *Handlers) GetLimits(w http.ResponseWriter, r *http.Request, clientID string) {
	tier, usage, limit, found := h.store.GetClient(r.Context(), clientID)
	if !found {
		writeError(w, http.StatusNotFound, "client not found")
		return
	}
	writeJSON(w, http.StatusOK, limitsResponse{ClientID: clientID, Tier: tier, CurrentUsage: usage, Limit: limit})
}

type overrideRequest struct {
	Limit int64 `json:"limit"`
}

// PutLimits overrides a client's limit.
// PUT /admin/limits/:client_id
func (h *Handlers) PutLimits(w http.ResponseWriter, r *http.Request, clientID string) {
	var req overrideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Limit <= 0 {
		writeError(w, http.StatusBadRequest, "invalid limit value")
		return
	}
	if err := h.store.SetClientLimit(r.Context(), clientID, req.Limit); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update limit")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"client_id": clientID, "limit": req.Limit})
}

// RequireJWT protects /admin/* routes by validating the bearer token.
func RequireJWT(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The login endpoint itself must not require a token.
			if r.URL.Path == "/admin/login" {
				next.ServeHTTP(w, r)
				return
			}
			authHeader := r.Header.Get("Authorization")
			const prefix = "Bearer "
			if len(authHeader) <= len(prefix) || authHeader[:len(prefix)] != prefix {
				writeError(w, http.StatusUnauthorized, "missing bearer token")
				return
			}
			tokenStr := authHeader[len(prefix):]

			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				return secret, nil
			})
			if err != nil || !token.Valid {
				writeError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
