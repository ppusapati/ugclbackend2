# UGCL Backend AI Coding Agent Instructions

## Architecture Overview

This Go backend serves a multi-tenant enterprise construction/infrastructure management system with sophisticated permission-based routing and business vertical isolation.

### Core Architecture Pattern
- **Multi-layered authorization**: JWT → RBAC → ABAC → Business Vertical → Site-level permissions
- **Business vertical isolation**: Each business unit (construction, water management, etc.) has isolated data and permissions
- **Permission-based routing**: Every route requires explicit permissions via middleware decorators
- **GORM with PostgreSQL + PostGIS**: Heavy use of spatial data for infrastructure projects

## Critical Project Patterns

### Permission Middleware Pattern
All protected routes use functional middleware wrapping:
```go
// Standard pattern - NEVER bypass middleware
api.Handle("/endpoint", middleware.RequirePermission("read_reports")(
    http.HandlerFunc(handlers.GetReports))).Methods("GET")

// Business-scoped permissions
api.Handle("/business/{businessCode}/data", 
    middleware.RequireBusinessPermission("read_reports")(handler))

// Complex authorization (use for multi-requirement routes)
middleware.Authorize(
    middleware.WithPermission("create_reports"),
    middleware.WithBusinessPermission("business_admin"),
)(handler)
```

### Business Vertical Architecture
- Routes: `/api/v1/business/{businessCode}/...` for business-scoped operations
- All business data is isolated by `business_vertical_id` 
- Use `middleware.GetCurrentBusinessID(r)` to extract business context
- Check `middleware/README.md` for complete authorization patterns

### Model Patterns
- UUID primary keys with `gorm:"type:uuid;primaryKey"` and `BeforeCreate` hooks
- Soft deletes with `gorm.DeletedAt` (standard GORM pattern)
- Business isolation via `BusinessVerticalID *uuid.UUID` fields
- Spatial data using PostGIS geometry fields for location-based features

### File Structure Navigation
- `routes/routes_v2.go` - Main route definitions (use this over routes.go)
- `middleware/authorization_refactored.go` - New authorization system (preferred)
- `middleware/README.md` - Complete authorization guide with examples
- `docs/` - Extensive documentation for ABAC, middleware, and implementation guides
- `config/migrations.go` - Database migrations and setup
- `form_definitions/` - JSON form schemas for dynamic forms

## Development Workflows

### Building & Running
```bash
# Development
go run main.go

# Production build (cross-platform)
./go_build.sh linux . ugcl-backend v1.0.0
./go_build.sh windows . ugcl-backend v1.0.0

# Docker
docker build -t ugcl-backend .
```

### Database Setup
- Requires PostgreSQL with PostGIS extension
- Auto-migrations run on startup via `config.Migrations(DB)`
- Connection pool configured in `config/config.go` with environment-based tuning
- Spatial queries for project management and site mapping

### Testing Authorization
- Use `/api/v1/test/auth` and `/api/v1/test/permission` endpoints
- JWT token required in Authorization header: `Bearer <token>`
- Check `middleware/helpers.go` for context extraction utilities

## Key Integration Points

### External Dependencies
- **Google Cloud Storage** for file uploads (auto-detects environment)
- **PostGIS** for spatial data queries in project management
- **JWT** for authentication with custom claims structure
- **Gorilla Mux** for routing (being migrated from Gin)

### Form System
- Dynamic forms defined in `form_definitions/*.json`
- Business vertical access controlled via database associations
- Forms rendered dynamically on frontend using JSON schemas

### ABAC Policy System
- Attribute-based access control layered on top of RBAC
- Policies defined in JSON format in database
- Context-aware (time, location, device) authorization
- See `docs/ABAC_IMPLEMENTATION_GUIDE.md` for complete implementation

## Code Style Conventions

### Error Handling
```go
// Standard pattern for API responses
if err != nil {
    c.JSON(http.StatusInternalServerError, gin.H{
        "error": "Failed to create resource",
        "details": err.Error(),
    })
    return
}
```

### Database Queries
- Use GORM preloading for relationships: `Preload("RoleModel.Permissions")`
- Filter by business vertical: `Where("business_vertical_id = ?", businessID)`
- Spatial queries use PostGIS functions: `ST_Contains`, `ST_DWithin`

### Middleware Usage
- Apply `middleware.SecurityMiddleware` and `middleware.JWTMiddleware` to all protected route groups
- Use permission-specific middleware for fine-grained access control
- Combine multiple middleware with functional composition for complex requirements

## Common Pitfalls to Avoid

1. **Don't bypass middleware**: Every protected route must use appropriate middleware
2. **Business isolation**: Always filter queries by business_vertical_id in multi-tenant operations  
3. **Spatial data**: Use PostGIS geometry types, not plain lat/lng for location data
4. **Permission names**: Use exact strings from database - typos cause silent authorization failures
5. **UUID handling**: Always validate UUID format before database queries

## Key Files for Context
- `middleware/README.md` - Authorization system overview
- `docs/MIDDLEWARE_QUICK_REFERENCE.md` - Common patterns
- `routes/routes_v2.go` - Route structure and permission mapping
- `IMPLEMENTATION_GUIDE.md` - Project management system integration
- `models/user.go` - User model with role hierarchy methods