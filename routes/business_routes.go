package routes

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"p9e.in/ugcl/handlers"
	"p9e.in/ugcl/handlers/masters"
	"p9e.in/ugcl/middleware"
)

// RegisterBusinessRoutes adds business vertical specific routes
func RegisterBusinessRoutes(r *mux.Router) {
	// Global business management (Super Admin only)
	admin := r.PathPrefix("/api/v1/admin").Subrouter()
	admin.Use(middleware.SecurityMiddleware)
	admin.Use(middleware.JWTMiddleware)

	// Business vertical management
	admin.Handle("/businesses", middleware.RequirePermission("manage_businesses")(
		http.HandlerFunc(handlers.GetAllBusinessVerticals))).Methods("GET")
	admin.Handle("/businesses", middleware.RequirePermission("manage_businesses")(
		http.HandlerFunc(handlers.CreateBusinessVertical))).Methods("POST")

	// Super admin dashboard
	admin.Handle("/dashboard", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.GetSuperAdminDashboard))).Methods("GET")

	// Site management (admin - all sites across all business verticals)
	admin.Handle("/sites", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(masters.GetAllSites))).Methods("GET")
	admin.Handle("/sites", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(masters.CreateSite))).Methods("POST")
	admin.Handle("/sites/{siteId}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(masters.GetSiteByID))).Methods("GET")
	admin.Handle("/sites/{siteId}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(masters.UpdateSite))).Methods("PUT")
	// admin.Handle("/sites/{siteId}", middleware.RequirePermission("admin_all")(
	// 	http.HandlerFunc(masters.DeleteSite))).Methods("DELETE")

	// Form management (admin only - unified system)
	admin.Handle("/app-forms", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.GetAllAppForms))).Methods("GET")
	admin.Handle("/app-forms", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.CreateForm))).Methods("POST")
	admin.Handle("/app-forms/{formCode}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.UpdateForm))).Methods("PUT")
	admin.Handle("/app-forms/{formCode}/verticals", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.UpdateFormVerticalAccess))).Methods("POST")

	// Workflow management (admin only)
	admin.Handle("/workflows", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.GetAllWorkflows))).Methods("GET")
	admin.Handle("/workflows", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.CreateWorkflowDefinition))).Methods("POST")
	admin.Handle("/workflows/{workflowId}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.UpdateWorkflowDefinition))).Methods("PUT")
	admin.Handle("/workflows/{workflowId}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.DeleteWorkflowDefinition))).Methods("DELETE")

	// User's accessible businesses (any authenticated user)
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(middleware.SecurityMiddleware)
	api.Use(middleware.JWTMiddleware)
	api.HandleFunc("/my-businesses", handlers.GetUserBusinessAccess).Methods("GET")
	api.HandleFunc("/modules", masters.GetModules).Methods("GET")

	// Business-specific routes using business codes
	business := r.PathPrefix("/api/v1/business/{businessCode}").Subrouter()
	business.Use(middleware.SecurityMiddleware)
	business.Use(middleware.JWTMiddleware)
	business.Use(middleware.RequireBusinessAccess())

	// Business role management
	business.Handle("/roles", middleware.RequireBusinessPermission("business_manage_roles")(
		http.HandlerFunc(handlers.GetBusinessRoles))).Methods("GET")
	business.Handle("/roles", middleware.RequireBusinessPermission("business_manage_roles")(
		http.HandlerFunc(handlers.CreateBusinessRole))).Methods("POST")

	// Business user management
	business.Handle("/users", middleware.RequireBusinessPermission("business_manage_users")(
		http.HandlerFunc(handlers.GetBusinessUsers))).Methods("GET")
	business.Handle("/users/assign", middleware.RequireBusinessPermission("business_manage_users")(
		http.HandlerFunc(handlers.AssignUserToBusinessRole))).Methods("POST")

	// Business-specific reports (with business context)
	business.Handle("/reports/dprsite", middleware.RequireBusinessPermission("read_reports")(
		http.HandlerFunc(handlers.GetBusinessSiteReports))).Methods("GET")
	business.Handle("/reports/dprsite", middleware.RequireBusinessPermission("create_reports")(
		http.HandlerFunc(handlers.CreateBusinessSiteReport))).Methods("POST")

	business.Handle("/reports/materials", middleware.RequireBusinessPermission("read_materials")(
		http.HandlerFunc(handlers.GetBusinessMaterials))).Methods("GET")
	business.Handle("/reports/materials", middleware.RequireBusinessPermission("create_materials")(
		http.HandlerFunc(handlers.CreateBusinessMaterial))).Methods("POST")

	// Business analytics
	business.Handle("/analytics", middleware.RequireBusinessPermission("business_view_analytics")(
		http.HandlerFunc(handlers.GetBusinessAnalytics))).Methods("GET")

	// Business info and context endpoints
	business.HandleFunc("/info", handlers.GetBusinessInfo).Methods("GET")
	business.HandleFunc("/context", func(w http.ResponseWriter, r *http.Request) {
		context := middleware.GetUserBusinessContext(r)
		if context == nil {
			http.Error(w, "unable to get business context", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(context)
	}).Methods("GET")

	// Form configuration endpoints (new unified system)
	admin.HandleFunc("/forms", handlers.GetFormsForVertical).Methods("GET")
	admin.HandleFunc("/forms/{code}", handlers.GetFormByCode).Methods("GET")
	business.HandleFunc("/forms", handlers.GetFormsForVertical).Methods("GET")
	business.HandleFunc("/forms/{code}", handlers.GetFormByCode).Methods("GET")

	// Workflow and form submission endpoints (generic table approach)
	business.HandleFunc("/forms/{formCode}/submissions", handlers.CreateFormSubmission).Methods("POST")
	business.HandleFunc("/forms/{formCode}/submissions", handlers.GetFormSubmissions).Methods("GET")
	business.HandleFunc("/forms/{formCode}/submissions/{submissionId}", handlers.GetFormSubmission).Methods("GET")
	business.HandleFunc("/forms/{formCode}/submissions/{submissionId}", handlers.UpdateFormSubmission).Methods("PUT")
	business.HandleFunc("/forms/{formCode}/submissions/{submissionId}/transition", handlers.TransitionFormSubmission).Methods("POST")
	business.HandleFunc("/forms/{formCode}/submissions/{submissionId}/history", handlers.GetWorkflowHistory).Methods("GET")
	business.HandleFunc("/forms/{formCode}/stats", handlers.GetWorkflowStats).Methods("GET")

	// Dedicated table form submission endpoints (recommended)
	business.HandleFunc("/forms/{formCode}/submissions/dedicated", handlers.CreateFormSubmissionDedicated).Methods("POST")
	business.HandleFunc("/forms/{formCode}/submissions/dedicated", handlers.GetFormSubmissionsDedicated).Methods("GET")
	business.HandleFunc("/forms/{formCode}/submissions/dedicated/{submissionId}", handlers.GetFormSubmissionDedicated).Methods("GET")
	business.HandleFunc("/forms/{formCode}/submissions/dedicated/{submissionId}", handlers.UpdateFormSubmissionDedicated).Methods("PUT")
	business.HandleFunc("/forms/{formCode}/submissions/dedicated/{submissionId}/transition", handlers.TransitionFormSubmissionDedicated).Methods("POST")
	business.HandleFunc("/forms/{formCode}/submissions/dedicated/{submissionId}", handlers.DeleteFormSubmissionDedicated).Methods("DELETE")

	// Site management endpoints
	business.Handle("/sites",
		middleware.RequireBusinessPermission("site:view")(
			http.HandlerFunc(masters.GetBusinessSites))).Methods("GET")
	business.HandleFunc("/sites/my-access", masters.GetUserSites).Methods("GET")
	business.Handle("/sites/access",
		middleware.RequireBusinessPermission("site:manage_access")(
			http.HandlerFunc(masters.AssignUserSiteAccess))).Methods("POST")
	business.Handle("/sites/access/{accessId}",
		middleware.RequireBusinessPermission("site:manage_access")(
			http.HandlerFunc(masters.RevokeUserSiteAccess))).Methods("DELETE")
	business.Handle("/sites/{siteId}/users",
		middleware.RequireBusinessPermission("site:view")(
			http.HandlerFunc(masters.GetSiteUsers))).Methods("GET")

	// Solar Farm specific routes
	solar := business.PathPrefix("/solar").Subrouter()
	solar.Handle("/generation", middleware.RequireBusinessPermission("solar_read_generation")(
		http.HandlerFunc(handlers.GetSolarGeneration))).Methods("GET")
	solar.Handle("/panels", middleware.RequireBusinessPermission("solar_manage_panels")(
		http.HandlerFunc(handlers.GetSolarPanels))).Methods("GET")
	solar.Handle("/maintenance", middleware.RequireBusinessPermission("solar_maintenance")(
		http.HandlerFunc(handlers.GetSolarMaintenance))).Methods("GET")

	// Water Works specific routes
	water := business.PathPrefix("/water").Subrouter()
	// In business_routes.go - add water tanker reports routes
	// Water Tanker Reports
	water.Handle("", middleware.RequirePermission("read_reports")(
		http.HandlerFunc(handlers.GetAllWaterTankerReports))).Methods("GET")
	water.Handle("", middleware.RequireBusinessPermission("create:reports")(
		http.HandlerFunc(handlers.CreateWaterTankerReport))).Methods("POST")
	water.Handle("/{id}", middleware.RequirePermission("read_reports")(
		http.HandlerFunc(handlers.GetWaterTankerReport))).Methods("GET")
	water.Handle("/{id}", middleware.RequirePermission("update_reports")(
		http.HandlerFunc(handlers.UpdateWaterTankerReport))).Methods("PUT")
	water.Handle("/{id}", middleware.RequirePermission("delete_reports")(
		http.HandlerFunc(handlers.DeleteWaterTankerReport))).Methods("DELETE")
	water.Handle("/batch", middleware.RequirePermission("create_reports")(
		http.HandlerFunc(handlers.BatchWaterReports))).Methods("POST")

	water.Handle("/reports/tanker", middleware.RequireBusinessPermission("water:read_consumption")(
		http.HandlerFunc(handlers.GetAllWaterTankerReports))).Methods("GET")

	water.Handle("/reports/tanker", middleware.RequireBusinessPermission("inventory:create")(
		http.HandlerFunc(handlers.CreateWaterTankerReport))).Methods("POST")

	water.Handle("/reports/tanker/{id}", middleware.RequireBusinessPermission("inventory:update")(
		http.HandlerFunc(handlers.UpdateWaterTankerReport))).Methods("PUT")

	water.Handle("/reports/tanker/{id}", middleware.RequireBusinessPermission("inventory:delete")(
		http.HandlerFunc(handlers.DeleteWaterTankerReport))).Methods("DELETE")

	// Role assignment routes (requires authentication)
	api.HandleFunc("/users/{id}/roles/assign", handlers.AssignBusinessRole).Methods("POST")
	api.HandleFunc("/users/{id}/roles/{roleId}", handlers.RemoveBusinessRole).Methods("DELETE")
	api.HandleFunc("/users/{id}/roles", handlers.GetUserRoles).Methods("GET")
	api.HandleFunc("/users/{id}/assignable-roles", handlers.GetAssignableRoles).Methods("GET")
	api.HandleFunc("/business-verticals/{id}/roles", handlers.GetVerticalRoles).Methods("GET")
}
