# Authorization System Documentation

## Overview

This application implements a comprehensive Role-Based Access Control (RBAC) system with the following features:

- **JWT Authentication**: Secure token-based authentication
- **Permission-Based Authorization**: Granular permissions for specific actions
- **Role Management**: Hierarchical role system with permission inheritance
- **API Key Authentication**: For server-to-server communication
- **IP Whitelisting**: Additional security for partner integrations
- **Backward Compatibility**: Supports legacy string-based roles

## Architecture

### 1. Authentication Flow

```
Client Request → API Key Check → JWT Validation → Permission Check → Resource Access
```

### 2. Database Schema

#### Users Table
- `id`: UUID primary key
- `name`, `email`, `phone`: User details
- `password_hash`: Bcrypt hashed password
- `role`: Legacy string role (for backward compatibility)
- `role_id`: Foreign key to roles table (new system)
- `is_active`: Soft delete flag

#### Roles Table
- `id`: UUID primary key
- `name`: Role name (e.g., "admin", "project_coordinator")
- `description`: Human-readable description
- `is_active`: Soft delete flag

#### Permissions Table
- `id`: UUID primary key
- `name`: Permission name (e.g., "read_reports")
- `description`: Human-readable description
- `resource`: Resource type (e.g., "reports", "users")
- `action`: Action type (e.g., "read", "create", "update", "delete")

#### Role_Permissions Table (Junction)
- `role_id`: Foreign key to roles
- `permission_id`: Foreign key to permissions

## Default Roles and Permissions

### Super Admin
- **Permission**: `admin_all` - Full system access
- **Permission**: `manage_roles` - Can create/modify roles and permissions

### Admin
- **Reports**: read, create, update, delete
- **Users**: read, create, update (cannot delete)
- **Materials**: read, create, update, delete
- **Payments**: read, create, update, delete
- **KPIs**: read

### Project Coordinator
- **Reports**: read, create, update
- **Materials**: read, create, update
- **Payments**: read, create, update
- **KPIs**: read

### User
- **Reports**: read, create
- **Materials**: read
- **KPIs**: read

## API Endpoints

### Authentication
```
POST /api/v1/login
POST /api/v1/register
GET  /api/v1/token
```

### User Management (Admin only)
```
GET    /api/v1/admin/users
POST   /api/v1/admin/users
PUT    /api/v1/admin/users/{id}
DELETE /api/v1/admin/users/{id}
```

### Role Management (Super Admin only)
```
GET    /api/v1/admin/roles
POST   /api/v1/admin/roles
PUT    /api/v1/admin/roles/{id}
DELETE /api/v1/admin/roles/{id}
GET    /api/v1/admin/permissions
```

### Reports (Permission-based)
```
GET    /api/v1/dprsite        # Requires: read_reports
POST   /api/v1/dprsite        # Requires: create_reports
PUT    /api/v1/dprsite/{id}   # Requires: update_reports
DELETE /api/v1/dprsite/{id}   # Requires: delete_reports
```

### Testing Endpoints
```
GET /api/v1/test/auth                    # Check authentication status
GET /api/v1/test/permission?permission=read_reports  # Test specific permission
```

## Usage Examples

### 1. Login and Get Token
```bash
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -H "x-api-key: YOUR_API_KEY" \
  -d '{
    "phone": "1234567890",
    "password": "password123"
  }'
```

### 2. Access Protected Endpoint
```bash
curl -X GET http://localhost:8080/api/v1/dprsite \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"
```

### 3. Test User Permissions
```bash
curl -X GET "http://localhost:8080/api/v1/test/permission?permission=read_reports" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "x-api-key: YOUR_API_KEY"
```

### 4. Create New Role (Super Admin only)
```bash
curl -X POST http://localhost:8080/api/v1/admin/roles \
  -H "Authorization: Bearer SUPER_ADMIN_TOKEN" \
  -H "x-api-key: YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "site_manager",
    "description": "Site Manager Role",
    "permissions": ["read_reports", "create_reports", "read_materials"]
  }'
```

## Middleware Usage

### 1. Require Specific Permission
```go
api.Handle("/endpoint", middleware.RequirePermission("read_reports")(
    http.HandlerFunc(handlers.YourHandler))).Methods("GET")
```

### 2. Require Any of Multiple Permissions
```go
api.Handle("/endpoint", middleware.RequireAnyPermission([]string{"read_reports", "admin_all"})(
    http.HandlerFunc(handlers.YourHandler))).Methods("GET")
```

### 3. Require Resource Ownership
```go
api.Handle("/reports/{id}", middleware.RequireResourceOwnership("reports")(
    http.HandlerFunc(handlers.UpdateReport))).Methods("PUT")
```

## Migration from Legacy System

The new system maintains backward compatibility with the existing string-based roles:

1. **Existing users** continue to work with their current `role` field
2. **New users** can be assigned to the new role system using `role_id`
3. **Permission checks** fall back to legacy role mapping if no new role is assigned

### Migration Steps

1. **Deploy the new system** with backward compatibility
2. **Create roles** using the admin interface
3. **Gradually migrate users** to the new role system
4. **Remove legacy role field** once all users are migrated

## Security Considerations

### 1. API Key Management
- Different API keys for different client types
- IP whitelisting for server-to-server communication
- Separate permissions for mobile vs web clients

### 2. JWT Security
- 24-hour token expiration
- Secure secret key management
- Token validation on every request

### 3. Permission Granularity
- Resource-level permissions (reports, users, materials)
- Action-level permissions (read, create, update, delete)
- Special admin permissions for system management

### 4. Audit Logging
- All security events are logged
- User actions are tracked with timestamps
- Failed authentication attempts are monitored

## Troubleshooting

### Common Issues

1. **"insufficient permissions" error**
   - Check user's role and permissions using `/test/permission` endpoint
   - Verify the required permission exists in the database

2. **"invalid or expired token" error**
   - Token may have expired (24-hour limit)
   - User needs to login again

3. **"Invalid or missing API key" error**
   - Check `x-api-key` header is present
   - Verify API key is configured in environment variables

### Debug Endpoints

- `GET /api/v1/test/auth` - Check authentication status
- `GET /api/v1/test/permission?permission=PERM_NAME` - Test specific permission
- `GET /api/v1/profile` - Get current user info and permissions

## Environment Variables

```bash
JWT_SECRET=your-secret-key-here
MOBILE_APP_KEY=mobile-api-key
PARTNER_PORTAL_KEY=partner-api-key
INTERNAL_OPS_KEY=internal-ops-key
DB_DSN=postgresql://user:pass@localhost/dbname
```

## Best Practices

1. **Use specific permissions** instead of broad role checks
2. **Implement resource ownership** for user-specific data
3. **Regular permission audits** to ensure least privilege
4. **Monitor failed authorization attempts**
5. **Use HTTPS** in production for token security