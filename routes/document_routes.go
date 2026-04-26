package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/middleware"
)

// RegisterDocumentRoutes registers all document management routes
func RegisterDocumentRoutes(api *mux.Router, admin *mux.Router) {
	_ = admin

	// Specific literal routes must be registered before /documents/{id}.
	api.Handle("/documents/search", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.SearchDocumentsHandler))).Methods("GET")

	api.Handle("/documents/statistics", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.GetDocumentStatisticsHandler))).Methods("GET")

	api.Handle("/documents/ai/integrations", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.ListDocumentAIIntegrationsHandler))).Methods("GET")
	api.Handle("/documents/workflows", middleware.RequirePermission("document:upload")(
		http.HandlerFunc(handlers.ListDocumentWorkflowsHandler))).Methods("GET")

	api.Handle("/documents/categories", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.GetDocumentCategoriesHandler))).Methods("GET")
	api.Handle("/documents/categories", middleware.RequirePermission("document:manage_categories")(
		http.HandlerFunc(handlers.CreateDocumentCategoryHandler))).Methods("POST")
	api.Handle("/documents/categories/{id}", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.GetDocumentCategoryHandler))).Methods("GET")
	api.Handle("/documents/categories/{id}", middleware.RequirePermission("document:manage_categories")(
		http.HandlerFunc(handlers.UpdateDocumentCategoryHandler))).Methods("PUT")
	api.Handle("/documents/categories/{id}", middleware.RequirePermission("document:manage_categories")(
		http.HandlerFunc(handlers.DeleteDocumentCategoryHandler))).Methods("DELETE")

	api.Handle("/documents/tags", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.GetDocumentTagsHandler))).Methods("GET")
	api.Handle("/documents/tags", middleware.RequirePermission("document:manage_tags")(
		http.HandlerFunc(handlers.CreateDocumentTagHandler))).Methods("POST")
	api.Handle("/documents/tags/{id}", middleware.RequirePermission("document:manage_tags")(
		http.HandlerFunc(handlers.UpdateDocumentTagHandler))).Methods("PUT")
	api.Handle("/documents/tags/{id}", middleware.RequirePermission("document:manage_tags")(
		http.HandlerFunc(handlers.DeleteDocumentTagHandler))).Methods("DELETE")

	api.Handle("/documents/bulk/delete", middleware.RequirePermission("document:delete")(
		http.HandlerFunc(handlers.BulkDeleteDocumentsHandler))).Methods("POST")
	api.Handle("/documents/bulk/update", middleware.RequirePermission("document:update")(
		http.HandlerFunc(handlers.BulkUpdateDocumentsHandler))).Methods("POST")
	api.Handle("/documents/bulk/download", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.BulkDownloadDocumentsHandler))).Methods("POST")
	api.Handle("/documents/bulk/tags", middleware.RequirePermission("document:update")(
		http.HandlerFunc(handlers.BulkAddTagsHandler))).Methods("POST")

	api.Handle("/documents", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.GetDocumentsHandler))).Methods("GET")
	api.Handle("/documents", middleware.RequirePermission("document:upload")(
		http.HandlerFunc(handlers.UploadDocumentHandler))).Methods("POST")
	api.Handle("/documents/{id}", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.GetDocumentHandler))).Methods("GET")
	api.Handle("/documents/{id}", middleware.RequirePermission("document:update")(
		http.HandlerFunc(handlers.UpdateDocumentHandler))).Methods("PUT")
	api.Handle("/documents/{id}", middleware.RequirePermission("document:delete")(
		http.HandlerFunc(handlers.DeleteDocumentHandler))).Methods("DELETE")
	api.Handle("/documents/{id}/download", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.DownloadDocumentHandler))).Methods("GET")
	api.Handle("/documents/{id}/ai/process", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.ProcessDocumentAIHandler))).Methods("POST")
	api.Handle("/documents/{id}/workflow", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.GetDocumentWorkflowHandler))).Methods("GET")
	api.Handle("/documents/{id}/workflow/transition", http.HandlerFunc(handlers.TransitionDocumentWorkflowHandler)).Methods("POST")
	api.Handle("/documents/{id}/audit", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.GetDocumentAuditLogsHandler))).Methods("GET")

	api.Handle("/documents/{id}/versions", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.GetDocumentVersionsHandler))).Methods("GET")
	api.Handle("/documents/{id}/versions", middleware.RequirePermission("document:update")(
		http.HandlerFunc(handlers.CreateDocumentVersionHandler))).Methods("POST")
	api.Handle("/documents/{id}/versions/{version_id}/download", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.DownloadDocumentVersionHandler))).Methods("GET")
	api.Handle("/documents/{id}/versions/compare", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.CompareDocumentVersionsHandler))).Methods("GET")
	api.Handle("/documents/{id}/versions/{version_id}/rollback", middleware.RequirePermission("document:update")(
		http.HandlerFunc(handlers.RollbackDocumentVersionHandler))).Methods("POST")

	api.Handle("/documents/{id}/shares", middleware.RequirePermission("document:share")(
		http.HandlerFunc(handlers.CreateDocumentShareHandler))).Methods("POST")
	api.Handle("/documents/{id}/shares", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.GetDocumentSharesHandler))).Methods("GET")
	api.Handle("/documents/shares/{share_id}/revoke", middleware.RequirePermission("document:share")(
		http.HandlerFunc(handlers.RevokeDocumentShareHandler))).Methods("POST")

	api.Handle("/documents/{id}/permissions", middleware.RequirePermission("document:manage_permissions")(
		http.HandlerFunc(handlers.GrantDocumentPermissionHandler))).Methods("POST")
	api.Handle("/documents/{id}/permissions", middleware.RequirePermission("document:read")(
		http.HandlerFunc(handlers.GetDocumentPermissionsHandler))).Methods("GET")
	api.Handle("/documents/permissions/{permission_id}/revoke", middleware.RequirePermission("document:manage_permissions")(
		http.HandlerFunc(handlers.RevokeDocumentPermissionHandler))).Methods("DELETE")

	// Public shared document access (no authentication required)
	// These routes are registered on the main router, not the api subrouter
	api.HandleFunc("/documents/shared/{token}", http.HandlerFunc(handlers.AccessSharedDocumentHandler)).Methods("GET")
	api.HandleFunc("/documents/shared/{token}/download", http.HandlerFunc(handlers.DownloadSharedDocumentHandler)).Methods("GET")
}
