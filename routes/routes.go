package routes

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	_ "p9e.in/ugcl/docs"
	"p9e.in/ugcl/handlers"
	kpi_handlers "p9e.in/ugcl/handlers/kpis"
	"p9e.in/ugcl/handlers/masters"
	"p9e.in/ugcl/middleware"
)

// RegisterRoutes sets up all application routes
func RegisterRoutes() http.Handler {
	r := mux.NewRouter()

	// =====================================================
	// Public Routes (no authentication)
	// =====================================================
	r.HandleFunc("/register", handlers.Register).Methods("POST")
	r.HandleFunc("/login", handlers.Login).Methods("POST")
	r.HandleFunc("/token", handlers.GetCurrentUser).Methods("GET")
	r.PathPrefix("/uploads/").Handler(
		http.StripPrefix("/uploads/", http.FileServer(http.Dir("./uploads"))),
	)

	// =====================================================
	// Protected API Routes (require JWT authentication)
	// =====================================================
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(middleware.SecurityMiddleware)
	api.Use(middleware.JWTMiddleware)

	// User profile endpoint
	api.HandleFunc("/profile", handleProfile).Methods("GET")

	// Register resource routes
	registerOperationalRoutes(api)
	registerKPIRoutes(api)
	registerFileRoutes(api)
	registerTestRoutes(api)

	// =====================================================
	// Admin Routes (require admin permissions)
	// =====================================================
	admin := api.PathPrefix("/admin").Subrouter()
	registerAdminRoutes(admin)

	// =====================================================
	// Partner API (read-only with API key)
	// =====================================================
	partner := r.PathPrefix("/api/v1/partner").Subrouter()
	partner.Use(middleware.SecurityMiddleware)
	registerPartnerRoutes(partner)

	// =====================================================
	// Feature-Specific Routes
	// =====================================================
	RegisterBusinessRoutes(r)
	RegisterABACRoutes(api)
	RegisterProjectRoutes(api)
	RegisterNotificationRoutes(api, admin)
	RegisterDocumentRoutes(api, admin)
	RegisterReportRoutes(r)
	RegisterChatRoutes(api)

	return r
}

// handleProfile returns user profile information
func handleProfile(w http.ResponseWriter, r *http.Request) {
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
}

// registerOperationalRoutes registers all operational data routes
func registerOperationalRoutes(api *mux.Router) {
	// DPR Site Reports
	registerCRUDRoutes(api, "/dprsite", "report", crudHandlers{
		getAll: handlers.GetAllSiteEngineerReports,
		create: handlers.CreateSiteEngineerReport,
		getOne: handlers.GetSiteEngineerReport,
		update: handlers.UpdateSiteEngineerReport,
		delete: handlers.DeleteSiteEngineerReport,
		batch:  handlers.BatchDprSites,
	})

	// Wrapping Reports
	registerCRUDRoutes(api, "/wrapping", "report", crudHandlers{
		getAll: handlers.GetAllWrappingReports,
		create: handlers.CreateWrappingReport,
		getOne: handlers.GetWrappingReport,
		update: handlers.UpdateWrappingReport,
		delete: handlers.DeleteWrappingReport,
		batch:  handlers.BatchWrappings,
	})

	// E-way Bills
	registerCRUDRoutes(api, "/eway", "report", crudHandlers{
		getAll: handlers.GetAllEways,
		create: handlers.CreateEway,
		getOne: handlers.GetEway,
		update: handlers.UpdateEway,
		delete: handlers.DeleteEway,
		batch:  handlers.BatchEwayss,
	})

	// Water Tanker Reports
	registerCRUDRoutes(api, "/water", "report", crudHandlers{
		getAll: handlers.GetAllWaterTankerReports,
		create: handlers.CreateWaterTankerReport,
		getOne: handlers.GetWaterTankerReport,
		update: handlers.UpdateWaterTankerReport,
		delete: handlers.DeleteWaterTankerReport,
		batch:  handlers.BatchWaterReports,
	})

	// Stock Reports
	registerCRUDRoutes(api, "/stock", "report", crudHandlers{
		getAll: handlers.GetAllStockReports,
		create: handlers.CreateStockReport,
		getOne: handlers.GetStockReport,
		update: handlers.UpdateStockReport,
		delete: handlers.DeleteStockReport,
		batch:  handlers.BatchStocks,
	})

	// Dairy Site Reports
	registerCRUDRoutes(api, "/dairysite", "report", crudHandlers{
		getAll: handlers.GetAllDairySiteReports,
		create: handlers.CreateDairySiteReport,
		getOne: handlers.GetDairySiteReport,
		update: handlers.UpdateDairySiteReport,
		delete: handlers.DeleteDairySiteReport,
		batch:  handlers.BatchDairySites,
	})

	// Payment Reports
	registerCRUDRoutes(api, "/payment", "payment", crudHandlers{
		getAll: handlers.GetAllPayments,
		create: handlers.CreatePayment,
		getOne: handlers.GetPayment,
		update: handlers.UpdatePayment,
		delete: handlers.DeletePayment,
		batch:  handlers.BatchPayments,
	})

	// Materials
	registerCRUDRoutes(api, "/material", "material", crudHandlers{
		getAll: handlers.GetAllMaterials,
		create: handlers.CreateMaterial,
		getOne: handlers.GetMaterial,
		update: handlers.UpdateMaterial,
		delete: handlers.DeleteMaterial,
		batch:  handlers.BatchMaterials,
	})

	// MNR Reports
	registerCRUDRoutes(api, "/mnr", "report", crudHandlers{
		getAll: handlers.GetAllMNRReports,
		create: handlers.CreateMNRReport,
		getOne: handlers.GetMNRReport,
		update: handlers.UpdateMNRReport,
		delete: handlers.DeleteMNRReport,
		batch:  handlers.BatchMnrs,
	})

	// NMR Vehicles
	registerCRUDRoutes(api, "/nmr_vehicle", "report", crudHandlers{
		getAll: handlers.GetAllNmrVehicle,
		create: handlers.CreateNmrVehicle,
		getOne: handlers.GetNmrVehicle,
		update: handlers.UpdateNmrVehicle,
		delete: handlers.DeleteNmrVehicle,
		batch:  handlers.BatchNmrVehicle,
	})

	// Contractors
	registerCRUDRoutes(api, "/contractor", "report", crudHandlers{
		getAll: handlers.GetAllContractorReports,
		create: handlers.CreateContractorReport,
		getOne: handlers.GetContractorReport,
		update: handlers.UpdateContractorReport,
		delete: handlers.DeleteContractorReport,
		batch:  handlers.BatchContractors,
	})

	// Painting Reports
	registerCRUDRoutes(api, "/painting", "report", crudHandlers{
		getAll: handlers.GetAllPaintingReports,
		create: handlers.CreatePaintingReport,
		getOne: handlers.GetPaintingReport,
		update: handlers.UpdatePaintingReport,
		delete: handlers.DeletePaintingReport,
		batch:  handlers.BatchPaintings,
	})

	// Diesel Reports
	registerCRUDRoutes(api, "/diesel", "report", crudHandlers{
		getAll: handlers.GetAllDieselReports,
		create: handlers.CreateDieselReport,
		getOne: handlers.GetDieselReport,
		update: handlers.UpdateDieselReport,
		delete: handlers.DeleteDieselReport,
		batch:  handlers.BatchDiesels,
	})

	// Tasks
	registerCRUDRoutes(api, "/tasks", "report", crudHandlers{
		getAll: handlers.GetAllTasks,
		create: handlers.CreateTask,
		getOne: handlers.GetTask,
		update: handlers.UpdateTask,
		delete: handlers.DeleteTask,
		batch:  handlers.BatchTasks,
	})

	// Vehicle Logs
	registerCRUDRoutes(api, "/vehiclelog", "report", crudHandlers{
		getAll: handlers.GetAllVehicleLogs,
		create: handlers.CreateVehicleLog,
		getOne: handlers.GetVehicleLog,
		update: handlers.UpdateVehicleLog,
		delete: handlers.DeleteVehicleLog,
		batch:  handlers.BatchVehicleLogs,
	})
}

// crudHandlers holds handlers for a CRUD resource
type crudHandlers struct {
	getAll func(http.ResponseWriter, *http.Request)
	create func(http.ResponseWriter, *http.Request)
	getOne func(http.ResponseWriter, *http.Request)
	update func(http.ResponseWriter, *http.Request)
	delete func(http.ResponseWriter, *http.Request)
	batch  func(http.ResponseWriter, *http.Request)
}

// registerCRUDRoutes registers standard CRUD routes for a resource
func registerCRUDRoutes(router *mux.Router, path string, resourceType string, h crudHandlers) {
	readPerm := "read_" + resourceType + "s"
	createPerm := "create_" + resourceType + "s"
	updatePerm := "update_" + resourceType + "s"
	deletePerm := "delete_" + resourceType + "s"

	// GET all
	router.Handle(path, middleware.RequirePermission(readPerm)(
		http.HandlerFunc(h.getAll))).Methods("GET")

	// POST create
	router.Handle(path, middleware.RequirePermission(createPerm)(
		http.HandlerFunc(h.create))).Methods("POST")

	// GET one by ID
	router.Handle(path+"/{id}", middleware.RequirePermission(readPerm)(
		http.HandlerFunc(h.getOne))).Methods("GET")

	// PUT update
	router.Handle(path+"/{id}", middleware.RequirePermission(updatePerm)(
		http.HandlerFunc(h.update))).Methods("PUT")

	// DELETE
	router.Handle(path+"/{id}", middleware.RequirePermission(deletePerm)(
		http.HandlerFunc(h.delete))).Methods("DELETE")

	// POST batch
	if h.batch != nil {
		router.Handle(path+"/batch", middleware.RequirePermission(createPerm)(
			http.HandlerFunc(h.batch))).Methods("POST")
	}
}

// registerKPIRoutes registers KPI endpoints
func registerKPIRoutes(api *mux.Router) {
	api.Handle("/kpi/stock", middleware.RequirePermission("read_kpis")(
		http.HandlerFunc(kpi_handlers.GetStockKPIs))).Methods("GET")
	api.Handle("/kpi/contractor", middleware.RequirePermission("read_kpis")(
		http.HandlerFunc(kpi_handlers.GetContractorKPIs))).Methods("GET")
	api.Handle("/kpi/dairysite", middleware.RequirePermission("read_kpis")(
		http.HandlerFunc(kpi_handlers.GetDairyKPIs))).Methods("GET")
	api.Handle("/kpi/diesel", middleware.RequirePermission("read_kpis")(
		http.HandlerFunc(kpi_handlers.GetDieselKPIs))).Methods("GET")
}

// registerFileRoutes registers file upload endpoints
func registerFileRoutes(api *mux.Router) {
	api.Handle("/files/upload", middleware.RequireAnyPermission([]string{"create_reports", "create_materials"})(
		http.HandlerFunc(handlers.UploadFileHandler))).Methods("POST")
}

// registerTestRoutes registers testing endpoints
func registerTestRoutes(api *mux.Router) {
	api.HandleFunc("/test/auth", handlers.TestAuthEndpoint).Methods("GET")
	api.HandleFunc("/test/permission", handlers.TestPermissionEndpoint).Methods("GET")

	// Password management
	api.Handle("/change-password", middleware.JWTMiddleware(
		http.HandlerFunc(handlers.ChangePassword))).Methods("POST")
}

// registerAdminRoutes registers admin-only routes
func registerAdminRoutes(admin *mux.Router) {
	projectHandler := handlers.NewProjectHandler()

	// Module management
	admin.Handle("/masters/modules", middleware.RequirePermission("masters:module:create")(
		http.HandlerFunc(masters.CreateModule))).Methods("POST")

	// User management
	admin.Handle("/users", middleware.RequirePermission("read_users")(
		http.HandlerFunc(handlers.GetAllUsers))).Methods("GET")
	admin.Handle("/users/{id}", middleware.RequirePermission("read_users")(
		http.HandlerFunc(handlers.GetbyID))).Methods("GET")
	admin.Handle("/users", middleware.RequirePermission("create_users")(
		http.HandlerFunc(handlers.Register))).Methods("POST")
	admin.Handle("/users/{id}", middleware.RequirePermission("update_users")(
		http.HandlerFunc(handlers.UpdateUser))).Methods("PUT")
	admin.Handle("/users/{id}", middleware.RequirePermission("delete_users")(
		http.HandlerFunc(handlers.DeleteUser))).Methods("DELETE")

	// Project creation (admin)
	admin.Handle("/projects", middleware.RequirePermission("project:create")(
		http.HandlerFunc(projectHandler.CreateProject))).Methods("POST")

	// Role and Permission management
	admin.Handle("/roles", middleware.RequirePermission("manage_roles")(
		http.HandlerFunc(handlers.GetAllRoles))).Methods("GET")
	admin.Handle("/roles/unified", middleware.RequirePermission("manage_roles")(
		http.HandlerFunc(handlers.GetAllRolesUnified))).Methods("GET")
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
}

// registerPartnerRoutes registers partner API routes (read-only)
func registerPartnerRoutes(partner *mux.Router) {
	// Read-only endpoints for partners
	partnerResources := []struct {
		path   string
		getAll func(http.ResponseWriter, *http.Request)
		getOne func(http.ResponseWriter, *http.Request)
	}{
		{"/dprsite", handlers.GetAllSiteEngineerReports, handlers.GetSiteEngineerReport},
		{"/wrapping", handlers.GetAllWrappingReports, handlers.GetWrappingReport},
		{"/eway", handlers.GetAllEways, handlers.GetEway},
		{"/water", handlers.GetAllWaterTankerReports, handlers.GetWaterTankerReport},
		{"/stock", handlers.GetAllStockReports, handlers.GetStockReport},
		{"/dairysite", handlers.GetAllDairySiteReports, handlers.GetDairySiteReport},
		{"/payment", handlers.GetAllPayments, handlers.GetPayment},
		{"/material", handlers.GetAllMaterials, handlers.GetMaterial},
		{"/mnr", handlers.GetAllMNRReports, handlers.GetMNRReport},
		{"/nmr_vehicle", handlers.GetAllNmrVehicle, handlers.GetNmrVehicle},
		{"/contractor", handlers.GetAllContractorReports, handlers.GetContractorReport},
		{"/painting", handlers.GetAllPaintingReports, handlers.GetPaintingReport},
		{"/diesel", handlers.GetAllDieselReports, handlers.GetDieselReport},
		{"/tasks", handlers.GetAllTasks, handlers.GetTask},
		{"/vehiclelog", handlers.GetAllVehicleLogs, handlers.GetVehicleLog},
	}

	for _, res := range partnerResources {
		partner.HandleFunc(res.path, res.getAll).Methods("GET")
		partner.HandleFunc(res.path+"/{id}", res.getOne).Methods("GET")
	}
}
