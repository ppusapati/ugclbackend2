package middleware

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"p9e.in/ugcl/models"
)

// RBACEnabled always returns true; the migration is permanent.
func RBACEnabled() bool { return true }

func preloadAuthorizationGraph(query *gorm.DB) *gorm.DB {
	if RBACEnabled() {
		return query.
			Preload("RoleAssignments", "is_active = ?", true).
			Preload("RoleAssignments.Role", "is_active = ?", true).
			Preload("RoleAssignments.Role.Permissions").
			Preload("RoleAssignments.Role.BusinessVertical")
	}

	return query.
		Preload("RoleModel.Permissions").
		Preload("UserBusinessRoles", "is_active = ?", true).
		Preload("UserBusinessRoles.BusinessRole", "is_active = ?", true).
		Preload("UserBusinessRoles.BusinessRole.Permissions").
		Preload("UserBusinessRoles.BusinessRole.BusinessVertical")
}

func collectGlobalPermissions(user models.User) []string {
	if !RBACEnabled() {
		permissions := make([]string, 0)
		if user.RoleModel != nil {
			permissions = make([]string, 0, len(user.RoleModel.Permissions))
			for _, permission := range user.RoleModel.Permissions {
				permissions = append(permissions, permission.Name)
			}
		}
		return permissions
	}

	permissions := make([]string, 0)
	seen := make(map[string]struct{})
	for _, assignment := range user.RoleAssignments {
		if !assignment.IsActive || !assignment.Role.IsActive || assignment.Role.ScopeType != models.RoleScopeGlobal {
			continue
		}
		for _, permission := range assignment.Role.Permissions {
			if _, exists := seen[permission.Name]; exists {
				continue
			}
			seen[permission.Name] = struct{}{}
			permissions = append(permissions, permission.Name)
		}
	}
	return permissions
}

func isUnifiedSuperAdmin(user models.User) bool {
	if !RBACEnabled() {
		return false
	}
	for _, assignment := range user.RoleAssignments {
		if assignment.IsActive &&
			assignment.Role.IsActive &&
			assignment.Role.ScopeType == models.RoleScopeGlobal &&
			assignment.Role.Name == "super_admin" {
			return true
		}
	}
	return false
}

func unifiedBusinessAssignments(user models.User, businessID uuid.UUID) []models.UserRoleAssignment {
	assignments := make([]models.UserRoleAssignment, 0)
	for _, assignment := range user.RoleAssignments {
		role := assignment.Role
		if !assignment.IsActive || !role.IsActive || role.ScopeType != models.RoleScopeBusinessVertical {
			continue
		}
		if role.BusinessVerticalID != nil && *role.BusinessVerticalID == businessID {
			assignments = append(assignments, assignment)
		}
	}
	return assignments
}

func unifiedAccessibleBusinessIDs(user models.User) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{})
	for _, assignment := range user.RoleAssignments {
		role := assignment.Role
		if !assignment.IsActive || !role.IsActive || role.ScopeType != models.RoleScopeBusinessVertical {
			continue
		}
		if role.BusinessVerticalID != nil && *role.BusinessVerticalID != uuid.Nil {
			seen[*role.BusinessVerticalID] = struct{}{}
		}
	}

	ids := make([]uuid.UUID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

// HasActiveGlobalRBACRole reports whether the user has any active global-scoped RBAC assignment.
func HasActiveGlobalRBACRole(user models.User) bool {
	return hasActiveGlobalRBACRole(user)
}

// hasActiveGlobalRBACRole reports whether the user has any active global-scoped RBAC assignment.
func hasActiveGlobalRBACRole(user models.User) bool {
	for _, a := range user.RoleAssignments {
		if a.IsActive && a.Role.IsActive && a.Role.ScopeType == models.RoleScopeGlobal {
			return true
		}
	}
	return false
}
