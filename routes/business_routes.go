package routes

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"p9e.in/ugcl/handlers"
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

	// User's accessible businesses (any authenticated user)
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(middleware.SecurityMiddleware)
	api.Use(middleware.JWTMiddleware)
	api.HandleFunc("/my-businesses", handlers.GetUserBusinessAccess).Methods("GET")

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
	water.Handle("/consumption", middleware.RequireBusinessPermission("water:read_consumption")(
		http.HandlerFunc(handlers.GetWaterConsumption))).Methods("GET")
	water.Handle("/supply", middleware.RequireBusinessPermission("water:manage_supply")(
		http.HandlerFunc(handlers.GetWaterSupply))).Methods("GET")
	water.Handle("/quality", middleware.RequireBusinessPermission("water:quality_control")(
		http.HandlerFunc(handlers.GetWaterQuality))).Methods("GET")

	// In business_routes.go - add water tanker reports routes
	// Water Tanker Reports
	water.Handle("/water", middleware.RequirePermission("read_reports")(
		http.HandlerFunc(handlers.GetAllWaterTankerReports))).Methods("GET")
	water.Handle("/water", middleware.RequirePermission("create_reports")(
		http.HandlerFunc(handlers.CreateWaterTankerReport))).Methods("POST")
	water.Handle("/water/{id}", middleware.RequirePermission("read_reports")(
		http.HandlerFunc(handlers.GetWaterTankerReport))).Methods("GET")
	water.Handle("/water/{id}", middleware.RequirePermission("update_reports")(
		http.HandlerFunc(handlers.UpdateWaterTankerReport))).Methods("PUT")
	water.Handle("/water/{id}", middleware.RequirePermission("delete_reports")(
		http.HandlerFunc(handlers.DeleteWaterTankerReport))).Methods("DELETE")
	water.Handle("/water/batch", middleware.RequirePermission("create_reports")(
		http.HandlerFunc(handlers.BatchWaterReports))).Methods("POST")

	water.Handle("/reports/tanker", middleware.RequireBusinessPermission("water:read_consumption")(
		http.HandlerFunc(handlers.GetAllWaterTankerReports))).Methods("GET")

	water.Handle("/reports/tanker", middleware.RequireBusinessPermission("inventory:create")(
		http.HandlerFunc(handlers.CreateWaterTankerReport))).Methods("POST")

	water.Handle("/reports/tanker/{id}", middleware.RequireBusinessPermission("inventory:update")(
		http.HandlerFunc(handlers.UpdateWaterTankerReport))).Methods("PUT")

	water.Handle("/reports/tanker/{id}", middleware.RequireBusinessPermission("inventory:delete")(
		http.HandlerFunc(handlers.DeleteWaterTankerReport))).Methods("DELETE")
}
