package routes

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	_ "p9e.in/ugcl/docs"
	"p9e.in/ugcl/handlers"
	kpi_handlers "p9e.in/ugcl/handlers/kpis"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/handlers/masters"
)

// RegisterRoutesV2 uses the new permission-based authorization system
func RegisterRoutesV2() http.Handler {
	r := mux.NewRouter()

	// Public routes (no authentication required)
	r.HandleFunc("/api/v1/register", handlers.Register).Methods("POST")
	r.HandleFunc("/api/v1/login", handlers.Login).Methods("POST")
	r.HandleFunc("/api/v1/token", handlers.GetCurrentUser).Methods("GET")
	r.PathPrefix("/uploads/").Handler(
		http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))),
	)

	// Protected API routes (require authentication)
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(middleware.SecurityMiddleware)
	api.Use(middleware.JWTMiddleware)

	// User profile (any authenticated user)
	api.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		claims := middleware.GetClaims(r)
		user := middleware.GetUser(r)
		permissions := middleware.GetUserPermissions(r)

		var globalRoleName string
		if user.RoleModel != nil {
			globalRoleName = user.RoleModel.Name
		}

		response := map[string]interface{}{
			"userID":      claims.UserID,
			"name":        user.Name,
			"phone":       user.Phone,
			"role_id":     user.RoleID,
			"global_role": globalRoleName,
			"permissions": permissions,
		}
		json.NewEncoder(w).Encode(response)
	}).Methods("GET")

	// DPR Site Reports
	api.Handle("/dprsite", middleware.RequirePermission("read_reports")(
		http.HandlerFunc(handlers.GetAllSiteEngineerReports))).Methods("GET")
	api.Handle("/dprsite", middleware.RequirePermission("create_reports")(
		http.HandlerFunc(handlers.CreateSiteEngineerReport))).Methods("POST")
	api.Handle("/dprsite/{id}", middleware.RequirePermission("read_reports")(
		http.HandlerFunc(handlers.GetSiteEngineerReport))).Methods("GET")
	api.Handle("/dprsite/{id}", middleware.RequirePermission("update_reports")(
		http.HandlerFunc(handlers.UpdateSiteEngineerReport))).Methods("PUT")
	api.Handle("/dprsite/{id}", middleware.RequirePermission("delete_reports")(
		http.HandlerFunc(handlers.DeleteSiteEngineerReport))).Methods("DELETE")
	api.Handle("/dprsite/batch", middleware.RequirePermission("create_reports")(
		http.HandlerFunc(handlers.BatchDprSites))).Methods("POST")

	// Wrapping Reports
	api.Handle("/wrapping", middleware.RequirePermission("read_reports")(
		http.HandlerFunc(handlers.GetAllWrappingReports))).Methods("GET")
	api.Handle("/wrapping", middleware.RequirePermission("create_reports")(
		http.HandlerFunc(handlers.CreateWrappingReport))).Methods("POST")
	api.Handle("/wrapping/{id}", middleware.RequirePermission("read_reports")(
		http.HandlerFunc(handlers.GetWrappingReport))).Methods("GET")
	api.Handle("/wrapping/{id}", middleware.RequirePermission("update_reports")(
		http.HandlerFunc(handlers.UpdateWrappingReport))).Methods("PUT")
	api.Handle("/wrapping/{id}", middleware.RequirePermission("delete_reports")(
		http.HandlerFunc(handlers.DeleteWrappingReport))).Methods("DELETE")
	api.Handle("/wrapping/batch", middleware.RequirePermission("create_reports")(
		http.HandlerFunc(handlers.BatchWrappings))).Methods("POST")

	// E-way Bills
	api.Handle("/eway", middleware.RequirePermission("read_reports")(
		http.HandlerFunc(handlers.GetAllEways))).Methods("GET")
	api.Handle("/eway", middleware.RequirePermission("create_reports")(
		http.HandlerFunc(handlers.CreateEway))).Methods("POST")
	api.Handle("/eway/{id}", middleware.RequirePermission("read_reports")(
		http.HandlerFunc(handlers.GetEway))).Methods("GET")
	api.Handle("/eway/{id}", middleware.RequirePermission("update_reports")(
		http.HandlerFunc(handlers.UpdateEway))).Methods("PUT")
	api.Handle("/eway/{id}", middleware.RequirePermission("delete_reports")(
		http.HandlerFunc(handlers.DeleteEway))).Methods("DELETE")
	api.Handle("/eway/batch", middleware.RequirePermission("create_reports")(
		http.HandlerFunc(handlers.BatchEwayss))).Methods("POST")

	// Water Tanker Reports
	api.Handle("/water", middleware.RequirePermission("read_reports")(
		http.HandlerFunc(handlers.GetAllWaterTankerReports))).Methods("GET")
	api.Handle("/water", middleware.RequirePermission("create_reports")(
		http.HandlerFunc(handlers.CreateWaterTankerReport))).Methods("POST")
	api.Handle("/water/{id}", middleware.RequirePermission("read_reports")(
		http.HandlerFunc(handlers.GetWaterTankerReport))).Methods("GET")
	api.Handle("/water/{id}", middleware.RequirePermission("update_reports")(
		http.HandlerFunc(handlers.UpdateWaterTankerReport))).Methods("PUT")
	api.Handle("/water/{id}", middleware.RequirePermission("delete_reports")(
		http.HandlerFunc(handlers.DeleteWaterTankerReport))).Methods("DELETE")
	api.Handle("/water/batch", middleware.RequirePermission("create_reports")(
		http.HandlerFunc(handlers.BatchWaterReports))).Methods("POST")

	// Materials
	api.Handle("/material", middleware.RequirePermission("read_materials")(
		http.HandlerFunc(handlers.GetAllMaterials))).Methods("GET")
	api.Handle("/material", middleware.RequirePermission("create_materials")(
		http.HandlerFunc(handlers.CreateMaterial))).Methods("POST")
	api.Handle("/material/{id}", middleware.RequirePermission("read_materials")(
		http.HandlerFunc(handlers.GetMaterial))).Methods("GET")
	api.Handle("/material/{id}", middleware.RequirePermission("update_materials")(
		http.HandlerFunc(handlers.UpdateMaterial))).Methods("PUT")
	api.Handle("/material/{id}", middleware.RequirePermission("delete_materials")(
		http.HandlerFunc(handlers.DeleteMaterial))).Methods("DELETE")
	api.Handle("/material/batch", middleware.RequirePermission("create_materials")(
		http.HandlerFunc(handlers.BatchMaterials))).Methods("POST")

	// Payments
	api.Handle("/payment", middleware.RequirePermission("read_payments")(
		http.HandlerFunc(handlers.GetAllPayments))).Methods("GET")
	api.Handle("/payment", middleware.RequirePermission("create_payments")(
		http.HandlerFunc(handlers.CreatePayment))).Methods("POST")
	api.Handle("/payment/{id}", middleware.RequirePermission("read_payments")(
		http.HandlerFunc(handlers.GetPayment))).Methods("GET")
	api.Handle("/payment/{id}", middleware.RequirePermission("update_payments")(
		http.HandlerFunc(handlers.UpdatePayment))).Methods("PUT")
	api.Handle("/payment/{id}", middleware.RequirePermission("delete_payments")(
		http.HandlerFunc(handlers.DeletePayment))).Methods("DELETE")
	api.Handle("/payment/batch", middleware.RequirePermission("create_payments")(
		http.HandlerFunc(handlers.BatchPayments))).Methods("POST")

	// KPIs
	api.Handle("/kpi/stock", middleware.RequirePermission("read_kpis")(
		http.HandlerFunc(kpi_handlers.GetStockKPIs))).Methods("GET")
	api.Handle("/kpi/contractor", middleware.RequirePermission("read_kpis")(
		http.HandlerFunc(kpi_handlers.GetContractorKPIs))).Methods("GET")
	api.Handle("/kpi/dairysite", middleware.RequirePermission("read_kpis")(
		http.HandlerFunc(kpi_handlers.GetDairyKPIs))).Methods("GET")
	api.Handle("/kpi/diesel", middleware.RequirePermission("read_kpis")(
		http.HandlerFunc(kpi_handlers.GetDieselKPIs))).Methods("GET")

	// File uploads (auto-detects GCS for production, local storage for dev)
	api.Handle("/files/upload", middleware.RequireAnyPermission([]string{"create_reports", "create_materials"})(
		http.HandlerFunc(handlers.UploadFileHandler))).Methods("POST")

	// Admin routes (require specific admin permissions)
	admin := api.PathPrefix("/admin").Subrouter()

	admin.Handle("/masters/modules", middleware.RequirePermission("masters:module:create")(
		http.HandlerFunc(masters.CreateModule))).Methods("POST")
	
	// User management
	admin.Handle("/users", middleware.RequirePermission("read_users")(
		http.HandlerFunc(handlers.GetAllUsers))).Methods("GET")
	admin.Handle("/users", middleware.RequirePermission("create_users")(
		http.HandlerFunc(handlers.Register))).Methods("POST")
	admin.Handle("/users/{id}", middleware.RequirePermission("update_users")(
		http.HandlerFunc(handlers.UpdateUser))).Methods("PUT")
	admin.Handle("/users/{id}", middleware.RequirePermission("delete_users")(
		http.HandlerFunc(handlers.DeleteUser))).Methods("DELETE")

	// Role and Permission management
	admin.Handle("/roles", middleware.RequirePermission("manage_roles")(
		http.HandlerFunc(handlers.GetAllRoles))).Methods("GET")
	admin.Handle("/roles", middleware.RequirePermission("manage_roles")(
		http.HandlerFunc(handlers.CreateRole))).Methods("POST")
	admin.Handle("/roles/{id}", middleware.RequirePermission("manage_roles")(
		http.HandlerFunc(handlers.UpdateRole))).Methods("PUT")
	admin.Handle("/roles/{id}", middleware.RequirePermission("manage_roles")(
		http.HandlerFunc(handlers.DeleteRole))).Methods("DELETE")
	admin.Handle("/permissions", middleware.RequirePermission("manage_roles")(
		http.HandlerFunc(handlers.GetAllPermissions))).Methods("GET")
	admin.Handle("/permissions", middleware.RequirePermission("manage_roles")(
		http.HandlerFunc(handlers.CreatePermission))).Methods("POST")

	// Password management
	api.Handle("/change-password", middleware.JWTMiddleware(
		http.HandlerFunc(handlers.ChangePassword))).Methods("POST")

	// Testing endpoints
	api.HandleFunc("/test/auth", handlers.TestAuthEndpoint).Methods("GET")
	api.HandleFunc("/test/permission", handlers.TestPermissionEndpoint).Methods("GET")

	// Partner API (read-only access with API key)
	partner := r.PathPrefix("/api/v1/partner").Subrouter()
	partner.Use(middleware.SecurityMiddleware) // API key + IP validation only

	// Partner routes (read-only)
	partner.HandleFunc("/dprsite", handlers.GetAllSiteEngineerReports).Methods("GET")
	partner.HandleFunc("/dprsite/{id}", handlers.GetSiteEngineerReport).Methods("GET")
	partner.HandleFunc("/wrapping", handlers.GetAllWrappingReports).Methods("GET")
	partner.HandleFunc("/wrapping/{id}", handlers.GetWrappingReport).Methods("GET")
	partner.HandleFunc("/eway", handlers.GetAllEways).Methods("GET")
	partner.HandleFunc("/eway/{id}", handlers.GetEway).Methods("GET")
	partner.HandleFunc("/water", handlers.GetAllWaterTankerReports).Methods("GET")
	partner.HandleFunc("/water/{id}", handlers.GetWaterTankerReport).Methods("GET")
	partner.HandleFunc("/material", handlers.GetAllMaterials).Methods("GET")
	partner.HandleFunc("/material/{id}", handlers.GetMaterial).Methods("GET")
	partner.HandleFunc("/payment", handlers.GetAllPayments).Methods("GET")
	partner.HandleFunc("/payment/{id}", handlers.GetPayment).Methods("GET")

	// Register business vertical routes
	RegisterBusinessRoutes(r)

	return r
}
