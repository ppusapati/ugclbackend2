package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"p9e.in/ugcl/middleware"
)

func TestGetCurrentUser_DirectWithoutClaims_Unauthorized(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/token", nil)
	rr := httptest.NewRecorder()

	GetCurrentUser(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestGetCurrentUser_WithJWTMiddleware_MissingHeader_Unauthorized(t *testing.T) {
	h := middleware.JWTMiddleware(http.HandlerFunc(GetCurrentUser))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/token", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestGetCurrentUser_WithJWTMiddleware_InvalidToken_Unauthorized(t *testing.T) {
	h := middleware.JWTMiddleware(http.HandlerFunc(GetCurrentUser))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/token", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestGetCurrentUser_WithJWTMiddleware_NonUUIDClaim_UserNotFound(t *testing.T) {
	token, err := middleware.GenerateToken("not-a-uuid", "user", "Test User", "9999999999")
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	h := middleware.JWTMiddleware(http.HandlerFunc(GetCurrentUser))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/token", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
}
