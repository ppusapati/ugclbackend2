package routes

import (
	"net/http"

	"github.com/gorilla/mux"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/middleware"
)

// RegisterDocumentRoutes registers all document management routes
func RegisterDocumentRoutes(api *mux.Router, admin *mux.Router) {

	// Document CRUD operations
	api.Handle("/documents", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.GetDocumentsHandler))).Methods("GET")

	api.Handle("/documents", middleware.RequirePermission("documents:create")(
		http.HandlerFunc(handlers.UploadDocumentHandler))).Methods("POST")

	api.Handle("/documents/{id}", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.GetDocumentHandler))).Methods("GET")

	api.Handle("/documents/{id}", middleware.RequirePermission("documents:update")(
		http.HandlerFunc(handlers.UpdateDocumentHandler))).Methods("PUT")

	api.Handle("/documents/{id}", middleware.RequirePermission("documents:delete")(
		http.HandlerFunc(handlers.DeleteDocumentHandler))).Methods("DELETE")

	// Document download
	api.Handle("/documents/{id}/download", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.DownloadDocumentHandler))).Methods("GET")

	// Document search
	api.Handle("/documents/search", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.SearchDocumentsHandler))).Methods("GET")

	// Document statistics
	api.Handle("/documents/statistics", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.GetDocumentStatisticsHandler))).Methods("GET")

	// Document audit logs
	api.Handle("/documents/{id}/audit", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.GetDocumentAuditLogsHandler))).Methods("GET")

	// Document version management
	api.Handle("/documents/{id}/versions", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.GetDocumentVersionsHandler))).Methods("GET")

	api.Handle("/documents/{id}/versions", middleware.RequirePermission("documents:update")(
		http.HandlerFunc(handlers.CreateDocumentVersionHandler))).Methods("POST")

	api.Handle("/documents/{id}/versions/{version_id}/download", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.DownloadDocumentVersionHandler))).Methods("GET")

	api.Handle("/documents/{id}/versions/compare", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.CompareDocumentVersionsHandler))).Methods("GET")

	api.Handle("/documents/{id}/versions/{version_id}/rollback", middleware.RequirePermission("documents:update")(
		http.HandlerFunc(handlers.RollbackDocumentVersionHandler))).Methods("POST")

	// Document categories
	api.Handle("/documents/categories", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.GetDocumentCategoriesHandler))).Methods("GET")

	api.Handle("/documents/categories", middleware.RequirePermission("documents:manage_categories")(
		http.HandlerFunc(handlers.CreateDocumentCategoryHandler))).Methods("POST")

	api.Handle("/documents/categories/{id}", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.GetDocumentCategoryHandler))).Methods("GET")

	api.Handle("/documents/categories/{id}", middleware.RequirePermission("documents:manage_categories")(
		http.HandlerFunc(handlers.UpdateDocumentCategoryHandler))).Methods("PUT")

	api.Handle("/documents/categories/{id}", middleware.RequirePermission("documents:manage_categories")(
		http.HandlerFunc(handlers.DeleteDocumentCategoryHandler))).Methods("DELETE")

	// Document tags
	api.Handle("/documents/tags", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.GetDocumentTagsHandler))).Methods("GET")

	api.Handle("/documents/tags", middleware.RequirePermission("documents:manage_tags")(
		http.HandlerFunc(handlers.CreateDocumentTagHandler))).Methods("POST")

	api.Handle("/documents/tags/{id}", middleware.RequirePermission("documents:manage_tags")(
		http.HandlerFunc(handlers.UpdateDocumentTagHandler))).Methods("PUT")

	api.Handle("/documents/tags/{id}", middleware.RequirePermission("documents:manage_tags")(
		http.HandlerFunc(handlers.DeleteDocumentTagHandler))).Methods("DELETE")

	// Document sharing
	api.Handle("/documents/{id}/shares", middleware.RequirePermission("documents:share")(
		http.HandlerFunc(handlers.CreateDocumentShareHandler))).Methods("POST")

	api.Handle("/documents/{id}/shares", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.GetDocumentSharesHandler))).Methods("GET")

	api.Handle("/documents/shares/{share_id}/revoke", middleware.RequirePermission("documents:share")(
		http.HandlerFunc(handlers.RevokeDocumentShareHandler))).Methods("POST")

	// Document permissions
	api.Handle("/documents/{id}/permissions", middleware.RequirePermission("documents:manage_permissions")(
		http.HandlerFunc(handlers.GrantDocumentPermissionHandler))).Methods("POST")

	api.Handle("/documents/{id}/permissions", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.GetDocumentPermissionsHandler))).Methods("GET")

	api.Handle("/documents/permissions/{permission_id}/revoke", middleware.RequirePermission("documents:manage_permissions")(
		http.HandlerFunc(handlers.RevokeDocumentPermissionHandler))).Methods("DELETE")

	// Bulk operations
	api.Handle("/documents/bulk/delete", middleware.RequirePermission("documents:delete")(
		http.HandlerFunc(handlers.BulkDeleteDocumentsHandler))).Methods("POST")

	api.Handle("/documents/bulk/update", middleware.RequirePermission("documents:update")(
		http.HandlerFunc(handlers.BulkUpdateDocumentsHandler))).Methods("POST")

	api.Handle("/documents/bulk/download", middleware.RequirePermission("documents:read")(
		http.HandlerFunc(handlers.BulkDownloadDocumentsHandler))).Methods("POST")

	api.Handle("/documents/bulk/tags", middleware.RequirePermission("documents:update")(
		http.HandlerFunc(handlers.BulkAddTagsHandler))).Methods("POST")

	// Public shared document access (no authentication required)
	// These routes are registered on the main router, not the api subrouter
	api.HandleFunc("/documents/shared/{token}", http.HandlerFunc(handlers.AccessSharedDocumentHandler)).Methods("GET")
	api.HandleFunc("/documents/shared/{token}/download", http.HandlerFunc(handlers.DownloadSharedDocumentHandler)).Methods("GET")
}
