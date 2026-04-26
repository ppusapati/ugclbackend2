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

// RegisterAdminIntegrationRoutes mounts CRUD routes for managing third-party integrations.
// Must be called with the /api/v1/admin subrouter that already has JWT + security middleware.
func RegisterAdminIntegrationRoutes(admin *mux.Router) {
	// List / create
	admin.Handle("/integrations", middleware.RequirePermission("manage_integrations")(
		http.HandlerFunc(handlers.ListIntegrations))).Methods(http.MethodGet)
	admin.Handle("/integrations", middleware.RequirePermission("manage_integrations")(
		http.HandlerFunc(handlers.CreateIntegration))).Methods(http.MethodPost)

	// Single-record operations — specific paths before parameterised {id}
	admin.Handle("/integrations/{id}/regenerate-key", middleware.RequirePermission("manage_integrations")(
		http.HandlerFunc(handlers.RegenerateIntegrationKey))).Methods(http.MethodPost)

	admin.Handle("/integrations/{id}", middleware.RequirePermission("manage_integrations")(
		http.HandlerFunc(handlers.GetIntegration))).Methods(http.MethodGet)
	admin.Handle("/integrations/{id}", middleware.RequirePermission("manage_integrations")(
		http.HandlerFunc(handlers.UpdateIntegration))).Methods(http.MethodPatch)
	admin.Handle("/integrations/{id}", middleware.RequirePermission("manage_integrations")(
		http.HandlerFunc(handlers.DeleteIntegration))).Methods(http.MethodDelete)
}
