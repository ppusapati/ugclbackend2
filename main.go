package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"p9e.in/ugcl/config"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/routes"
)

var (
	Version   = "dev"
	BuildTime = ""
)

func main() {

	versionFlag := flag.Bool("version", false, "Print version info and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("Version:   %s\n", Version)
		fmt.Printf("BuildTime: %s\n", BuildTime)
		os.Exit(0)
	}
	config.Connect()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// // Run seeding (will skip if data already exists)
	// if err := config.RunAllSeeding(); err != nil {
	// 	log.Printf("Warning: seeding encountered issues: %v", err)
	// }

	handler := routes.RegisterRoutes()

	// Prewarm authorization caches in background to reduce first-hit latency after restarts.
	prewarmUsers := 1
	if raw := os.Getenv("AUTH_CACHE_PREWARM_USERS"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n >= 0 {
			prewarmUsers = n
		}
	}
	go middleware.PrewarmAuthorizationCaches(prewarmUsers)

	// Auto-sync report views for active forms so report execution never depends on manual setup.
	if strings.EqualFold(strings.TrimSpace(os.Getenv("REPORT_VIEW_AUTOSYNC_ON_STARTUP")), "false") {
		log.Println("[REPORT-VIEWS] startup autosync disabled via REPORT_VIEW_AUTOSYNC_ON_STARTUP=false")
	} else {
		go func() {
			synced, err := handlers.EnsureAllActiveFormReportViews(config.DB)
			if err != nil {
				log.Printf("[REPORT-VIEWS] startup autosync failed after %d forms: %v", synced, err)
				return
			}
			log.Printf("[REPORT-VIEWS] startup autosync completed: %d active forms synced", synced)
		}()
	}

	handlerWithCORS := enableCORS(handler)
	log.Println("Server starting at port", port)
	log.Fatal(http.ListenAndServe(":"+port, handlerWithCORS))
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Required CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization, x-api-key, X-Requested-With, X-Client-ID, X-Business-ID, X-Business-Code")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Add("Vary", "Origin")
		w.Header().Add("Vary", "Access-Control-Request-Method")
		w.Header().Add("Vary", "Access-Control-Request-Headers")

		// Handle preflight (OPTIONS)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
