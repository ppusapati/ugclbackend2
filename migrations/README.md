# Database Migrations - Standardized Strategy

## Overview

This project uses **Gormigrate** as the single, standardized database migration system. All production database schema changes flow through a consolidated migration process defined in `config/migrations.go`.

## Active Migration System: Gormigrate (config/migrations.go)

All database schema and migrations are defined programmatically in [config/migrations.go](../config/migrations.go) using the gormigrate library.

### Key Characteristics:
- **Single Source of Truth**: All migrations tracked in one file
- **Programmatic Approach**: Use GORM's `AutoMigrate` for models and raw SQL for complex operations
- **Backward-compatible**: Migrations are cumulative; earlier migrations remain in the migration registry
- **Automatic Application**: Migrations run automatically on application startup

### Migration IDs
Migrations use timestamp-based IDs (e.g., `"20260429_perf_indexes_hotpath"`) to ensure sequencing and avoid conflicts.

## Adding New Migrations

1. Open `config/migrations.go`
2. Add a new migration entry to the migrations slice with:
   - Unique ID (use date prefix: `"20YYMMDD_description"`)
   - `Migrate` function: contains up/forward migrations
   - Optional `Rollback` function: contains down/reverse migrations
3. Use GORM's `AutoMigrate()` for model-based schema or raw SQL for complex changes
4. Test the migration locally before deployment

### Example:
```go
{
    ID: "20260430_add_new_feature",
    Migrate: func(tx *gorm.DB) error {
        return tx.AutoMigrate(&models.NewFeatureModel{})
    },
    Rollback: func(tx *gorm.DB) error {
        return tx.Migrator().DropTable(&models.NewFeatureModel{})
    },
}
```

## Archive Directory

The `archive/` directory contains legacy SQL migration files (prefixed `000010-000017`, `010_create_project_management_tables.sql`) from a previous go-migrate-based approach. These are preserved for historical reference but are **not used by the application**.

**Important**: Do not attempt to run these SQL files directly. They are superseded by the consolidated gormigrate migration in `config/migrations.go`.

## Production Deployment

- **Database initialization** is automatic when the Go application starts
- **No manual SQL execution required** - migrations are applied programmatically
- **Version tracking** is maintained in the `migrations` table by Gormigrate
- **Failed migrations** prevent application startup (fail-fast safety)

## Troubleshooting

### Migration Not Applied
- Check `config/migrations.go` for the migration entry
- Verify the migration ID is unique
- Ensure the migration's `Migrate` function has no syntax errors
- Check application logs for the specific error

### Rolling Back a Migration
- Ensure the migration entry includes a `Rollback` function
- Stop the application
- Manually execute the rollback SQL or call `Rollback()` directly
- Remove or comment out the migration ID from `config/migrations.go`

### Duplicate Migration IDs
- Migration IDs must be globally unique within `config/migrations.go`
- Use timestamp-based naming: `"20YYMMDD_description"`
- Gormigrate will fail with a clear error if duplicates exist

## Related Files
- **Active**: [config/migrations.go](../config/migrations.go)
- **Inactive Archive**: `migrations/archive/*.sql`
