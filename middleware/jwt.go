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
		tokenStr := ""
		if auth != "" {
			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, "invalid auth header", http.StatusUnauthorized)
				return
			}
			tokenStr = parts[1]
		} else {
			// EventSource cannot set custom Authorization headers.
			// Allow token query fallback for SSE/auth-constrained clients.
			tokenStr = strings.TrimSpace(r.URL.Query().Get("token"))
			if tokenStr == "" {
				tokenStr = strings.TrimSpace(r.URL.Query().Get("access_token"))
			}
			if tokenStr == "" {
				http.Error(w, "missing Authorization header", http.StatusUnauthorized)
				return
			}
		}

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

// GetUser returns the authenticated user, reading from the shared in-process cache when
// available.  It never writes to the cache: only LoadUserContext (which uses the full
// Preload chain required for permission checks) is authoritative for cache writes.  This
// prevents a partial-preload entry from poisoning the cache and causing silent auth
// failures for subsequent permission middleware calls.
func GetUser(r *http.Request) models.User {
	if c := GetClaims(r); c != nil {
		// Fast path: auth middleware has already loaded and cached the full user.
		if cached, ok := userCache.get(c.UserID); ok {
			return cached
		}
		// Slow path: cache miss (e.g. test/debug endpoints bypassing auth middleware).
		// Load from DB but do NOT write to cache — the partial preload here lacks
		// RoleModel.Permissions which permission checks depend on.
		var user models.User
		if err := config.DB.
			Preload("RoleModel").
			Preload("UserBusinessRoles.BusinessRole").
			First(&user, "id = ?", c.UserID).Error; err == nil {
			return user
		}
		// Fallback: return minimal user from claims so callers still get a non-nil struct.
		return models.User{Name: c.Name, Phone: c.Phone}
	}
	return models.User{}
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
	AllowedIPs     map[string]bool
}

var apiKeyConfigs = loadAPIKeyConfigs()

// Define fixed IP whitelist for server-to-server apps (skip for mobile)
var defaultWhitelistedIPs = map[string]bool{
	"20.204.19.129": true,
	"127.0.0.1":     true,
	"::1":           true,
}

func loadAPIKeyConfigs() map[string]APIClientConfig {
	configs := make(map[string]APIClientConfig)

	addAPIKeyConfig(configs, os.Getenv("MOBILE_APP_KEY"), APIClientConfig{
		AppName:      "MobileApp",
		AllowedPaths: []string{"/api/v1"},
		AllowedMethods: map[string]bool{
			http.MethodGet:  true,
			http.MethodPost: true,
		},
		SkipIPCheck: true,
	})

	addAPIKeyConfig(configs, os.Getenv("PARTNER_PORTAL_KEY"), APIClientConfig{
		AppName:      "PartnerPortal",
		AllowedPaths: []string{"/api/v1"},
		AllowedMethods: map[string]bool{
			http.MethodGet: true,
		},
		SkipIPCheck: false,
		AllowedIPs:  buildAllowedIPsFromEnv("PARTNER_PORTAL_ALLOWED_IPS"),
	})

	addAPIKeyConfig(configs, os.Getenv("INTERNAL_OPS_KEY"), APIClientConfig{
		AppName:      "InternalOps",
		AllowedPaths: []string{"/api/v1/*"},
		AllowedMethods: map[string]bool{
			http.MethodGet:    true,
			http.MethodPost:   true,
			http.MethodPut:    true,
			http.MethodPatch:  true,
			http.MethodDelete: true,
		},
		SkipIPCheck: true,
	})

	addAPIKeyConfig(configs, os.Getenv("THIRD_PARTY_SYNC_KEY"), APIClientConfig{
		AppName: "ThirdPartySync",
		AllowedPaths: []string{
			"/api/v1/integrations/*",
			"/api/v1/webhooks/incoming",
			"/api/v1/webhooks/incoming/",
		},
		AllowedMethods: map[string]bool{
			http.MethodGet:  true,
			http.MethodPost: true,
		},
		SkipIPCheck: false,
		AllowedIPs:  buildAllowedIPsFromEnv("THIRD_PARTY_SYNC_ALLOWED_IPS"),
	})

	addAPIKeyConfig(configs, os.Getenv("THIRD_PARTY_PROVIDER_A_KEY"), APIClientConfig{
		AppName: "ThirdPartyProviderA",
		AllowedPaths: []string{
			"/api/v1/integrations/provider-a/*",
			"/api/v1/webhooks/incoming/provider-a",
		},
		AllowedMethods: map[string]bool{
			http.MethodGet:  true,
			http.MethodPost: true,
		},
		SkipIPCheck: false,
		AllowedIPs:  buildAllowedIPsFromEnv("THIRD_PARTY_PROVIDER_A_ALLOWED_IPS"),
	})

	addAPIKeyConfig(configs, os.Getenv("THIRD_PARTY_PROVIDER_B_KEY"), APIClientConfig{
		AppName: "ThirdPartyProviderB",
		AllowedPaths: []string{
			"/api/v1/integrations/provider-b/*",
			"/api/v1/webhooks/incoming/provider-b",
		},
		AllowedMethods: map[string]bool{
			http.MethodGet:  true,
			http.MethodPost: true,
		},
		SkipIPCheck: false,
		AllowedIPs:  buildAllowedIPsFromEnv("THIRD_PARTY_PROVIDER_B_ALLOWED_IPS"),
	})

	return configs
}

func addAPIKeyConfig(configs map[string]APIClientConfig, key string, cfg APIClientConfig) {
	if strings.TrimSpace(key) == "" {
		return
	}
	configs[key] = cfg
}

func buildAllowedIPsFromEnv(envName string) map[string]bool {
	allowed := make(map[string]bool)

	for ip := range defaultWhitelistedIPs {
		allowed[ip] = true
	}

	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return allowed
	}

	for _, part := range strings.Split(raw, ",") {
		ip := strings.TrimSpace(part)
		if ip != "" {
			allowed[ip] = true
		}
	}

	return allowed
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
		if !clientConfig.SkipIPCheck {
			allowedIPs := clientConfig.AllowedIPs
			if len(allowedIPs) == 0 {
				allowedIPs = defaultWhitelistedIPs
			}

			if !allowedIPs[clientIP] {
				http.Error(w, "Access from this IP is not allowed", http.StatusForbidden)
				log.Printf("[SECURITY] 🚫 Blocked - IP not whitelisted. App=%s IP=%s Path=%s", clientConfig.AppName, clientIP, r.URL.Path)
				return
			}
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
