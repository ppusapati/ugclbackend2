# Middleware Quick Reference

## Common Authorization Patterns

### Global Permissions

```go
// Single permission
middleware.RequirePermission("read_users")

// Any of multiple permissions
middleware.RequireAnyPermission([]string{"create_reports", "create_materials"})

// Super admin only
middleware.RequireSuperAdmin()
```

### Business Permissions

```go
// Specific business permission
middleware.RequireBusinessPermission("read_reports")

// Business admin
middleware.RequireBusinessAdmin()

// Any business access
middleware.RequireBusinessAccess()
```

### Advanced (Functional Options)

```go
// Multiple requirements
middleware.Authorize(
    middleware.WithPermission("create_reports"),
    middleware.WithBusinessPermission("business_admin"),
)

// Any of multiple permissions
middleware.Authorize(
    middleware.WithAnyPermission("create_reports", "update_reports"),
)
```

### Site-Level Authorization

```go
// Require site access (chain after business access)
middleware.RequireBusinessAccess()(
    middleware.RequireSiteAccess()(handler))
```

## Helper Functions

### Get User Info
```go
claims := middleware.GetClaims(r)           // JWT claims
user := middleware.GetUser(r)               // Full user object
userID := middleware.GetUserID(r)           // User ID string
role := middleware.GetRole(r)               // User role
```

### Business Context
```go
businessID := middleware.GetCurrentBusinessID(r)
bizCtx := middleware.GetUserBusinessContext(r)
bizPerms := middleware.GetBusinessPermissions(r)
hasPermission := middleware.HasBusinessPermissionInContext(r, "read_reports")
```

### Site Access
```go
siteCtx := middleware.GetSiteAccessContext(r)
canAccess := middleware.CanAccessSite(r, siteID)
canCreate := middleware.CanCreateInSite(r, siteID)
canUpdate := middleware.CanUpdateInSite(r, siteID)
canDelete := middleware.CanDeleteInSite(r, siteID)
```

### Permissions
```go
perms := middleware.GetUserPermissions(r)
```

### Role Management
```go
level := middleware.GetUserRoleLevel(userID)
canAssign := middleware.CanUserAssignRole(userID, targetRoleLevel)
maxLevel := middleware.GetMaxAssignableLevel(userID)
isSuperAdmin := middleware.IsSuperAdminByID(userID)
```

### Business Verticals
```go
verticals := middleware.GetUserAccessibleVerticals(userID)
hasPermission := middleware.HasPermissionInVertical(userID, "read_reports", verticalID)
```

## Functional Options

```go
middleware.WithPermission("permission_name")
middleware.WithAnyPermission("perm1", "perm2", "perm3")
middleware.WithBusinessPermission("business_perm")
middleware.WithBusinessAccess()
middleware.WithSuperAdmin()
```

## Route Examples

### Public Routes (No Auth)
```go
r.HandleFunc("/api/v1/login", handlers.Login).Methods("POST")
r.HandleFunc("/api/v1/health", handlers.Health).Methods("GET")
```

### Authenticated Routes
```go
// All routes under /api/v1 require authentication
api := r.PathPrefix("/api/v1").Subrouter()
api.Use(middleware.SecurityMiddleware)
api.Use(middleware.JWTMiddleware)
```

### Permission-Based Routes
```go
// Users management
api.Handle("/users", middleware.RequirePermission("read_users")(
    http.HandlerFunc(handlers.GetUsers))).Methods("GET")

api.Handle("/users", middleware.RequirePermission("create_users")(
    http.HandlerFunc(handlers.CreateUser))).Methods("POST")

api.Handle("/users/{id}", middleware.RequirePermission("update_users")(
    http.HandlerFunc(handlers.UpdateUser))).Methods("PUT")

api.Handle("/users/{id}", middleware.RequirePermission("delete_users")(
    http.HandlerFunc(handlers.DeleteUser))).Methods("DELETE")
```

### Business-Specific Routes
```go
// Business reports
api.Handle("/business/{businessCode}/reports",
    middleware.RequireBusinessPermission("read_reports")(
        http.HandlerFunc(handlers.GetReports))).Methods("GET")

api.Handle("/business/{businessCode}/reports",
    middleware.RequireBusinessPermission("create_reports")(
        http.HandlerFunc(handlers.CreateReport))).Methods("POST")
```

### Admin Routes
```go
admin := r.PathPrefix("/api/v1/admin").Subrouter()
admin.Use(middleware.SecurityMiddleware)
admin.Use(middleware.JWTMiddleware)

// Super admin only
admin.Handle("/system-config", middleware.RequireSuperAdmin()(
    http.HandlerFunc(handlers.SystemConfig))).Methods("GET")

// Specific admin permission
admin.Handle("/roles", middleware.RequirePermission("manage_roles")(
    http.HandlerFunc(handlers.GetRoles))).Methods("GET")
```

### Complex Authorization
```go
// Require multiple permissions
api.Handle("/complex-endpoint",
    middleware.Authorize(
        middleware.WithPermission("base_permission"),
        middleware.WithBusinessPermission("business_admin"),
    )(http.HandlerFunc(handlers.ComplexHandler))).Methods("POST")

// Any of multiple permissions
api.Handle("/files/upload",
    middleware.RequireAnyPermission([]string{
        "create_reports",
        "create_materials",
    })(http.HandlerFunc(handlers.FileUpload))).Methods("POST")
```

### Site-Level Routes
```go
// Site access required
api.Handle("/business/{businessCode}/sites/{siteId}/data",
    middleware.RequireBusinessAccess()(
        middleware.RequireSiteAccess()(
            http.HandlerFunc(handlers.GetSiteData)))).Methods("GET")
```

## Error Codes

| Code | Message | Meaning |
|------|---------|---------|
| 401 | unauthorized | Not authenticated (no/invalid JWT) |
| 401 | user not found | User ID in JWT doesn't exist |
| 403 | insufficient permissions | User lacks required permission |
| 403 | no access to this business vertical | User not assigned to business |
| 403 | no site access granted | User has no site access |
| 400 | business vertical not specified | Business ID missing in request |
| 400 | invalid resource path | Resource ID extraction failed |

## Permission Naming Conventions

### Global Permissions
- `admin_all` - Super admin (bypass all checks)
- `read_{resource}` - Read access
- `create_{resource}` - Create access
- `update_{resource}` - Update access
- `delete_{resource}` - Delete access
- `manage_{resource}` - Full management

### Business Permissions
- `business_admin` - Business administrator
- `business_manage_users` - Manage business users
- `business_manage_roles` - Manage business roles
- `business_view_analytics` - View business analytics

### Examples
- `read_users`, `create_users`, `update_users`, `delete_users`
- `read_reports`, `create_reports`, `update_reports`, `delete_reports`
- `read_materials`, `create_materials`, `update_materials`, `delete_materials`
- `read_payments`, `create_payments`, `update_payments`, `delete_payments`
- `read_kpis`
- `manage_roles`, `manage_policies`, `manage_attributes`

## Business ID Resolution

The middleware can extract business ID from:

1. **URL Path Variables**: `{businessCode}`, `{businessId}`
2. **Query Parameters**: `?business_code=CODE`, `?business_id=UUID`
3. **Headers**: `X-Business-Code: CODE`, `X-Business-ID: UUID`
4. **Path Segments**: `/api/v1/business/CODE/reports`

Supports:
- UUID format: `123e4567-e89b-12d3-a456-426614174000`
- Business code: `COAL_MINING`
- Business name: `Coal Mining Division`

## Testing Authorization

### Unit Tests
```go
// Mock user context
mockUser := models.User{
    ID: uuid.New(),
    RoleModel: &models.Role{
        Name: "admin",
        Permissions: []models.Permission{
            {Name: "read_users"},
        },
    },
}

// Test authorization
authService := middleware.NewAuthService()
hasPermission := authService.HasPermission(userCtx, "read_users")
```

### Integration Tests
```go
// Create test request with JWT
token, _ := middleware.GenerateToken(userID, role, name, phone)
req.Header.Set("Authorization", "Bearer "+token)

// Test middleware
handler := middleware.RequirePermission("read_users")(testHandler)
handler.ServeHTTP(rr, req)

// Assert response
assert.Equal(t, http.StatusOK, rr.Code)
```

## Debugging Tips

### Enable Logging
```go
// In authorization.go, add logging
fmt.Printf("Checking permission: %s for user: %s\n", permission, userCtx.User.ID)
```

### Check User Context
```go
// In handler
userCtx, _ := authService.LoadUserContext(r)
fmt.Printf("User: %+v\n", userCtx)
fmt.Printf("Global Permissions: %v\n", userCtx.GlobalPermissions)
fmt.Printf("Business Context: %+v\n", userCtx.BusinessContext)
```

### Verify Business ID Resolution
```go
businessID := middleware.GetCurrentBusinessID(r)
fmt.Printf("Resolved Business ID: %s\n", businessID)
```

## Performance Tips

1. **Preload Only What You Need**: Adjust preloads in `LoadUserContext()`
2. **Cache User Context**: Consider request-scoped caching
3. **Avoid Multiple DB Calls**: Use preloading efficiently
4. **Index Permissions**: Ensure database indexes on permission lookups

## Common Mistakes

❌ **Don't**: Apply middleware without JWT middleware
```go
// Wrong
r.Handle("/api/users", middleware.RequirePermission("read_users")(handler))
```

✅ **Do**: Apply JWT middleware first
```go
// Correct
api := r.PathPrefix("/api/v1").Subrouter()
api.Use(middleware.JWTMiddleware)
api.Handle("/users", middleware.RequirePermission("read_users")(handler))
```

❌ **Don't**: Check business permission without business context
```go
// Won't work if business ID not in request
middleware.RequireBusinessPermission("read_reports")
```

✅ **Do**: Ensure business ID is in request (URL, query, header)
```go
// Business ID in URL path
api.Handle("/business/{businessCode}/reports",
    middleware.RequireBusinessPermission("read_reports")(handler))
```

❌ **Don't**: Duplicate authorization logic in handlers
```go
// Don't do authorization in handler
func GetUsers(w http.ResponseWriter, r *http.Request) {
    if !hasPermission(r, "read_users") {
        http.Error(w, "forbidden", 403)
        return
    }
    // ...
}
```

✅ **Do**: Use middleware for authorization
```go
// Use middleware
api.Handle("/users", middleware.RequirePermission("read_users")(
    http.HandlerFunc(handlers.GetUsers)))
```
