package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"p9e.in/ugcl/config"
	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

const platformAdminBcryptCost = 12

type platformAdminLoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type platformAdminLoginResp struct {
	Token string           `json:"token"`
	Admin platformAdminOut `json:"admin"`
}

type platformAdminOut struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
}

// PlatformAdminLogin authenticates a platform admin against control.platform_admins.
// POST /api/v1/platform/login — public, no tenant_slug (a platform admin
// operates above tenants, so there's nothing to resolve).
func PlatformAdminLogin(w http.ResponseWriter, r *http.Request) {
	var req platformAdminLoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return
	}

	var admin models.PlatformAdmin
	if err := config.DB. // config-db-ok: control-schema lookup, platform admins have no tenant
				Where("email = ? AND is_active = ?", req.Email, true).
				First(&admin).Error; err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := middleware.GeneratePlatformAdminToken(admin.ID.String(), admin.Name, admin.Email)
	if err != nil {
		http.Error(w, "couldn't create token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(platformAdminLoginResp{
		Token: token,
		Admin: platformAdminOut{ID: admin.ID, Name: admin.Name, Email: admin.Email},
	})
}

// PlatformAdminMe returns the authenticated platform admin's own profile.
// GET /api/v1/platform/me
func PlatformAdminMe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.GetPlatformClaims(r)
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":    claims.AdminID,
		"name":  claims.Name,
		"email": claims.Email,
	})
}

type createTenantReq struct {
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	SchemaName string `json:"schema_name"`
}

type tenantOut struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Slug       string    `json:"slug"`
	SchemaName string    `json:"schema_name"`
	Status     string    `json:"status"`
	IsActive   bool      `json:"is_active"`
}

func toTenantOut(t models.Tenant) tenantOut {
	return tenantOut{
		ID: t.ID, Name: t.Name, Slug: t.Slug, SchemaName: t.SchemaName,
		Status: t.Status, IsActive: t.IsActive,
	}
}

// ListPlatformTenants lists every tenant in the registry, any status.
// GET /api/v1/platform/tenants
func ListPlatformTenants(w http.ResponseWriter, r *http.Request) {
	var tenants []models.Tenant
	if err := config.DB. // config-db-ok: control-schema registry, not tenant-scoped
				Order("created_at DESC").
				Find(&tenants).Error; err != nil {
		http.Error(w, "failed to list tenants", http.StatusInternalServerError)
		return
	}

	out := make([]tenantOut, len(tenants))
	for i, t := range tenants {
		out[i] = toTenantOut(t)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"tenants": out, "count": len(out)})
}

// CreatePlatformTenant creates the control.tenants row only — step 1 of 3.
// Status starts as "provisioning". Does not touch the tenant's schema; call
// ProvisionPlatformTenant next.
// POST /api/v1/platform/tenants
func CreatePlatformTenant(w http.ResponseWriter, r *http.Request) {
	var req createTenantReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Slug = strings.TrimSpace(req.Slug)
	req.SchemaName = strings.TrimSpace(req.SchemaName)
	if req.SchemaName == "" {
		req.SchemaName = strings.ReplaceAll(req.Slug, "-", "_")
	}

	if req.Name == "" || req.Slug == "" {
		http.Error(w, "name and slug are required", http.StatusBadRequest)
		return
	}
	if err := config.ValidateTenantSchemaName(req.SchemaName); err != nil {
		http.Error(w, "invalid schema_name: "+err.Error(), http.StatusBadRequest)
		return
	}

	tenant := models.Tenant{
		Name:       req.Name,
		Slug:       req.Slug,
		SchemaName: req.SchemaName,
		Status:     models.TenantStatusProvisioning,
		IsActive:   true,
	}
	if err := config.DB.Create(&tenant).Error; err != nil { // config-db-ok: control-schema registry write
		if strings.Contains(err.Error(), "duplicate key") {
			http.Error(w, "a tenant with this slug or schema_name already exists", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create tenant: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"tenant": toTenantOut(tenant)})
}

// ProvisionPlatformTenant creates and migrates the tenant's Postgres schema
// — step 2 of 3. Idempotent: safe to retry if a prior attempt failed
// partway (see config.ProvisionTenantSchema).
// POST /api/v1/platform/tenants/{id}/provision
func ProvisionPlatformTenant(w http.ResponseWriter, r *http.Request) {
	tenant, err := loadPlatformTenant(w, r)
	if err != nil {
		return
	}

	if err := config.ProvisionTenantSchema(tenant.SchemaName); err != nil {
		config.DB.Model(tenant).Update("status", models.TenantStatusFailed) // config-db-ok: control-schema registry write
		http.Error(w, "failed to provision tenant schema: "+err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "tenant schema provisioned",
		"tenant":  toTenantOut(*tenant),
	})
}

type seedTenantAdminReq struct {
	AdminName     string `json:"admin_name"`
	AdminEmail    string `json:"admin_email"`
	AdminPhone    string `json:"admin_phone"`
	AdminPassword string `json:"admin_password"`
}

// SeedPlatformTenantAdmin seeds the tenant's first super_admin user and
// flips the tenant to active — step 3 of 3.
// POST /api/v1/platform/tenants/{id}/seed-admin
func SeedPlatformTenantAdmin(w http.ResponseWriter, r *http.Request) {
	tenant, err := loadPlatformTenant(w, r)
	if err != nil {
		return
	}

	var req seedTenantAdminReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.AdminPhone) == "" || req.AdminPassword == "" {
		http.Error(w, "admin_phone and admin_password are required", http.StatusBadRequest)
		return
	}

	tenantDB, cleanup, sessionErr := config.TenantScopedSession(tenant.SchemaName)
	if sessionErr != nil {
		http.Error(w, "failed to open tenant session: "+sessionErr.Error(), http.StatusInternalServerError)
		return
	}
	defer cleanup()

	seedErr := config.SeedNewTenantRBAC(tenantDB, config.SeedNewTenantRBACInput{
		SuperAdminName:     strings.TrimSpace(req.AdminName),
		SuperAdminEmail:    strings.TrimSpace(req.AdminEmail),
		SuperAdminPhone:    strings.TrimSpace(req.AdminPhone),
		SuperAdminPassword: req.AdminPassword,
	})
	if seedErr != nil {
		http.Error(w, "failed to seed tenant admin: "+seedErr.Error(), http.StatusInternalServerError)
		return
	}

	if err := config.DB.Model(tenant).Updates(map[string]interface{}{ // config-db-ok: control-schema registry write
		"status": models.TenantStatusActive,
	}).Error; err != nil {
		http.Error(w, "seeded admin but failed to activate tenant: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tenant.Status = models.TenantStatusActive

	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "tenant admin seeded and tenant activated",
		"tenant":  toTenantOut(*tenant),
	})
}

func loadPlatformTenant(w http.ResponseWriter, r *http.Request) (*models.Tenant, error) {
	idStr := mux.Vars(r)["id"]
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "invalid tenant id", http.StatusBadRequest)
		return nil, err
	}

	var tenant models.Tenant
	if err := config.DB.First(&tenant, "id = ?", id).Error; err != nil { // config-db-ok: control-schema registry read
		if errors.Is(err, gorm.ErrRecordNotFound) {
			http.Error(w, "tenant not found", http.StatusNotFound)
		} else {
			http.Error(w, "failed to load tenant", http.StatusInternalServerError)
		}
		return nil, err
	}
	return &tenant, nil
}
