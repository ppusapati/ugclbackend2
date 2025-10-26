// auth/jwt.go
package middleware

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

// Grab your secret from env (or config)
var jwtKey = []byte(os.Getenv("JWT_SECRET"))

// Claims are the custom payload in your JWT
type Claims struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
	Phone  string `json:"phone"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// unexported type prevents collisions in context
type ctxKey int

const (
	userClaimsKey ctxKey = iota
)

// GenerateToken creates a signed JWT valid for 24 h
func GenerateToken(userID, role, name, phone string) (string, error) {
	claims := Claims{
		UserID: userID,
		Name:   name,
		Phone:  phone,
		Role:   role,

		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// JWTMiddleware validates the token and stashes the Claims in ctx
func JWTMiddleware(next http.Handler) http.Handler {
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

		tokenStr := parts[1]
		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(*Claims)
		if !ok {
			http.Error(w, "invalid token claims", http.StatusUnauthorized)
			return
		}

		// attach the full Claims object to context
		ctx := context.WithValue(r.Context(), userClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole wraps a handler and ensures the JWT’s role matches
//
//	func RequireRole(role string, next http.Handler) http.Handler {
//		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			if GetRole(r) != role {
//				http.Error(w, "forbidden", http.StatusForbidden)
//				return
//			}
//			next.ServeHTTP(w, r)
//		})
//	}
//
//	func RequireRole(role string, next http.Handler) http.Handler {
//		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
//			userRole := GetRole(r)
//			// Super Admin can do anything!
//			if userRole == "Super Admin" || userRole == role {
//				next.ServeHTTP(w, r)
//				return
//			}
//			http.Error(w, "forbidden", http.StatusForbidden)
//		})
//	}
//
// In your middleware package:
func RequireRole(roles []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := GetRole(r)
		if slices.Contains(roles, role) {
			next.ServeHTTP(w, r)
			return
		}
		http.Error(w, "forbidden", http.StatusForbidden)
	})
}

// GetClaims pulls the *Claims out of the request context (or nil)
func GetClaims(r *http.Request) *Claims {
	if c, ok := r.Context().Value(userClaimsKey).(*Claims); ok {
		return c
	}
	return nil
}

// Convenience methods:
func GetUserID(r *http.Request) string {
	if c := GetClaims(r); c != nil {
		return c.UserID
	}
	return ""
}

func GetUser(r *http.Request) models.User {
	if c := GetClaims(r); c != nil {
		// Load full user from database to get role relationships
		var user models.User
		if err := config.DB.
			Preload("RoleModel").
			Preload("UserBusinessRoles.BusinessRole").
			First(&user, "id = ?", c.UserID).Error; err == nil {
			return user
		}
		// Fallback: return minimal user from claims
		User := models.User{
			Name:  c.Name,
			Phone: c.Phone,
		}
		return User
	}
	return models.User{} // return zero value if no claims found
}
func GetRole(r *http.Request) string {
	if c := GetClaims(r); c != nil {
		return c.Role
	}
	return ""
}

type APIClientConfig struct {
	AppName        string
	AllowedPaths   []string        // Exact or prefix match (supports "*")
	AllowedMethods map[string]bool // e.g., "GET": true, "POST": true
	SkipIPCheck    bool
}

var apiKeyConfigs = map[string]APIClientConfig{
	os.Getenv("MOBILE_APP_KEY"): {
		AppName:      "MobileApp",
		AllowedPaths: []string{"/api/v1"},
		AllowedMethods: map[string]bool{
			http.MethodGet:  true, // Allow GET for fetching business verticals, sites, etc.
			http.MethodPost: true, // Allow POST for form submissions
		},
		SkipIPCheck: true,
	},
	os.Getenv("PARTNER_PORTAL_KEY"): {
		AppName:      "PartnerPortal",
		AllowedPaths: []string{"/api/v1"},
		AllowedMethods: map[string]bool{
			http.MethodGet: true,
		},
		SkipIPCheck: false,
	},
	os.Getenv("INTERNAL_OPS_KEY"): {
		AppName:      "InternalOps",
		AllowedPaths: []string{"/api/v1/*"},
		AllowedMethods: map[string]bool{
			http.MethodGet:    true,
			http.MethodPost:   true,
			http.MethodPut:    true,
			http.MethodDelete: true,
		},
		SkipIPCheck: true,
	},
}

// Define fixed IP whitelist for server-to-server apps (skip for mobile)
var whitelistedIPs = map[string]bool{
	"20.204.19.129": true,
	"127.0.0.1":     true,
	"::1":           true,
}

// SecurityMiddleware enforces API key, IP filtering, and logging
func SecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("x-api-key")
		clientConfig, ok := apiKeyConfigs[apiKey]
		if !ok {
			http.Error(w, "Invalid or missing API key", http.StatusUnauthorized)
			log.Printf("[SECURITY] 🔒 Blocked - Invalid API key. IP=%s Path=%s", getClientIP(r), r.URL.Path)
			return
		}

		clientIP := getClientIP(r)
		if !clientConfig.SkipIPCheck && !whitelistedIPs[clientIP] {
			http.Error(w, "Access from this IP is not allowed", http.StatusForbidden)
			log.Printf("[SECURITY] 🚫 Blocked - IP not whitelisted. App=%s IP=%s Path=%s", clientConfig.AppName, clientIP, r.URL.Path)
			return
		}

		// ✅ Path-based access check
		pathAllowed := false
		for _, path := range clientConfig.AllowedPaths {
			if strings.HasSuffix(path, "*") {
				prefix := strings.TrimSuffix(path, "*")
				if strings.HasPrefix(r.URL.Path, prefix) {
					pathAllowed = true
					break
				}
			} else if r.URL.Path == path || strings.HasPrefix(r.URL.Path, path+"/") {
				pathAllowed = true
				break
			}
		}
		if !pathAllowed {
			http.Error(w, "Access to this endpoint is not allowed for this app", http.StatusForbidden)
			log.Printf("[SECURITY] ⛔️ Denied - Path not allowed. App=%s IP=%s Path=%s", clientConfig.AppName, clientIP, r.URL.Path)
			return
		}

		// ✅ Method-based access check
		if !clientConfig.AllowedMethods[r.Method] {
			http.Error(w, "This HTTP method is not allowed for this app", http.StatusMethodNotAllowed)
			log.Printf("[SECURITY] ⛔️ Denied - Method not allowed. App=%s Method=%s Path=%s", clientConfig.AppName, r.Method, r.URL.Path)
			return
		}

		// User info from JWT if available
		claims := GetClaims(r)
		userID, userRole, userName := "-", "-", "-"
		if claims != nil {
			userID = claims.UserID
			userRole = claims.Role
			userName = claims.Name
		}

		log.Printf("[SECURITY] ✅ Allowed - App=%s UserID=%s Name=%s Role=%s IP=%s Path=%s Method=%s Time=%s",
			clientConfig.AppName, userID, userName, userRole,
			clientIP, r.URL.Path, r.Method, time.Now().Format(time.RFC3339))

		next.ServeHTTP(w, r)
	})
}

// Extracts client IP from headers or remote addr
func getClientIP(r *http.Request) string {
	// Priority: X-Forwarded-For → X-Real-IP → RemoteAddr
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return strings.Split(ip, ",")[0]
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
