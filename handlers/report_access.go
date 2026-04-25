package handlers

import (
	"net/http"

	"p9e.in/ugcl/middleware"
	"p9e.in/ugcl/models"
)

// canViewReport determines whether the requesting user may view (read/execute) the given report.
//
// Access rules (evaluated in priority order):
//  1. Super-admin – always allowed.
//  2. Creator – the user who originally created the report always has access.
//  3. Public report (is_public=true) – any authenticated user with the report:read permission
//     may view it; route-level middleware already enforces report:read.
//  4. Role-based – if the report is private but allowed_roles is non-empty, the user's global
//     role name must appear in allowed_roles.
//  5. Legacy compatibility – if the report is private AND allowed_roles is empty AND is_public
//     is false (i.e. was saved before access-control was enforced), allow access so existing
//     reports are not accidentally locked out during rollout.
func canViewReport(r *http.Request, report *models.ReportDefinition) bool {
	userCtx, err := authSvc.LoadUserContext(r)
	if err != nil {
		return false
	}

	// Rule 1: super-admin bypass
	if userCtx.IsSuperAdmin {
		return true
	}

	// Rule 2: creator always has access
	if userCtx.Claims != nil && userCtx.Claims.UserID == report.CreatedBy {
		return true
	}

	// Rule 3: public report
	if report.IsPublic {
		return true
	}

	// Rule 4: role-based access
	if len(report.AllowedRoles) > 0 {
		userRole := ""
		if userCtx.User.RoleModel != nil {
			userRole = userCtx.User.RoleModel.Name
		}
		for _, r := range report.AllowedRoles {
			if r == userRole {
				return true
			}
		}
		return false
	}

	// Rule 5: legacy report with no access settings – allow (backward compatibility)
	return true
}

// canModifyReport determines whether the requesting user may update or delete the given report.
//
// Only super-admins and the report creator may modify a report.
func canModifyReport(r *http.Request, report *models.ReportDefinition) bool {
	userCtx, err := authSvc.LoadUserContext(r)
	if err != nil {
		return false
	}

	if userCtx.IsSuperAdmin {
		return true
	}

	if userCtx.Claims != nil && userCtx.Claims.UserID == report.CreatedBy {
		return true
	}

	return false
}

// authSvc is a package-level AuthService used by access helpers.
// It is the same singleton already used in authorization_refactored.go
// (declared there as `var authService = NewAuthService()`).
// We declare a separate local alias to avoid import cycles and name collisions.
var authSvc = middleware.NewAuthService()

// reportAccessDenied writes a uniform 403 response.
func reportAccessDenied(w http.ResponseWriter) {
	http.Error(w, "You do not have access to this report", http.StatusForbidden)
}
