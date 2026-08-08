# Scoped RBAC Production Cutover

## Authorization Contract

- A role has exactly one scope: `global` or `business_vertical`.
- A business-scoped role belongs to exactly one business vertical.
- A user can have one active role per scope key: one global assignment and one assignment per business vertical.
- Global permissions apply system-wide. Business permissions apply only when that business vertical is the active request context.
- A business route still requires an active assignment for its vertical; a global permission does not create business membership.
- `users.business_vertical_id` is a UI preference and never grants access.
- Site access remains independent of role assignment.

## Canonical API

All endpoints require JWT authentication and `manage_roles`:

- `GET /api/v1/admin/rbac/roles`
- `POST /api/v1/admin/rbac/roles`
- `PUT /api/v1/admin/rbac/roles/{roleId}`
- `DELETE /api/v1/admin/rbac/roles/{roleId}` (soft deactivation)
- `GET /api/v1/admin/rbac/users/{userId}/assignments`
- `POST /api/v1/admin/rbac/users/{userId}/assignments`
- `DELETE /api/v1/admin/rbac/users/{userId}/assignments/{assignmentId}` (soft deactivation)

Role and assignment mutations are transactional. The API validates permission IDs, scope consistency, actor hierarchy, protected `super_admin` behavior, self-lockout, and assignment uniqueness.

## Prerequisites

Do not migrate or set `RBAC_ENABLED=true` until every item is available:

1. PostgreSQL client tools containing `pg_dump` and `pg_restore`.
2. `DB_DSN` for the source database.
3. `RESTORE_DB_DSN` for a disposable database that is not the source database.
4. Enough storage for a custom-format backup and restored database.
5. A maintenance window covering backup, restore, audit, migration, and smoke checks.

The migration is additive. It does not delete or alter legacy role, permission, assignment, or user rows.

## 1. Verify The Backup

From the backend repository root:

```powershell
$env:DB_DSN = '<source PostgreSQL DSN>'
$env:RESTORE_DB_DSN = '<disposable restore PostgreSQL DSN>'
.\scripts\backup_and_verify_database.ps1 -PostgresBin 'C:\Program Files\PostgreSQL\17\bin'
```

Omit `-PostgresBin` when PostgreSQL tools are on `PATH`.

The command must produce all four artifacts under `backups/database` and finish with `Restore audit matches the source RBAC audit`:

- `ugcl-<timestamp>.dump`
- `ugcl-<timestamp>.dump.sha256`
- `ugcl-<timestamp>-source-rbac.json`
- `ugcl-<timestamp>-restore-rbac.json`

Any dump, restore, checksum, row-count, or checksum-parity failure blocks the cutover.

## 2. Apply The Additive Copy

Keep the application on legacy authorization (`RBAC_ENABLED=false`). Use the four artifacts from the same verified backup:

```powershell
go run ./scripts/prepare-rbac -apply `
  -backup '<backup.dump>' `
  -checksum '<backup.dump.sha256>' `
  -source-audit '<source-rbac.json>' `
  -restore-audit '<restore-rbac.json>'
```

The command rechecks the backup checksum, restored audit, and current live legacy audit before opening a transaction. It creates and copies:

- `rbac_roles`
- `rbac_role_permissions`
- `user_role_assignments`

It then verifies copy parity and confirms that all legacy table checksums are unchanged. Any failure rolls back the transaction.

## 3. Audit Before Enablement

```powershell
go run ./scripts/rbac-audit -output '<post-migration-legacy-audit.json>'
$env:JWT_SECRET = '<deployment JWT secret>'
go test ./... -count=1
```

Compare the post-migration legacy audit with the source audit. Counts and checksums must match.

## 4. Enable RBAC

Set the production environment and restart all backend instances:

```text
RBAC_ENABLED=true
```

`UNIFIED_RBAC_ENABLED` is a deprecated compatibility alias. Do not configure both flags.

Startup fails closed when an RBAC table is missing or copied role/assignment counts differ from legacy sources. Do not bypass this check.

After restart:

1. Sign in as a protected administrator and verify `/token` includes `role_assignments`.
2. Load the role editor and verify global and business-scoped roles.
3. Load one user with multiple business assignments and switch active verticals.
4. Verify a permitted attendance request in the assigned vertical succeeds.
5. Verify the same request in an unassigned vertical returns `403`.
6. Verify a global permission works after valid business context is selected.
7. Verify an offline mobile bootstrap retains assignments and active-vertical isolation.
8. Create, update, assign, remove, and deactivate a non-protected test role.

## Rollback

If RBAC authorization is incorrect:

1. Set `RBAC_ENABLED=false` on every backend instance.
2. Restart every backend instance.
3. Confirm legacy authentication and a representative global, business, attendance, and site request.
4. Preserve the RBAC tables and migration artifacts for diagnosis.

No database restore is required for a feature-flag rollback because the migration is additive and legacy data remains unchanged. Restore the verified dump only for an independently confirmed database corruption event, using the normal database recovery procedure.

## Removal Gate

Do not remove legacy role tables, columns, routes, or fallback code in this release. Removal requires a separate migration after production parity has been observed, rollback is no longer required, and all deployed web/mobile versions consume `role_assignments`.
