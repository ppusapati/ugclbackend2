-- Verification Script for RBAC Permissions
-- Run this to check the state of your permissions and roles

-- 1. Check all roles
SELECT
    'GLOBAL ROLES' as category,
    r.id,
    r.name,
    r.description,
    r.is_global,
    r.is_active,
    COUNT(rp.permission_id) as permission_count
FROM roles r
LEFT JOIN role_permissions rp ON r.id = rp.role_id
GROUP BY r.id, r.name, r.description, r.is_global, r.is_active
ORDER BY r.name;

-- 2. Check Consultant role specifically
SELECT
    'CONSULTANT ROLE DETAILS' as info,
    r.id as role_id,
    r.name as role_name,
    p.id as permission_id,
    p.name as permission_name,
    p.resource,
    p.action
FROM roles r
LEFT JOIN role_permissions rp ON r.id = rp.role_id
LEFT JOIN permissions p ON rp.permission_id = p.id
WHERE r.name = 'Consultant'
ORDER BY p.name;

-- 3. Check if planning:update permission exists
SELECT
    'PLANNING:UPDATE PERMISSION' as info,
    id,
    name,
    resource,
    action,
    description
FROM permissions
WHERE name = 'planning:update';

-- 4. Check role_permissions junction table for Consultant
SELECT
    'ROLE_PERMISSIONS JUNCTION TABLE' as info,
    rp.role_id,
    r.name as role_name,
    rp.permission_id,
    p.name as permission_name,
    rp.created_at
FROM role_permissions rp
JOIN roles r ON rp.role_id = r.id
JOIN permissions p ON rp.permission_id = p.id
WHERE r.name = 'Consultant'
ORDER BY p.name;

-- 5. Check for duplicate permissions
SELECT
    'DUPLICATE PERMISSIONS CHECK' as info,
    name,
    COUNT(*) as count,
    STRING_AGG(id::text, ', ') as permission_ids
FROM permissions
GROUP BY name
HAVING COUNT(*) > 1;

-- 6. Count total permissions
SELECT
    'TOTAL COUNTS' as info,
    (SELECT COUNT(*) FROM permissions) as total_permissions,
    (SELECT COUNT(*) FROM roles WHERE is_global = true) as total_global_roles,
    (SELECT COUNT(*) FROM role_permissions) as total_role_permission_assignments;

-- 7. Check business roles and their permissions
SELECT
    'BUSINESS ROLES' as category,
    br.id,
    br.name,
    br.display_name,
    bv.name as business_vertical,
    COUNT(brp.permission_id) as permission_count
FROM business_roles br
JOIN business_verticals bv ON br.business_vertical_id = bv.id
LEFT JOIN business_role_permissions brp ON br.id = brp.business_role_id
GROUP BY br.id, br.name, br.display_name, bv.name
ORDER BY bv.name, br.name;

-- 8. Find missing permissions in role_permissions
-- (Permissions that should be assigned to Consultant but aren't)
SELECT
    'MISSING PERMISSIONS FOR CONSULTANT' as info,
    p.id,
    p.name,
    p.resource,
    p.action
FROM permissions p
WHERE p.name IN ('project:read', 'project:update', 'planning:read', 'planning:update')
AND p.id NOT IN (
    SELECT rp.permission_id
    FROM role_permissions rp
    JOIN roles r ON rp.role_id = r.id
    WHERE r.name = 'Consultant'
);
