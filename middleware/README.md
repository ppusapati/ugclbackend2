# Middleware Documentation

## Overview

This directory contains authentication and authorization middleware for the UGCL backend application. The middleware has been refactored to provide a cleaner, more maintainable approach while maintaining backward compatibility.

## Quick Start

### Simple Permission Check
```go
// Require a global permission
r.Handle("/api/v1/users",
    middleware.RequirePermission("read_users")(handler))
```

### Business Permission Check
```go
// Require a business-specific permission
r.Handle("/api/v1/business/{businessCode}/reports",
    middleware.RequireBusinessPermission("read_reports")(handler))
```

### Complex Authorization
```go
// Combine multiple requirements
r.Handle("/api/v1/complex",
    middleware.Authorize(
        middleware.WithPermission("create_reports"),
        middleware.WithBusinessPermission("business_admin"),
    )(handler))
```

## Architecture

### New (Recommended) Files

#### Core Service
- **[auth_service.go](auth_service.go)** - Centralized authorization service
  - `AuthService` - Core service for all authorization logic
  - `UserContext` - Complete user authorization context
  - `BusinessContext` - Business-specific authorization info

#### Main Middleware
- **[authorization_refactored.go](authorization_refactored.go)** - Unified authorization middleware
  - `Authorize()` - Main middleware with functional options
  - `RequirePermission()` - Global permission check
  - `RequireBusinessPermission()` - Business permission check
  - `RequireSuperAdmin()` - Super admin check

#### Utilities
- **[helpers.go](helpers.go)** - Shared utility functions
  - Business ID resolution
  - Role level management
  - Permission checks by ID

#### Specialized
- **[site_auth_refactored.go](site_auth_refactored.go)** - Site-level authorization
  - `RequireSiteAccess()` - Site access middleware
  - Site permission helpers

### Existing Files

#### Core Authentication
- **[jwt.go](jwt.go)** - JWT authentication (unchanged)
  - `JWTMiddleware` - JWT validation
  - `GenerateToken()` - Token generation
  - `GetClaims()`, `GetUser()`, `GetUserID()` - Context helpers

#### Legacy Authorization (Backward Compatible)
- **[authorization.go](authorization.go)** - Original authorization middleware
- **[business_auth.go](business_auth.go)** - Original business authorization
- **[role_level.go](role_level.go)** - Original role level utilities
- **[site_auth.go](site_auth.go)** - Original site authorization

#### Specialized
- **[abac_middleware.go](abac_middleware.go)** - ABAC policy-based authorization

## Documentation

Comprehensive documentation is available in the [`docs/`](../docs/) directory:

### Getting Started
- 📖 **[Quick Reference](../docs/MIDDLEWARE_QUICK_REFERENCE.md)** - Common patterns and examples
- 🚀 **[Refactoring Guide](../docs/MIDDLEWARE_REFACTORING_GUIDE.md)** - Complete migration guide

### Understanding the Changes
- 📊 **[Refactoring Summary](../docs/REFACTORING_SUMMARY.md)** - What changed and why
- 🔄 **[Before/After Examples](../docs/BEFORE_AFTER_EXAMPLES.md)** - Code comparisons

## Middleware Types

### 1. Authentication Middleware

#### JWTMiddleware
Validates JWT tokens and extracts user claims.

```go
api := r.PathPrefix("/api/v1").Subrouter()
api.Use(middleware.JWTMiddleware)
```

**Use when:** All authenticated routes

#### SecurityMiddleware
Validates API keys and IP addresses.

```go
api.Use(middleware.SecurityMiddleware)
```

**Use when:** API key validation needed

### 2. Authorization Middleware

#### Global Permissions

##### RequirePermission
Requires a specific global permission.

```go
middleware.RequirePermission("read_users")
```

**Use when:** Checking global permissions like `read_users`, `create_reports`

##### RequireAnyPermission
Requires any of the specified permissions.

```go
middleware.RequireAnyPermission([]string{"create_reports", "create_materials"})
```

**Use when:** User needs any one of multiple permissions

##### RequireSuperAdmin
Requires super admin privileges.

```go
middleware.RequireSuperAdmin()
```

**Use when:** Only super admins should access

#### Business Permissions

##### RequireBusinessPermission
Requires a business-specific permission.

```go
middleware.RequireBusinessPermission("read_reports")
```

**Use when:** Permission scoped to business vertical

##### RequireBusinessAdmin
Requires business admin role.

```go
middleware.RequireBusinessAdmin()
```

**Use when:** Business admin actions

##### RequireBusinessAccess
Requires any access to the business.

```go
middleware.RequireBusinessAccess()
```

**Use when:** User just needs to be part of the business

#### Site Permissions

##### RequireSiteAccess
Requires access to at least one site.

```go
middleware.RequireBusinessAccess()(
    middleware.RequireSiteAccess()(handler))
```

**Use when:** Site-level operations

#### Advanced

##### Authorize (Functional Options)
Flexible authorization with composable requirements.

```go
middleware.Authorize(
    middleware.WithPermission("create_reports"),
    middleware.WithBusinessPermission("business_admin"),
)
```

**Use when:** Complex authorization requirements

### 3. ABAC Middleware

#### RequireABACPolicy
Evaluates attribute-based access control policies.

```go
middleware.RequireABACPolicy("read", "report")
```

**Use when:** Fine-grained attribute-based access control

## Helper Functions

### Context Helpers

```go
// Get JWT claims
claims := middleware.GetClaims(r)

// Get full user object
user := middleware.GetUser(r)

// Get user ID
userID := middleware.GetUserID(r)

// Get current business ID
businessID := middleware.GetCurrentBusinessID(r)

// Get user's business context
bizCtx := middleware.GetUserBusinessContext(r)
```

### Permission Helpers

```go
// Get all user permissions
perms := middleware.GetUserPermissions(r)

// Get business-specific permissions
bizPerms := middleware.GetBusinessPermissions(r)

// Check business permission in context
hasPermission := middleware.HasBusinessPermissionInContext(r, "read_reports")
```

### Site Access Helpers

```go
// Get site access context
siteCtx := middleware.GetSiteAccessContext(r)

// Check site access
canAccess := middleware.CanAccessSite(r, siteID)

// Check site permissions
canCreate := middleware.CanCreateInSite(r, siteID)
canUpdate := middleware.CanUpdateInSite(r, siteID)
canDelete := middleware.CanDeleteInSite(r, siteID)
```

### Role Management Helpers

```go
// Get user's role level
level := middleware.GetUserRoleLevel(userID)

// Check if user can assign role
canAssign := middleware.CanUserAssignRole(userID, targetRoleLevel)

// Get max assignable level
maxLevel := middleware.GetMaxAssignableLevel(userID)

// Check super admin status
isSuperAdmin := middleware.IsSuperAdminByID(userID)
```

### Business Vertical Helpers

```go
// Get accessible verticals
verticals := middleware.GetUserAccessibleVerticals(userID)

// Check permission in vertical
hasPermission := middleware.HasPermissionInVertical(userID, "read_reports", verticalID)
```

## Common Patterns

### REST Resource with CRUD
```go
// List
api.Handle("/users",
    middleware.RequirePermission("read_users")(
        http.HandlerFunc(handlers.GetUsers))).Methods("GET")

// Create
api.Handle("/users",
    middleware.RequirePermission("create_users")(
        http.HandlerFunc(handlers.CreateUser))).Methods("POST")

// Read
api.Handle("/users/{id}",
    middleware.RequirePermission("read_users")(
        http.HandlerFunc(handlers.GetUser))).Methods("GET")

// Update
api.Handle("/users/{id}",
    middleware.RequirePermission("update_users")(
        http.HandlerFunc(handlers.UpdateUser))).Methods("PUT")

// Delete
api.Handle("/users/{id}",
    middleware.RequirePermission("delete_users")(
        http.HandlerFunc(handlers.DeleteUser))).Methods("DELETE")
```

### Business Vertical Resource
```go
// Business-scoped resource
api.Handle("/business/{businessCode}/reports",
    middleware.RequireBusinessPermission("read_reports")(
        http.HandlerFunc(handlers.GetBusinessReports))).Methods("GET")

api.Handle("/business/{businessCode}/reports",
    middleware.RequireBusinessPermission("create_reports")(
        http.HandlerFunc(handlers.CreateBusinessReport))).Methods("POST")
```

### Site-Scoped Resource
```go
// Site-scoped resource (requires both business and site access)
api.Handle("/business/{businessCode}/sites/{siteId}/data",
    middleware.RequireBusinessAccess()(
        middleware.RequireSiteAccess()(
            http.HandlerFunc(handlers.GetSiteData)))).Methods("GET")
```

### Admin-Only Routes
```go
admin := r.PathPrefix("/api/v1/admin").Subrouter()
admin.Use(middleware.SecurityMiddleware)
admin.Use(middleware.JWTMiddleware)

// Super admin only
admin.Handle("/system-config",
    middleware.RequireSuperAdmin()(
        http.HandlerFunc(handlers.SystemConfig))).Methods("GET")

// Specific admin permission
admin.Handle("/roles",
    middleware.RequirePermission("manage_roles")(
        http.HandlerFunc(handlers.GetRoles))).Methods("GET")
```

## Error Handling

### Error Codes

| Code | Message | Cause |
|------|---------|-------|
| 401 | unauthorized | No/invalid JWT token |
| 401 | user not found | User ID in JWT doesn't exist |
| 403 | insufficient permissions | User lacks required permission |
| 403 | no access to this business vertical | User not part of business |
| 403 | no site access granted | User has no site access |
| 400 | business vertical not specified | Business ID missing |
| 400 | invalid resource path | Resource ID extraction failed |

### Custom Error Types

```go
// AuthError for consistent error handling
type AuthError struct {
    Code    int
    Message string
}
```

## Testing

### Unit Testing Service Layer
```go
func TestAuthService_HasPermission(t *testing.T) {
    authService := middleware.NewAuthService()
    userCtx := &middleware.UserContext{
        GlobalPermissions: []string{"read_users"},
    }

    assert.True(t, authService.HasPermission(userCtx, "read_users"))
    assert.False(t, authService.HasPermission(userCtx, "delete_users"))
}
```

### Integration Testing Middleware
```go
func TestRequirePermission(t *testing.T) {
    req := httptest.NewRequest("GET", "/api/v1/users", nil)
    token, _ := middleware.GenerateToken(userID, "admin", "Test", "1234567890")
    req.Header.Set("Authorization", "Bearer "+token)

    handler := middleware.RequirePermission("read_users")(testHandler)
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)

    assert.Equal(t, http.StatusOK, rr.Code)
}
```

## Best Practices

### 1. Always Apply JWT Middleware First
```go
✅ Good:
api := r.PathPrefix("/api/v1").Subrouter()
api.Use(middleware.JWTMiddleware)
api.Handle("/users", middleware.RequirePermission("read_users")(handler))

❌ Bad:
r.Handle("/api/users", middleware.RequirePermission("read_users")(handler))
```

### 2. Use Convenience Functions for Simple Cases
```go
✅ Good:
middleware.RequirePermission("read_users")

❌ Overkill:
middleware.Authorize(middleware.WithPermission("read_users"))
```

### 3. Use Authorize() for Complex Cases
```go
✅ Good:
middleware.Authorize(
    middleware.WithPermission("create_reports"),
    middleware.WithBusinessPermission("business_admin"),
)
```

### 4. Don't Mix Authorization in Handlers
```go
❌ Bad:
func GetUsers(w http.ResponseWriter, r *http.Request) {
    if !hasPermission(r, "read_users") {
        http.Error(w, "forbidden", 403)
        return
    }
    // ...
}

✅ Good:
// Use middleware
api.Handle("/users", middleware.RequirePermission("read_users")(
    http.HandlerFunc(handlers.GetUsers)))
```

### 5. Super Admins Automatically Bypass
```go
// No need to explicitly check for super admin
// They automatically pass all permission checks
if user.HasPermission("read_users") {
    // This is true for super admins automatically
}
```

## Migration Guide

### Step 1: Keep Both Implementations
The new refactored middleware works alongside the old middleware. No immediate changes needed.

### Step 2: Use New Middleware for New Routes
```go
// New routes can use the new patterns
r.Handle("/api/v1/new-endpoint",
    middleware.Authorize(
        middleware.WithPermission("permission_name"),
    )(handler))
```

### Step 3: Optionally Migrate Existing Routes
Existing routes continue to work as-is. Migrate when convenient.

## Performance Considerations

### User Context Caching
User context is loaded once per request and can be reused:

```go
// Loaded once in middleware
userCtx, _ := authService.LoadUserContext(r)

// Reused throughout the request lifecycle
```

### Database Query Optimization
Preloading is optimized in `LoadUserContext()`:

```go
config.DB.
    Preload("RoleModel.Permissions").
    Preload("UserBusinessRoles.BusinessRole.Permissions").
    // ... efficient loading
```

## Troubleshooting

### "unauthorized" error
- Check JWT middleware is applied
- Verify JWT token is valid
- Check user exists in database

### "insufficient permissions"
- Verify user has the required permission
- Check permission name matches exactly
- For super admins, check `admin_all` permission exists

### "business vertical not specified"
- Ensure business ID is in URL, query param, or header
- Check business ID format (UUID or code)
- Verify business is active

## Contributing

When adding new authorization logic:

1. Add business logic to `AuthService` ([auth_service.go](auth_service.go))
2. Add middleware wrapper in ([authorization_refactored.go](authorization_refactored.go))
3. Add tests for new functionality
4. Update documentation

## Support

For questions or issues:
- Check [Quick Reference](../docs/MIDDLEWARE_QUICK_REFERENCE.md)
- Review [Before/After Examples](../docs/BEFORE_AFTER_EXAMPLES.md)
- Consult [Refactoring Guide](../docs/MIDDLEWARE_REFACTORING_GUIDE.md)

## Summary

The middleware provides:
- ✅ Clean, maintainable authorization
- ✅ Backward compatible with existing code
- ✅ Flexible and composable
- ✅ Well-tested and documented
- ✅ Performance optimized
- ✅ Easy to extend

Choose the right middleware for your needs and enjoy clean authorization! 🚀
