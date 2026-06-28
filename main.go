package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/handlers/reports"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/routes"
)

var (
	Version   = "dev"
	BuildTime = ""

	reportViewAutosyncOnce sync.Once
)

func safeGo(taskName string, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("background task panic", "task", taskName, "panic", r, "stack", string(debug.Stack()))
			}
		}()
		fn()
	}()
}

func configureLogger() {
	format := strings.ToLower(strings.TrimSpace(os.Getenv("LOG_FORMAT")))
	level := slog.LevelInfo
	if strings.EqualFold(strings.TrimSpace(os.Getenv("LOG_LEVEL")), "debug") {
		level = slog.LevelDebug
	}

	options := &slog.HandlerOptions{Level: level}
	if format == "json" {
		slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, options)))
		return
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, options)))
}

func main() {
	configureLogger()

	versionFlag := flag.Bool("version", false, "Print version info and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Version:   %s\n", Version)
		fmt.Printf("BuildTime: %s\n", BuildTime)
		os.Exit(0)
	}

	if strings.TrimSpace(os.Getenv("JWT_SECRET")) == "" {
		slog.Error("startup failed", "reason", "JWT_SECRET is required")
		log.Fatal("JWT_SECRET is required")
	}

	config.Connect()

	// Auto-generate the integration secret encryption key on first run if not set.
	handlers.EnsureIntegrationEncryptionKey()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Keep finance workflows and dynamic forms synchronized with code-defined seeds.
	// config.SeedWorkflows()
	// config.SeedFinanceModulesAndForms()

	handler := routes.RegisterRoutes()

	// Prewarm authorization caches in background to reduce first-hit latency after restarts.
	prewarmUsers := 1
	if raw := os.Getenv("AUTH_CACHE_PREWARM_USERS"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			prewarmUsers = n
		}
	}
	safeGo("prewarm-authorization-caches", func() {
		middleware.PrewarmAuthorizationCaches(prewarmUsers)
	})

	// Auto-sync report views for active forms so report execution never depends on manual setup.
	if strings.EqualFold(strings.TrimSpace(os.Getenv("REPORT_VIEW_AUTOSYNC_ON_STARTUP")), "false") {
		slog.Info("report view autosync disabled", "env", "REPORT_VIEW_AUTOSYNC_ON_STARTUP")
	} else {
		safeGo("report-view-autosync", func() {
			reportViewAutosyncOnce.Do(func() {
				synced, err := reports.EnsureAllActiveFormReportViews(config.DB)
				if err != nil {
					slog.Error("report view autosync failed", "synced_forms", synced, "error", err)
					return
				}
				slog.Info("report view autosync completed", "synced_forms", synced)
			})
		})
	}

	handlerWithCORS := enableCORS(handler)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handlerWithCORS,
		ReadHeaderTimeout: getDurationFromEnv("API_READ_HEADER_TIMEOUT", 10*time.Second),
		ReadTimeout:       getDurationFromEnv("API_READ_TIMEOUT", 30*time.Second),
		WriteTimeout:      getDurationFromEnv("API_WRITE_TIMEOUT", 60*time.Second),
		IdleTimeout:       getDurationFromEnv("API_IDLE_TIMEOUT", 120*time.Second),
		MaxHeaderBytes:    getIntFromEnv("API_MAX_HEADER_BYTES", 1<<20),
	}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-shutdownCtx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("server graceful shutdown failed", "error", err)
		}
	}()

	slog.Info("server starting", "port", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server terminated unexpectedly", "error", err)
		log.Fatal(err)
	}
}

func enableCORS(next http.Handler) http.Handler {
	allowedOrigins := buildAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	allowAnyOrigin := len(allowedOrigins) == 0

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		originAllowed := origin != "" && (allowAnyOrigin || allowedOrigins[origin])

		// Required CORS headers
		if allowAnyOrigin {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else if originAllowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization, x-api-key, X-Requested-With, X-Client-ID, X-Business-ID, X-Business-Code")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")
		w.Header().Add("Vary", "Access-Control-Request-Headers")

		// Handle preflight (OPTIONS)
		if r.Method == http.MethodOptions {
			if !allowAnyOrigin && origin != "" && !originAllowed {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}

		if !allowAnyOrigin && origin != "" && !originAllowed {
			http.Error(w, "origin is not allowed", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func buildAllowedOrigins(raw string) map[string]bool {
	allowed := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		allowed[origin] = true
	}
	return allowed
}

func getDurationFromEnv(key string, defaultVal time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return defaultVal
	}

	return parsed
}

func getIntFromEnv(key string, defaultVal int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}

	return parsed
}
