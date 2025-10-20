-- Quick check for Consultant role permissions
-- Run this query to verify planning:update is assigned

SELECT
    'Consultant Role Permissions' as query_name,
    r.id as role_id,
    r.name as role_name,
    p.id as permission_id,
    p.name as permission_name,
    rp.created_at as assigned_at
FROM role_permissions rp
JOIN roles r ON rp.role_id = r.id
JOIN permissions p ON rp.permission_id = p.id
WHERE r.id = 'fc2b9b02-c0d3-4edc-8981-691c84c598e2'
ORDER BY p.name;

-- Check specifically for planning:update
SELECT
    'Planning Update Permission Check' as query_name,
    CASE
        WHEN EXISTS (
            SELECT 1
            FROM role_permissions
            WHERE role_id = 'fc2b9b02-c0d3-4edc-8981-691c84c598e2'
            AND permission_id = 'e65247fc-534d-44bd-921b-45299059d84a'
        ) THEN '✅ YES - Permission is assigned!'
        ELSE '❌ NO - Permission is NOT assigned'
    END as result;

-- Count all permissions for Consultant
SELECT
    'Total Permissions Count' as query_name,
    COUNT(*) as total_permissions
FROM role_permissions
WHERE role_id = 'fc2b9b02-c0d3-4edc-8981-691c84c598e2';
