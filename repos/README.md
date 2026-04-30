# Repository Layer - Tenant-Scoped Data Access

## Overview

The `repos/` package provides a data access layer that enforces **automatic tenant isolation** across all database queries. This prevents accidental data leakage between business verticals and provides a consistent, secure interface for handlers to interact with the database.

## Architecture

### TenantContext
Every repository operation is tied to a `TenantContext`, which holds:
- **UserID**: The ID of the requesting user
- **BusinessVertical**: The UUID of the user's business vertical (nil for super-admin context)

### BaseRepo
All specific repositories inherit from `BaseRepo`, which provides:
- **Automatic Filtering**: `DB()` returns a GORM instance with `business_vertical_id` filtering automatically applied
- **Admin Access**: `DBAdmin()` provides unfiltered access for system-level operations (protected by permission checks)
- **Enforcement**: `EnforceBusinessVertical()` validates that a record belongs to the tenant
- **Audit Logging**: `LogQuery()` logs all operations for debugging/auditing

### Specific Repositories
Each domain has a dedicated repository:
- **UserRepo**: User management with tenant scoping
- **SiteRepo**: Site/location management within a business vertical
- **BusinessVerticalRepo**: Admin-level business vertical management

## Usage Pattern

### Basic Usage in Handlers

```go
// In a handler function
func GetSiteHandler(w http.ResponseWriter, r *http.Request) {
    // Get user context from middleware
    userCtx := middleware.GetUserContext(r)
    if userCtx == nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    // Create repository with user context
    siteRepo := repos.NewSiteRepo(userCtx)

    // Query - automatically scoped to user's business vertical
    siteID := uuid.MustParse(mux.Vars(r)["siteId"])
    site, err := siteRepo.GetByID(siteID)
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }

    // If user from Business A tries to access a site from Business B,
    // GetByID returns "access denied" - automatic protection
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(site)
}
```

### Creating Repositories

Each repository is created with a `*middleware.UserContext`:

```go
// For site operations
siteRepo := repos.NewSiteRepo(userCtx)

// For user operations
userRepo := repos.NewUserRepo(userCtx)

// For business vertical management (admin-only)
bvRepo := repos.NewBusinessVerticalRepo(userCtx)
```

## Tenant Scoping Behavior

### For Tenant-Scoped Users
- **DB()**: Returns filtered queries that only include records matching `business_vertical_id = user.business_vertical_id`
- **DBAdmin()**: Should NOT be used for user-facing operations; use only for audit/sync operations
- All CRUD operations automatically enforce tenant isolation

### For Super-Admins (no business_vertical_id)
- **DB()**: Returns unfiltered queries (no automatic filtering)
- **DBAdmin()**: Same as DB() for super-admins
- Explicit permission checks at handler level should still protect sensitive operations

## Creating New Repositories

### Step 1: Define the Repository Struct
```go
type MyEntityRepo struct {
    *BaseRepo
}

func NewMyEntityRepo(userCtx *middleware.UserContext) *MyEntityRepo {
    return &MyEntityRepo{
        BaseRepo: NewBaseRepo(userCtx),
    }
}
```

### Step 2: Implement CRUD Methods
```go
func (mer *MyEntityRepo) GetByID(id uuid.UUID) (*models.MyEntity, error) {
    var entity models.MyEntity
    
    // Use mer.DB() for tenant-scoped queries
    if err := mer.DB().Where("id = ?", id).First(&entity).Error; err != nil {
        return nil, fmt.Errorf("failed to get entity: %w", err)
    }

    mer.LogQuery("my_entity.get_by_id", map[string]interface{}{
        "entity_id": id.String(),
    })
    
    return &entity, nil
}

func (mer *MyEntityRepo) Create(entity *models.MyEntity) error {
    // Ensure entity belongs to tenant if applicable
    if mer.TenantID() != nil && entity.BusinessVerticalID != mer.TenantID() {
        return fmt.Errorf("entity business vertical mismatch")
    }

    if err := mer.DB().Create(entity).Error; err != nil {
        return fmt.Errorf("failed to create entity: %w", err)
    }

    mer.LogQuery("my_entity.create", map[string]interface{}{
        "entity_id": entity.ID.String(),
    })
    
    return nil
}
```

### Step 3: Register Query Logging
Always call `LogQuery()` for auditing:
```go
mer.LogQuery("operation_name", map[string]interface{}{
    "param_name": paramValue,
})
```

## Security Guarantees

### Automatic Tenant Filtering
The repository layer ensures that:
1. All tenant-scoped queries automatically filter by `business_vertical_id`
2. A user cannot accidentally query data from other business verticals
3. Even if a handler implements incorrect WHERE clauses, the repository layer adds the business vertical filter

### Example Security Benefit
```go
// Handler code (potentially buggy)
var sites []models.Site
db.Where("active = true").Find(&sites)  // Developer forgot tenant filter!

// Using repository (safe)
var sites []models.Site
siteRepo := repos.NewSiteRepo(userCtx)
siteRepo.DB().Where("active = true").Find(&sites)  
// Repository.DB() automatically adds: AND business_vertical_id = ?
```

### Permission vs. Data Access
- **Permissions** (JWT/RBAC): Determine if a user CAN perform an operation
- **Repositories** (tenant filtering): Determine WHICH records the user can access

Both layers work together for defense-in-depth.

## Performance Considerations

### Query Filtering
- Tenant filtering adds `WHERE business_vertical_id = ?` to every query
- Ensure **indexes** exist on `business_vertical_id` for all multi-tenant tables
- The ORM query builder does not add redundant filters if already present

### Batch Operations
```go
// Batch create within tenant
var sites []models.Site
siteRepo := repos.NewSiteRepo(userCtx)
for _, site := range newSites {
    if err := siteRepo.Create(site); err != nil {
        // Handle error
    }
}

// Or batch with DB().CreateInBatches()
siteRepo.DB().CreateInBatches(newSites, 100)
```

## Troubleshooting

### "Access Denied" Errors on Legitimate Operations
- Verify the entity's `business_vertical_id` matches the user's tenant
- Check if user's `UserContext.BusinessContext.BusinessVerticalID` is set correctly in middleware
- For super-admin operations, verify user has appropriate permissions before calling handlers

### Missing Data
- Ensure the entity has the `business_vertical_id` field populated when creating
- Check indexes on `business_vertical_id` are being used (use EXPLAIN query plans)
- Verify tenant context is being passed correctly to the repository

### Slow Queries
- Add indexes: `CREATE INDEX idx_business_vertical_id ON table_name(business_vertical_id);`
- Use `Preload` for eager loading of relationships
- Monitor slow query logs

## Related Files
- **Base Repository**: [base.go](./base.go)
- **User Repository**: [user_repo.go](./user_repo.go)
- **Site Repository**: [site_repo.go](./site_repo.go)
- **Business Vertical Repository**: [business_vertical_repo.go](./business_vertical_repo.go)
- **Middleware User Context**: [../middleware/auth_service.go](../middleware/auth_service.go)
