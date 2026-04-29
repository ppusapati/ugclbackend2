package routes

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"p9e.in/ugcl/config"
	_ "p9e.in/ugcl/docs"
	"p9e.in/ugcl/handlers"
	kpi_handlers "p9e.in/ugcl/handlers/kpis"
	"p9e.in/ugcl/handlers/masters"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// RegisterRoutes sets up all application routes
func RegisterRoutes() http.Handler {
	r := mux.NewRouter()
	r.Use(middleware.RequestObservabilityMiddleware)

	// =====================================================
	// Public Routes (no authentication)
	// =====================================================
	r.HandleFunc("/register", handlers.Register).Methods("POST")
	r.HandleFunc("/api/v1/login", handlers.Login).Methods("POST")
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
	api.HandleFunc("/profile", handleUpdateProfile).Methods("PUT")
	api.HandleFunc("/token", handlers.GetCurrentUser).Methods("GET")
	api.HandleFunc("/context/business", handlers.GetActiveBusinessContext).Methods("GET")
	api.HandleFunc("/context/business", handlers.SetActiveBusinessContext).Methods("PUT")

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
	RegisterWebhookMuxRoutes(r)
	RegisterIntegrationRoutes(r)
	RegisterAdminIntegrationRoutes(admin)

	return r
}

// handleProfile returns user profile information
func handleProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	authService := middleware.NewAuthService()
	userCtx, err := authService.LoadUserContext(r)
	if err != nil {
		http.Error(w, "failed to load profile", http.StatusUnauthorized)
		return
	}

	user := userCtx.User
	permissions := middleware.GetEffectivePermissions(r)

	var globalRoleName string
	if user.RoleModel != nil {
		globalRoleName = user.RoleModel.Name
	}

	businessRoles := make([]map[string]interface{}, 0)
	for _, ubr := range user.UserBusinessRoles {
		if !ubr.IsActive || ubr.BusinessRole.ID == uuid.Nil {
			continue
		}

		rolePermissions := make([]string, 0, len(ubr.BusinessRole.Permissions))
		isBusinessAdmin := false
		for _, perm := range ubr.BusinessRole.Permissions {
			rolePermissions = append(rolePermissions, perm.Name)
			if perm.Name == "business_admin" || perm.Name == "*:*:*" {
				isBusinessAdmin = true
			}
		}

		businessRoles = append(businessRoles, map[string]interface{}{
			"business_vertical_id":   ubr.BusinessRole.BusinessVerticalID,
			"business_vertical_name": ubr.BusinessRole.BusinessVertical.Name,
			"business_vertical_code": ubr.BusinessRole.BusinessVertical.Code,
			"business_role_id":       ubr.BusinessRole.ID,
			"business_role_name":     ubr.BusinessRole.Name,
			"display_name":           ubr.BusinessRole.DisplayName,
			"is_admin":               isBusinessAdmin,
			"permissions":            rolePermissions,
		})
	}

	accessScope := "global/basic"
	if userCtx.IsSuperAdmin {
		accessScope = "all_business_verticals"
	} else if len(businessRoles) > 0 {
		accessScope = "business_roles"
	}

	userID, parseErr := uuid.Parse(claims.UserID)
	if parseErr != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	var loginEvents []models.UserLoginEvent
	if err := config.DB.
		Where("user_id = ?", userID).
		Order("login_at DESC").
		Limit(5).
		Find(&loginEvents).Error; err != nil {
		http.Error(w, "failed to load login history", http.StatusInternalServerError)
		return
	}

	recentLogins := make([]map[string]interface{}, 0, len(loginEvents))
	for _, evt := range loginEvents {
		recentLogins = append(recentLogins, map[string]interface{}{
			"id":         evt.ID,
			"timestamp":  evt.LoginAt,
			"ip_address": evt.IPAddress,
			"user_agent": evt.UserAgent,
			"status":     "success",
		})
	}

	response := map[string]interface{}{
		"userID":           claims.UserID,
		"name":             user.Name,
		"email":            user.Email,
		"phone":            user.Phone,
		"role_id":          user.RoleID,
		"global_role":      globalRoleName,
		"is_super_admin":   userCtx.IsSuperAdmin,
		"is_active":        user.IsActive,
		"created_at":       user.CreatedAt,
		"updated_at":       user.UpdatedAt,
		"permissions":      permissions,
		"business_roles":   businessRoles,
		"access_scope":     accessScope,
		"permission_count": len(permissions),
		"recent_logins":    recentLogins,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

type updateProfileRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

func handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusBadRequest)
		return
	}

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	req.Phone = strings.TrimSpace(req.Phone)

	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}
	if req.Phone == "" {
		http.Error(w, "phone is required", http.StatusBadRequest)
		return
	}

	var user models.User
	if err := config.DB.First(&user, "id = ?", userID).Error; err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	user.Name = req.Name
	user.Email = req.Email
	user.Phone = req.Phone

	if err := config.DB.Save(&user).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			http.Error(w, "email or phone already in use", http.StatusConflict)
			return
		}
		http.Error(w, "failed to update profile", http.StatusInternalServerError)
		return
	}

	middleware.InvalidateUserCache(userID.String())

	response := map[string]interface{}{
		"id":    user.ID,
		"name":  user.Name,
		"email": user.Email,
		"phone": user.Phone,
	}
	w.Header().Set("Content-Type", "application/json")
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
	api.Handle("/files/upload", middleware.RequireUploadAccess([]string{"create_reports", "create_materials"})(
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
	admin.Handle("/masters/modules/{id}", middleware.RequirePermission("masters:module:update")(
		http.HandlerFunc(masters.UpdateModule))).Methods("PUT")
	admin.Handle("/masters/modules/{id}", middleware.RequirePermission("masters:module:delete")(
		http.HandlerFunc(masters.DeleteModule))).Methods("DELETE")

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
		scope  string
		getAll func(http.ResponseWriter, *http.Request)
		getOne func(http.ResponseWriter, *http.Request)
	}{
		{"/dprsite", models.IntegrationScopePartnerDPRSiteRead, handlers.GetAllSiteEngineerReports, handlers.GetSiteEngineerReport},
		{"/wrapping", models.IntegrationScopePartnerWrappingRead, handlers.GetAllWrappingReports, handlers.GetWrappingReport},
		{"/eway", models.IntegrationScopePartnerEWayRead, handlers.GetAllEways, handlers.GetEway},
		{"/water", models.IntegrationScopePartnerWaterRead, handlers.GetAllWaterTankerReports, handlers.GetWaterTankerReport},
		{"/stock", models.IntegrationScopePartnerStockRead, handlers.GetAllStockReports, handlers.GetStockReport},
		{"/dairysite", models.IntegrationScopePartnerDairySiteRead, handlers.GetAllDairySiteReports, handlers.GetDairySiteReport},
		{"/payment", models.IntegrationScopePartnerPaymentRead, handlers.GetAllPayments, handlers.GetPayment},
		{"/material", models.IntegrationScopePartnerMaterialRead, handlers.GetAllMaterials, handlers.GetMaterial},
		{"/mnr", models.IntegrationScopePartnerMNRRead, handlers.GetAllMNRReports, handlers.GetMNRReport},
		{"/nmr_vehicle", models.IntegrationScopePartnerNMRVehicleRead, handlers.GetAllNmrVehicle, handlers.GetNmrVehicle},
		{"/contractor", models.IntegrationScopePartnerContractorRead, handlers.GetAllContractorReports, handlers.GetContractorReport},
		{"/painting", models.IntegrationScopePartnerPaintingRead, handlers.GetAllPaintingReports, handlers.GetPaintingReport},
		{"/diesel", models.IntegrationScopePartnerDieselRead, handlers.GetAllDieselReports, handlers.GetDieselReport},
		{"/tasks", models.IntegrationScopePartnerTasksRead, handlers.GetAllTasks, handlers.GetTask},
		{"/vehiclelog", models.IntegrationScopePartnerVehicleLogRead, handlers.GetAllVehicleLogs, handlers.GetVehicleLog},
	}

	for _, res := range partnerResources {
		partner.Handle(res.path, middleware.RequireIntegrationScope(res.scope)(http.HandlerFunc(res.getAll))).Methods("GET")
		partner.Handle(res.path+"/{id}", middleware.RequireIntegrationScope(res.scope)(http.HandlerFunc(res.getOne))).Methods("GET")
	}
}
