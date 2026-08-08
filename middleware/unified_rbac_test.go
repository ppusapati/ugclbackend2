package middleware

import (
	"testing"

	"github.com/google/uuid"
	"p9e.in/ugcl/models"
)

func TestRBACCollectsOnlyGlobalPermissions(t *testing.T) {
	t.Setenv("RBAC_ENABLED", "true")
	businessID := uuid.New()
	user := models.User{RoleAssignments: []models.UserRoleAssignment{
		{
			IsActive: true,
			Role: models.RBACRole{
				IsActive:    true,
				ScopeType:   models.RoleScopeGlobal,
				Permissions: []models.Permission{{Name: "read_users"}},
			},
		},
		{
			IsActive: true,
			Role: models.RBACRole{
				IsActive:           true,
				ScopeType:          models.RoleScopeBusinessVertical,
				BusinessVerticalID: &businessID,
				Permissions:        []models.Permission{{Name: "attendance:checkin"}},
			},
		},
	}}

	permissions := collectGlobalPermissions(user)
	if len(permissions) != 1 || permissions[0] != "read_users" {
		t.Fatalf("collectGlobalPermissions() = %v, want [read_users]", permissions)
	}
}

func TestRBACSuperAdminRequiresActiveGlobalAssignment(t *testing.T) {
	t.Setenv("RBAC_ENABLED", "true")
	user := models.User{RoleAssignments: []models.UserRoleAssignment{
		{
			IsActive: true,
			Role: models.RBACRole{
				Name:      "super_admin",
				ScopeType: models.RoleScopeGlobal,
				IsActive:  true,
			},
		},
	}}
	if !isUnifiedSuperAdmin(user) {
		t.Fatal("expected active global super_admin assignment")
	}

	user.RoleAssignments[0].IsActive = false
	if isUnifiedSuperAdmin(user) {
		t.Fatal("did not expect inactive super_admin assignment to grant access")
	}
}

func TestRBACIgnoresLegacySuperAdminRole(t *testing.T) {
	t.Setenv("RBAC_ENABLED", "true")
	legacyRole := &models.Role{Name: "super_admin"}
	user := models.User{RoleModel: legacyRole}

	if NewAuthService().IsSuperAdmin(user) {
		t.Fatal("legacy super_admin role must not grant access in RBAC mode")
	}
}

func TestRBACBusinessAccessComesFromScopedAssignment(t *testing.T) {
	t.Setenv("RBAC_ENABLED", "true")
	assignedBusinessID := uuid.New()
	primaryBusinessID := uuid.New()
	user := models.User{
		BusinessVerticalID: &primaryBusinessID,
		RoleAssignments: []models.UserRoleAssignment{
			{
				IsActive: true,
				Role: models.RBACRole{
					IsActive:           true,
					ScopeType:          models.RoleScopeBusinessVertical,
					BusinessVerticalID: &assignedBusinessID,
				},
			},
		},
	}
	ctx := &UserContext{User: &user}

	if !CanAccessBusiness(ctx, assignedBusinessID) {
		t.Fatal("expected scoped assignment to grant business access")
	}
	if CanAccessBusiness(ctx, primaryBusinessID) {
		t.Fatal("primary business preference must not grant RBAC access")
	}
}

func TestRBACBusinessPermissionsStayInVertical(t *testing.T) {
	t.Setenv("RBAC_ENABLED", "true")
	waterID := uuid.New()
	solarID := uuid.New()
	user := models.User{RoleAssignments: []models.UserRoleAssignment{
		{
			IsActive: true,
			Role: models.RBACRole{
				IsActive:           true,
				ScopeType:          models.RoleScopeBusinessVertical,
				BusinessVerticalID: &waterID,
				Permissions:        []models.Permission{{Name: "attendance:checkin"}},
			},
		},
	}}
	service := NewAuthService()

	water := service.LoadBusinessContext(user, waterID)
	if _, ok := water.permissionSet["attendance:checkin"]; !ok {
		t.Fatal("expected WATER assignment permission in WATER context")
	}

	solar := service.LoadBusinessContext(user, solarID)
	if _, ok := solar.permissionSet["attendance:checkin"]; ok {
		t.Fatal("WATER permission must not leak into SOLAR context")
	}
}

func TestRBACAppliesGlobalPermissionInBusiness(t *testing.T) {
	t.Setenv("RBAC_ENABLED", "true")
	service := NewAuthService()
	ctx := &UserContext{
		GlobalPermissions: []string{"attendance:checkin"},
		globalPermSet:     map[string]struct{}{"attendance:checkin": {}},
		BusinessContext: &BusinessContext{
			permissionSet: make(map[string]struct{}),
		},
	}

	if !service.HasBusinessPermission(ctx, "attendance:checkin") {
		t.Fatal("global permission must apply after business context is resolved")
	}
}
