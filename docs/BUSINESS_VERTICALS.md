# Business Verticals Authorization System

## Overview

The Business Verticals system allows you to organize your application into separate business units (like Solar Farm, Water Works, etc.), each with their own administrative hierarchies, roles, and permissions.

## Key Concepts

### 1. Business Verticals
- **Solar Farm Operations**: Solar energy generation and maintenance
- **Water Works**: Water supply and distribution management  
- **Corporate Services**: Corporate administration and support

### 2. Super Admin Access
- **Super Admins** (`super_admin` role) have automatic access to ALL business verticals
- They don't need to be explicitly assigned to each business
- They have all permissions in every business vertical
- They can manage users, roles, and data across all businesses

### 3. Hierarchical Roles per Business
Each business vertical has its own role hierarchy:
- **Level 1**: Business Administrator (highest authority within that business)
- **Level 2**: Business Manager
- **Level 3**: Supervisor
- **Level 4**: Operator (lowest level)

### 4. Multi-Business User Access
Users can have different roles across multiple business verticals:
- John could be "Admin" in Solar Farm and "Manager" in Water Works
- Sarah could be "Supervisor" in Water Works only
- **Super Admin** Mike has full access to all businesses automatically

## Database Schema

### Business Verticals Table
```sql
business_verticals:
- id (UUID)
- name (e.g., "Solar Farm Operations")
- code (e.g., "SOLAR")
- description
- is_active
- settings (JSON)
```

### Business Roles Table
```sql
business_roles:
- id (UUID)
- name (e.g., "admin", "manager")
- display_name (e.g., "Solar Farm Administrator")
- business_vertical_id (FK)
- level (1-4, hierarchy level)
- permissions (many-to-many)
```

### User Business Roles Table
```sql
user_business_roles:
- id (UUID)
- user_id (FK)
- business_role_id (FK)
- is_active
- assigned_at
- assigned_by (FK to users)
```

## API Endpoints

### Global Business Management (Super Admin Only)
```
GET    /api/v1/admin/businesses           # List all business verticals
POST   /api/v1/admin/businesses           # Create new business vertical
GET    /api/v1/admin/dashboard            # Super admin dashboard with all stats
```

### User Business Access (Any Authenticated User)
```
GET    /api/v1/my-businesses              # Get businesses user can access
```

### Business-Specific Management (Using Business Codes)
```
GET    /api/v1/business/{businessCode}/info             # Get business information
GET    /api/v1/business/{businessCode}/roles            # List roles in business
POST   /api/v1/business/{businessCode}/roles            # Create role in business
GET    /api/v1/business/{businessCode}/users            # List users in business
POST   /api/v1/business/{businessCode}/users/assign     # Assign user to role
GET    /api/v1/business/{businessCode}/context          # Get user's business context
```

### Business-Specific Operations
```
GET    /api/v1/business/{businessCode}/reports/dprsite  # Business-filtered reports
POST   /api/v1/business/{businessCode}/reports/dprsite  # Create report in business
GET    /api/v1/business/{businessCode}/analytics        # Business analytics
```

### Industry-Specific Endpoints

#### Solar Farm Operations (using code "SOLAR")
```
GET    /api/v1/business/SOLAR/solar/generation          # Solar generation data
GET    /api/v1/business/SOLAR/solar/panels              # Panel management
GET    /api/v1/business/SOLAR/solar/maintenance         # Maintenance records
```

#### Water Works Operations (using code "WATER")
```
GET    /api/v1/business/WATER/water/consumption         # Water consumption data
GET    /api/v1/business/WATER/water/supply              # Supply management
GET    /api/v1/business/WATER/water/quality             # Quality control
```

### Flexible Business Identification
The system supports multiple ways to identify businesses in URLs:
- **Business Code**: `/api/v1/business/SOLAR/users` (recommended)
- **Business Name**: `/api/v1/business/Solar%20Farm%20Operations/users`
- **Business UUID**: `/api/v1/business/uuid-here/users` (legacy support)

## Permission System

### Global Permissions
- `admin_all`: Super admin access across all businesses
- `manage_businesses`: Create/manage business verticals

### Business-Specific Permissions
- `business_admin`: Admin within a specific business
- `business_manage_users`: Manage users within business
- `business_manage_roles`: Manage roles within business
- `business_view_analytics`: View business analytics

### Industry-Specific Permissions
- `solar_read_generation`: View solar generation data
- `solar_manage_panels`: Manage solar panels
- `water_read_consumption`: View water consumption
- `water_quality_control`: Manage water quality

## Usage Examples

### 1. Create a New Business Vertical
```bash
curl -X POST http://localhost:8080/api/v1/admin/businesses \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN" \
  -H "x-api-key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Wind Farm Operations",
    "code": "WIND", 
    "description": "Wind energy generation and maintenance"
  }'
```

### 2. Assign User to Business Role (Using Business Code)
```bash
curl -X POST http://localhost:8080/api/v1/business/SOLAR/users/assign \
  -H "Authorization: Bearer BUSINESS_ADMIN_TOKEN" \
  -H "x-api-key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user-uuid-here",
    "business_role_id": "role-uuid-here"
  }'
```

### 3. Access Business-Specific Data (Using Business Codes)
```bash
# Get solar generation data (requires solar_read_generation permission)
curl -X GET http://localhost:8080/api/v1/business/SOLAR/solar/generation \
  -H "Authorization: Bearer USER_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"

# Get water quality data (requires water_quality_control permission)  
curl -X GET http://localhost:8080/api/v1/business/WATER/water/quality \
  -H "Authorization: Bearer USER_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"

# Alternative: Using business name (URL encoded)
curl -X GET "http://localhost:8080/api/v1/business/Solar%20Farm%20Operations/solar/generation" \
  -H "Authorization: Bearer USER_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"
```

### 4. Check User's Business Context (Using Business Code)
```bash
curl -X GET http://localhost:8080/api/v1/business/SOLAR/context \
  -H "Authorization: Bearer USER_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"
```

### 5. Get Business Information
```bash
curl -X GET http://localhost:8080/api/v1/business/SOLAR/info \
  -H "Authorization: Bearer USER_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"
```

Response:
```json
{
  "id": "business-uuid",
  "name": "Solar Farm Operations",
  "code": "SOLAR",
  "description": "Solar energy generation and maintenance operations",
  "is_active": true,
  "user_count": 15,
  "role_count": 4,
  "created_at": "2025-10-14T10:00:00Z",
  "url_examples": {
    "by_code": "/api/v1/business/SOLAR/users",
    "by_name": "/api/v1/business/Solar%20Farm%20Operations/users",
    "by_id": "/api/v1/business/uuid-here/users"
  }
}
```

Response for regular user:
```json
{
  "business_id": "business-uuid",
  "business_roles": [
    {
      "id": "role-uuid",
      "name": "manager", 
      "display_name": "Solar Farm Manager",
      "level": 2
    }
  ],
  "permissions": ["read_reports", "create_reports", "solar_read_generation"],
  "is_admin": false,
  "is_super_admin": false
}
```

Response for super admin:
```json
{
  "business_id": "business-uuid",
  "business_roles": [],
  "permissions": ["admin_all", "business_admin", "read_reports", "create_reports", "..."],
  "is_admin": true,
  "is_super_admin": true
}
```

### 5. Get User's Accessible Businesses
```bash
curl -X GET http://localhost:8080/api/v1/my-businesses \
  -H "Authorization: Bearer USER_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"
```

Response for super admin:
```json
{
  "user_id": "user-uuid",
  "user_name": "Super Admin User",
  "user_role": "super_admin",
  "is_super_admin": true,
  "accessible_businesses": [
    {
      "id": "solar-business-uuid",
      "name": "Solar Farm Operations",
      "code": "SOLAR",
      "access_type": "super_admin",
      "roles": ["Super Administrator"],
      "permissions": ["all"]
    },
    {
      "id": "water-business-uuid", 
      "name": "Water Works",
      "code": "WATER",
      "access_type": "super_admin",
      "roles": ["Super Administrator"],
      "permissions": ["all"]
    }
  ],
  "total_businesses": 2
}
```

### 6. Super Admin Dashboard
```bash
curl -X GET http://localhost:8080/api/v1/admin/dashboard \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"
```

## Middleware Usage

### Require Business Access
```go
// User must have any role in the business
business.Use(middleware.RequireBusinessAccess())
```

### Require Specific Business Permission
```go
// User must have specific permission in the business
business.Handle("/reports", middleware.RequireBusinessPermission("read_reports")(
    http.HandlerFunc(handlers.GetBusinessReports))).Methods("GET")
```

### Require Business Admin
```go
// User must be admin of the business
business.Handle("/admin", middleware.RequireBusinessAdmin()(
    http.HandlerFunc(handlers.BusinessAdminPanel))).Methods("GET")
```

## Business ID Resolution

The system automatically extracts business ID from:
1. **URL Path Variables**: `/business/{businessId}/reports`
2. **Query Parameters**: `?business_id=uuid`
3. **Headers**: `X-Business-ID: uuid`

## Role Hierarchy

Users with higher-level roles can manage users with lower-level roles:

```
Level 1 (Admin)     → Can manage Levels 2, 3, 4
Level 2 (Manager)   → Can manage Levels 3, 4  
Level 3 (Supervisor)→ Can manage Level 4
Level 4 (Operator)  → Cannot manage others
```

## Migration from Single-Tenant

1. **Existing users** keep their global roles
2. **Business verticals** are created with default roles
3. **Users are gradually assigned** to business-specific roles
4. **Global permissions** still work for super admins

## Security Considerations

### 1. Business Isolation
- Users can only access businesses they're assigned to
- Data is filtered by business vertical ID
- Cross-business access requires explicit permissions

### 2. Role Management
- Only business admins can assign roles within their business
- Super admins can manage all businesses
- Role hierarchy prevents privilege escalation

### 3. Audit Trail
- All role assignments are logged with timestamps
- Assignment history includes who made the assignment
- Business access is logged for security monitoring

## URL Structure Benefits

### Using Business Codes Instead of UUIDs
- **User-Friendly**: `/api/v1/business/SOLAR/users` vs `/api/v1/business/4d28d770-3b54-4eac-b7e0-30a36c5e7986/users`
- **No URL Rewriting**: Frontend doesn't need to lookup UUIDs
- **Memorable**: Easy to remember and type
- **SEO-Friendly**: Better for documentation and API exploration
- **Flexible**: Supports codes, names, or UUIDs for backward compatibility

### Business Code Guidelines
- Use **uppercase** letters and underscores: `SOLAR`, `WATER_WORKS`
- Keep codes **short** but descriptive: `SOLAR` not `SOLAR_FARM_OPERATIONS`
- Make codes **unique** across all business verticals
- Avoid special characters that need URL encoding

## Best Practices

### 1. Business Organization
- Create separate businesses for distinct operational units
- Use clear naming conventions (e.g., "Solar Farm - North", "Solar Farm - South")
- Assign primary business vertical to users for default context
- Choose meaningful business codes that won't change over time

### 2. Role Design
- Keep role hierarchies simple (4 levels max)
- Use descriptive display names for user interfaces
- Assign minimal required permissions to each role

### 3. User Management
- Assign users to their primary business first
- Add secondary business roles as needed
- Regular audit of user business assignments

### 4. Permission Granularity
- Use business-specific permissions for sensitive operations
- Combine with resource-level permissions for fine control
- Monitor permission usage for optimization

## Troubleshooting

### Common Issues

1. **"business vertical not specified" error**
   - Ensure business ID is in URL path, query param, or header
   - Check URL format: `/api/v1/business/{businessId}/...`

2. **"no access to this business vertical" error**
   - User needs to be assigned a role in that business
   - Check user's business roles with `/context` endpoint

3. **"insufficient permissions for this business vertical" error**
   - User's role doesn't have the required permission
   - Check role permissions in business admin panel

### Debug Endpoints

- `GET /api/v1/business/{businessId}/context` - Check user's business context
- `GET /api/v1/admin/businesses` - List all business verticals
- `GET /api/v1/business/{businessId}/users` - See all users in business

This multi-tenant business vertical system provides complete organizational separation while maintaining centralized user management and flexible permission assignment.