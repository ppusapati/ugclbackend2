package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/middleware"
)

// RegisterIntegrationRoutes registers third-party integration endpoints.
func RegisterIntegrationRoutes(r *mux.Router) {
	integrations := r.PathPrefix("/api/v1/integrations").Subrouter()
	integrations.Use(middleware.SecurityMiddleware)

	integrations.HandleFunc("/health", handlers.IntegrationHealth).Methods(http.MethodGet)
	integrations.HandleFunc("/webhook-contract", handlers.WebhookContract).Methods(http.MethodGet)
	integrations.HandleFunc("/forms", handlers.IntegrationFormCatalog).Methods(http.MethodGet)
	integrations.HandleFunc("/provider-a/health", handlers.IntegrationHealth).Methods(http.MethodGet)
	integrations.HandleFunc("/provider-a/webhook-contract", handlers.WebhookContract).Methods(http.MethodGet)
	integrations.HandleFunc("/provider-a/forms", handlers.IntegrationFormCatalog).Methods(http.MethodGet)
	integrations.HandleFunc("/provider-b/health", handlers.IntegrationHealth).Methods(http.MethodGet)
	integrations.HandleFunc("/provider-b/webhook-contract", handlers.WebhookContract).Methods(http.MethodGet)
	integrations.HandleFunc("/provider-b/forms", handlers.IntegrationFormCatalog).Methods(http.MethodGet)
}
