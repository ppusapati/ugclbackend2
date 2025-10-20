# Site-Level Permission System

## Overview

This document explains the site-level permission system that allows fine-grained access control for users within business verticals. Instead of giving users access to all data in a business vertical, you can now restrict access to specific sites.

## Use Case

- **Water Works**: 4 sites (Water Site A, B, C, D)
- **Solar Works**: 12 sites (Solar Site 01-12)

With site-level permissions, you can:
- Give an Engineer access to only Water Site A and B (not C and D)
- Give an Operator access to only Solar Site 01, 02, 03 (not the other 9)
- Control read/create/update/delete permissions per site

## Architecture

### 1. Database Models

#### `Site` Model
Represents a physical site within a business vertical.

```go
type Site struct {
    ID                 uuid.UUID
    Name               string              // "Water Site A"
    Code               string              // "WATER_SITE_A"
    Description        string
    BusinessVerticalID uuid.UUID
    Location           *string             // JSON: {lat, lng, address}
    IsActive           bool
}
```

#### `UserSiteAccess` Model
Defines which sites a user can access and their permissions for each site.

```go
type UserSiteAccess struct {
    ID        uuid.UUID
    UserID    uuid.UUID
    SiteID    uuid.UUID

    // Granular permissions per site
    CanRead   bool
    CanCreate bool
    CanUpdate bool
    CanDelete bool

    AssignedBy *uuid.UUID  // Who granted this access
}
```

## Implementation Steps

### Step 1: Run Migrations

Add the following to your `main.go` or migration runner:

```go
import "p9e.in/ugcl/config"

func main() {
    // ... existing code ...

    // Migrate site tables
    config.MigrateSites()

    // Seed default sites
    config.SeedSites()
}
```

This will create:
- 4 sites for Water Works
- 12 sites for Solar Works

### Step 2: Update Routes

Update [routes/business_routes.go](../routes/business_routes.go) to use site-aware middleware:

```go
// Water Works specific routes with site-level access control
water := business.PathPrefix("/water").Subrouter()

// Add site access middleware
water.Use(middleware.RequireSiteAccess())

// Replace old handlers with site-aware versions
water.Handle("/reports/tanker",
    middleware.RequireBusinessPermission("water:read_consumption")(
        http.HandlerFunc(handlers.GetAllWaterTankerReportsWithSiteFilter))).Methods("GET")

water.Handle("/reports/tanker",
    middleware.RequireBusinessPermission("inventory:create")(
        http.HandlerFunc(handlers.CreateWaterTankerReportWithSiteCheck))).Methods("POST")

water.Handle("/reports/tanker/{id}",
    middleware.RequireBusinessPermission("inventory:update")(
        http.HandlerFunc(handlers.UpdateWaterTankerReportWithSiteCheck))).Methods("PUT")

water.Handle("/reports/tanker/{id}",
    middleware.RequireBusinessPermission("inventory:delete")(
        http.HandlerFunc(handlers.DeleteWaterTankerReportWithSiteCheck))).Methods("DELETE")
```

### Step 3: Add Site Management Routes

Add routes for managing site access:

```go
// Site management endpoints
business.HandleFunc("/sites", handlers.GetAllSites).Methods("GET")
business.HandleFunc("/sites/my-access", handlers.GetUserSites).Methods("GET")

business.Handle("/sites/access",
    middleware.RequireBusinessPermission("business_manage_users")(
        http.HandlerFunc(handlers.AssignUserSiteAccess))).Methods("POST")

business.Handle("/sites/access/{accessId}",
    middleware.RequireBusinessPermission("business_manage_users")(
        http.HandlerFunc(handlers.RevokeUserSiteAccess))).Methods("DELETE")

business.HandleFunc("/sites/{siteId}/users", handlers.GetSiteUsers).Methods("GET")
```

## API Usage Examples

### 1. Get All Sites in a Business Vertical

```bash
GET /api/v1/business/WATER/sites
```

Response:
```json
[
  {
    "id": "uuid-1",
    "name": "Water Site A",
    "code": "WATER_SITE_A",
    "description": "Primary water distribution site",
    "businessVerticalId": "uuid-water",
    "isActive": true
  },
  {
    "id": "uuid-2",
    "name": "Water Site B",
    "code": "WATER_SITE_B",
    "description": "Secondary water distribution site",
    "businessVerticalId": "uuid-water",
    "isActive": true
  }
]
```

### 2. Get User's Accessible Sites

```bash
GET /api/v1/business/WATER/sites/my-access
```

Response:
```json
[
  {
    "id": "uuid-1",
    "name": "Water Site A",
    "code": "WATER_SITE_A",
    "canRead": true,
    "canCreate": true,
    "canUpdate": true,
    "canDelete": false
  },
  {
    "id": "uuid-2",
    "name": "Water Site B",
    "code": "WATER_SITE_B",
    "canRead": true,
    "canCreate": true,
    "canUpdate": false,
    "canDelete": false
  }
]
```

### 3. Assign Site Access to a User

Only users with `business_manage_users` permission can do this.

```bash
POST /api/v1/business/WATER/sites/access
Content-Type: application/json

{
  "userId": "user-uuid",
  "siteId": "site-uuid",
  "canRead": true,
  "canCreate": true,
  "canUpdate": true,
  "canDelete": false
}
```

### 4. Get Water Reports (Filtered by Site Access)

```bash
GET /api/v1/business/WATER/water/reports/tanker
```

This will automatically return only reports from sites the user has access to.

### 5. Get Users with Access to a Site

```bash
GET /api/v1/business/WATER/sites/{siteId}/users
```

Response:
```json
[
  {
    "userId": "uuid-1",
    "name": "John Engineer",
    "phone": "1234567890",
    "canRead": true,
    "canCreate": true,
    "canUpdate": true,
    "canDelete": false
  }
]
```

## How It Works

### Authorization Flow

1. **User logs in** → JWT token with user ID
2. **User accesses business endpoint** → `RequireBusinessAccess()` middleware validates business access
3. **User accesses site-restricted endpoint** → `RequireSiteAccess()` middleware:
   - Loads all sites user has access to in this business
   - Stores accessible site IDs in request context
   - Stores per-site permissions (read/create/update/delete)
4. **Handler filters data** → Only returns data from accessible sites
5. **Handler checks operation permission** → Verifies user can create/update/delete in specific site

### Middleware Chain Example

```
Request → JWTMiddleware → RequireBusinessAccess → RequireSiteAccess → RequireBusinessPermission → Handler
```

## Example Scenarios

### Scenario 1: Engineer with Limited Site Access

**Setup:**
- User: Engineer (role with `inventory:create` and `inventory:update` permissions)
- Site Access: Water Site A (read + create + update), Water Site B (read only)

**What they can do:**
- ✅ View reports from Water Site A and B
- ✅ Create reports for Water Site A
- ✅ Update reports in Water Site A
- ❌ Create reports for Water Site B (no create permission)
- ❌ Update reports in Water Site B (no update permission)
- ❌ View/modify reports from Water Site C or D (no access)

### Scenario 2: Water Admin with Full Access

**Setup:**
- User: Water_Admin (role with all permissions)
- Site Access: All 4 water sites (all permissions)

**What they can do:**
- ✅ View, create, update, delete reports from all 4 water sites
- ✅ Assign site access to other users
- ✅ Manage all water-related operations

### Scenario 3: Supervisor with Update Rights

**Setup:**
- User: Supervisor (role with `inventory:update` permission)
- Site Access: Water Site A and C (read + update only)

**What they can do:**
- ✅ View reports from Water Site A and C
- ✅ Update reports in Water Site A and C
- ❌ Create new reports (role doesn't have inventory:create)
- ❌ Delete reports (no delete permission in site access)
- ❌ Access Water Site B or D

## Permission Matrix

For a user to perform an action, they need BOTH:

1. **Role Permission** (from BusinessRole)
2. **Site Permission** (from UserSiteAccess)

| Action | Required Role Permission | Required Site Permission |
|--------|-------------------------|--------------------------|
| Read   | `water:read_consumption` | `CanRead = true` |
| Create | `inventory:create` | `CanCreate = true` |
| Update | `inventory:update` | `CanUpdate = true` |
| Delete | `inventory:delete` | `CanDelete = true` |

## Migration from Business-Level to Site-Level

If you're currently using business-level permissions only:

1. Run migrations to create site tables
2. Seed default sites
3. **Assign all existing users access to all sites** in their business vertical:

```sql
-- Grant all current Water users access to all Water sites
INSERT INTO user_site_accesses (id, user_id, site_id, can_read, can_create, can_update, can_delete, assigned_at)
SELECT
    gen_random_uuid(),
    ubr.user_id,
    s.id,
    true,  -- can_read
    true,  -- can_create (adjust based on role)
    true,  -- can_update (adjust based on role)
    false, -- can_delete (adjust based on role)
    NOW()
FROM user_business_roles ubr
JOIN business_roles br ON br.id = ubr.business_role_id
JOIN sites s ON s.business_vertical_id = br.business_vertical_id
WHERE br.business_vertical_id = (SELECT id FROM business_verticals WHERE code = 'WATER');
```

4. Gradually restrict access as needed

## Security Considerations

1. **Admin Bypass**: Consider if super admins should bypass site restrictions
2. **Audit Trail**: Log all site access assignments and revocations
3. **Default Deny**: Users have no site access by default
4. **Cascading Deletes**: Handle what happens when a site is deleted
5. **Site Deactivation**: Deactivating a site should restrict access immediately

## Testing

Test cases to verify:

1. ✅ User can only see sites they have access to
2. ✅ User cannot create in sites without create permission
3. ✅ User cannot update in sites without update permission
4. ✅ User cannot delete in sites without delete permission
5. ✅ Reports are filtered by accessible sites
6. ✅ Creating a report for an inaccessible site is blocked
7. ✅ Admin can assign/revoke site access
8. ✅ Non-admin cannot assign site access

## Future Enhancements

- **Temporary Access**: Add expiration dates to UserSiteAccess
- **Access Requests**: Allow users to request access to additional sites
- **Role Templates**: Pre-defined site access patterns for common roles
- **Bulk Assignment**: Assign multiple users to multiple sites at once
- **Site Groups**: Group sites together for easier management
