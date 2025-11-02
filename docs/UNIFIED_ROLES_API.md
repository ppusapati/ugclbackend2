# Unified Roles API - Documentation

## Overview

The Unified Roles API consolidates both Global Roles and Business Roles into a single endpoint, eliminating the need for multiple API calls and providing a more efficient way to manage roles across the system.

## Motivation

**Before (Old Approach):**
- Frontend made N+1 API calls:
  - 1 call to `/admin/roles` for global roles
  - N calls to `/business/{code}/roles` for each business vertical
- Increased network overhead
- More complex frontend logic
- Harder to filter/search across all roles

**After (New Approach):**
- Single API call to `/admin/roles/unified`
- Returns both global and business roles in unified format
- Built-in filtering support
- Simpler frontend implementation

## API Endpoint

### GET /api/v1/admin/roles/unified

Returns all roles (global and business) in a unified format.

**Authentication:** Required (JWT)
**Permission:** `manage_roles`

#### Query Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `include_business` | boolean | `true` | Include business roles in response. Set to `false` to get only global roles. |
| `business_vertical_id` | UUID | - | Filter business roles by specific vertical. Only applies when `include_business=true`. |

#### Response Format

```json
{
  "roles": [
    {
      "id": "uuid",
      "name": "role_name",
      "display_name": "Display Name",
      "description": "Role description",
      "level": 0,
      "is_active": true,
      "is_global": true,
      "business_vertical_id": null,
      "business_vertical": null,
      "permissions": [
        {
          "id": "uuid",
          "name": "permission_name",
          "description": "Permission description",
          "resource": "resource_name",
          "action": "action_name"
        }
      ],
      "user_count": 5
    },
    {
      "id": "uuid",
      "name": "business_role_name",
      "display_name": "Business Role Display Name",
      "description": "Business role description",
      "level": 3,
      "is_active": true,
      "is_global": false,
      "business_vertical_id": "uuid",
      "business_vertical": {
        "id": "uuid",
        "code": "MINING",
        "name": "Mining Operations"
      },
      "permissions": [...],
      "user_count": 12
    }
  ],
  "total": 2
}
```

## Usage Examples

### Get All Roles (Global + Business)

```bash
GET /api/v1/admin/roles/unified?include_business=true
```

### Get Only Global Roles

```bash
GET /api/v1/admin/roles/unified?include_business=false
```

### Get Roles for Specific Vertical

```bash
GET /api/v1/admin/roles/unified?business_vertical_id=550e8400-e29b-41d4-a716-446655440000
```

## Response Fields

### Common Fields (All Roles)

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Unique role identifier |
| `name` | string | Internal role name (e.g., "admin") |
| `display_name` | string | Human-readable role name |
| `description` | string | Role description |
| `level` | integer | Hierarchy level (0=highest, 5=lowest) |
| `is_active` | boolean | Whether role is active |
| `is_global` | boolean | **true** for global roles, **false** for business roles |
| `permissions` | array | List of permissions assigned to role |
| `user_count` | integer | Number of users assigned to this role |

### Business Role Specific Fields

| Field | Type | Description |
|-------|------|-------------|
| `business_vertical_id` | UUID | ID of business vertical (null for global roles) |
| `business_vertical` | object | Business vertical details (null for global roles) |
| `business_vertical.id` | UUID | Vertical ID |
| `business_vertical.code` | string | Vertical code (e.g., "MINING") |
| `business_vertical.name` | string | Vertical name (e.g., "Mining Operations") |

## Performance Optimizations

### Efficient Queries

1. **Batch User Counts:** User counts are fetched in bulk using aggregation queries instead of N individual queries
2. **Preloading:** Permissions and business verticals are preloaded to avoid N+1 queries
3. **Single Response:** All data returned in one request, reducing network round trips

### Query Optimization

```go
// Before: N+1 queries for user counts
for _, role := range roles {
    db.Model(&User{}).Where("role_id = ?", role.ID).Count(&count)
}

// After: Single aggregated query
db.Model(&UserBusinessRole{}).
    Select("business_role_id, COUNT(*) as count").
    Where("business_role_id IN ?", roleIDs).
    Group("business_role_id").
    Scan(&roleUserCounts)
```

## Implementation Details

### Backend Files Modified

1. **`handlers/role_management.go`**
   - Added `GetAllRolesUnified()` handler
   - Added `UnifiedRoleResponse` struct
   - Added `BusinessVerticalInfo` struct

2. **`routes/routes_v2.go`**
   - Added route: `GET /admin/roles/unified`

### Frontend Files Modified

1. **`src/routes/admin/rbac/roles/index.tsx`**
   - Simplified `loadAllRoles()` to use unified endpoint
   - Removed N+1 query pattern
   - Single API call replaces multiple calls

**Before:**
```typescript
// Multiple API calls
const globalResponse = await apiClient.get('/admin/roles');
const businessRolesPromises = state.verticals.map(async (vertical) => {
  await apiClient.get(`/business/${vertical.code}/roles`)
});
```

**After:**
```typescript
// Single API call
const response = await apiClient.get<{ roles: Role[]; total: number }>(
  '/admin/roles/unified?include_business=true'
);
state.roles = response.roles || [];
```

## Backward Compatibility

The unified endpoint is **additive** - existing endpoints remain unchanged:

- ✅ `GET /admin/roles` - Still returns only global roles
- ✅ `GET /business/{code}/roles` - Still returns business roles for specific vertical
- ✅ `POST /admin/roles` - Still creates global roles
- ✅ `POST /business/{code}/roles` - Still creates business roles
- ✅ `PUT /admin/roles/{id}` - Still updates global roles
- ✅ `PUT /business/{code}/roles/{id}` - Still updates business roles
- ✅ `DELETE /admin/roles/{id}` - Still deletes global roles
- ✅ `DELETE /business/{code}/roles/{id}` - Still deletes business roles

The new `/admin/roles/unified` endpoint is for **read operations only**. Write operations (create/update/delete) continue to use the existing endpoints.

## Benefits

### For Frontend

1. **Simpler Code:** Single API call instead of N+1 calls
2. **Better Performance:** Fewer network round trips
3. **Consistent Data:** All roles loaded atomically
4. **Easier Filtering:** Can filter across all roles in a single response

### For Backend

1. **Optimized Queries:** Batch operations for user counts
2. **Less Network Traffic:** Single response instead of multiple
3. **Better Caching:** Single endpoint easier to cache
4. **Flexible Filtering:** Built-in support for vertical filtering

### For System

1. **Reduced Load:** Fewer database queries
2. **Better Scalability:** Less overhead as verticals grow
3. **Cleaner Architecture:** Unified interface for role management
4. **Future-Proof:** Easy to add more filtering options

## Testing

### Manual Testing

```bash
# 1. Test unified endpoint with both types
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/admin/roles/unified?include_business=true

# 2. Test global roles only
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/admin/roles/unified?include_business=false

# 3. Test filtering by vertical
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/admin/roles/unified?business_vertical_id=<UUID>
```

### Expected Results

- Response should contain both global and business roles
- Each role should have `is_global` flag set correctly
- Business roles should have `business_vertical` populated
- Global roles should have `business_vertical` as null
- User counts should be accurate

## Migration Guide

### For Frontend Developers

**Old Pattern:**
```typescript
// Load global roles
const globalRoles = await apiClient.get('/admin/roles');

// Load business roles
const businessRoles = await Promise.all(
  verticals.map(v => apiClient.get(`/business/${v.code}/roles`))
);

// Merge manually
const allRoles = [...globalRoles, ...businessRoles.flat()];
```

**New Pattern:**
```typescript
// Single call gets everything
const { roles } = await apiClient.get('/admin/roles/unified');
```

### For Backend Developers

The unified endpoint follows these principles:

1. **Query Optimization:** Always use batch queries for counts
2. **Preloading:** Load related data upfront (Preload)
3. **Filtering:** Support query parameter filtering
4. **Consistency:** Return unified format for all role types

## Future Enhancements

Potential improvements for future versions:

1. **Pagination:** Add pagination support for large role lists
2. **Sorting:** Add sort parameter (by name, level, user_count)
3. **Search:** Add search/filter by name or description
4. **Caching:** Implement Redis caching for frequently accessed data
5. **GraphQL:** Consider GraphQL API for more flexible querying

## Conclusion

The unified roles API provides a more efficient and developer-friendly way to manage roles across the system. It eliminates N+1 queries, simplifies frontend code, and maintains backward compatibility with existing endpoints.

**Key Takeaways:**
- ✅ Single endpoint for all roles
- ✅ Optimized database queries
- ✅ Backward compatible
- ✅ Flexible filtering
- ✅ Better performance
