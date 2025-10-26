# Site Management API Endpoints

## Overview

These endpoints allow you to manage sites within business verticals and control which users have access to which sites.

## Base URL

All site management endpoints are under:
```
/api/v1/business/{businessCode}/sites
```

Where `{businessCode}` is one of:
- `WATER` - Water Works
- `SOLAR` - Solar Works
- `HO` - Head Office
- `CONTRACTORS` - Contractors

## Authentication

All endpoints require:
1. **JWT Authentication** - Valid JWT token in Authorization header
2. **Business Access** - User must have access to the business vertical
3. **Permissions** - Specific business permissions as noted below

## Endpoints

### 1. Get All Sites in Business Vertical

Get all sites within the current business vertical.

**Endpoint**: `GET /api/v1/business/{businessCode}/sites`

**Required Permission**: `site:view`

**Who can access**:
- Water_Admin
- Solar_Admin

**Example Request**:
```bash
GET /api/v1/business/WATER/sites
Authorization: Bearer <jwt-token>
```

**Example Response**:
```json
[
  {
    "id": "uuid-1",
    "name": "Water Site A",
    "code": "WATER_SITE_A",
    "description": "Primary water distribution site",
    "businessVerticalId": "uuid-water",
    "isActive": true,
    "createdAt": "2025-01-15T10:00:00Z",
    "updatedAt": "2025-01-15T10:00:00Z"
  },
  {
    "id": "uuid-2",
    "name": "Water Site B",
    "code": "WATER_SITE_B",
    "description": "Secondary water distribution site",
    "businessVerticalId": "uuid-water",
    "isActive": true,
    "createdAt": "2025-01-15T10:00:00Z",
    "updatedAt": "2025-01-15T10:00:00Z"
  }
]
```

---

### 2. Get User's Accessible Sites

Get all sites the current logged-in user has access to within the business vertical.

**Endpoint**: `GET /api/v1/business/{businessCode}/sites/my-access`

**Required Permission**: None (any authenticated user in the business)

**Who can access**: Any user with business access

**Example Request**:
```bash
GET /api/v1/business/WATER/sites/my-access
Authorization: Bearer <jwt-token>
```

**Example Response**:
```json
[
  {
    "id": "uuid-1",
    "name": "Water Site A",
    "code": "WATER_SITE_A",
    "description": "Primary water distribution site",
    "businessVerticalId": "uuid-water",
    "isActive": true,
    "canRead": true,
    "canCreate": true,
    "canUpdate": true,
    "canDelete": false,
    "createdAt": "2025-01-15T10:00:00Z",
    "updatedAt": "2025-01-15T10:00:00Z"
  },
  {
    "id": "uuid-2",
    "name": "Water Site B",
    "code": "WATER_SITE_B",
    "businessVerticalId": "uuid-water",
    "isActive": true,
    "canRead": true,
    "canCreate": false,
    "canUpdate": false,
    "canDelete": false,
    "createdAt": "2025-01-15T10:00:00Z",
    "updatedAt": "2025-01-15T10:00:00Z"
  }
]
```

---

### 3. Assign Site Access to User

Assign or update a user's access permissions for a specific site.

**Endpoint**: `POST /api/v1/business/{businessCode}/sites/access`

**Required Permission**: `site:manage_access`

**Who can access**:
- Water_Admin
- Solar_Admin

**Request Body**:
```json
{
  "userId": "user-uuid",
  "siteId": "site-uuid",
  "canRead": true,
  "canCreate": true,
  "canUpdate": true,
  "canDelete": false
}
```

**Example Request**:
```bash
POST /api/v1/business/WATER/sites/access
Authorization: Bearer <jwt-token>
Content-Type: application/json

{
  "userId": "123e4567-e89b-12d3-a456-426614174000",
  "siteId": "987fcdeb-51a2-43e1-8d6f-123456789abc",
  "canRead": true,
  "canCreate": true,
  "canUpdate": true,
  "canDelete": false
}
```

**Example Response**:
```json
{
  "id": "access-uuid",
  "userId": "123e4567-e89b-12d3-a456-426614174000",
  "siteId": "987fcdeb-51a2-43e1-8d6f-123456789abc",
  "canRead": true,
  "canCreate": true,
  "canUpdate": true,
  "canDelete": false,
  "assignedAt": "2025-01-15T12:00:00Z",
  "assignedBy": "admin-uuid",
  "createdAt": "2025-01-15T12:00:00Z",
  "updatedAt": "2025-01-15T12:00:00Z"
}
```

**Notes**:
- If access already exists for this user-site combination, it will be updated
- If access doesn't exist, a new access record will be created
- The site must belong to the current business vertical
- The `assignedBy` field is automatically set to the current user

---

### 4. Revoke Site Access

Remove a user's access to a specific site.

**Endpoint**: `DELETE /api/v1/business/{businessCode}/sites/access/{accessId}`

**Required Permission**: `site:manage_access`

**Who can access**:
- Water_Admin
- Solar_Admin

**Example Request**:
```bash
DELETE /api/v1/business/WATER/sites/access/access-uuid-123
Authorization: Bearer <jwt-token>
```

**Example Response**:
```
Status: 204 No Content
```

---

### 5. Get Users with Access to a Site

Get all users who have access to a specific site.

**Endpoint**: `GET /api/v1/business/{businessCode}/sites/{siteId}/users`

**Required Permission**: `site:view`

**Who can access**:
- Water_Admin
- Solar_Admin

**Example Request**:
```bash
GET /api/v1/business/WATER/sites/987fcdeb-51a2-43e1-8d6f-123456789abc/users
Authorization: Bearer <jwt-token>
```

**Example Response**:
```json
[
  {
    "userId": "user-1-uuid",
    "name": "John Engineer",
    "phone": "1234567890",
    "canRead": true,
    "canCreate": true,
    "canUpdate": true,
    "canDelete": false
  },
  {
    "userId": "user-2-uuid",
    "name": "Jane Supervisor",
    "phone": "0987654321",
    "canRead": true,
    "canCreate": false,
    "canUpdate": true,
    "canDelete": false
  }
]
```

---

## Permission Matrix

| Endpoint | Required Permission | Water_Admin | Solar_Admin | Engineer | Supervisor | Operator |
|----------|-------------------|-------------|-------------|----------|------------|----------|
| `GET /sites` | `site:view` | ✅ | ✅ | ❌ | ❌ | ❌ |
| `GET /sites/my-access` | None | ✅ | ✅ | ✅ | ✅ | ✅ |
| `POST /sites/access` | `site:manage_access` | ✅ | ✅ | ❌ | ❌ | ❌ |
| `DELETE /sites/access/{id}` | `site:manage_access` | ✅ | ✅ | ❌ | ❌ | ❌ |
| `GET /sites/{id}/users` | `site:view` | ✅ | ✅ | ❌ | ❌ | ❌ |

---

## Common Use Cases

### Use Case 1: Admin Assigns Engineer to Water Site A

```bash
# 1. Get all sites
GET /api/v1/business/WATER/sites
# Find the ID for "Water Site A"

# 2. Get engineer's user ID
# (from your user management system)

# 3. Assign access
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

### Use Case 2: Engineer Checks Which Sites They Can Access

```bash
GET /api/v1/business/WATER/sites/my-access
```

Response shows only the sites the engineer has access to with their specific permissions.

### Use Case 3: Admin Reviews Who Has Access to a Site

```bash
GET /api/v1/business/WATER/sites/water-site-a-uuid/users
```

Response shows all users with access and their permission levels.

### Use Case 4: Admin Revokes Access

```bash
# 1. Get users for the site
GET /api/v1/business/WATER/sites/water-site-a-uuid/users

# 2. Note the access ID (from database or separate query)

# 3. Revoke access
DELETE /api/v1/business/WATER/sites/access/access-uuid
```

---

## Error Responses

| Status Code | Error | Description |
|------------|-------|-------------|
| 400 | `business context not found` | User is not properly authenticated or business context is missing |
| 401 | `unauthorized` | No valid JWT token provided |
| 403 | `permission denied` | User doesn't have required permission |
| 403 | `no access to this site` | User is trying to assign access to a site they don't manage |
| 404 | `site not found` | Site ID doesn't exist or doesn't belong to this business |
| 404 | `record not found` | Access record doesn't exist |
| 500 | `internal server error` | Database or server error |

---

## Integration with Water Tanker Reports

When site-level permissions are enabled, water tanker report APIs will automatically:

1. **Filter by accessible sites** - Users only see reports from sites they can access
2. **Validate on create** - Users can only create reports for sites where `canCreate = true`
3. **Validate on update** - Users can only update reports for sites where `canUpdate = true`
4. **Validate on delete** - Users can only delete reports for sites where `canDelete = true`

See [WATER_HANDLER_SITE_INTEGRATION.md](WATER_HANDLER_SITE_INTEGRATION.md) for details.

---

## Seeded Sites

After running migrations and seeding, the following sites are automatically created:

### Water Works (4 sites)
- Water Site A (`WATER_SITE_A`)
- Water Site B (`WATER_SITE_B`)
- Water Site C (`WATER_SITE_C`)
- Water Site D (`WATER_SITE_D`)

### Solar Works (12 sites)
- Solar Site 01 through Solar Site 12 (`SOLAR_SITE_01` - `SOLAR_SITE_12`)

---

## Next Steps

1. **Run migrations**: `config.MigrateSites()`
2. **Seed sites**: `config.SeedSites()`
3. **Assign site access** to users using the `POST /sites/access` endpoint
4. **Enable site middleware** in routes to enforce site-level filtering

See [SITE_LEVEL_PERMISSIONS.md](SITE_LEVEL_PERMISSIONS.md) for complete implementation guide.
