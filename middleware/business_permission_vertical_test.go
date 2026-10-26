package middleware

import (
	"testing"

	"github.com/google/uuid"
	"p9e.in/ugcl/models"
)

func TestHasBusinessPermissionInVertical(t *testing.T) {
	t.Setenv("RBAC_ENABLED", "true")
	waterID := uuid.New()
	solarID := uuid.New()
	user := models.User{RoleAssignments: []models.UserRoleAssignment{{
		IsActive: true,
		Role: models.RBACRole{
			IsActive:           true,
			ScopeType:          models.RoleScopeBusinessVertical,
			BusinessVerticalID: &waterID,
			Permissions:        []models.Permission{{Name: "attendance:checkout"}},
		},
	}}}
	// The request itself is scoped to SOLAR, as when the app's selection
	// changed after checking in to a WATER site.
	userCtx := &UserContext{User: &user, BusinessContext: NewAuthService().LoadBusinessContext(user, solarID)}

	if !hasBusinessPermissionInVertical(userCtx, waterID, "attendance:checkout") {
		t.Error("expected attendance:checkout in the vertical the role is scoped to")
	}
	if hasBusinessPermissionInVertical(userCtx, solarID, "attendance:checkout") {
		t.Error("permission must not apply to a vertical the user has no role in")
	}
	if userCtx.BusinessContext.BusinessID != solarID {
		t.Error("checking another vertical must not change the request's own business context")
	}
}
