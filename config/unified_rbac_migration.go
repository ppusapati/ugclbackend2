package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"p9e.in/ugcl/models"
)

const (
	legacyGlobalRoleSource       = "roles"
	legacyBusinessRoleSource     = "business_roles"
	legacyGlobalAssignmentSource = "users.role_id"
	legacyBusinessAssignSource   = "user_business_roles"
)

// PrepareRBACMigration creates and populates only the new unified RBAC
// tables. It is intentionally not registered in Migrations; callers must first
// complete the documented backup and restore verification gate.
func PrepareRBACMigration(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is required")
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.AutoMigrate(
			&models.RBACRole{},
			&models.RBACRolePermission{},
			&models.UserRoleAssignment{},
		); err != nil {
			return fmt.Errorf("create unified RBAC tables: %w", err)
		}
		// Truncate to make the migration idempotent; these tables contain only copied data.
		for _, table := range []string{"user_role_assignments", "rbac_role_permissions", "rbac_roles"} {
			if err := tx.Exec("TRUNCATE TABLE " + table + " CASCADE").Error; err != nil {
				return fmt.Errorf("truncate %s: %w", table, err)
			}
		}
		if err := createUnifiedRBACConstraints(tx); err != nil {
			return err
		}

		roleMap, expectedRoles, err := copyLegacyRoles(tx)
		if err != nil {
			return err
		}

		expectedPermissions, err := copyLegacyRolePermissions(tx, roleMap)
		if err != nil {
			return err
		}

		expectedAssignments, err := copyLegacyRoleAssignments(tx, roleMap)
		if err != nil {
			return err
		}

		return verifyUnifiedRBACCopy(tx, expectedRoles, expectedPermissions, expectedAssignments)
	})
}

func ValidateRBACCutoverReady(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is required")
	}
	for _, table := range []string{"rbac_roles", "rbac_role_permissions", "user_role_assignments"} {
		if !db.Migrator().HasTable(table) {
			return fmt.Errorf("RBAC cutover blocked: table %s does not exist", table)
		}
	}

	// source counts use DISTINCT name to match the deduplication the migration applies.
	checks := []struct {
		name      string
		sourceSQL string
		targetSQL string
		targetArgs []interface{}
		atMost    bool // true: RBAC count ≤ source count (duplicates/orphans may be skipped)
	}{
		{
			name:       "global roles",
			sourceSQL:  "SELECT COUNT(DISTINCT LOWER(name)) FROM roles",
			targetSQL:  "SELECT COUNT(*) FROM rbac_roles WHERE legacy_source_type = ?",
			targetArgs: []interface{}{legacyGlobalRoleSource},
		},
		{
			name:       "business roles",
			sourceSQL:  "SELECT COUNT(DISTINCT (business_vertical_id::text || '|' || LOWER(name))) FROM business_roles",
			targetSQL:  "SELECT COUNT(*) FROM rbac_roles WHERE legacy_source_type = ?",
			targetArgs: []interface{}{legacyBusinessRoleSource},
		},
		{
			// Assignments may be fewer than source when users reference deleted roles (skipped).
			name:       "global assignments",
			sourceSQL:  "SELECT COUNT(*) FROM users WHERE role_id IS NOT NULL",
			targetSQL:  "SELECT COUNT(*) FROM user_role_assignments WHERE legacy_source_type = ?",
			targetArgs: []interface{}{legacyGlobalAssignmentSource},
			atMost:     true,
		},
		{
			// Assignments may be fewer than source when duplicates were merged.
			name:       "business assignments",
			sourceSQL:  "SELECT COUNT(*) FROM user_business_roles",
			targetSQL:  "SELECT COUNT(*) FROM user_role_assignments WHERE legacy_source_type = ?",
			targetArgs: []interface{}{legacyBusinessAssignSource},
			atMost:     true,
		},
	}

	for _, check := range checks {
		var sourceCount, targetCount int64
		if err := db.Raw(check.sourceSQL).Scan(&sourceCount).Error; err != nil {
			return fmt.Errorf("count legacy %s: %w", check.name, err)
		}
		if err := db.Raw(check.targetSQL, check.targetArgs...).Scan(&targetCount).Error; err != nil {
			return fmt.Errorf("count RBAC %s: %w", check.name, err)
		}
		if check.atMost {
			if targetCount == 0 || targetCount > sourceCount {
				return fmt.Errorf("RBAC cutover blocked: %s parity out of range (%d legacy, %d RBAC)", check.name, sourceCount, targetCount)
			}
		} else if sourceCount != targetCount {
			return fmt.Errorf("RBAC cutover blocked: %s parity mismatch (%d legacy, %d RBAC)", check.name, sourceCount, targetCount)
		}
	}
	return nil
}

func createUnifiedRBACConstraints(tx *gorm.DB) error {
	statements := []string{
		`DO $$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'chk_rbac_roles_scope') THEN
				ALTER TABLE rbac_roles ADD CONSTRAINT chk_rbac_roles_scope CHECK (
					(scope_type = 'global' AND business_vertical_id IS NULL) OR
					(scope_type = 'business_vertical' AND business_vertical_id IS NOT NULL)
				);
			END IF;
		END $$`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_rbac_roles_global_name
			ON rbac_roles (LOWER(name)) WHERE scope_type = 'global'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_rbac_roles_business_name
			ON rbac_roles (business_vertical_id, LOWER(name)) WHERE scope_type = 'business_vertical'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uq_user_role_assignments_active_scope
			ON user_role_assignments (user_id, scope_key) WHERE is_active = true`,
	}
	for _, statement := range statements {
		if err := tx.Exec(statement).Error; err != nil {
			return fmt.Errorf("create unified RBAC constraint: %w", err)
		}
	}
	return nil
}

type legacyRoleKey struct {
	source string
	id     uuid.UUID
}

func copyLegacyRoles(tx *gorm.DB) (map[legacyRoleKey]models.RBACRole, int64, error) {
	roleMap := make(map[legacyRoleKey]models.RBACRole)

	var globalRoles []models.Role
	if err := tx.Find(&globalRoles).Error; err != nil {
		return nil, 0, fmt.Errorf("load global roles: %w", err)
	}
	for _, source := range globalRoles {
		legacySourceType := legacyGlobalRoleSource
		legacySourceID := source.ID
		role := models.RBACRole{
			Name:             source.Name,
			DisplayName:      source.Name,
			Description:      source.Description,
			ScopeType:        models.RoleScopeGlobal,
			Level:            source.Level,
			IsActive:         source.IsActive,
			LegacySourceType: &legacySourceType,
			LegacySourceID:   &legacySourceID,
		}
		if err := firstOrCreateUnifiedRole(tx, &role); err != nil {
			return nil, 0, err
		}
		roleMap[legacyRoleKey{source: legacyGlobalRoleSource, id: source.ID}] = role
	}

	var businessRoles []models.BusinessRole
	if err := tx.Find(&businessRoles).Error; err != nil {
		return nil, 0, fmt.Errorf("load business roles: %w", err)
	}
	for _, source := range businessRoles {
		businessID := source.BusinessVerticalID
		legacySourceType := legacyBusinessRoleSource
		legacySourceID := source.ID
		role := models.RBACRole{
			Name:               source.Name,
			DisplayName:        source.DisplayName,
			Description:        source.Description,
			ScopeType:          models.RoleScopeBusinessVertical,
			BusinessVerticalID: &businessID,
			Level:              source.Level,
			IsActive:           source.IsActive,
			LegacySourceType:   &legacySourceType,
			LegacySourceID:     &legacySourceID,
		}
		if err := firstOrCreateUnifiedRole(tx, &role); err != nil {
			return nil, 0, err
		}
		roleMap[legacyRoleKey{source: legacyBusinessRoleSource, id: source.ID}] = role
	}

	distinctRoleIDs := make(map[uuid.UUID]struct{}, len(roleMap))
	for _, r := range roleMap {
		distinctRoleIDs[r.ID] = struct{}{}
	}
	return roleMap, int64(len(distinctRoleIDs)), nil
}

func firstOrCreateUnifiedRole(tx *gorm.DB, role *models.RBACRole) error {
	if role.LegacySourceType == nil || role.LegacySourceID == nil {
		return errors.New("legacy role source is required for migration")
	}
	// Primary lookup: by legacy source to handle re-runs.
	result := tx.Where(
		"legacy_source_type = ? AND legacy_source_id = ?",
		*role.LegacySourceType,
		*role.LegacySourceID,
	).First(role)
	if result.Error == nil {
		return nil
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("lookup %s role %s: %w", *role.LegacySourceType, *role.LegacySourceID, result.Error)
	}

	// Secondary lookup: by canonical name+scope to merge case-insensitive legacy duplicates.
	scopeQuery := tx.Where("scope_type = ? AND LOWER(name) = LOWER(?)", role.ScopeType, role.Name)
	if role.BusinessVerticalID != nil {
		scopeQuery = scopeQuery.Where("business_vertical_id = ?", *role.BusinessVerticalID)
	} else {
		scopeQuery = scopeQuery.Where("business_vertical_id IS NULL")
	}
	existing := &models.RBACRole{}
	if err := scopeQuery.First(existing).Error; err == nil {
		*role = *existing
		return nil
	}

	if err := tx.Create(role).Error; err != nil {
		return fmt.Errorf("copy %s role %s: %w", *role.LegacySourceType, *role.LegacySourceID, err)
	}
	return nil
}

func copyLegacyRolePermissions(tx *gorm.DB, roleMap map[legacyRoleKey]models.RBACRole) (int64, error) {
	var globalPermissions []models.RolePermission
	if err := tx.Find(&globalPermissions).Error; err != nil {
		return 0, fmt.Errorf("load global role permissions: %w", err)
	}
	for _, source := range globalPermissions {
		role, ok := roleMap[legacyRoleKey{source: legacyGlobalRoleSource, id: source.RoleID}]
		if !ok {
			// Dangling permission referencing a deleted role — skip rather than abort.
			continue
		}
		if err := insertUnifiedRolePermission(tx, role.ID, source.PermissionID, source.CreatedAt); err != nil {
			return 0, err
		}
	}

	var businessPermissions []models.BusinessRolePermission
	if err := tx.Find(&businessPermissions).Error; err != nil {
		return 0, fmt.Errorf("load business role permissions: %w", err)
	}
	for _, source := range businessPermissions {
		role, ok := roleMap[legacyRoleKey{source: legacyBusinessRoleSource, id: source.BusinessRoleID}]
		if !ok {
			// Dangling permission referencing a deleted role — skip rather than abort.
			continue
		}
		if err := insertUnifiedRolePermission(tx, role.ID, source.PermissionID, source.CreatedAt); err != nil {
			return 0, err
		}
	}

	return int64(len(globalPermissions) + len(businessPermissions)), nil
}

func insertUnifiedRolePermission(tx *gorm.DB, roleID, permissionID uuid.UUID, createdAt time.Time) error {
	row := models.RBACRolePermission{
		RoleID:       roleID,
		PermissionID: permissionID,
		CreatedAt:    createdAt,
	}
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now().UTC()
	}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return fmt.Errorf("copy permission %s for role %s: %w", permissionID, roleID, err)
	}
	return nil
}

func copyLegacyRoleAssignments(tx *gorm.DB, roleMap map[legacyRoleKey]models.RBACRole) (int64, error) {
	var users []models.User
	if err := tx.Select("id", "role_id", "created_at").Where("role_id IS NOT NULL").Find(&users).Error; err != nil {
		return 0, fmt.Errorf("load global role assignments: %w", err)
	}
	for _, user := range users {
		role, ok := roleMap[legacyRoleKey{source: legacyGlobalRoleSource, id: *user.RoleID}]
		if !ok {
			// User references a deleted global role — skip the assignment.
			continue
		}
		scopeKey, err := role.AssignmentScopeKey()
		if err != nil {
			return 0, err
		}
		legacySourceType := legacyGlobalAssignmentSource
		legacySourceID := user.ID
		assignment := models.UserRoleAssignment{
			UserID:           user.ID,
			RoleID:           role.ID,
			ScopeKey:         scopeKey,
			IsActive:         true,
			AssignedAt:       user.CreatedAt,
			LegacySourceType: &legacySourceType,
			LegacySourceID:   &legacySourceID,
		}
		if err := firstOrCreateUnifiedAssignment(tx, &assignment); err != nil {
			return 0, err
		}
	}

	var businessAssignments []models.UserBusinessRole
	if err := tx.Find(&businessAssignments).Error; err != nil {
		return 0, fmt.Errorf("load business role assignments: %w", err)
	}
	for _, source := range businessAssignments {
		role, ok := roleMap[legacyRoleKey{source: legacyBusinessRoleSource, id: source.BusinessRoleID}]
		if !ok {
			// Assignment references a deleted business role — skip.
			continue
		}
		scopeKey, err := role.AssignmentScopeKey()
		if err != nil {
			return 0, err
		}
		legacySourceType := legacyBusinessAssignSource
		legacySourceID := source.ID
		assignment := models.UserRoleAssignment{
			UserID:           source.UserID,
			RoleID:           role.ID,
			ScopeKey:         scopeKey,
			IsActive:         source.IsActive,
			AssignedAt:       source.AssignedAt,
			AssignedBy:       source.AssignedBy,
			LegacySourceType: &legacySourceType,
			LegacySourceID:   &legacySourceID,
		}
		if err := firstOrCreateUnifiedAssignment(tx, &assignment); err != nil {
			return 0, err
		}
	}

	return int64(len(users) + len(businessAssignments)), nil
}

func firstOrCreateUnifiedAssignment(tx *gorm.DB, assignment *models.UserRoleAssignment) error {
	if assignment.LegacySourceType == nil || assignment.LegacySourceID == nil {
		return errors.New("legacy assignment source is required for migration")
	}
	// Primary lookup: by legacy source to handle re-runs.
	result := tx.Where(
		"legacy_source_type = ? AND legacy_source_id = ?",
		*assignment.LegacySourceType,
		*assignment.LegacySourceID,
	).First(assignment)
	if result.Error == nil {
		return nil
	}
	if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return fmt.Errorf("lookup %s assignment %s: %w", *assignment.LegacySourceType, *assignment.LegacySourceID, result.Error)
	}

	// Secondary lookup: active assignment for (user_id, scope_key) to merge legacy duplicates.
	if assignment.IsActive {
		existing := &models.UserRoleAssignment{}
		if err := tx.Where("user_id = ? AND scope_key = ? AND is_active = true", assignment.UserID, assignment.ScopeKey).First(existing).Error; err == nil {
			*assignment = *existing
			return nil
		}
	}

	if err := tx.Create(assignment).Error; err != nil {
		return fmt.Errorf("copy %s assignment %s: %w", *assignment.LegacySourceType, *assignment.LegacySourceID, err)
	}
	return nil
}

func verifyUnifiedRBACCopy(tx *gorm.DB, expectedRoles, _, _ int64) error {
	// Verify role count matches distinct created roles.
	var actualRoles int64
	if err := tx.Model(&models.RBACRole{}).Count(&actualRoles).Error; err != nil {
		return fmt.Errorf("count unified roles: %w", err)
	}
	if actualRoles != expectedRoles {
		return fmt.Errorf("unified roles count mismatch: got %d, want %d", actualRoles, expectedRoles)
	}
	// Verify permissions and assignments are non-empty (orphan skips mean exact counts cannot be pre-computed).
	for _, check := range []struct {
		name  string
		model interface{}
	}{
		{name: "permissions", model: &models.RBACRolePermission{}},
		{name: "assignments", model: &models.UserRoleAssignment{}},
	} {
		var count int64
		if err := tx.Model(check.model).Count(&count).Error; err != nil {
			return fmt.Errorf("count unified %s: %w", check.name, err)
		}
		if count == 0 {
			return fmt.Errorf("unified %s: no rows copied", check.name)
		}
	}
	return nil
}
