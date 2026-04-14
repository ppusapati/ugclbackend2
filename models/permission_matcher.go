package models

import "strings"

// matchesPermission checks whether a stored permission grants a required permission.
func matchesPermission(userPerm, requiredPerm string) bool {
	if userPerm == requiredPerm {
		return true
	}

	if userPerm == "*:*:*" || userPerm == "*" {
		return true
	}

	userParts := strings.Split(userPerm, ":")
	reqParts := strings.Split(requiredPerm, ":")

	if len(userParts) < 2 || len(reqParts) < 2 {
		return userPerm == requiredPerm
	}

	resourceMatch := userParts[0] == "*" || userParts[0] == reqParts[0]
	actionMatch := userParts[1] == "*" || userParts[1] == reqParts[1]

	return resourceMatch && actionMatch
}
