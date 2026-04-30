package routes

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"p9e.in/ugcl/handlers"
	biz "p9e.in/ugcl/handlers/business"
	"p9e.in/ugcl/handlers/masters"
	"p9e.in/ugcl/middleware"
)

// RegisterBusinessRoutes adds business vertical specific routes
func RegisterBusinessRoutes(r *mux.Router) {
	// =====================================================
	// Global Admin Routes (Super Admin only)
	// =====================================================
	admin := r.PathPrefix("/api/v1/admin").Subrouter()
	admin.Use(middleware.SecurityMiddleware)
	admin.Use(middleware.JWTMiddleware)

	registerGlobalAdminRoutes(admin)

	// =====================================================
	// User's Business Access (any authenticated user)
	// =====================================================
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(middleware.SecurityMiddleware)
	api.Use(middleware.JWTMiddleware)

	api.HandleFunc("/my-businesses", biz.GetUserBusinessAccess).Methods("GET")
	api.HandleFunc("/modules", masters.GetModules).Methods("GET")

	// Role assignment routes
	api.HandleFunc("/users/{id}/roles/assign", handlers.AssignBusinessRole).Methods("POST")
	api.HandleFunc("/users/{id}/roles/{roleId}", handlers.RemoveBusinessRole).Methods("DELETE")
	api.HandleFunc("/users/{id}/roles", handlers.GetUserRoles).Methods("GET")
	api.HandleFunc("/users/{id}/assignable-roles", handlers.GetAssignableRoles).Methods("GET")
	api.HandleFunc("/business-verticals/{id}/roles", handlers.GetVerticalRoles).Methods("GET")

	// =====================================================
	// Business-Specific Routes
	// =====================================================
	business := r.PathPrefix("/api/v1/business/{businessCode}").Subrouter()
	business.Use(middleware.SecurityMiddleware)
	business.Use(middleware.JWTMiddleware)
	business.Use(middleware.RequireBusinessAccess())

	registerBusinessRoleRoutes(business)
	registerBusinessReportRoutes(business)
	registerBusinessFormRoutes(business)
	registerBusinessSiteRoutes(business)
	registerBusinessIntegrationRoutes(business)
	registerBusinessAttendanceRoutes(business)
	registerSolarRoutes(business)
	registerWaterRoutes(business)
}

// registerGlobalAdminRoutes registers admin-level business management routes
func registerGlobalAdminRoutes(admin *mux.Router) {
	// Business vertical management
	admin.Handle("/businesses", middleware.RequirePermission("manage_businesses")(
		http.HandlerFunc(biz.GetAllBusinessVerticals))).Methods("GET")
	admin.Handle("/businesses", middleware.RequirePermission("manage_businesses")(
		http.HandlerFunc(biz.CreateBusinessVertical))).Methods("POST")
	admin.Handle("/businesses/{id}", middleware.RequirePermission("manage_businesses")(
		http.HandlerFunc(biz.UpdateBusinessVertical))).Methods("PUT")
	admin.Handle("/businesses/{id}", middleware.RequirePermission("manage_businesses")(
		http.HandlerFunc(biz.DeleteBusinessVertical))).Methods("DELETE")

	// Super admin dashboard
	admin.Handle("/dashboard", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(biz.GetSuperAdminDashboard))).Methods("GET")

	// Site management (all sites across all business verticals)
	admin.Handle("/sites", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(masters.GetAllSites))).Methods("GET")
	admin.Handle("/sites", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(masters.CreateSite))).Methods("POST")
	admin.Handle("/sites/{siteId}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(masters.GetSiteByID))).Methods("GET")
	admin.Handle("/sites/{siteId}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(masters.UpdateSite))).Methods("PUT")

	// App Form management
	admin.Handle("/app-forms", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.GetAllAppForms))).Methods("GET")
	admin.Handle("/app-forms", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.CreateForm))).Methods("POST")
	// More specific routes FIRST (with path segments)
	admin.Handle("/app-forms/{formCode}/status", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.ToggleFormStatus))).Methods("PATCH")
	admin.Handle("/app-forms/{formCode}/verticals", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.UpdateFormVerticalAccess))).Methods("POST")
	// General form routes LAST
	admin.Handle("/app-forms/{formCode}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.UpdateForm))).Methods("PUT")
	admin.Handle("/app-forms/{formCode}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.DeleteForm))).Methods("DELETE")

	// Workflow management
	admin.Handle("/workflows", middleware.RequirePermission("super_admin")(
		http.HandlerFunc(handlers.GetAllWorkflows))).Methods("GET")
	admin.Handle("/workflows", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.CreateWorkflowDefinition))).Methods("POST")
	admin.Handle("/workflows/{workflowId}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.UpdateWorkflowDefinition))).Methods("PUT")
	admin.Handle("/workflows/{workflowId}", middleware.RequirePermission("admin_all")(
		http.HandlerFunc(handlers.DeleteWorkflowDefinition))).Methods("DELETE")

	// Form configuration endpoints
	admin.HandleFunc("/forms", handlers.GetFormsForVertical).Methods("GET")
	admin.HandleFunc("/forms/{code}", handlers.GetFormByCode).Methods("GET")
}

// registerBusinessRoleRoutes registers role management routes for a business
func registerBusinessRoleRoutes(business *mux.Router) {
	// Business role management
	business.Handle("/roles", middleware.RequireBusinessPermission("business_manage_roles")(
		http.HandlerFunc(biz.GetBusinessRoles))).Methods("GET")
	business.Handle("/roles", middleware.RequireBusinessPermission("business_manage_roles")(
		http.HandlerFunc(biz.CreateBusinessRole))).Methods("POST")
	business.Handle("/roles/{roleId}", middleware.RequireBusinessPermission("business_manage_roles")(
		http.HandlerFunc(biz.UpdateBusinessRole))).Methods("PUT")
	business.Handle("/roles/{roleId}", middleware.RequireBusinessPermission("business_manage_roles")(
		http.HandlerFunc(biz.DeleteBusinessRole))).Methods("DELETE")

	// Business user management
	business.Handle("/users", middleware.RequireBusinessPermission("business_manage_users")(
		http.HandlerFunc(biz.GetBusinessUsers))).Methods("GET")
	business.Handle("/users/assign", middleware.RequireBusinessPermission("business_manage_users")(
		http.HandlerFunc(biz.AssignUserToBusinessRole))).Methods("POST")
}

// registerBusinessReportRoutes registers business-specific report routes
func registerBusinessReportRoutes(business *mux.Router) {
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
	business.HandleFunc("/info", biz.GetBusinessInfo).Methods("GET")
	business.HandleFunc("/context", func(w http.ResponseWriter, r *http.Request) {
		context := middleware.GetUserBusinessContext(r)
		if context == nil {
			http.Error(w, "unable to get business context", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(context)
	}).Methods("GET")
}

// registerBusinessFormRoutes registers form submission routes
func registerBusinessFormRoutes(business *mux.Router) {
	// Form configuration
	business.HandleFunc("/forms", handlers.GetFormsForVertical).Methods("GET")
	business.HandleFunc("/forms/{code}", handlers.GetFormByCode).Methods("GET")

	// Generic table form submissions
	business.HandleFunc("/forms/{formCode}/submissions", handlers.CreateFormSubmission).Methods("POST")
	business.HandleFunc("/forms/{formCode}/submissions", handlers.GetFormSubmissions).Methods("GET")
	business.HandleFunc("/forms/{formCode}/submissions/{submissionId}", handlers.GetFormSubmission).Methods("GET")
	business.HandleFunc("/forms/{formCode}/submissions/{submissionId}/resolved", handlers.GetResolvedFormSubmission).Methods("GET")
	business.HandleFunc("/forms/{formCode}/submissions/{submissionId}", handlers.UpdateFormSubmission).Methods("PUT")
	business.HandleFunc("/forms/{formCode}/submissions/{submissionId}/transition", handlers.TransitionFormSubmission).Methods("POST")
	business.HandleFunc("/forms/{formCode}/submissions/{submissionId}/history", handlers.GetWorkflowHistory).Methods("GET")
	business.HandleFunc("/forms/{formCode}/stats", handlers.GetWorkflowStats).Methods("GET")

	// Dedicated table form submissions (recommended)
	business.HandleFunc("/forms/{formCode}/submissions/dedicated", handlers.CreateFormSubmissionDedicated).Methods("POST")
	business.HandleFunc("/forms/{formCode}/submissions/dedicated", handlers.GetFormSubmissionsDedicated).Methods("GET")
	business.HandleFunc("/forms/{formCode}/submissions/dedicated/{submissionId}", handlers.GetFormSubmissionDedicated).Methods("GET")
	business.HandleFunc("/forms/{formCode}/submissions/dedicated/{submissionId}", handlers.UpdateFormSubmissionDedicated).Methods("PUT")
	business.HandleFunc("/forms/{formCode}/submissions/dedicated/{submissionId}/transition", handlers.TransitionFormSubmissionDedicated).Methods("POST")
	business.HandleFunc("/forms/{formCode}/submissions/dedicated/{submissionId}", handlers.DeleteFormSubmissionDedicated).Methods("DELETE")
}

// registerBusinessSiteRoutes registers site management routes
func registerBusinessSiteRoutes(business *mux.Router) {
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
	business.Handle("/sites/user/{userId}/access",
		middleware.RequireBusinessPermission("site:view")(
			http.HandlerFunc(masters.GetUserSiteAccessByUserID))).Methods("GET")
}

func registerBusinessIntegrationRoutes(business *mux.Router) {
	business.Handle("/integrations/vendor/sites",
		middleware.RequireBusinessPermission("site:view")(
			http.HandlerFunc(handlers.IntegrationVendorSitesDropdown))).Methods("GET")
	business.Handle("/integrations/external-dropdown",
		middleware.RequireBusinessPermission("site:view")(
			http.HandlerFunc(handlers.IntegrationExternalDropdownProxy))).Methods("GET")
}

func registerBusinessAttendanceRoutes(business *mux.Router) {
	business.Handle("/attendance/check-in",
		middleware.RequireBusinessPermission("attendance:checkin")(
			http.HandlerFunc(handlers.CheckInAttendance))).Methods("POST")
	business.Handle("/attendance/heartbeat",
		middleware.RequireBusinessPermission("attendance:heartbeat")(
			http.HandlerFunc(handlers.HeartbeatAttendance))).Methods("POST")
	business.Handle("/attendance/check-out",
		middleware.RequireBusinessPermission("attendance:checkout")(
			http.HandlerFunc(handlers.CheckOutAttendance))).Methods("POST")
	business.Handle("/attendance/active",
		middleware.RequireBusinessPermission("attendance:read")(
			http.HandlerFunc(handlers.GetActiveAttendanceSessions))).Methods("GET")
	business.Handle("/attendance/logs",
		middleware.RequireBusinessPermission("attendance:read")(
			http.HandlerFunc(handlers.GetAttendanceLogs))).Methods("GET")
	business.Handle("/attendance/headcount",
		middleware.RequireBusinessPermission("attendance:headcount")(
			http.HandlerFunc(handlers.GetAttendanceHeadcount))).Methods("GET")
	business.Handle("/attendance/users/{userId}/timeline",
		middleware.RequireBusinessPermission("attendance:read")(
			http.HandlerFunc(handlers.GetEmployeeAttendanceTimeline))).Methods("GET")
}

// registerSolarRoutes registers Solar Farm specific routes
func registerSolarRoutes(business *mux.Router) {
	solar := business.PathPrefix("/solar").Subrouter()

	solar.Handle("/generation", middleware.RequireBusinessPermission("solar_read_generation")(
		http.HandlerFunc(handlers.GetSolarGeneration))).Methods("GET")
	solar.Handle("/panels", middleware.RequireBusinessPermission("solar_manage_panels")(
		http.HandlerFunc(handlers.GetSolarPanels))).Methods("GET")
	solar.Handle("/maintenance", middleware.RequireBusinessPermission("solar_maintenance")(
		http.HandlerFunc(handlers.GetSolarMaintenance))).Methods("GET")
}

// registerWaterRoutes registers Water Works specific routes
func registerWaterRoutes(business *mux.Router) {
	water := business.PathPrefix("/water").Subrouter()

	// Water Tanker Reports (business-scoped)
	water.Handle("/reports/tanker", middleware.RequireBusinessPermission("water:read_consumption")(
		http.HandlerFunc(handlers.GetAllWaterTankerReports))).Methods("GET")
	water.Handle("/reports/tanker", middleware.RequireBusinessPermission("inventory:create")(
		http.HandlerFunc(handlers.CreateWaterTankerReport))).Methods("POST")
	water.Handle("/reports/tanker/{id}", middleware.RequireBusinessPermission("inventory:update")(
		http.HandlerFunc(handlers.UpdateWaterTankerReport))).Methods("PUT")
	water.Handle("/reports/tanker/{id}", middleware.RequireBusinessPermission("inventory:delete")(
		http.HandlerFunc(handlers.DeleteWaterTankerReport))).Methods("DELETE")
}
