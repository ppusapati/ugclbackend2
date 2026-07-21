package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"p9e.in/ugcl/models"
)

type migrationState struct {
	permIDByName            map[string]string
	roleIDByName            map[string]string
	businessIDByCode        map[string]string
	businessRoleIDByKey     map[string]string // key: BUSINESS_CODE::ROLE_NAME
	businessRoleIDBySourceID map[string]string
	userIDByEmail           map[string]string
	siteIDByCode            map[string]string
	attributeIDByName       map[string]string
	migratedUserEmails      []string
	migratedUserIDs         []string
	sourcePermByID          map[string]models.Permission
	sourceRoleByID          map[string]models.Role
	sourceBusinessByID      map[string]models.BusinessVertical
	sourceBusinessRoleByID  map[string]models.BusinessRole
	sourceUserByID          map[string]models.User
	sourceSiteByID          map[string]models.Site
	sourceAttributeByID     map[string]models.Attribute
}

func main() {
	srcDSN := strings.TrimSpace(os.Getenv("SOURCE_DB_DSN"))
	dstDSN := strings.TrimSpace(os.Getenv("TARGET_DB_DSN"))
	if srcDSN == "" || dstDSN == "" {
		log.Fatal("SOURCE_DB_DSN and TARGET_DB_DSN are required")
	}

	src, err := gorm.Open(postgres.Open(srcDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("connect source failed: %v", err)
	}
	dst, err := gorm.Open(postgres.Open(dstDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("connect target failed: %v", err)
	}

	state := &migrationState{}
	if err := loadSourceState(src, state); err != nil {
		log.Fatalf("load source state failed: %v", err)
	}

	if err := dst.Transaction(func(tx *gorm.DB) error {
		if err := migratePermissions(tx, state); err != nil {
			return err
		}
		if err := migrateRoles(tx, state); err != nil {
			return err
		}
		if err := migrateBusinessVerticals(tx, state); err != nil {
			return err
		}
		if err := migrateBusinessRoles(tx, state); err != nil {
			return err
		}
		if err := migrateSites(tx, state); err != nil {
			return err
		}
		if err := migrateUsers(tx, state); err != nil {
			return err
		}
		if err := migrateRolePermissions(src, tx, state); err != nil {
			return err
		}
		if err := migrateBusinessRolePermissions(src, tx, state); err != nil {
			return err
		}
		if err := migrateUserBusinessRoles(src, tx, state); err != nil {
			return err
		}
		if err := migrateUserSiteAccess(src, tx, state); err != nil {
			return err
		}
		if err := migrateAttributes(src, tx, state); err != nil {
			return err
		}
		if err := migrateUserAttributes(src, tx, state); err != nil {
			return err
		}
		if err := migrateActiveBusinessContext(src, tx, state); err != nil {
			return err
		}
		if err := migrateTrustedDevices(src, tx, state); err != nil {
			return err
		}
		if err := migrateMobileTokens(src, tx, state); err != nil {
			return err
		}
		if err := migrateWebPushSubscriptions(src, tx, state); err != nil {
			return err
		}
		if err := migrateUserLoginEvents(src, tx, state); err != nil {
			return err
		}
		return nil
	}); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	if err := printCounts(src, dst); err != nil {
		log.Fatalf("count verification failed: %v", err)
	}

	log.Println("user data migration completed successfully")
}

func loadSourceState(src *gorm.DB, s *migrationState) error {
	var perms []models.Permission
	if err := src.Find(&perms).Error; err != nil {
		return err
	}
	s.sourcePermByID = make(map[string]models.Permission, len(perms))
	for _, p := range perms {
		s.sourcePermByID[p.ID.String()] = p
	}

	var roles []models.Role
	if err := src.Find(&roles).Error; err != nil {
		return err
	}
	s.sourceRoleByID = make(map[string]models.Role, len(roles))
	for _, r := range roles {
		s.sourceRoleByID[r.ID.String()] = r
	}

	var businesses []models.BusinessVertical
	if err := src.Find(&businesses).Error; err != nil {
		return err
	}
	s.sourceBusinessByID = make(map[string]models.BusinessVertical, len(businesses))
	for _, b := range businesses {
		s.sourceBusinessByID[b.ID.String()] = b
	}

	var businessRoles []models.BusinessRole
	if err := src.Find(&businessRoles).Error; err != nil {
		return err
	}
	s.sourceBusinessRoleByID = make(map[string]models.BusinessRole, len(businessRoles))
	for _, br := range businessRoles {
		s.sourceBusinessRoleByID[br.ID.String()] = br
	}

	var users []models.User
	if err := src.Find(&users).Error; err != nil {
		return err
	}
	s.sourceUserByID = make(map[string]models.User, len(users))
	for _, u := range users {
		s.sourceUserByID[u.ID.String()] = u
	}

	var sites []models.Site
	if err := src.Unscoped().Find(&sites).Error; err != nil {
		return err
	}
	s.sourceSiteByID = make(map[string]models.Site, len(sites))
	for _, st := range sites {
		s.sourceSiteByID[st.ID.String()] = st
	}

	var attrs []models.Attribute
	if err := src.Find(&attrs).Error; err != nil {
		return err
	}
	s.sourceAttributeByID = make(map[string]models.Attribute, len(attrs))
	for _, a := range attrs {
		s.sourceAttributeByID[a.ID.String()] = a
	}

	return nil
}

func migratePermissions(dst *gorm.DB, s *migrationState) error {
	items := make([]models.Permission, 0, len(s.sourcePermByID))
	for _, p := range s.sourcePermByID {
		items = append(items, p)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })

	for _, p := range items {
		if err := dst.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"description", "resource", "action", "updated_at"}),
		}).Create(&p).Error; err != nil {
			return err
		}
	}

	var out []models.Permission
	if err := dst.Find(&out).Error; err != nil {
		return err
	}
	s.permIDByName = map[string]string{}
	for _, p := range out {
		s.permIDByName[p.Name] = p.ID.String()
	}
	return nil
}

func migrateRoles(dst *gorm.DB, s *migrationState) error {
	items := make([]models.Role, 0, len(s.sourceRoleByID))
	for _, r := range s.sourceRoleByID {
		items = append(items, r)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })

	for _, r := range items {
		if err := dst.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"description", "is_active", "is_global", "level", "updated_at"}),
		}).Create(&r).Error; err != nil {
			return err
		}
	}

	var out []models.Role
	if err := dst.Find(&out).Error; err != nil {
		return err
	}
	s.roleIDByName = map[string]string{}
	for _, r := range out {
		s.roleIDByName[r.Name] = r.ID.String()
	}
	return nil
}

func migrateBusinessVerticals(dst *gorm.DB, s *migrationState) error {
	items := make([]models.BusinessVertical, 0, len(s.sourceBusinessByID))
	for _, b := range s.sourceBusinessByID {
		items = append(items, b)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Code < items[j].Code })

	for _, b := range items {
		if err := dst.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "description", "is_active", "settings", "updated_at"}),
		}).Create(&b).Error; err != nil {
			return err
		}
	}

	var out []models.BusinessVertical
	if err := dst.Find(&out).Error; err != nil {
		return err
	}
	s.businessIDByCode = map[string]string{}
	for _, b := range out {
		s.businessIDByCode[b.Code] = b.ID.String()
	}
	return nil
}

func migrateBusinessRoles(dst *gorm.DB, s *migrationState) error {
	items := make([]models.BusinessRole, 0, len(s.sourceBusinessRoleByID))
	for _, br := range s.sourceBusinessRoleByID {
		items = append(items, br)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name+items[i].BusinessVerticalID.String() < items[j].Name+items[j].BusinessVerticalID.String()
	})

	for _, br := range items {
		srcBusiness, ok := s.sourceBusinessByID[br.BusinessVerticalID.String()]
		if !ok {
			continue
		}
		dstBusinessID, ok := s.businessIDByCode[srcBusiness.Code]
		if !ok {
			continue
		}
		br.BusinessVerticalID = parseUUID(dstBusinessID)
		if err := dst.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "display_name", "description", "business_vertical_id", "is_active", "level", "updated_at"}),
		}).Create(&br).Error; err != nil {
			return err
		}
	}

	var out []models.BusinessRole
	if err := dst.Find(&out).Error; err != nil {
		return err
	}
	s.businessRoleIDByKey = map[string]string{}
	s.businessRoleIDBySourceID = map[string]string{}
	for _, br := range out {
		var bv models.BusinessVertical
		if err := dst.First(&bv, "id = ?", br.BusinessVerticalID).Error; err != nil {
			continue
		}
		key := bv.Code + "::" + br.Name
		s.businessRoleIDByKey[key] = br.ID.String()
		s.businessRoleIDBySourceID[br.ID.String()] = br.ID.String()
	}
	return nil
}

func migrateSites(dst *gorm.DB, s *migrationState) error {
	items := make([]models.Site, 0, len(s.sourceSiteByID))
	for _, st := range s.sourceSiteByID {
		items = append(items, st)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Code < items[j].Code })

	for _, st := range items {
		srcBusiness, ok := s.sourceBusinessByID[st.BusinessVerticalID.String()]
		if !ok {
			continue
		}
		dstBusinessID, ok := s.businessIDByCode[srcBusiness.Code]
		if !ok {
			continue
		}

		var existing models.Site
		err := dst.Unscoped().Where("code = ?", st.Code).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			st.BusinessVerticalID = parseUUID(dstBusinessID)
			if err := dst.Unscoped().Create(&st).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			existing.Name = st.Name
			existing.Description = st.Description
			existing.BusinessVerticalID = parseUUID(dstBusinessID)
			existing.Location = st.Location
			existing.Geofence = st.Geofence
			existing.IsActive = st.IsActive
			existing.DeletedAt = st.DeletedAt
			existing.UpdatedAt = st.UpdatedAt
			if err := dst.Unscoped().Save(&existing).Error; err != nil {
				return err
			}
		}
	}

	var out []models.Site
	if err := dst.Unscoped().Find(&out).Error; err != nil {
		return err
	}
	s.siteIDByCode = map[string]string{}
	for _, st := range out {
		s.siteIDByCode[st.Code] = st.ID.String()
	}
	return nil
}

func migrateUsers(dst *gorm.DB, s *migrationState) error {
	items := make([]models.User, 0, len(s.sourceUserByID))
	for _, u := range s.sourceUserByID {
		items = append(items, u)
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Email) < strings.ToLower(items[j].Email) })

	s.userIDByEmail = map[string]string{}
	for _, u := range items {
		if strings.TrimSpace(u.Email) == "" {
			continue
		}
		var roleIDPtr *string
		if u.RoleID != nil {
			srcRole, ok := s.sourceRoleByID[u.RoleID.String()]
			if ok {
				if rid, ok := s.roleIDByName[srcRole.Name]; ok {
					roleIDPtr = &rid
				}
			}
		}
		var businessIDPtr *string
		if u.BusinessVerticalID != nil {
			srcBiz, ok := s.sourceBusinessByID[u.BusinessVerticalID.String()]
			if ok {
				if bid, ok := s.businessIDByCode[srcBiz.Code]; ok {
					businessIDPtr = &bid
				}
			}
		}

		var existing models.User
		err := dst.Where("email = ?", u.Email).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			var byPhone models.User
			phoneErr := dst.Where("phone = ?", u.Phone).First(&byPhone).Error
			if phoneErr == nil {
				byPhone.Name = u.Name
				byPhone.Email = u.Email
				byPhone.PasswordHash = u.PasswordHash
				byPhone.IsActive = u.IsActive
				byPhone.CreatedAt = u.CreatedAt
				byPhone.UpdatedAt = u.UpdatedAt
				if roleIDPtr != nil {
					rid := parseUUID(*roleIDPtr)
					byPhone.RoleID = &rid
				} else {
					byPhone.RoleID = nil
				}
				if businessIDPtr != nil {
					bid := parseUUID(*businessIDPtr)
					byPhone.BusinessVerticalID = &bid
				} else {
					byPhone.BusinessVerticalID = nil
				}
				if sErr := dst.Save(&byPhone).Error; sErr != nil {
					return sErr
				}
				continue
			}
			if phoneErr != nil && !errors.Is(phoneErr, gorm.ErrRecordNotFound) {
				return phoneErr
			}

			u.RoleID = nil
			if roleIDPtr != nil {
				rid := parseUUID(*roleIDPtr)
				u.RoleID = &rid
			}
			u.BusinessVerticalID = nil
			if businessIDPtr != nil {
				bid := parseUUID(*businessIDPtr)
				u.BusinessVerticalID = &bid
			}
			if err := dst.Create(&u).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			existing.Name = u.Name
			existing.Phone = u.Phone
			existing.PasswordHash = u.PasswordHash
			existing.IsActive = u.IsActive
			existing.CreatedAt = u.CreatedAt
			existing.UpdatedAt = u.UpdatedAt
			if roleIDPtr != nil {
				rid := parseUUID(*roleIDPtr)
				existing.RoleID = &rid
			} else {
				existing.RoleID = nil
			}
			if businessIDPtr != nil {
				bid := parseUUID(*businessIDPtr)
				existing.BusinessVerticalID = &bid
			} else {
				existing.BusinessVerticalID = nil
			}
			if err := dst.Save(&existing).Error; err != nil {
				return err
			}
		}
	}

	var out []models.User
	if err := dst.Find(&out).Error; err != nil {
		return err
	}
	s.userIDByEmail = map[string]string{}
	s.migratedUserEmails = make([]string, 0, len(items))
	s.migratedUserIDs = make([]string, 0, len(items))
	for _, u := range out {
		email := strings.ToLower(strings.TrimSpace(u.Email))
		if email == "" {
			continue
		}
		s.userIDByEmail[email] = u.ID.String()
	}
	for _, u := range items {
		email := strings.ToLower(strings.TrimSpace(u.Email))
		if id, ok := s.userIDByEmail[email]; ok {
			s.migratedUserEmails = append(s.migratedUserEmails, email)
			s.migratedUserIDs = append(s.migratedUserIDs, id)
		}
	}
	return nil
}

func migrateRolePermissions(src, dst *gorm.DB, s *migrationState) error {
	type rp struct {
		RoleID       string
		PermissionID string
	}
	var srcRows []rp
	if err := src.Table("role_permissions").Select("role_id", "permission_id").Scan(&srcRows).Error; err != nil {
		return err
	}

	if err := dst.Exec("DELETE FROM role_permissions").Error; err != nil {
		return err
	}

	for _, row := range srcRows {
		srcRole, okR := s.sourceRoleByID[row.RoleID]
		srcPerm, okP := s.sourcePermByID[row.PermissionID]
		if !okR || !okP {
			continue
		}
		dstRoleID, okR := s.roleIDByName[srcRole.Name]
		dstPermID, okP := s.permIDByName[srcPerm.Name]
		if !okR || !okP {
			continue
		}
		if err := dst.Exec(
			"INSERT INTO role_permissions (role_id, permission_id, created_at) VALUES (?, ?, ?) ON CONFLICT DO NOTHING",
			dstRoleID, dstPermID, time.Now().UTC(),
		).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateBusinessRolePermissions(src, dst *gorm.DB, s *migrationState) error {
	type brp struct {
		BusinessRoleID string
		PermissionID   string
	}
	var srcRows []brp
	if err := src.Table("business_role_permissions").Select("business_role_id", "permission_id").Scan(&srcRows).Error; err != nil {
		return err
	}

	if err := dst.Exec("DELETE FROM business_role_permissions").Error; err != nil {
		return err
	}

	for _, row := range srcRows {
		srcBR, okBR := s.sourceBusinessRoleByID[row.BusinessRoleID]
		srcPerm, okP := s.sourcePermByID[row.PermissionID]
		if !okBR || !okP {
			continue
		}
		dstBRID, okBR := s.businessRoleIDBySourceID[srcBR.ID.String()]
		if !okBR {
			srcBiz, okBiz := s.sourceBusinessByID[srcBR.BusinessVerticalID.String()]
			if !okBiz {
				continue
			}
			key := srcBiz.Code + "::" + srcBR.Name
			dstBRID, okBR = s.businessRoleIDByKey[key]
		}
		dstPermID, okP := s.permIDByName[srcPerm.Name]
		if !okBR || !okP {
			continue
		}

		if err := dst.Exec(
			"INSERT INTO business_role_permissions (business_role_id, permission_id, created_at) VALUES (?, ?, ?) ON CONFLICT DO NOTHING",
			dstBRID, dstPermID, time.Now().UTC(),
		).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateUserBusinessRoles(src, dst *gorm.DB, s *migrationState) error {
	type ubr struct {
		UserID         string
		BusinessRoleID string
		IsActive       bool
		AssignedAt     time.Time
		AssignedBy     *string
	}
	var srcRows []ubr
	if err := src.Table("user_business_roles").Select("user_id", "business_role_id", "is_active", "assigned_at", "assigned_by").Scan(&srcRows).Error; err != nil {
		return err
	}

	if len(s.migratedUserIDs) > 0 {
		if err := dst.Exec("DELETE FROM user_business_roles WHERE user_id IN ?", s.migratedUserIDs).Error; err != nil {
			return err
		}
	}

	for _, row := range srcRows {
		srcUser, okU := s.sourceUserByID[row.UserID]
		srcBR, okBR := s.sourceBusinessRoleByID[row.BusinessRoleID]
		if !okU || !okBR {
			continue
		}
		dstUID, okU := s.userIDByEmail[strings.ToLower(srcUser.Email)]
		dstBRID, okBR := s.businessRoleIDBySourceID[srcBR.ID.String()]
		if !okBR {
			srcBiz, okBiz := s.sourceBusinessByID[srcBR.BusinessVerticalID.String()]
			if !okBiz {
				continue
			}
			dstBRID, okBR = s.businessRoleIDByKey[srcBiz.Code+"::"+srcBR.Name]
		}
		if !okU || !okBR {
			continue
		}

		if err := dst.Exec(
			"INSERT INTO user_business_roles (id, user_id, business_role_id, is_active, assigned_at, assigned_by, created_at, updated_at) VALUES (gen_random_uuid(), ?, ?, ?, ?, ?, NOW(), NOW())",
			dstUID, dstBRID, row.IsActive, row.AssignedAt, row.AssignedBy,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateUserSiteAccess(src, dst *gorm.DB, s *migrationState) error {
	type usa struct {
		UserID     string
		SiteID     string
		CanRead    bool
		CanCreate  bool
		CanUpdate  bool
		CanDelete  bool
		AssignedAt time.Time
		AssignedBy *string
	}
	var srcRows []usa
	if err := src.Table("user_site_accesses").Select("user_id", "site_id", "can_read", "can_create", "can_update", "can_delete", "assigned_at", "assigned_by").Scan(&srcRows).Error; err != nil {
		// Table may not exist in older schema; treat as non-fatal.
		if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			return nil
		}
		return err
	}

	if len(s.migratedUserIDs) > 0 {
		if err := dst.Exec("DELETE FROM user_site_accesses WHERE user_id IN ?", s.migratedUserIDs).Error; err != nil {
			return err
		}
	}

	for _, row := range srcRows {
		srcUser, okU := s.sourceUserByID[row.UserID]
		srcSite, okS := s.sourceSiteByID[row.SiteID]
		if !okU || !okS {
			continue
		}
		dstUID, okU := s.userIDByEmail[strings.ToLower(srcUser.Email)]
		dstSID, okS := s.siteIDByCode[srcSite.Code]
		if !okU || !okS {
			continue
		}

		if err := dst.Exec(
			"INSERT INTO user_site_accesses (id, user_id, site_id, can_read, can_create, can_update, can_delete, assigned_at, assigned_by, created_at, updated_at) VALUES (gen_random_uuid(), ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())",
			dstUID, dstSID, row.CanRead, row.CanCreate, row.CanUpdate, row.CanDelete, row.AssignedAt, row.AssignedBy,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateAttributes(src, dst *gorm.DB, s *migrationState) error {
	items := make([]models.Attribute, 0, len(s.sourceAttributeByID))
	for _, a := range s.sourceAttributeByID {
		items = append(items, a)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	for _, a := range items {
		if err := dst.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "name"}},
			DoUpdates: clause.AssignmentColumns([]string{"display_name", "description", "type", "data_type", "is_system", "is_active", "metadata", "updated_at"}),
		}).Create(&a).Error; err != nil {
			return err
		}
	}

	var out []models.Attribute
	if err := dst.Find(&out).Error; err != nil {
		return err
	}
	s.attributeIDByName = map[string]string{}
	for _, a := range out {
		s.attributeIDByName[a.Name] = a.ID.String()
	}
	return nil
}

func migrateUserAttributes(src, dst *gorm.DB, s *migrationState) error {
	type ua struct {
		UserID      string
		AttributeID string
		Value       string
		IsActive    bool
		ValidFrom   time.Time
		ValidUntil  *time.Time
		AssignedBy  string
	}
	var srcRows []ua
	if err := src.Table("user_attributes").Select("user_id", "attribute_id", "value", "is_active", "valid_from", "valid_until", "assigned_by").Scan(&srcRows).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			return nil
		}
		return err
	}

	if len(s.migratedUserIDs) > 0 {
		if err := dst.Exec("DELETE FROM user_attributes WHERE user_id IN ?", s.migratedUserIDs).Error; err != nil {
			return err
		}
	}

	for _, row := range srcRows {
		srcUser, okU := s.sourceUserByID[row.UserID]
		srcAttr, okA := s.sourceAttributeByID[row.AttributeID]
		if !okU || !okA {
			continue
		}
		dstUID, okU := s.userIDByEmail[strings.ToLower(srcUser.Email)]
		dstAID, okA := s.attributeIDByName[srcAttr.Name]
		if !okU || !okA {
			continue
		}
		if err := dst.Exec(
			"INSERT INTO user_attributes (id, user_id, attribute_id, value, is_active, valid_from, valid_until, assigned_by, created_at, updated_at) VALUES (gen_random_uuid(), ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())",
			dstUID, dstAID, row.Value, row.IsActive, row.ValidFrom, row.ValidUntil, nullableUUID(row.AssignedBy),
		).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateActiveBusinessContext(src, dst *gorm.DB, s *migrationState) error {
	type abc struct {
		UserID     string
		BusinessID string
		ClientKey  string
	}
	var srcRows []abc
	if err := src.Table("user_active_business_contexts").Select("user_id", "business_id", "client_key").Scan(&srcRows).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			return nil
		}
		return err
	}

	for _, row := range srcRows {
		srcUser, okU := s.sourceUserByID[row.UserID]
		srcBiz, okB := s.sourceBusinessByID[row.BusinessID]
		if !okU || !okB {
			continue
		}
		dstUID, okU := s.userIDByEmail[strings.ToLower(srcUser.Email)]
		dstBID, okB := s.businessIDByCode[srcBiz.Code]
		if !okU || !okB {
			continue
		}
		if err := dst.Exec(
			"INSERT INTO user_active_business_contexts (id, user_id, business_id, client_key, created_at, updated_at) VALUES (gen_random_uuid(), ?, ?, ?, NOW(), NOW()) ON CONFLICT (user_id, client_key) DO UPDATE SET business_id = EXCLUDED.business_id, updated_at = NOW()",
			dstUID, dstBID, row.ClientKey,
		).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateTrustedDevices(src, dst *gorm.DB, s *migrationState) error {
	var srcRows []models.TrustedDevice
	if err := src.Unscoped().Find(&srcRows).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			return nil
		}
		return err
	}

	if len(s.migratedUserIDs) > 0 {
		if err := dst.Unscoped().Exec("DELETE FROM trusted_devices WHERE user_id IN ?", s.migratedUserIDs).Error; err != nil {
			return err
		}
	}

	for _, td := range srcRows {
		srcUser, ok := s.sourceUserByID[td.UserID.String()]
		if !ok {
			continue
		}
		dstUID, ok := s.userIDByEmail[strings.ToLower(srcUser.Email)]
		if !ok {
			continue
		}
		td.UserID = parseUUID(dstUID)
		if err := dst.Unscoped().Create(&td).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateMobileTokens(src, dst *gorm.DB, s *migrationState) error {
	var srcRows []models.MobilePushToken
	if err := src.Find(&srcRows).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			return nil
		}
		return err
	}

	for _, t := range srcRows {
		srcUser, ok := s.sourceUserByID[t.UserID]
		if ok {
			if dstUID, ok := s.userIDByEmail[strings.ToLower(srcUser.Email)]; ok {
				t.UserID = dstUID
			}
		}
		if err := dst.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "token"}},
			DoUpdates: clause.AssignmentColumns([]string{"user_id", "platform", "device_id", "device_name", "app_version", "is_active", "last_seen_at", "updated_at"}),
		}).Create(&t).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateWebPushSubscriptions(src, dst *gorm.DB, s *migrationState) error {
	var srcRows []models.WebPushSubscription
	if err := src.Find(&srcRows).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			return nil
		}
		return err
	}

	for _, t := range srcRows {
		srcUser, ok := s.sourceUserByID[t.UserID]
		if ok {
			if dstUID, ok := s.userIDByEmail[strings.ToLower(srcUser.Email)]; ok {
				t.UserID = dstUID
			}
		}
		if err := dst.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "endpoint"}},
			DoUpdates: clause.AssignmentColumns([]string{"user_id", "p256dh", "auth", "expiration_time", "user_agent", "updated_at"}),
		}).Create(&t).Error; err != nil {
			return err
		}
	}
	return nil
}

func migrateUserLoginEvents(src, dst *gorm.DB, s *migrationState) error {
	var srcRows []models.UserLoginEvent
	if err := src.Find(&srcRows).Error; err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			return nil
		}
		return err
	}

	for _, e := range srcRows {
		srcUser, ok := s.sourceUserByID[e.UserID.String()]
		if !ok {
			continue
		}
		dstUID, ok := s.userIDByEmail[strings.ToLower(srcUser.Email)]
		if !ok {
			continue
		}
		e.UserID = parseUUID(dstUID)
		if err := dst.Clauses(clause.OnConflict{DoNothing: true}).Create(&e).Error; err != nil {
			return err
		}
	}
	return nil
}

func printCounts(src, dst *gorm.DB) error {
	tables := []string{
		"users",
		"roles",
		"permissions",
		"role_permissions",
		"business_verticals",
		"business_roles",
		"business_role_permissions",
		"user_business_roles",
		"sites",
		"user_site_accesses",
		"attributes",
		"user_attributes",
		"user_active_business_contexts",
		"trusted_devices",
		"mobile_push_tokens",
		"web_push_subscriptions",
		"user_login_events",
	}

	for _, t := range tables {
		var sc, dc int64
		sErr := src.Table(t).Count(&sc).Error
		dErr := dst.Table(t).Count(&dc).Error
		if sErr != nil || dErr != nil {
			fmt.Printf("COUNT %s source_err=%v target_err=%v\n", t, sErr, dErr)
			continue
		}
		fmt.Printf("COUNT %s source=%d target=%d\n", t, sc, dc)
	}
	return nil
}

func parseUUID(v string) uuid.UUID {
	id, err := uuid.Parse(v)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func nullableUUID(v string) interface{} {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return v
}
