package middleware

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultLoginRateRPS       = 5
	defaultLoginRateBurst     = 10
	defaultLoginRateEntryTTL  = 15 * time.Minute
	defaultLoginCleanupPeriod = 5 * time.Minute
)

type loginLimiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type loginRateLimiterStore struct {
	mu            sync.Mutex
	entries       map[string]*loginLimiterEntry
	ratePerSecond rate.Limit
	burst         int
	entryTTL      time.Duration
	cleanupPeriod time.Duration
}

var loginRateLimiter = newLoginRateLimiterStore(
	loadEnvAsFloat("LOGIN_RATE_LIMIT_RPS", defaultLoginRateRPS),
	loadEnvAsInt("LOGIN_RATE_LIMIT_BURST", defaultLoginRateBurst),
	loadEnvAsDuration("LOGIN_RATE_LIMIT_ENTRY_TTL", defaultLoginRateEntryTTL),
	loadEnvAsDuration("LOGIN_RATE_LIMIT_CLEANUP_PERIOD", defaultLoginCleanupPeriod),
)

func init() {
	go loginRateLimiter.startCleanupWorker()
}

func newLoginRateLimiterStore(rps float64, burst int, entryTTL, cleanupPeriod time.Duration) *loginRateLimiterStore {
	if rps <= 0 {
		rps = defaultLoginRateRPS
	}
	if burst <= 0 {
		burst = defaultLoginRateBurst
	}
	if entryTTL <= 0 {
		entryTTL = defaultLoginRateEntryTTL
	}
	if cleanupPeriod <= 0 {
		cleanupPeriod = defaultLoginCleanupPeriod
	}

	return &loginRateLimiterStore{
		entries:       make(map[string]*loginLimiterEntry),
		ratePerSecond: rate.Limit(rps),
		burst:         burst,
		entryTTL:      entryTTL,
		cleanupPeriod: cleanupPeriod,
	}
}

func (s *loginRateLimiterStore) allow(clientIP string, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[clientIP]
	if !ok {
		entry = &loginLimiterEntry{limiter: rate.NewLimiter(s.ratePerSecond, s.burst)}
		s.entries[clientIP] = entry
	}

	entry.lastSeen = now
	return entry.limiter.Allow()
}

func (s *loginRateLimiterStore) startCleanupWorker() {
	ticker := time.NewTicker(s.cleanupPeriod)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-s.entryTTL)
		s.mu.Lock()
		for ip, entry := range s.entries {
			if entry.lastSeen.Before(cutoff) {
				delete(s.entries, ip)
			}
		}
		s.mu.Unlock()
	}
}

func LoginRateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := loginClientIP(r)
		if !loginRateLimiter.allow(clientIP, time.Now()) {
			http.Error(w, "too many login attempts", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loginClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	if trimmed := strings.TrimSpace(r.RemoteAddr); trimmed != "" {
		return trimmed
	}
	return "unknown"
}

func loadEnvAsFloat(key string, defaultVal float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return defaultVal
	}
	return value
}

func loadEnvAsInt(key string, defaultVal int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	return value
}

func loadEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return defaultVal
	}
	return value
}
