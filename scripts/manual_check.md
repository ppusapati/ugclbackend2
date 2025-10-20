# Manual Database Check Guide

## Quick Verification (Copy-Paste These Queries)

### 1. Check if planning:update is assigned to Consultant

```sql
SELECT * FROM role_permissions
WHERE role_id = 'fc2b9b02-c0d3-4edc-8981-691c84c598e2'
  AND permission_id = 'e65247fc-534d-44bd-921b-45299059d84a';
```

**Expected Result:** 1 row with:
- `role_id`: fc2b9b02-c0d3-4edc-8981-691c84c598e2
- `permission_id`: e65247fc-534d-44bd-921b-45299059d84a
- `created_at`: (timestamp when you ran the app)

### 2. List ALL Consultant permissions

```sql
SELECT
    p.name as permission_name,
    p.id as permission_id,
    rp.created_at
FROM role_permissions rp
JOIN permissions p ON rp.permission_id = p.id
WHERE rp.role_id = 'fc2b9b02-c0d3-4edc-8981-691c84c598e2'
ORDER BY p.name;
```

**Expected Result:** 4 rows:
1. planning:read
2. planning:update ← This one should be present now!
3. project:read
4. project:update

### 3. Count total permissions for Consultant

```sql
SELECT COUNT(*) as total_permissions
FROM role_permissions
WHERE role_id = 'fc2b9b02-c0d3-4edc-8981-691c84c598e2';
```

**Expected Result:** 4

## If Still No Results

### Step 1: Verify the role exists
```sql
SELECT * FROM roles WHERE id = 'fc2b9b02-c0d3-4edc-8981-691c84c598e2';
```

### Step 2: Verify the permission exists
```sql
SELECT * FROM permissions WHERE id = 'e65247fc-534d-44bd-921b-45299059d84a';
```

### Step 3: Check if ANY permissions exist for Consultant
```sql
SELECT COUNT(*) FROM role_permissions WHERE role_id = 'fc2b9b02-c0d3-4edc-8981-691c84c598e2';
```

### Step 4: Check application logs
Look for these lines in your console:
```
✅ Inserted permission 'planning:update' into role_permissions
✅ Successfully assigned 4/4 permissions to role 'Consultant' (verified in DB)
```

If you see errors like:
```
❌ Failed to insert permission 'planning:update' for role 'Consultant': <error>
```

Then there's a database constraint or connection issue.

## Common Issues

### Issue 1: Stale Database Connection
**Solution:** Restart your Go application to trigger fresh seeding

### Issue 2: Database Transaction Not Committed
**Solution:** The new code uses `DB.Create()` which auto-commits. Check your DB connection settings.

### Issue 3: Wrong Database
**Solution:** Verify you're connected to the correct database:
```sql
SELECT current_database();
```

### Issue 4: Duplicate Key Constraint
If you see errors about duplicate keys, the table might have orphaned records:
```sql
-- Clean up and re-seed
DELETE FROM role_permissions WHERE role_id = 'fc2b9b02-c0d3-4edc-8981-691c84c598e2';
-- Then restart your Go app
```

## Success Indicators

✅ **Application logs show:**
- `✅ Inserted permission 'planning:update' into role_permissions`
- `✅ Successfully assigned 4/4 permissions to role 'Consultant' (verified in DB)`

✅ **Database query returns:**
- 1 row when checking for the specific permission
- 4 rows total for Consultant role

✅ **No errors in logs:**
- No `❌` symbols
- No GORM errors
- No database connection issues

## Next Steps After Verification

Once verified:
1. ✅ The seeding is working correctly
2. ✅ All roles have their permissions
3. ✅ Your RBAC system is ready to use
4. ✅ Middleware will correctly check permissions

## Test the Authorization

Create a test user with Consultant role and try to access endpoints:

```go
// User with Consultant role should be able to:
// ✅ Read projects (has project:read)
// ✅ Update projects (has project:update)
// ✅ Read plans (has planning:read)
// ✅ Update plans (has planning:update) ← Now working!

// User should NOT be able to:
// ❌ Delete projects (no project:delete)
// ❌ Approve plans (no planning:approve)
// ❌ Manage users (no user:*)
```
