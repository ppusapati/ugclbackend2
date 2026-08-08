package handlers

import (
	"testing"

	"github.com/google/uuid"
	"p9e.in/ugcl/models"
)

func TestActorRoleCanManageTargetHonorsHierarchyAndScope(t *testing.T) {
	waterID := uuid.New()
	solarID := uuid.New()
	tests := []struct {
		name   string
		actor  models.RBACRole
		target models.RBACRole
		want   bool
	}{
		{
			name:   "global higher role manages business role",
			actor:  models.RBACRole{ScopeType: models.RoleScopeGlobal, Level: 1, IsActive: true},
			target: models.RBACRole{ScopeType: models.RoleScopeBusinessVertical, BusinessVerticalID: &waterID, Level: 3},
			want:   true,
		},
		{
			name:   "same vertical higher role manages lower role",
			actor:  models.RBACRole{ScopeType: models.RoleScopeBusinessVertical, BusinessVerticalID: &waterID, Level: 1, IsActive: true},
			target: models.RBACRole{ScopeType: models.RoleScopeBusinessVertical, BusinessVerticalID: &waterID, Level: 3},
			want:   true,
		},
		{
			name:   "business role cannot cross verticals",
			actor:  models.RBACRole{ScopeType: models.RoleScopeBusinessVertical, BusinessVerticalID: &waterID, Level: 1, IsActive: true},
			target: models.RBACRole{ScopeType: models.RoleScopeBusinessVertical, BusinessVerticalID: &solarID, Level: 3},
		},
		{
			name:   "business role cannot manage global role",
			actor:  models.RBACRole{ScopeType: models.RoleScopeBusinessVertical, BusinessVerticalID: &waterID, Level: 1, IsActive: true},
			target: models.RBACRole{ScopeType: models.RoleScopeGlobal, Level: 3},
		},
		{
			name:   "equal hierarchy is denied",
			actor:  models.RBACRole{ScopeType: models.RoleScopeGlobal, Level: 3, IsActive: true},
			target: models.RBACRole{ScopeType: models.RoleScopeGlobal, Level: 3},
		},
		{
			name:   "inactive actor role is denied",
			actor:  models.RBACRole{ScopeType: models.RoleScopeGlobal, Level: 1},
			target: models.RBACRole{ScopeType: models.RoleScopeGlobal, Level: 3},
		},
		{
			name:   "super admin bypasses numeric hierarchy",
			actor:  models.RBACRole{Name: "super_admin", ScopeType: models.RoleScopeGlobal, Level: 0, IsActive: true},
			target: models.RBACRole{ScopeType: models.RoleScopeGlobal, Level: 0},
			want:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := actorRoleCanManageTarget(test.actor, test.target); got != test.want {
				t.Fatalf("actorRoleCanManageTarget() = %t, want %t", got, test.want)
			}
		})
	}
}
