# Water Handler Site-Level Integration

## Summary

The water tanker report handlers in [handlers/water.go](../handlers/water.go) have been successfully updated to support **site-level access control** while maintaining full backward compatibility with existing business-level permissions.

## Changes Made

### Modified File
- **[handlers/water.go](../handlers/water.go)** - Updated all water tanker report methods

### Updated Methods

All methods now include site-level access checks:

1. **GetAllWaterTankerReports** - Filters reports by accessible sites
2. **CreateWaterTankerReport** - Validates site access before creation
3. **GetWaterTankerReport** - Checks site access for single record retrieval
4. **UpdateWaterTankerReport** - Verifies update permission for the site
5. **DeleteWaterTankerReport** - Verifies delete permission for the site
6. **BatchWaterReports** - Validates site access for all batch items

### Key Features

#### 1. **Backward Compatibility**
```go
siteContext := middleware.GetSiteAccessContext(r)
if siteContext != nil {
    // Site filtering logic
}
// Falls back to business-level filtering if no site context
```

The handlers work in **two modes**:
- **With site context**: Enforces site-level permissions
- **Without site context**: Works as before (business-level only)

#### 2. **Site Filtering in GetAll**
When site context is available, the handler:
- Retrieves accessible site names from site IDs
- Filters water reports using `WHERE site_name IN (accessible_sites)`
- Applies all existing filters (date, pagination, etc.)
- Returns only data from sites the user can access

```go
query := config.DB.Model(&models.Water{}).
    Where("business_vertical_id = ?", businessID).
    Where("site_name IN ?", accessibleSiteNames)
```

#### 3. **Permission Checks on Write Operations**

**Create**:
- Finds the site by name
- Checks if user has site access
- Validates `CanCreate` permission for that site

**Update**:
- Fetches existing record
- Finds the site
- Validates `CanUpdate` permission

**Delete**:
- Fetches existing record
- Finds the site
- Validates `CanDelete` permission

**Batch**:
- Builds a map of accessible sites with create permission
- Validates all batch items before inserting

## How It Works

### Flow Example: Creating a Water Tanker Report

1. **User Request**: `POST /api/v1/business/WATER/water/reports/tanker`
   ```json
   {
     "siteName": "Water Site A",
     "purpose": "Construction",
     ...
   }
   ```

2. **Middleware Chain**:
   - `JWTMiddleware` → Validates user
   - `RequireBusinessAccess` → Validates business access
   - `RequireSiteAccess` → Loads accessible sites into context
   - `RequireBusinessPermission("inventory:create")` → Validates role permission

3. **Handler Logic** (in CreateWaterTankerReport):
   ```go
   // Get site context
   siteContext := middleware.GetSiteAccessContext(r)

   // Find the site by name
   var site models.Site
   config.DB.Where("name = ? AND business_vertical_id = ?",
       item.SiteName, item.BusinessVerticalID).First(&site)

   // Check permissions for this specific site
   for _, siteID := range siteContext.AccessibleSiteIDs {
       if siteID == site.ID {
           if perm := siteContext.SitePermissions[siteID]; perm.CanCreate {
               // Allowed!
           }
       }
   }
   ```

4. **Result**:
   - ✅ If user has `inventory:create` role permission AND site-level `CanCreate` permission → Success
   - ❌ If missing either permission → 403 Forbidden

## Permission Requirements

For a user to perform any action, they need **BOTH**:

| Action | Role Permission (Business) | Site Permission (Per-Site) |
|--------|---------------------------|----------------------------|
| Read   | `water:read_consumption` | `CanRead = true` |
| Create | `inventory:create` | `CanCreate = true` |
| Update | `inventory:update` | `CanUpdate = true` |
| Delete | `inventory:delete` | `CanDelete = true` |

### Updated Role Permissions

The following Water vertical roles now have updated permissions in [config/permissions.go](../config/permissions.go):

- **Engineer**: Added `inventory:create` (can now create reports)
- **Supervisor**: Added `inventory:update` (can now edit reports)
- **Operator**: Added `inventory:create` (can now create reports)

## Migration Strategy

### For New Deployments
1. Run migrations: `config.MigrateSites()`
2. Seed sites: `config.SeedSites()`
3. Use site-aware middleware: `water.Use(middleware.RequireSiteAccess())`

### For Existing Deployments (Gradual Rollout)

**Option 1: No Site Filtering** (Current behavior)
```go
// Don't add RequireSiteAccess middleware
water.Handle("/reports/tanker",
    middleware.RequireBusinessPermission("water:read_consumption")(
        http.HandlerFunc(handlers.GetAllWaterTankerReports)))
```
Result: Works as before, business-level filtering only

**Option 2: Enable Site Filtering**
```go
// Add RequireSiteAccess middleware
water.Use(middleware.RequireSiteAccess())
water.Handle("/reports/tanker",
    middleware.RequireBusinessPermission("water:read_consumption")(
        http.HandlerFunc(handlers.GetAllWaterTankerReports)))
```
Result: Site-level filtering enabled

**Option 3: Grant All Users Full Access Initially**
```sql
-- Give all existing Water users access to all Water sites
INSERT INTO user_site_accesses (id, user_id, site_id, can_read, can_create, can_update, can_delete)
SELECT gen_random_uuid(), ubr.user_id, s.id, true, true, true, false
FROM user_business_roles ubr
JOIN business_roles br ON br.id = ubr.business_role_id
JOIN sites s ON s.business_vertical_id = br.business_vertical_id
WHERE br.business_vertical_id = (SELECT id FROM business_verticals WHERE code = 'WATER');
```

Then gradually restrict access as needed.

## Testing Checklist

- [x] Build compiles without errors
- [ ] User can see only their accessible sites' reports
- [ ] User can create reports only for sites with create permission
- [ ] User can update reports only for sites with update permission
- [ ] User can delete reports only for sites with delete permission
- [ ] Batch operations validate all items
- [ ] Without site middleware, handlers work as before
- [ ] Proper error messages for permission denials

## Error Messages

The handlers return specific error messages:

| Error | Status | When |
|-------|--------|------|
| `"site not found"` | 404 | Site doesn't exist or doesn't belong to business |
| `"no access to this site"` | 403 | User doesn't have any access to the site |
| `"no create permission for this site"` | 403 | User can't create in this site |
| `"no update permission for this site"` | 403 | User can't update in this site |
| `"no delete permission for this site"` | 403 | User can't delete in this site |

## Example Usage

### Admin Assigns Site Access
```bash
POST /api/v1/business/WATER/sites/access
{
  "userId": "engineer-uuid",
  "siteId": "water-site-a-uuid",
  "canRead": true,
  "canCreate": true,
  "canUpdate": true,
  "canDelete": false
}
```

### Engineer Creates Report (Site-Aware)
```bash
POST /api/v1/business/WATER/water/reports/tanker
{
  "siteName": "Water Site A",  // Must be an accessible site
  "purpose": "Construction",
  "tankerVehicleNumber": "MH-12-AB-1234",
  ...
}
```

Result:
- ✅ If Engineer has access to "Water Site A" with `canCreate = true` → Success
- ❌ If "Water Site A" not in accessible sites → 403 Forbidden

## Benefits

1. **Fine-Grained Control**: Restrict users to specific sites within a business
2. **Backward Compatible**: Works without site context (existing behavior)
3. **Performance**: Efficient queries using site name filtering
4. **Security**: Double-layered (role + site permissions)
5. **Flexible**: Can enable/disable per route using middleware

## Related Files

- [models/site.go](../models/site.go) - Site and UserSiteAccess models
- [middleware/site_auth.go](../middleware/site_auth.go) - Site access middleware
- [config/permissions.go](../config/permissions.go) - Updated role permissions
- [handlers/site_management.go](../handlers/site_management.go) - Site management handlers
- [docs/SITE_LEVEL_PERMISSIONS.md](SITE_LEVEL_PERMISSIONS.md) - Complete documentation

## Next Steps

1. Update routes in [routes/business_routes.go](../routes/business_routes.go) to use site middleware
2. Run database migrations and seeding
3. Assign site access to existing users
4. Test all CRUD operations with site filtering
5. Apply same pattern to other report handlers (DPR Site, Wrapping, etc.)
