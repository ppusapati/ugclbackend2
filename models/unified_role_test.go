package models

import (
	"testing"

	"github.com/google/uuid"
)

func TestUnifiedRoleValidateScope(t *testing.T) {
	businessID := uuid.New()

	tests := []struct {
		name    string
		role    RBACRole
		wantErr bool
	}{
		{name: "global", role: RBACRole{ScopeType: RoleScopeGlobal}},
		{name: "business", role: RBACRole{ScopeType: RoleScopeBusinessVertical, BusinessVerticalID: &businessID}},
		{name: "global with business", role: RBACRole{ScopeType: RoleScopeGlobal, BusinessVerticalID: &businessID}, wantErr: true},
		{name: "business without business", role: RBACRole{ScopeType: RoleScopeBusinessVertical}, wantErr: true},
		{name: "unknown scope", role: RBACRole{ScopeType: "site"}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.role.ValidateScope()
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateScope() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

func TestUnifiedRoleAssignmentScopeKey(t *testing.T) {
	businessID := uuid.New()

	tests := []struct {
		name string
		role RBACRole
		want string
	}{
		{name: "global", role: RBACRole{ScopeType: RoleScopeGlobal}, want: "global"},
		{
			name: "business",
			role: RBACRole{ScopeType: RoleScopeBusinessVertical, BusinessVerticalID: &businessID},
			want: "business_vertical:" + businessID.String(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := test.role.AssignmentScopeKey()
			if err != nil {
				t.Fatalf("AssignmentScopeKey() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("AssignmentScopeKey() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestUnifiedRoleHasPermission(t *testing.T) {
	role := RBACRole{Permissions: []Permission{{Name: "attendance:*"}}}
	if !role.HasPermission("attendance:checkin") {
		t.Fatal("expected wildcard attendance permission to allow check-in")
	}
	if role.HasPermission("finance:read") {
		t.Fatal("did not expect attendance permission to allow finance access")
	}
}
