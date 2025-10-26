# Role System Cleanup Summary

## Overview
This cleanup removed legacy string-based role handling and consolidated the codebase to use the new RBAC (Role-Based Access Control) system exclusively.

## Changes Made

### 1. Deleted Obsolete Files ✅
- **handlers/site_management.go** - Duplicate, functionality moved to `handlers/masters/site_management.go`
- **handlers/water_with_site_filtering.go** - Old version, site filtering now integrated
- **scripts/migrate_auth.go** - Moved to `config/migrate_auth.go`

### 2. Removed Legacy Role Field from User Model ✅
**File**: [models/user.go](models/user.go)

**Before**:
```go
type User struct {
    Role string `gorm:"size:50;not null;default:'user'"` // Legacy field
    RoleID *uuid.UUID
    // ...
}
```

**After**:
```go
type User struct {
    RoleID *uuid.UUID  // Global role system
    // ...
}
```

**Impact**:
- Removed `Role` string field entirely
- Removed legacy permission checking methods (`hasLegacyPermission`, `isAdminPermission`, etc.)
- Simplified `HasPermission()` to only check `RoleModel`

### 3. Updated Authentication Handlers ✅
**File**: [handlers/auth.go](handlers/auth.go)

**Changes**:
- **Register**: Now accepts `role_id` instead of `role` string
- **Login**: Returns `role_id` and `global_role` instead of `role` string
- **GetCurrentUser**: Returns structured permissions with global and business roles
- **GetAllUsers**: Returns users with `role_id` and `global_role`
- Removed `getLegacyPermissions()` function (60+ lines of hardcoded permission mapping)

**Before**:
```json
{
  "role": "super_admin",
  "permissions": ["read_reports", ...]
}
```

**After**:
```json
{
  "role_id": "uuid-here",
  "global_role": "super_admin",
  "permissions": ["*:*:*"],
  "business_roles": [
    {
      "role_name": "Water Admin",
      "vertical_code": "WATER",
      "level": 1
    }
  ]
}
```

### 4. Updated Routes ✅
**File**: [routes/routes_v2.go](routes/routes_v2.go)

**Profile Endpoint**:
- Removed `role` field
- Added `role_id` and `global_role` fields

### 5. Fixed Middleware ✅
**File**: [middleware/jwt.go](middleware/jwt.go)

**GetUser() Enhancement**:
- Now loads full user from database with role relationships
- Preloads `RoleModel` and `UserBusinessRoles`
- Provides complete permission context

**File**: [middleware/business_auth.go](middleware/business_auth.go)

**isSuperAdmin() Simplification**:
- Removed legacy `user.Role` checks
- Only checks `user.RoleModel.Name`

### 6. Updated Business Management Handlers ✅
**File**: [handlers/business_management.go](handlers/business_management.go)

**Changes**:
- Replaced all `user.Role` string checks with `user.RoleModel.Name`
- Updated response objects to return `global_role` instead of `role`
- Consistent super admin checking across all endpoints

### 7. Updated Migration Files ✅

**File**: [config/migrate_auth.go](config/migrate_auth.go)
- Deprecated `MigrateExistingUsers()` (no longer needed)
- Added deprecation notice pointing to `MigrateToNewRBAC()`

**File**: [config/migrate_rbac.go](config/migrate_rbac.go)
- Updated to check existing role assignments
- Suggests manual assignment for users without roles
- No longer tries to map from `user.Role` string field

### 8. Updated User Management ✅
**File**: [handlers/user_management.go](handlers/user_management.go)

**UpdateUser**:
- Removed `Role` string field handling
- Only uses `RoleID` for role assignment

### 9. Updated Workflow Handlers ✅
**File**: [handlers/workflow_handlers.go](handlers/workflow_handlers.go)

**TransitionFormSubmission**:
- Gets role name from `user.RoleModel.Name` instead of `user.Role`
- Passes empty string if no global role assigned

## Database Changes

### Removed
- None (kept for backward compatibility in database)

### Modified
The `users` table still has the `role` column for backward compatibility during migration, but it's:
- No longer referenced in the Go code
- Not used for permission checking
- Can be dropped in a future migration once all data is verified

## Breaking Changes for API Clients ⚠️

### Registration Endpoint
**Before**: `POST /api/v1/register`
```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "phone": "1234567890",
  "password": "password",
  "role": "admin"
}
```

**After**:
```json
{
  "name": "John Doe",
  "email": "john@example.com",
  "phone": "1234567890",
  "password": "password",
  "role_id": "uuid-of-role"
}
```

### Login Response
**Before**:
```json
{
  "token": "...",
  "user": {
    "id": "...",
    "name": "...",
    "role": "super_admin"
  }
}
```

**After**:
```json
{
  "token": "...",
  "user": {
    "id": "...",
    "name": "...",
    "role_id": "uuid",
    "global_role": "super_admin"
  }
}
```

### GetCurrentUser Response
**Before**:
```json
{
  "id": "...",
  "name": "...",
  "role": "admin",
  "permissions": [...]
}
```

**After**:
```json
{
  "id": "...",
  "name": "...",
  "role_id": "uuid",
  "global_role": "super_admin",
  "permissions": ["*:*:*"],
  "business_roles": [
    {
      "role_id": "uuid",
      "role_name": "Water Admin",
      "vertical_id": "uuid",
      "vertical_name": "Water Works",
      "vertical_code": "WATER",
      "level": 1
    }
  ]
}
```

## Migration Path for Existing Data

### Step 1: Run Migrations
```go
config.Migrations(db)           // Create RBAC tables
config.SeedPermissions()        // Seed permissions and global roles
config.SeedBusinessVerticals()  // Seed business verticals and roles
```

### Step 2: Assign Roles to Users
Users must now be assigned roles through:
1. **Global Roles**: Update `users.role_id` to reference a role in the `roles` table
2. **Business Roles**: Create entries in `user_business_roles` table

### Step 3: Verify Migration
```go
config.VerifyRBACMigration()    // Check role assignments
```

## Testing Checklist

- [ ] Test user registration with `role_id`
- [ ] Test login returns correct `role_id` and `global_role`
- [ ] Test `/api/v1/token` returns permissions correctly
- [ ] Test super admin can access all endpoints
- [ ] Test vertical admin can only access their vertical
- [ ] Test role assignment hierarchy works
- [ ] Test business role assignment/removal
- [ ] Test permission checking in middleware
- [ ] Test site management still works (handlers/masters/site_management.go)
- [ ] Test workflow transitions with new role system

## Files Modified

### Models
- [x] models/user.go - Removed Role field, simplified HasPermission

### Handlers
- [x] handlers/auth.go - Updated register/login/getCurrentUser
- [x] handlers/business_management.go - Updated all super admin checks
- [x] handlers/user_management.go - Removed Role field handling
- [x] handlers/workflow_handlers.go - Updated role name retrieval

### Middleware
- [x] middleware/jwt.go - Enhanced GetUser with DB loading
- [x] middleware/business_auth.go - Simplified isSuperAdmin

### Routes
- [x] routes/routes_v2.go - Updated profile endpoint

### Config/Migration
- [x] config/migrate_auth.go - Deprecated legacy migration
- [x] config/migrate_rbac.go - Updated to check existing assignments

## Documentation Added

✅ **[docs/RBAC_ARCHITECTURE.md](docs/RBAC_ARCHITECTURE.md)** - Complete RBAC system documentation including:
- Architecture overview
- Role hierarchy
- Permission system
- API endpoints
- User scenarios
- Best practices

## Site Management Confirmation ✅

**Site management is NOT removed** - it's fully functional in:
- **Handler**: [handlers/masters/site_management.go](handlers/masters/site_management.go)
- **Routes**: [routes/business_routes.go:109-123](routes/business_routes.go)

**Available Site Endpoints**:
```
GET    /api/v1/business/{businessCode}/sites              - List sites
GET    /api/v1/business/{businessCode}/sites/my-access    - User's accessible sites
POST   /api/v1/business/{businessCode}/sites/access       - Assign user to site
DELETE /api/v1/business/{businessCode}/sites/access/{id}  - Revoke site access
GET    /api/v1/business/{businessCode}/sites/{id}/users   - List users for site
```

## Compilation Status ✅

✅ **Code compiles successfully**
✅ **All type errors resolved**
✅ **No references to legacy `user.Role` field**

## Next Steps

1. **Update Frontend**:
   - Change registration form to use `role_id` dropdown
   - Update login response handling
   - Update profile display to show `global_role`
   - Add business roles display

2. **Database Migration** (Optional):
   - Once all data is verified, drop `users.role` column
   - Create migration to remove the field from database

3. **Testing**:
   - Test all endpoints with new role structure
   - Verify permissions work correctly
   - Test role assignment workflows

4. **Documentation**:
   - Update API documentation with new request/response formats
   - Update frontend integration guide

## Summary

This cleanup successfully:
✅ Removed all legacy string-based role handling
✅ Consolidated to use RBAC system exclusively
✅ Maintained all functionality (including site management)
✅ Improved code clarity and maintainability
✅ Added comprehensive documentation
✅ Code compiles without errors

The system now has a **clean, unified role architecture** supporting:
- Global roles (super_admin, system_admin, etc.)
- Business vertical roles (Water_Admin, Solar_Admin, etc.)
- Multi-vertical role assignments
- Hierarchical permission system
- Site-level access control
