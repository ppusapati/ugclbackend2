# Middleware Refactoring Guide

## Overview

The authorization middleware has been refactored to provide a cleaner, more maintainable, and unified approach to authorization across the application.

## Key Improvements

### 1. **Unified Authorization Service** (`auth_service.go`)
   - Centralized authorization logic in `AuthService`
   - Single source of truth for permission checking
   - Eliminates code duplication across multiple middleware files

### 2. **Flexible Authorization Middleware** (`authorization_refactored.go`)
   - Single `Authorize()` function with functional options
   - Composable authorization rules
   - Cleaner and more readable code

### 3. **Helper Functions** (`helpers.go`)
   - Consolidated utility functions
   - Reusable across all middleware
   - Better separation of concerns

## New Middleware Architecture

### Core Components

```
middleware/
├── auth_service.go              # Core authorization service
├── authorization_refactored.go  # Main authorization middleware
├── helpers.go                   # Utility functions
├── site_auth_refactored.go      # Site-level authorization
├── jwt.go                       # JWT authentication (unchanged)
└── abac_middleware.go           # ABAC policies (unchanged)
```

## Migration Guide

### Before (Old Way)

```go
// Multiple middleware files with duplicated logic
r.Handle("/api/v1/users",
    middleware.RequirePermission("read_users")(handler))

r.Handle("/api/v1/business/{id}/reports",
    middleware.RequireBusinessPermission("read_reports")(handler))

r.Handle("/api/v1/admin/users",
    middleware.RequirePermission("admin_all")(handler))
```

### After (New Way - Option 1: Simple)

```go
// Using convenience functions (same as before, but cleaner internally)
r.Handle("/api/v1/users",
    middleware.RequirePermission("read_users")(handler))

r.Handle("/api/v1/business/{id}/reports",
    middleware.RequireBusinessPermission("read_reports")(handler))

r.Handle("/api/v1/admin/users",
    middleware.RequireSuperAdmin()(handler))
```

### After (New Way - Option 2: Advanced)

```go
// Using functional options for complex authorization
r.Handle("/api/v1/complex-endpoint",
    middleware.Authorize(
        middleware.WithPermission("read_reports"),
        middleware.WithBusinessPermission("business_admin"),
    )(handler))

// Multiple permission options
r.Handle("/api/v1/files/upload",
    middleware.Authorize(
        middleware.WithAnyPermission("create_reports", "create_materials"),
    )(handler))
```

## Usage Examples

### 1. Global Permission Check

```go
// Require a specific global permission
r.Handle("/api/v1/users",
    middleware.RequirePermission("read_users")(handler))

// Alternative using Authorize
r.Handle("/api/v1/users",
    middleware.Authorize(
        middleware.WithPermission("read_users"),
    )(handler))
```

### 2. Multiple Permission Options

```go
// Require ANY of the specified permissions
r.Handle("/api/v1/files/upload",
    middleware.RequireAnyPermission([]string{
        "create_reports",
        "create_materials",
    })(handler))

// Alternative using Authorize
r.Handle("/api/v1/files/upload",
    middleware.Authorize(
        middleware.WithAnyPermission("create_reports", "create_materials"),
    )(handler))
```

### 3. Business-Specific Permissions

```go
// Require business-specific permission
r.Handle("/api/v1/business/{id}/reports",
    middleware.RequireBusinessPermission("read_reports")(handler))

// Require business admin
r.Handle("/api/v1/business/{id}/settings",
    middleware.RequireBusinessAdmin()(handler))

// Require any business access
r.Handle("/api/v1/business/{id}/info",
    middleware.RequireBusinessAccess()(handler))
```

### 4. Super Admin Only

```go
// Require super admin privileges
r.Handle("/api/v1/admin/system-config",
    middleware.RequireSuperAdmin()(handler))
```

### 5. Combined Authorization

```go
// Require both global AND business permissions
r.Handle("/api/v1/complex-endpoint",
    middleware.Authorize(
        middleware.WithPermission("create_reports"),
        middleware.WithBusinessPermission("business_admin"),
    )(handler))
```

### 6. Site-Level Authorization

```go
// Require site access (use after business access)
r.Handle("/api/v1/business/{id}/sites/{siteId}/data",
    middleware.RequireBusinessAccess()(
        middleware.RequireSiteAccess()(handler)))
```

## Authorization Flow

```
Request → JWT Middleware → Authorize Middleware → Handler
                              ↓
                        AuthService
                              ↓
                    Load User Context
                              ↓
                ┌─────────────┴─────────────┐
                ↓                           ↓
        Check Permissions           Check Business Context
                ↓                           ↓
            Allow/Deny                  Allow/Deny
```

## Key Features

### 1. **UserContext**
Central authorization context containing:
- User information
- JWT claims
- Super admin status
- Global permissions
- Business context (if applicable)
- Site context (if applicable)

### 2. **AuthService Methods**
- `LoadUserContext()` - Load complete user authorization context
- `IsSuperAdmin()` - Check super admin status
- `HasPermission()` - Check global permission
- `HasAnyPermission()` - Check any of multiple permissions
- `HasBusinessPermission()` - Check business-specific permission
- `HasBusinessAccess()` - Check business access
- `GetAccessibleBusinessVerticals()` - Get user's accessible businesses

### 3. **Functional Options**
- `WithPermission()` - Require specific global permission
- `WithAnyPermission()` - Require any of specified permissions
- `WithBusinessPermission()` - Require business permission
- `WithBusinessAccess()` - Require business access
- `WithSuperAdmin()` - Require super admin

## Benefits

### 1. **Reduced Code Duplication**
- Authorization logic centralized in `AuthService`
- Single implementation reused across all middleware

### 2. **Better Testability**
- Service layer can be unit tested independently
- Mock-friendly design

### 3. **Improved Maintainability**
- Changes to authorization logic in one place
- Consistent behavior across all endpoints

### 4. **More Flexible**
- Easy to combine multiple authorization requirements
- Extensible with new authorization options

### 5. **Better Performance**
- User context loaded once per request
- Caching opportunities for future optimization

## Helper Functions

### Context Helpers
```go
// Get current business ID from request
businessID := middleware.GetCurrentBusinessID(r)

// Get user's business context
bizCtx := middleware.GetUserBusinessContext(r)

// Get all user permissions
perms := middleware.GetUserPermissions(r)

// Get business-specific permissions
bizPerms := middleware.GetBusinessPermissions(r)
```

### Site Access Helpers
```go
// Check if user can access site
canAccess := middleware.CanAccessSite(r, siteID)

// Check specific site permissions
canCreate := middleware.CanCreateInSite(r, siteID)
canUpdate := middleware.CanUpdateInSite(r, siteID)
canDelete := middleware.CanDeleteInSite(r, siteID)
```

### Role Level Helpers
```go
// Get user's role level
level := middleware.GetUserRoleLevel(userID)

// Check if user can assign role
canAssign := middleware.CanUserAssignRole(userID, targetRoleLevel)

// Get max assignable level
maxLevel := middleware.GetMaxAssignableLevel(userID)
```

## Migration Strategy

### Phase 1: Add New Files ✅
- Add `auth_service.go`
- Add `authorization_refactored.go`
- Add `helpers.go`
- Add `site_auth_refactored.go`

### Phase 2: Test New Implementation
- Test all authorization scenarios
- Verify backward compatibility
- Performance testing

### Phase 3: Gradual Migration
- Keep old middleware for backward compatibility
- Migrate routes incrementally
- Monitor for issues

### Phase 4: Cleanup (Optional)
- Remove old middleware files once fully migrated:
  - `authorization.go` → `authorization_refactored.go`
  - `business_auth.go` → `authorization_refactored.go`
  - `role_level.go` → `helpers.go`
  - `site_auth.go` → `site_auth_refactored.go`

## Best Practices

### 1. **Use Convenience Functions for Simple Cases**
```go
// Good: Simple and readable
middleware.RequirePermission("read_users")

// Overkill: Too verbose for simple case
middleware.Authorize(middleware.WithPermission("read_users"))
```

### 2. **Use Authorize() for Complex Cases**
```go
// Good: Complex requirements clearly expressed
middleware.Authorize(
    middleware.WithPermission("create_reports"),
    middleware.WithBusinessPermission("business_admin"),
)
```

### 3. **Chain Middleware for Layered Authorization**
```go
// Good: Clear authorization layers
router.Handle("/path",
    middleware.RequireBusinessAccess()(
        middleware.RequireSiteAccess()(handler)))
```

### 4. **Super Admin Bypass**
All authorization checks automatically pass for super admins. No need to explicitly check.

### 5. **Error Handling**
Authorization errors are handled consistently:
- `401 Unauthorized` - Not authenticated
- `403 Forbidden` - Insufficient permissions
- `400 Bad Request` - Missing required context (e.g., business ID)

## Troubleshooting

### Issue: "unauthorized" error
- Check if JWT middleware is applied
- Verify JWT token is valid
- Check if user exists in database

### Issue: "insufficient permissions"
- Verify user has the required permission
- Check if permission name matches exactly
- For super admins, check if admin_all permission exists

### Issue: "business vertical not specified"
- Ensure business ID is in URL path, query param, or header
- Check business ID format (UUID or code)
- Verify business is active

### Issue: Performance concerns
- User context is loaded once per request
- Consider caching for high-traffic endpoints
- Use appropriate preloading in AuthService

## Future Enhancements

1. **Caching Layer**
   - Cache user permissions
   - Cache business context
   - Redis integration

2. **Audit Logging**
   - Log all authorization decisions
   - Track permission checks
   - Security monitoring

3. **Dynamic Permissions**
   - Runtime permission evaluation
   - Conditional permissions
   - Time-based permissions

4. **Performance Optimization**
   - Lazy loading of context
   - Batch permission checks
   - Query optimization

## Summary

The refactored middleware provides:
- ✅ Cleaner, more maintainable code
- ✅ Centralized authorization logic
- ✅ Flexible and composable
- ✅ Backward compatible
- ✅ Better testability
- ✅ Improved performance opportunities

All existing middleware functions continue to work, but now use the unified service internally for consistency and maintainability.
