package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeClientStore struct{}

func (f *fakeClientStore) GetClient(ctx context.Context, clientID string) (string, int64, int64, bool) {
	if clientID == "known" {
		return "pro", 340, 1000, true
	}
	return "", 0, 0, false
}
func (f *fakeClientStore) SetClientLimit(ctx context.Context, clientID string, limit int64) error {
	return nil
}

func TestLogin_ValidCredentials_ReturnsToken(t *testing.T) {
	h := NewHandlers(&fakeClientStore{}, []byte("test-secret"), "admin", "s3cret")
	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "s3cret"})
	req := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp loginResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Token == "" {
		t.Fatal("expected a non-empty token")
	}
}

func TestLogin_InvalidCredentials_Returns401(t *testing.T) {
	h := NewHandlers(&fakeClientStore{}, []byte("test-secret"), "admin", "s3cret")
	body, _ := json.Marshal(loginRequest{Username: "admin", Password: "wrong"})
	req := httptest.NewRequest(http.MethodPost, "/admin/login", bytes.NewReader(body))
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestGetLimits_KnownClient(t *testing.T) {
	h := NewHandlers(&fakeClientStore{}, []byte("test-secret"), "admin", "s3cret")
	req := httptest.NewRequest(http.MethodGet, "/admin/limits/known", nil)
	rec := httptest.NewRecorder()

	h.GetLimits(rec, req, "known")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp limitsResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if resp.Tier != "pro" || resp.Limit != 1000 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestRequireJWT_RejectsMissingToken(t *testing.T) {
	secret := []byte("test-secret")
	handler := RequireJWT(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/admin/limits/known", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d", rec.Code)
	}
}

func TestRequireJWT_AllowsLoginWithoutToken(t *testing.T) {
	secret := []byte("test-secret")
	called := false
	handler := RequireJWT(secret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/admin/login", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("login endpoint should be reachable without a token")
	}
}
