package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"p9e.in/ugcl/config"
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

	// Run migrations
	if err := config.Migrations(config.DB); err != nil {
		log.Fatalf("could not run migrations: %v", err)
	}

	// // Run seeding (will skip if data already exists)
	// if err := config.RunAllSeeding(); err != nil {
	// 	log.Printf("Warning: seeding encountered issues: %v", err)
	// }

	handler := routes.RegisterRoutes()
	handlerWithCORS := enableCORS(handler)
	log.Println("Server starting at port", port)
	log.Fatal(http.ListenAndServe(":"+port, handlerWithCORS))
}

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Required CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, x-api-key, X-Requested-With")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")

		// Handle preflight (OPTIONS)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
