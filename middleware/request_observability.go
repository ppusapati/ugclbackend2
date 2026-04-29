package middleware

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type requestContextKey string

const requestIDContextKey requestContextKey = "request_id"

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
	bytes      int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(data)
	r.bytes += n
	return n, err
}

// RequestObservabilityMiddleware attaches request IDs and logs slow request timings.
func RequestObservabilityMiddleware(next http.Handler) http.Handler {
	enabled := getEnvAsBool("API_REQUEST_LOG_ENABLED", true)
	logAll := getEnvAsBool("API_REQUEST_LOG_ALL", false)
	slowThreshold := getEnvAsDuration("API_SLOW_REQUEST_THRESHOLD", 500*time.Millisecond)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" {
			requestID = uuid.NewString()
		}

		ctx := context.WithValue(r.Context(), requestIDContextKey, requestID)
		r = r.WithContext(ctx)

		w.Header().Set("X-Request-ID", requestID)

		if !enabled {
			next.ServeHTTP(w, r)
			return
		}

		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(recorder, r)
		duration := time.Since(start)

		if !logAll && duration < slowThreshold {
			return
		}

		routePath := r.URL.Path
		if currentRoute := mux.CurrentRoute(r); currentRoute != nil {
			if template, err := currentRoute.GetPathTemplate(); err == nil {
				routePath = template
			}
		}

		log.Printf("[HTTP] id=%s method=%s route=%s status=%d duration_ms=%d bytes=%d ip=%s",
			requestID,
			r.Method,
			routePath,
			recorder.statusCode,
			duration.Milliseconds(),
			recorder.bytes,
			clientIP(r),
		)
	})
}

// GetRequestID returns the correlation ID associated with the request context.
func GetRequestID(r *http.Request) string {
	if r == nil {
		return ""
	}
	if value, ok := r.Context().Value(requestIDContextKey).(string); ok {
		return value
	}
	return ""
}

func clientIP(r *http.Request) string {
	forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	remoteIP, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return remoteIP
	}

	return r.RemoteAddr
}

func getEnvAsBool(key string, defaultVal bool) bool {
	valueStr := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if valueStr == "" {
		return defaultVal
	}

	switch valueStr {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultVal
	}
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	valueStr := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if valueStr == "" {
		return defaultVal
	}

	if value, err := time.ParseDuration(valueStr); err == nil {
		return value
	}

	if valueMS, err := strconv.Atoi(valueStr); err == nil {
		return time.Duration(valueMS) * time.Millisecond
	}

	return defaultVal
}
