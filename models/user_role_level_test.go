package models

import (
	"testing"

	"github.com/google/uuid"
)

func TestCanAssignRole_SuperAdminCanAssignLevelOne(t *testing.T) {
	u := User{
		RoleModel: &Role{Name: "super_admin", IsGlobal: true},
	}

	if !u.CanAssignRole(1) {
		t.Fatalf("expected super admin to assign level 1 role")
	}
}

func TestCanAssignRole_LevelOneCanAssignLevelTwo(t *testing.T) {
	u := User{
		RoleModel: &Role{Name: "system_admin", IsGlobal: true},
	}

	if !u.CanAssignRole(2) {
		t.Fatalf("expected level 1 user to assign level 2 role")
	}
}

func TestCanAssignRole_LevelOneCannotAssignLevelOne(t *testing.T) {
	u := User{
		RoleModel: &Role{Name: "system_admin", IsGlobal: true},
	}

	if u.CanAssignRole(1) {
		t.Fatalf("expected level 1 user to be blocked from assigning level 1 role")
	}
}

func TestCanAssignRole_BusinessRoleBoundary(t *testing.T) {
	u := User{
		UserBusinessRoles: []UserBusinessRole{
			{
				IsActive:     true,
				BusinessRole: BusinessRole{ID: uuid.New(), Level: 2},
			},
		},
	}

	if !u.CanAssignRole(3) {
		t.Fatalf("expected level 2 business user to assign level 3 role")
	}

	if u.CanAssignRole(2) {
		t.Fatalf("expected level 2 business user to be blocked from assigning level 2 role")
	}
}
