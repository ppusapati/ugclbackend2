package config

import (
	"log"
)

// MigrateExistingUsers - DEPRECATED: Legacy string-based role migration
// This function is no longer needed as the User.Role field has been removed
// Use MigrateToNewRBAC() instead for migrating users to the new RBAC system
func MigrateExistingUsers() {
	log.Printf("⚠️  MigrateExistingUsers is deprecated - use MigrateToNewRBAC() instead")
	// No-op: Legacy role field removed from User model
}
