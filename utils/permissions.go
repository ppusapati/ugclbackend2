package utils

import "strings"

// MatchesPermission checks if a user permission matches the required permission
// Supports wildcard patterns:
//
// Examples:
//   - "*:*:*" or "*" matches everything (super admin wildcard)
//   - "project:*" matches all actions on project resource (e.g., project:create, project:read, project:delete)
//   - "*:read" matches read action on all resources (e.g., project:read, user:read, report:read)
//   - "project:create" exact match
//
// Permission format: "resource:action" or "resource:action:scope"
//
// This allows for dynamic permission expansion without code changes:
//   - Add new permissions to database
//   - Users with wildcard patterns automatically get access
//   - No code deployment needed for new permissions
func MatchesPermission(userPerm, requiredPerm string) bool {
	// Exact match (fastest path)
	if userPerm == requiredPerm {
		return true
	}

	// Full wildcard - grants everything
	if userPerm == "*:*:*" || userPerm == "*" {
		return true
	}

	// Split permissions into parts (format: resource:action or resource:action:scope)
	userParts := strings.Split(userPerm, ":")
	reqParts := strings.Split(requiredPerm, ":")

	// Ensure both have at least 2 parts (resource:action)
	if len(userParts) < 2 || len(reqParts) < 2 {
		// Backward compatibility: if old format (no colons), only exact match works
		return userPerm == requiredPerm
	}

	// Check resource match (first part)
	resourceMatch := userParts[0] == "*" || userParts[0] == reqParts[0]

	// Check action match (second part)
	actionMatch := userParts[1] == "*" || userParts[1] == reqParts[1]

	// Both resource and action must match for permission to be granted
	return resourceMatch && actionMatch
}
