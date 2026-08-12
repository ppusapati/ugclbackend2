package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// PlatformClaims are the JWT payload for a platform admin — a cross-tenant
// operator identity (models.PlatformAdmin), distinct from a tenant user's
// Claims. Kept as a separate type rather than adding fields to Claims so a
// platform-admin token can never be mistaken for (or accidentally treated
// as) a tenant-user token by code that only checks for a non-nil Claims —
// the types don't overlap, so the compiler enforces which one a handler
// actually asked for.
type PlatformClaims struct {
	AdminID string `json:"adminId"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	jwt.RegisteredClaims
}

type platformCtxKey int

const platformClaimsKey platformCtxKey = iota

// GeneratePlatformAdminToken creates a signed JWT valid for 8 h — shorter
// than a tenant user's 24 h, since this token can create and provision
// tenants and is expected to be used interactively by an operator, not held
// by a long-running client.
func GeneratePlatformAdminToken(adminID, name, email string) (string, error) {
	claims := PlatformClaims{
		AdminID: adminID,
		Name:    name,
		Email:   email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(8 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// PlatformAdminMiddleware validates a platform-admin JWT and stashes the
// PlatformClaims in context. Reuses the same signing key as tenant-user
// tokens (jwtKey, from JWT_SECRET) — the two token types are distinguished
// by which claims struct successfully parses them and by which endpoints
// accept which context key, not by a different secret.
func PlatformAdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			http.Error(w, "missing Authorization header", http.StatusUnauthorized)
			return
		}
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			http.Error(w, "invalid auth header", http.StatusUnauthorized)
			return
		}

		token, err := jwt.ParseWithClaims(parts[1], &PlatformClaims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return jwtKey, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(*PlatformClaims)
		if !ok {
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), platformClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetPlatformClaims pulls the *PlatformClaims out of the request context
// (set by PlatformAdminMiddleware), or nil if absent.
func GetPlatformClaims(r *http.Request) *PlatformClaims {
	if c, ok := r.Context().Value(platformClaimsKey).(*PlatformClaims); ok {
		return c
	}
	return nil
}
