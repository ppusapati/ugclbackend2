// auth/jwt.go
package middleware

import (
	"container/list"
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/models"
)

var jwtKey []byte

func init() {
	jwtKey = []byte(strings.TrimSpace(config.JWTSecret))
	if len(jwtKey) == 0 {
		log.Fatal("JWT_SECRET is required")
	}
	startThirdPartyAccessBatcher()
}

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
	thirdPartyIntegrationKey
)

type thirdPartyRequestContext struct {
	IntegrationID string
	Name          string
	Scopes        map[string]bool
	AllowedURLs   map[string]bool
}

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
			// Allow token query fallback only for SSE endpoints to reduce token-leak surface.
			if isSSETokenQueryAllowed(r) {
				tokenStr = strings.TrimSpace(r.URL.Query().Get("token"))
				if tokenStr == "" {
					tokenStr = strings.TrimSpace(r.URL.Query().Get("access_token"))
				}
			}
			if tokenStr == "" {
				http.Error(w, "missing Authorization header", http.StatusUnauthorized)
				return
			}
		}

		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
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
			return *cached
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
	Integration    *thirdPartyRequestContext
}

const (
	thirdPartyLookupCacheTTL         = 5 * time.Minute
	thirdPartyLookupNegativeCacheTTL = 10 * time.Second
	thirdPartyLookupCacheSize        = 1024
	thirdPartyAccessFlushInterval    = 10 * time.Second
	thirdPartyAccessQueueSize        = 8192
)

type thirdPartyLookupCacheEntry struct {
	apiKey    string
	config    APIClientConfig
	ok        bool
	expiresAt time.Time
}

type thirdPartyLookupCache struct {
	mu         sync.Mutex
	ttl        time.Duration
	maxEntries int
	ll         *list.List
	entries    map[string]*list.Element
}

func newThirdPartyLookupCache(maxEntries int, ttl time.Duration) *thirdPartyLookupCache {
	return &thirdPartyLookupCache{
		ttl:        ttl,
		maxEntries: maxEntries,
		ll:         list.New(),
		entries:    make(map[string]*list.Element, maxEntries),
	}
}

func (c *thirdPartyLookupCache) get(apiKey string) (APIClientConfig, bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.entries[apiKey]
	if !ok {
		return APIClientConfig{}, false, false
	}

	entry := elem.Value.(*thirdPartyLookupCacheEntry)
	if time.Now().After(entry.expiresAt) {
		c.ll.Remove(elem)
		delete(c.entries, apiKey)
		return APIClientConfig{}, false, false
	}

	c.ll.MoveToFront(elem)
	return entry.config, entry.ok, true
}

func (c *thirdPartyLookupCache) set(apiKey string, cfg APIClientConfig, valid bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, exists := c.entries[apiKey]; exists {
		entry := elem.Value.(*thirdPartyLookupCacheEntry)
		entry.config = cfg
		entry.ok = valid
		if valid {
			entry.expiresAt = time.Now().Add(c.ttl)
		} else {
			entry.expiresAt = time.Now().Add(thirdPartyLookupNegativeCacheTTL)
		}
		c.ll.MoveToFront(elem)
		return
	}

	expiresAt := time.Now().Add(thirdPartyLookupNegativeCacheTTL)
	if valid {
		expiresAt = time.Now().Add(c.ttl)
	}

	elem := c.ll.PushFront(&thirdPartyLookupCacheEntry{
		apiKey:    apiKey,
		config:    cfg,
		ok:        valid,
		expiresAt: expiresAt,
	})
	c.entries[apiKey] = elem

	if c.maxEntries > 0 && c.ll.Len() > c.maxEntries {
		oldest := c.ll.Back()
		if oldest != nil {
			c.ll.Remove(oldest)
			oldestEntry := oldest.Value.(*thirdPartyLookupCacheEntry)
			delete(c.entries, oldestEntry.apiKey)
		}
	}
}

var apiKeyConfigs = loadAPIKeyConfigs()
var thirdPartyAPIKeyLookupCache = newThirdPartyLookupCache(thirdPartyLookupCacheSize, thirdPartyLookupCacheTTL)
var trustedProxyNetworks = loadTrustedProxyNetworks()
var thirdPartyIntegrationAccessQueue = make(chan string, thirdPartyAccessQueueSize)

func startThirdPartyAccessBatcher() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("integration access batcher panic", "panic", r)
			}
		}()

		pending := make(map[string]int64)
		ticker := time.NewTicker(thirdPartyAccessFlushInterval)
		defer ticker.Stop()

		flush := func() {
			if len(pending) == 0 {
				return
			}
			now := time.Now().UTC()
			for id, count := range pending {
				if count <= 0 {
					delete(pending, id)
					continue
				}
				if err := config.DB.Model(&models.ThirdPartyIntegration{}).
					Where("id = ?", id).
					Updates(map[string]interface{}{
						"last_access_at": now,
						"access_count":   gorm.Expr("access_count + ?", count),
					}).Error; err != nil {
					slog.Error("failed to batch record integration access", "integration_id", id, "error", err)
				}
				delete(pending, id)
			}
		}

		for {
			select {
			case id := <-thirdPartyIntegrationAccessQueue:
				if strings.TrimSpace(id) != "" {
					pending[id]++
				}
			case <-ticker.C:
				flush()
			}
		}
	}()
}

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
			http.MethodPut:  true,
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

func lookupThirdPartyIntegrationConfig(apiKey string) (APIClientConfig, bool) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return APIClientConfig{}, false
	}

	if cfg, valid, cached := thirdPartyAPIKeyLookupCache.get(apiKey); cached {
		if valid {
			return cfg, true
		}
		return APIClientConfig{}, false
	}

	var items []models.ThirdPartyIntegration
	if err := config.DB.Where("status = ?", models.IntegrationStatusActive).Find(&items).Error; err != nil {
		log.Printf("[SECURITY] failed to load third-party integrations: %v", err)
		return APIClientConfig{}, false
	}

	for _, item := range items {
		if bcrypt.CompareHashAndPassword([]byte(item.APIKeyHash), []byte(apiKey)) != nil {
			continue
		}

		allowedIPs := make(map[string]bool, len(item.AllowedIPs))
		for _, ip := range item.AllowedIPs {
			allowedIPs[ip] = true
		}

		scopes := make(map[string]bool, len(item.DataScopes))
		for _, scope := range item.DataScopes {
			scopes[scope] = true
		}

		allowedURLs := make(map[string]bool, len(item.AllowedURLs))
		for _, value := range item.AllowedURLs {
			allowedURLs[value] = true
		}

		cfg := APIClientConfig{
			AppName:      item.Name,
			AllowedPaths: []string{"/api/v1/partner/*", "/api/v1/integrations/*"},
			AllowedMethods: map[string]bool{
				http.MethodGet: true,
			},
			SkipIPCheck: len(allowedIPs) == 0,
			AllowedIPs:  allowedIPs,
			Integration: &thirdPartyRequestContext{
				IntegrationID: item.ID.String(),
				Name:          item.Name,
				Scopes:        scopes,
				AllowedURLs:   allowedURLs,
			},
		}

		thirdPartyAPIKeyLookupCache.set(apiKey, cfg, true)
		return cfg, true
	}

	thirdPartyAPIKeyLookupCache.set(apiKey, APIClientConfig{}, false)

	return APIClientConfig{}, false
}

func ipAllowed(clientIP string, allowedIPs map[string]bool) bool {
	if allowedIPs[clientIP] {
		return true
	}

	parsedIP := net.ParseIP(clientIP)
	if parsedIP == nil {
		return false
	}

	for candidate := range allowedIPs {
		if !strings.Contains(candidate, "/") {
			continue
		}
		_, network, err := net.ParseCIDR(candidate)
		if err == nil && network.Contains(parsedIP) {
			return true
		}
	}

	return false
}

func GetThirdPartyIntegration(r *http.Request) *thirdPartyRequestContext {
	if r == nil {
		return nil
	}
	if value, ok := r.Context().Value(thirdPartyIntegrationKey).(*thirdPartyRequestContext); ok {
		return value
	}
	return nil
}

func RequireIntegrationScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			integration := GetThirdPartyIntegration(r)
			if integration == nil || integration.Scopes[scope] {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "integration is not allowed to access this dataset", http.StatusForbidden)
		})
	}
}

func enqueueThirdPartyIntegrationAccess(id string) {
	if strings.TrimSpace(id) == "" {
		return
	}
	select {
	case thirdPartyIntegrationAccessQueue <- id:
	default:
		slog.Warn("integration access queue full; dropping event", "integration_id", id)
	}
}

// SecurityMiddleware enforces API key, IP filtering, and logging
func SecurityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := GetRequestID(r)

		// Browser EventSource clients cannot send custom x-api-key headers.
		// For approved SSE endpoints, rely on JWT auth (token/access_token query) in JWTMiddleware.
		if isSSETokenQueryAllowed(r) {
			token := strings.TrimSpace(r.URL.Query().Get("token"))
			if token == "" {
				token = strings.TrimSpace(r.URL.Query().Get("access_token"))
			}
			if token != "" {
				slog.Info("security api-key bypass for approved SSE endpoint", "request_id", requestID, "path", r.URL.Path)
				next.ServeHTTP(w, r)
				return
			}
		}

		apiKey := r.Header.Get("x-api-key")
		clientConfig, ok := apiKeyConfigs[apiKey]
		if !ok && strings.TrimSpace(apiKey) != "" {
			clientConfig, ok = lookupThirdPartyIntegrationConfig(apiKey)
		}
		if !ok {
			http.Error(w, "Invalid or missing API key", http.StatusUnauthorized)
			slog.Warn("blocked invalid API key", "request_id", requestID, "ip", getClientIP(r), "path", r.URL.Path)
			return
		}

		clientIP := getClientIP(r)
		if !clientConfig.SkipIPCheck {
			allowedIPs := clientConfig.AllowedIPs
			if len(allowedIPs) == 0 {
				allowedIPs = defaultWhitelistedIPs
			}

			if !ipAllowed(clientIP, allowedIPs) {
				http.Error(w, "Access from this IP is not allowed", http.StatusForbidden)
				slog.Warn("blocked non-whitelisted IP", "request_id", requestID, "app", clientConfig.AppName, "ip", clientIP, "path", r.URL.Path)
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
			slog.Warn("denied disallowed path", "request_id", requestID, "app", clientConfig.AppName, "ip", clientIP, "path", r.URL.Path)
			return
		}

		// ✅ Method-based access check
		if !clientConfig.AllowedMethods[r.Method] {
			http.Error(w, "This HTTP method is not allowed for this app", http.StatusMethodNotAllowed)
			slog.Warn("denied disallowed method", "request_id", requestID, "app", clientConfig.AppName, "method", r.Method, "path", r.URL.Path)
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

		slog.Info("security request allowed",
			"request_id", requestID,
			"app", clientConfig.AppName,
			"user_id", userID,
			"user_name", userName,
			"role", userRole,
			"ip", clientIP,
			"path", r.URL.Path,
			"method", r.Method,
			"timestamp", time.Now().Format(time.RFC3339),
		)

		if clientConfig.Integration != nil {
			ctx := context.WithValue(r.Context(), thirdPartyIntegrationKey, clientConfig.Integration)
			r = r.WithContext(ctx)
			enqueueThirdPartyIntegrationAccess(clientConfig.Integration.IntegrationID)
		}

		next.ServeHTTP(w, r)
	})
}

func isSSETokenQueryAllowed(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.Method != http.MethodGet {
		return false
	}
	path := strings.TrimSpace(r.URL.Path)
	return path == "/api/v1/chat/events" || path == "/api/v1/notifications/stream"
}

func loadTrustedProxyNetworks() []*net.IPNet {
	raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if raw == "" {
		return nil
	}

	networks := make([]*net.IPNet, 0)
	for _, part := range strings.Split(raw, ",") {
		candidate := strings.TrimSpace(part)
		if candidate == "" {
			continue
		}
		_, network, err := net.ParseCIDR(candidate)
		if err != nil {
			slog.Warn("invalid TRUSTED_PROXY_CIDRS entry ignored", "value", candidate)
			continue
		}
		networks = append(networks, network)
	}

	return networks
}

func ipInTrustedProxyNetworks(ip net.IP) bool {
	if ip == nil || len(trustedProxyNetworks) == 0 {
		return false
	}
	for _, network := range trustedProxyNetworks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

// Extracts client IP from headers or remote addr
func getClientIP(r *http.Request) string {
	remoteHost, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		remoteHost = strings.TrimSpace(r.RemoteAddr)
	}
	remoteIP := net.ParseIP(remoteHost)

	// Only trust forwarding headers when the direct peer is in TRUSTED_PROXY_CIDRS.
	if ipInTrustedProxyNetworks(remoteIP) {
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			parts := strings.Split(xff, ",")
			for i := len(parts) - 1; i >= 0; i-- {
				candidate := net.ParseIP(strings.TrimSpace(parts[i]))
				if candidate == nil {
					continue
				}
				if ipInTrustedProxyNetworks(candidate) {
					continue
				}
				return candidate.String()
			}
			for i := 0; i < len(parts); i++ {
				candidate := net.ParseIP(strings.TrimSpace(parts[i]))
				if candidate != nil {
					return candidate.String()
				}
			}
		}

		if xRealIP := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); xRealIP != nil {
			return xRealIP.String()
		}
	}

	if remoteIP != nil {
		return remoteIP.String()
	}
	return remoteHost
}
