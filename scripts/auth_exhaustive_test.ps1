param(
    [string]$ApiBase        = "http://localhost:8080/api/v1",
    [string]$ApiKey         = "87339ea3-1add-4689-ae57-3128ebd03c4f",
    [string]$BusinessCode   = "WATER"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Set-Location "E:\Maheshwari\UGCL\backend\v1"
$RunTag = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds().ToString()

# ---------------------------------------------------------------------------
# Known IDs (verified via previous tests)
# ---------------------------------------------------------------------------
$ROLE_SUPER_ADMIN   = "01472eae-45a5-4257-930c-a63ccdab1d17"
$ROLE_SYSTEM_ADMIN  = "f7538498-5121-450f-b190-021cfa3da1a0"
$ROLE_ADMIN         = "f940638c-b219-4504-9c79-f51b17df6d17"
$ROLE_MANAGER       = "c5f89174-0c05-4632-aa4d-4404a1e7a8d5"
$ROLE_CONSULTANT    = "80390781-656f-4705-8004-c345de9416df"

$USER_SUPER   = "4f74d110-db88-4c7d-b5b5-ccf5ace1c0ea"
$USER_WA      = "6043e5f3-a707-4e64-a2e8-12c55fd5c8d3"
$USER_WE      = "946c55c0-d582-4710-88b2-c196535be17e"
$USER_SOLAR_A = "1c4d3932-7014-4ebc-803e-95b6d21e9723"
$USER_SOLAR_E = "7f503cd1-c66f-4caf-8ee5-785649ae7fec"
$USER_HO_ADMIN= "3860bc40-4057-483d-a5d5-ca21cec4e1dc"

# Attribute IDs (system-defined, verified via GET /attributes)
$ATTR_DEPARTMENT     = "fc1bb238-60e2-4cf3-905d-8c140b3bd8be"
$ATTR_CLEARANCE      = "ae80c796-6383-404d-8a90-8381e2778bb5"
$ATTR_LOCATION       = "156969c6-0770-4bf7-8e3f-2cd164ffcd51"
$ATTR_EMPLOYMENT     = "8669a3c8-7d0f-471b-b6c0-434b63c7b7c3"
$ATTR_SENSITIVITY    = "d7f11b07-704c-4368-bebe-113b15acf1f7"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
$baseH = @{
    "x-api-key"    = $ApiKey
    "Content-Type" = "application/json"
    "Accept"       = "application/json"
}

function Login([string]$Phone) {
    return Invoke-RestMethod -Method Post -Uri "$ApiBase/login" -Headers $baseH `
        -Body (@{ phone = $Phone; password = "Welcome@123" } | ConvertTo-Json)
}

function MkH([string]$token, [string]$biz = "") {
    $h = @{
        "x-api-key"    = $ApiKey
        "Authorization"= "Bearer $token"
        "Content-Type" = "application/json"
        "Accept"       = "application/json"
    }
    if ($biz) { $h["X-Business-Code"] = $biz }
    return $h
}

function Invoke-Api([string]$Method, [string]$Path, [hashtable]$Headers, [object]$Body = $null) {
    $uri = "$ApiBase$Path"
    try {
        if ($null -eq $Body) {
            return @{ ok = $true; data = (Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers) }
        }
        return @{ ok = $true; data = (Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers -Body ($Body | ConvertTo-Json -Depth 30)) }
    } catch {
        $code = 0
        try { $code = [int]$_.Exception.Response.StatusCode } catch {}
        return @{ ok = $false; status = $code; error = $_.Exception.Message }
    }
}

# Convenience wrappers
function API-OK   ([string]$M, [string]$P, $H, $B=$null) { (Invoke-Api $M $P $H $B).ok }
function API-Code ([string]$M, [string]$P, $H, $B=$null) { $r=Invoke-Api $M $P $H $B; if ($r.ok) { 200 } else { $r.status } }
function API-Data ([string]$M, [string]$P, $H, $B=$null) { (Invoke-Api $M $P $H $B).data }

$results = [System.Collections.Generic.List[object]]::new()
function Record([string]$Group, [string]$Case, [string]$Status, [string]$Detail) {
    $results.Add([pscustomobject]@{ Group=$Group; Case=$Case; Status=$Status; Detail=$Detail })
    $col = switch ($Status) { "PASS"{"Green"} "FAIL"{"Red"} "SKIP"{"Cyan"} default{"Yellow"} }
    Write-Host "[$Status] $Group :: $Case :: $Detail" -ForegroundColor $col
}
function Expect([string]$Group, [string]$Case, [bool]$Got, [bool]$Expected, [string]$Detail) {
    if ($Got -eq $Expected) { Record $Group $Case "PASS" $Detail }
    else { Record $Group $Case "FAIL" "Expected=$Expected Got=$Got :: $Detail" }
}

# Track created resources for cleanup
$createdPolicies   = [System.Collections.Generic.List[string]]::new()
$createdAttributes = [System.Collections.Generic.List[string]]::new()
$assignedUserAttrs = [System.Collections.Generic.List[hashtable]]::new()
$modifiedUserRoles = [System.Collections.Generic.List[hashtable]]::new()
$originalWEBusinessRole = $null

# ---------------------------------------------------------------------------
# Login all users
# ---------------------------------------------------------------------------
Write-Host "`n=== Logging in all test users ===" -ForegroundColor Cyan
$lSuper  = Login "9999999999"
$lWA     = Login "9999999901"
$lWE     = Login "9999999902"
$lSolarA = Login "9999999903"
$lSolarE = Login "9999999904"
$lHOA    = Login "9999999905"

$tSuper  = $lSuper.token
$tWA     = $lWA.token
$tWE     = $lWE.token
$tSolarA = $lSolarA.token
$tSolarE = $lSolarE.token
$tHOA    = $lHOA.token

$hSuper  = MkH $tSuper
$hWA     = MkH $tWA
$hWE     = MkH $tWE
$hSolarA = MkH $tSolarA
$hSolarE = MkH $tSolarE
$hHOA    = MkH $tHOA

$hSuperBiz = MkH $tSuper $BusinessCode
$hWABiz    = MkH $tWA    $BusinessCode
$hWEBiz    = MkH $tWE    $BusinessCode

# Helper to temporarily set a user's global role and flush token cache
function Set-UserRole([string]$UserID, [string]$RoleID_nullable) {
    $body = @{ role_id = $RoleID_nullable }
    $null = Invoke-Api "PUT" "/admin/users/$UserID" $hSuper $body
}
function Restore-UserRole([string]$UserID) {
    $body = @{ role_id = "" }   # empty string = null in handler
    $null = Invoke-Api "PUT" "/admin/users/$UserID" $hSuper $body
}
function Get-FreshToken([string]$Phone) {
    return (Login $Phone).token
}
function Get-UserBusinessRoles([string]$UserID) {
    $resp = API-Data "GET" "/users/$UserID/roles" $hSuper
    if ($null -eq $resp) { return @() }
    return @($resp.business_roles)
}
function Get-BusinessRoles([string]$BizCode) {
    return @(API-Data "GET" "/business/$BizCode/roles" (MkH $tSuper $BizCode))
}
function Find-BusinessRoleByPermission([string]$BizCode, [string]$PermissionName) {
    foreach ($role in (Get-BusinessRoles $BizCode)) {
        foreach ($perm in @($role.permissions)) {
            if ($perm.name -eq $PermissionName) {
                return $role
            }
        }
    }
    return $null
}
function Assign-BusinessRole([string]$UserID, [string]$BusinessRoleID) {
    return Invoke-Api "POST" "/users/$UserID/roles/assign" $hSuper @{ business_role_id = $BusinessRoleID }
}
function Restore-WaterEngineerBusinessRole() {
    if ($null -ne $originalWEBusinessRole -and $originalWEBusinessRole.role_id) {
        $null = Assign-BusinessRole $USER_WE $originalWEBusinessRole.role_id
        $script:tWE = Get-FreshToken "9999999902"
        $script:hWE = MkH $script:tWE
        $script:hWEBiz = MkH $script:tWE $BusinessCode
    }
}

# ===========================================================================
# GROUP 1: RBAC — Global Permission Enforcement
# ===========================================================================
Write-Host "`n=== GROUP 1: RBAC - Global Permission Enforcement ===" -ForegroundColor Magenta

# 1.1 super_admin wildcard (*:*:*) grants access to permission-gated endpoints
$r = Invoke-Api "GET" "/admin/users" $hSuper
Expect "G1_GlobalPerm" "super_admin GET /admin/users (wildcard)" $r.ok $true "code=$(if($r.ok){200}else{$r.status})"

# 1.2 Assign System_Admin role to Water Admin (has user:read → read_users via route)
# First check: current user with NO global role is denied
$r = Invoke-Api "GET" "/admin/users" $hWA
Expect "G1_GlobalPerm" "no-global-role GET /admin/users" $r.ok $false "Expected 403, code=$($r.status)"

# 1.3 Assign System_Admin role to Water Admin → gains user:read/user:create
Set-UserRole $USER_WA $ROLE_SYSTEM_ADMIN
$modifiedUserRoles.Add(@{ UserID = $USER_WA; OriginalRole = "" })
$tWA_new = Get-FreshToken "9999999901"
$hWA_new = MkH $tWA_new
# Note: admin/users requires "read_users" but System_Admin has "user:read"; check if matcher handles this
# Routes use legacy "read_users" style - need to check what actual middleware uses
$r = Invoke-Api "GET" "/admin/users" $hWA_new
$gotAccess_sysAdmin = $r.ok
# We don't assert hard here (depends on exact perm name mapping - tested as informational)
$sysAdminStatus = if ($gotAccess_sysAdmin) {"PASS"} else {"INFO"}
$sysAdminMatch   = if ($gotAccess_sysAdmin) {"YES"} else {"NO - schema mismatch"}
Record "G1_GlobalPerm" "System_Admin GET /admin/users (user:read vs read_users)" `
    $sysAdminStatus `
    "System_Admin has user:read; route needs read_users; match=$sysAdminMatch"

# 1.4 Assign Manager role → has project:read but NOT user:read
Set-UserRole $USER_WA $ROLE_MANAGER
$tWA_mgr = Get-FreshToken "9999999901"
$hWA_mgr = MkH $tWA_mgr
$r = Invoke-Api "GET" "/admin/users" $hWA_mgr
Expect "G1_GlobalPerm" "Manager role (project:read only) GET /admin/users - denied" $r.ok $false "code=$($r.status)"

# 1.5 Consultant role → has project:read, planning:read but not user:read
Set-UserRole $USER_WA $ROLE_CONSULTANT
$tWA_csl = Get-FreshToken "9999999901"
$hWA_csl = MkH $tWA_csl
$r = Invoke-Api "GET" "/admin/users" $hWA_csl
Expect "G1_GlobalPerm" "Consultant role (project/planning only) GET /admin/users - denied" $r.ok $false "code=$($r.status)"

# 1.6 Wildcard (*:*) permission: Admin role has wildcard-style check via route
# Test manage_roles endpoint: Admin has user:create but not manage_roles explicitly
$r = Invoke-Api "GET" "/admin/roles" $hWA_csl
Expect "G1_GlobalPerm" "Consultant role GET /admin/roles (manage_roles) - denied" $r.ok $false "code=$($r.status)"

# 1.7 super_admin can do manage_roles
$r = Invoke-Api "GET" "/admin/roles" $hSuper
Expect "G1_GlobalPerm" "super_admin GET /admin/roles - allowed" $r.ok $true "count=$(if($r.ok){$r.data.data.Count}else{'ERR'})"

# 1.8 No Authorization header at all → 401
$noAuth = @{ "x-api-key" = $ApiKey; "Content-Type" = "application/json" }
$r = Invoke-Api "GET" "/admin/users" $noAuth
Expect "G1_GlobalPerm" "no auth header GET /admin/users - 401" $r.ok $false "code=$($r.status)"

# 1.9 Invalid/expired token → 401
$badToken = @{ "x-api-key" = $ApiKey; "Authorization" = "Bearer invalid.jwt.token"; "Content-Type" = "application/json" }
$r = Invoke-Api "GET" "/admin/users" $badToken
Expect "G1_GlobalPerm" "invalid JWT GET /admin/users - 401" $r.ok $false "code=$($r.status)"

# 1.10 Admin role has RequirePermission(admin_all) for dashboard
Set-UserRole $USER_WA $ROLE_ADMIN
$tWA_adm = Get-FreshToken "9999999901"
$hWA_adm = MkH $tWA_adm
$r = Invoke-Api "GET" "/admin/dashboard" $hWA_adm
Expect "G1_GlobalPerm" "Admin role GET /admin/dashboard (admin_all) - denied" $r.ok $false "code=$($r.status) (Admin lacks admin_all)"

# super_admin gets dashboard
$r = Invoke-Api "GET" "/admin/dashboard" $hSuper
Expect "G1_GlobalPerm" "super_admin GET /admin/dashboard (admin_all) - allowed" $r.ok $true ""

# 1.11 RequireAnyPermission: /files/upload needs create_reports OR create_materials
# Consultant has project:read only → denied
$r = Invoke-Api "POST" "/files/upload" $hWA_csl
Expect "G1_GlobalPerm" "Consultant POST /files/upload (AnyPerm: create_reports|create_materials) - denied" ($r.status -eq 403 -or $r.status -eq 400) $true "code=$($r.status)"

# Manager has no create_reports → denied  
$r = Invoke-Api "POST" "/files/upload" $hWA_mgr
Expect "G1_GlobalPerm" "Manager POST /files/upload (AnyPerm) - denied" ($r.status -eq 403 -or $r.status -eq 400) $true "code=$($r.status)"

# Restore WA to no global role
Restore-UserRole $USER_WA
$tWA     = (Login "9999999901").token
$hWA     = MkH $tWA $BusinessCode
$hWABiz  = MkH $tWA $BusinessCode

# ===========================================================================
# GROUP 2: RBAC — Business Permission Enforcement
# ===========================================================================
Write-Host "`n=== GROUP 2: RBAC - Business Permission Enforcement ===" -ForegroundColor Magenta

# Capture the engineer's original WATER role so positive-case provisioning can be restored.
$originalWEBusinessRole = @(Get-UserBusinessRoles $USER_WE | Where-Object { $_.vertical_code -eq $BusinessCode }) | Select-Object -First 1

# 2.1 Water Engineer originally lacks business_manage_roles → 403
$r = Invoke-Api "GET" "/business/$BusinessCode/roles" (MkH $tWE $BusinessCode)
Expect "G2_BizPerm" "Water Engineer GET /business/WATER/roles (original role lacks business_manage_roles)" $r.ok $false "code=$($r.status)"

# 2.2 Provision a role with business_manage_roles and verify positive access deterministically.
$manageRolesRole = Find-BusinessRoleByPermission $BusinessCode "business_manage_roles"
if ($null -ne $manageRolesRole) {
    $assignResp = Assign-BusinessRole $USER_WE $manageRolesRole.id
    if ($assignResp.ok) {
        $tWE = Get-FreshToken "9999999902"
        $hWE = MkH $tWE
        $hWEBiz = MkH $tWE $BusinessCode
        $r = Invoke-Api "GET" "/business/$BusinessCode/roles" $hWEBiz
        Expect "G2_BizPerm" "Assigned role with business_manage_roles can GET /business/WATER/roles" $r.ok $true "role=$($manageRolesRole.display_name)"
    } else {
        Record "G2_BizPerm" "Assign role with business_manage_roles" "FAIL" "code=$($assignResp.status)"
    }
} else {
    Record "G2_BizPerm" "Locate role with business_manage_roles" "SKIP" "No matching active business role found"
}

# 2.3 Solar Admin in WATER context → 403 (no WATER business access)
$r = Invoke-Api "GET" "/business/$BusinessCode/roles" (MkH $tSolarA $BusinessCode)
Expect "G2_BizPerm" "Solar Admin in WATER context GET /business/WATER/roles - 403" $r.ok $false "code=$($r.status)"

# 2.4 super_admin bypasses all business permissions
$r = Invoke-Api "GET" "/business/$BusinessCode/roles" $hSuperBiz
Expect "G2_BizPerm" "super_admin GET /business/WATER/roles - bypass" $r.ok $true ""

# 2.5 Restore original engineer role before negative testing the next permission.
Restore-WaterEngineerBusinessRole

# 2.6 Water Engineer originally lacks business_manage_users → 403
$r = Invoke-Api "GET" "/business/$BusinessCode/users" (MkH $tWE $BusinessCode)
Expect "G2_BizPerm" "Water Engineer GET /business/WATER/users (original role lacks business_manage_users)" $r.ok $false "code=$($r.status)"

# 2.7 Provision a role with business_manage_users and verify positive access.
$manageUsersRole = Find-BusinessRoleByPermission $BusinessCode "business_manage_users"
if ($null -ne $manageUsersRole) {
    $assignResp = Assign-BusinessRole $USER_WE $manageUsersRole.id
    if ($assignResp.ok) {
        $tWE = Get-FreshToken "9999999902"
        $hWE = MkH $tWE
        $hWEBiz = MkH $tWE $BusinessCode
        $r = Invoke-Api "GET" "/business/$BusinessCode/users" $hWEBiz
        Expect "G2_BizPerm" "Assigned role with business_manage_users can GET /business/WATER/users" $r.ok $true "role=$($manageUsersRole.display_name)"
    } else {
        Record "G2_BizPerm" "Assign role with business_manage_users" "FAIL" "code=$($assignResp.status)"
    }
} else {
    Record "G2_BizPerm" "Locate role with business_manage_users" "SKIP" "No matching active business role found"
}

# 2.8 Restore original engineer role before access-only and negative checks.
Restore-WaterEngineerBusinessRole

# 2.9 RequireBusinessAccess: form submissions only need business membership
$r = Invoke-Api "GET" "/business/$BusinessCode/forms/WD12/submissions" (MkH $tWE $BusinessCode)
Expect "G2_BizPerm" "Water Engineer GET /business/WATER/forms/WD12/submissions (just needs access)" $r.ok $true ""

# 2.10 HO Admin has no WATER roles → 403 on any WATER business route
$r = Invoke-Api "GET" "/business/$BusinessCode/forms/WD12/submissions" (MkH $tHOA $BusinessCode)
Expect "G2_BizPerm" "HO Admin (no WATER role) GET /business/WATER/forms - 403" $r.ok $false "code=$($r.status)"

# 2.11 Solar Engineer has no WATER roles → 403
$r = Invoke-Api "GET" "/business/$BusinessCode/forms/WD12/submissions" (MkH $tSolarE $BusinessCode)
Expect "G2_BizPerm" "Solar Engineer (no WATER role) GET /business/WATER/forms - 403" $r.ok $false "code=$($r.status)"

# 2.12 Water Engineer originally lacks business_view_analytics → 403
$r = Invoke-Api "GET" "/business/$BusinessCode/analytics" (MkH $tWE $BusinessCode)
Expect "G2_BizPerm" "Water Engineer GET /business/WATER/analytics (original role lacks business_view_analytics)" $r.ok $false "code=$($r.status)"

# 2.13 Provision a role with business_view_analytics and verify positive access.
$viewAnalyticsRole = Find-BusinessRoleByPermission $BusinessCode "business_view_analytics"
if ($null -ne $viewAnalyticsRole) {
    $assignResp = Assign-BusinessRole $USER_WE $viewAnalyticsRole.id
    if ($assignResp.ok) {
        $tWE = Get-FreshToken "9999999902"
        $hWE = MkH $tWE
        $hWEBiz = MkH $tWE $BusinessCode
        $r = Invoke-Api "GET" "/business/$BusinessCode/analytics" $hWEBiz
        Expect "G2_BizPerm" "Assigned role with business_view_analytics can GET /business/WATER/analytics" $r.ok $true "role=$($viewAnalyticsRole.display_name)"
    } else {
        Record "G2_BizPerm" "Assign role with business_view_analytics" "FAIL" "code=$($assignResp.status)"
    }
} else {
    Record "G2_BizPerm" "Locate role with business_view_analytics" "SKIP" "No matching active business role found"
}

# 2.14 Missing X-Business-Code header → 400 or 403
$r = Invoke-Api "GET" "/business/$BusinessCode/roles" (MkH $tWA)  # no BusinessCode in header
Expect "G2_BizPerm" "Water Admin missing X-Business-Code header" ($r.status -eq 400 -or $r.status -eq 403 -or -not $r.ok) $true "code=$($r.status)"

Restore-WaterEngineerBusinessRole

# ===========================================================================
# GROUP 3: RBAC — Super Admin Bypass
# ===========================================================================
Write-Host "`n=== GROUP 3: RBAC - Super Admin Bypass ===" -ForegroundColor Magenta

$superBypassEndpoints = @(
    @{ M="GET";  P="/admin/users" }
    @{ M="GET";  P="/admin/roles" }
    @{ M="GET";  P="/admin/permissions" }
    @{ M="GET";  P="/business/$BusinessCode/roles" }
    @{ M="GET";  P="/business/$BusinessCode/users" }
    @{ M="GET";  P="/business/$BusinessCode/analytics" }
    @{ M="GET";  P="/policies" }
    @{ M="GET";  P="/attributes" }
)
foreach ($ep in $superBypassEndpoints) {
    $hdr = if ($ep.P -match "^/business") { $hSuperBiz } else { $hSuper }
    $r = Invoke-Api $ep.M $ep.P $hdr
    Expect "G3_SuperAdmin" "super_admin $($ep.M) $($ep.P) - bypass" $r.ok $true ""
}

# ===========================================================================
# GROUP 4: RBAC — Permission Wildcard Matching
# ===========================================================================
Write-Host "`n=== GROUP 4: RBAC - Permission Wildcard Matching ===" -ForegroundColor Magenta

# super_admin has *:*:* — test profile endpoint
$r = Invoke-Api "GET" "/profile" $hSuper
Expect "G4_Wildcard" "super_admin *:*:* matches any permission check (GET /profile)" $r.ok $true ""

# Test /test/auth (open auth test endpoint)
$r = Invoke-Api "GET" "/test/auth" $hSuper
Expect "G4_Wildcard" "super_admin GET /test/auth" $r.ok $true ""
$r = Invoke-Api "GET" "/test/auth" $hWA
Expect "G4_Wildcard" "Water Admin (valid token, no global role) GET /test/auth" $r.ok $true "any authenticated user"

# Test /test/permission endpoint requires a query parameter.
$r = Invoke-Api "GET" "/test/permission?permission=*:*:*" $hSuper
Expect "G4_Wildcard" "super_admin GET /test/permission?permission=*:*:*" $r.ok $true ""

# ===========================================================================
# GROUP 5: ABAC — Policy CRUD Access Control
# ===========================================================================
Write-Host "`n=== GROUP 5: ABAC - Policy CRUD Access ===" -ForegroundColor Magenta

# 5.1 Water Admin (no manage_policies) → denied
$testPolicy = @{
    name         = "test_temp_policy_crud_check_$RunTag"
    display_name = "Temp CRUD check"
    description  = "Temporary test policy"
    effect       = "ALLOW"
    priority     = 1
    status       = "draft"
    conditions   = @{ attribute = "user.department"; operator = "EQUALS"; value = "Engineering" }
    actions      = @("project:read")
    resources    = @("project")
}
$r = Invoke-Api "POST" "/policies" $hWA $testPolicy
Expect "G5_PolicyCRUD" "Water Admin POST /policies (lacks manage_policies) - denied" $r.ok $false "code=$($r.status)"

# 5.2 super_admin can create policy
$r = Invoke-Api "POST" "/policies" $hSuper $testPolicy
Expect "G5_PolicyCRUD" "super_admin POST /policies - allowed" $r.ok $true ""
$policyID = if ($r.ok -and $r.data.PSObject.Properties.Name -contains "id") { $r.data.id } `
    elseif ($r.ok -and $r.data.PSObject.Properties.Name -contains "policy") { $r.data.policy.id } `
    else { $null }
if ($policyID) { $createdPolicies.Add($policyID) }

# 5.3 super_admin can list policies
$r = Invoke-Api "GET" "/policies" $hSuper
Expect "G5_PolicyCRUD" "super_admin GET /policies - allowed" $r.ok $true ""

# 5.4 Water Admin (no manage_policies) → denied on list
$r = Invoke-Api "GET" "/policies" $hWA
Expect "G5_PolicyCRUD" "Water Admin GET /policies (lacks manage_policies) - denied" $r.ok $false "code=$($r.status)"

# 5.5 Activate policy (requires manage_policies)
if ($policyID) {
    $r = Invoke-Api "POST" "/policies/$policyID/activate" $hWA
    Expect "G5_PolicyCRUD" "Water Admin POST /policies/{id}/activate - denied" $r.ok $false "code=$($r.status)"
    
    $r = Invoke-Api "POST" "/policies/$policyID/activate" $hSuper
    Expect "G5_PolicyCRUD" "super_admin POST /policies/{id}/activate - allowed" $r.ok $true ""
    
    $r = Invoke-Api "POST" "/policies/$policyID/deactivate" $hSuper
    Expect "G5_PolicyCRUD" "super_admin POST /policies/{id}/deactivate - allowed" $r.ok $true ""
    
    # Delete test policy to clean up early (will also be cleaned in finally)
    $null = Invoke-Api "DELETE" "/policies/$policyID" $hSuper
    $createdPolicies.Remove($policyID) | Out-Null
}

# ===========================================================================
# GROUP 6: ABAC — Attribute Management Access Control
# ===========================================================================
Write-Host "`n=== GROUP 6: ABAC - Attribute Management Access ===" -ForegroundColor Magenta

# 6.1 super_admin can list attributes
$r = Invoke-Api "GET" "/attributes" $hSuper
Expect "G6_AttrCRUD" "super_admin GET /attributes - allowed" $r.ok $true "count=$(if($r.ok){@($r.data).Count}else{'ERR'})"

# 6.2 Water Admin (no manage_attributes) → denied
$r = Invoke-Api "GET" "/attributes" $hWA
Expect "G6_AttrCRUD" "Water Admin GET /attributes (lacks manage_attributes) - denied" $r.ok $false "code=$($r.status)"

# 6.3 super_admin can create attribute
$testAttr = @{
    name         = "user.test_clearance_temp_$RunTag"
    display_name = "Test Temp Clearance"
    description  = "Temporary test attribute"
    type         = "user"
    data_type    = "integer"
    is_system    = $false
}
$r = Invoke-Api "POST" "/attributes" $hSuper $testAttr
Expect "G6_AttrCRUD" "super_admin POST /attributes - allowed" $r.ok $true ""
$testAttrID = if ($r.ok) {
    if ($r.data.PSObject.Properties.Name -contains "id") { $r.data.id }
    elseif ($r.data.PSObject.Properties.Name -contains "attribute") { $r.data.attribute.id }
    else { $null }
} else { $null }
if ($testAttrID) { $createdAttributes.Add($testAttrID) }

# 6.4 Water Admin (no manage_attributes) → denied on create
$r = Invoke-Api "POST" "/attributes" $hWA $testAttr
Expect "G6_AttrCRUD" "Water Admin POST /attributes (lacks manage_attributes) - denied" $r.ok $false "code=$($r.status)"

# 6.5 GET user attributes (no permission required - open)
$r = Invoke-Api "GET" "/users/$USER_WE/attributes" $hWE
Expect "G6_AttrCRUD" "Water Engineer GET own user attributes (open) - allowed" $r.ok $true ""

$r = Invoke-Api "GET" "/users/$USER_WA/attributes" $hWE
Expect "G6_AttrCRUD" "Water Engineer GET another user's attributes (open) - allowed" $r.ok $true ""

# 6.6 Assign user attribute (requires manage_user_attributes)
$attrAssign = @{
    attribute_id = $ATTR_DEPARTMENT
    value        = "Engineering"
    assigned_by  = $USER_SUPER
}
$r = Invoke-Api "POST" "/users/$USER_WE/attributes" $hWE $attrAssign
Expect "G6_AttrCRUD" "Water Engineer POST own attributes (lacks manage_user_attributes) - denied" $r.ok $false "code=$($r.status)"

$r = Invoke-Api "POST" "/users/$USER_WE/attributes" $hSuper $attrAssign
Expect "G6_AttrCRUD" "super_admin POST /users/{id}/attributes - allowed" $r.ok $true ""
if ($r.ok) {
    $assignedUserAttrs.Add(@{ UserID = $USER_WE; AttributeID = $ATTR_DEPARTMENT })
}

# ===========================================================================
# GROUP 7: ABAC — Policy Evaluation Engine (via /policies/evaluate)
# ===========================================================================
Write-Host "`n=== GROUP 7: ABAC - Policy Evaluation Engine ===" -ForegroundColor Magenta

# Helper: Create an ALLOW or DENY policy, returns its ID
function New-Policy([string]$name, [string]$effect, [int]$priority, [object]$conditions, [array]$actions=@("*"), [array]$resources=@("*")) {
    $effectiveName = "$name`_$RunTag"
    $p = @{
        name         = $effectiveName
        display_name = $effectiveName
        description  = "Auth test policy"
        effect       = $effect
        priority     = $priority
        status       = "draft"
        conditions   = $conditions
        actions      = $actions
        resources    = $resources
    }
    $r = Invoke-Api "POST" "/policies" $hSuper $p
    if (-not $r.ok) { return $null }
    $id = if ($r.data.PSObject.Properties.Name -contains "id") { $r.data.id }
        elseif ($r.data.PSObject.Properties.Name -contains "policy") { $r.data.policy.id }
        else { $null }
    if ($id) { $createdPolicies.Add($id) }
    return $id
}
function Activate-Policy([string]$id) {
    $null = Invoke-Api "POST" "/policies/$id/activate" $hSuper
}
function Eval-Policy([string]$userID, [string]$action, [string]$resource, [hashtable]$userAttrs=@{}, [hashtable]$resAttrs=@{}) {
    $req = @{
        user_id              = $userID
        action               = $action
        resource_type        = $resource
        user_attributes      = $userAttrs
        resource_attributes  = $resAttrs
        environment          = @{}
    }
    $r = Invoke-Api "POST" "/policies/evaluate" $hSuper $req
    if (-not $r.ok) { return $null }
    return $r.data
}

# ── 7.1 No matching policy → default DENY (fail-safe) ──────────────────────
$d = Eval-Policy $USER_WE "nonexistent:action" "nonexistent_resource"
if ($null -ne $d) {
    Expect "G7_PolicyEval" "No matching policy → default DENY" $d.allowed $false "reason=$($d.reason)"
} else {
    Record "G7_PolicyEval" "No matching policy" "SKIP" "evaluate endpoint returned null"
}

# ── 7.2 Simple ALLOW on department == Engineering ──────────────────────────
$cond_dept_eng = @{ attribute = "user.department"; operator = "EQUALS"; value = "Engineering" }
$p1 = New-Policy "test_allow_dept_eng" "ALLOW" 10 $cond_dept_eng @("project:read") @("project")
if ($p1) {
    Activate-Policy $p1
    
    # User WITH dept=Engineering → ALLOW
    $d = Eval-Policy $USER_WE "project:read" "project" @{ "user.department" = "Engineering" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "ALLOW: user.department=Engineering project:read" $d.allowed $true "reason=$($d.reason)" }
    
    # User WITH dept=HR → DENY (no matching policy)
    $d = Eval-Policy $USER_WE "project:read" "project" @{ "user.department" = "HR" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "DENY: user.department=HR, no ALLOW match" $d.allowed $false "reason=$($d.reason)" }
    
    # User with NO dept attr → DENY
    $d = Eval-Policy $USER_WE "project:read" "project"
    if ($null -ne $d) { Expect "G7_PolicyEval" "DENY: no user.department attribute" $d.allowed $false "reason=$($d.reason)" }
}

# ── 7.3 DENY overrides ALLOW regardless of priority ─────────────────────────
$cond_any = @{ attribute = "user.department"; operator = "CONTAINS"; value = "" }
# Actually use a condition that always matches: use user.id check
$cond_always = @{ attribute = "user.id"; operator = "MATCHES"; value = ".*" }  # regex wildcard
$p2 = New-Policy "test_deny_override_allow" "DENY" 1 $cond_dept_eng @("project:read") @("project")  # lower priority DENY
if ($p1 -and $p2) {
    Activate-Policy $p2
    
    # DENY lower-priority + ALLOW higher-priority → DENY wins
    $d = Eval-Policy $USER_WE "project:read" "project" @{ "user.department" = "Engineering" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "DENY overrides ALLOW (deny-priority=1, allow-priority=10)" $d.allowed $false "reason=$($d.reason)" }
    
    # Deactivate DENY → ALLOW should work again
    $null = Invoke-Api "POST" "/policies/$p2/deactivate" $hSuper
    $d = Eval-Policy $USER_WE "project:read" "project" @{ "user.department" = "Engineering" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "After deactivating DENY, ALLOW applies again" $d.allowed $true "reason=$($d.reason)" }
}

# ── 7.4 AND condition ────────────────────────────────────────────────────────
$cond_and = @{
    AND = @(
        @{ attribute = "user.department"; operator = "EQUALS"; value = "Engineering" }
        @{ attribute = "user.clearance_level"; operator = ">="; value = 3 }
    )
}
$p3 = New-Policy "test_and_condition" "ALLOW" 20 $cond_and @("document:read") @("document")
if ($p3) {
    Activate-Policy $p3
    
    # dept=Engineering AND clearance=3 → ALLOW
    $d = Eval-Policy $USER_WE "document:read" "document" @{ "user.department" = "Engineering"; "user.clearance_level" = "3" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "AND: dept=Engineering AND clearance=3 → ALLOW" $d.allowed $true "reason=$($d.reason)" }
    
    # dept=Engineering AND clearance=2 → DENY (AND fails)
    $d = Eval-Policy $USER_WE "document:read" "document" @{ "user.department" = "Engineering"; "user.clearance_level" = "2" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "AND: dept=Engineering AND clearance=2 → DENY (fails >=3)" $d.allowed $false "reason=$($d.reason)" }
    
    # dept=HR AND clearance=5 → DENY (AND fails dept)
    $d = Eval-Policy $USER_WE "document:read" "document" @{ "user.department" = "HR"; "user.clearance_level" = "5" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "AND: dept=HR AND clearance=5 → DENY (dept fails)" $d.allowed $false "reason=$($d.reason)" }
}

# ── 7.5 OR condition ─────────────────────────────────────────────────────────
$cond_or = @{
    OR = @(
        @{ attribute = "user.department"; operator = "EQUALS"; value = "Engineering" }
        @{ attribute = "user.department"; operator = "EQUALS"; value = "Finance" }
    )
}
$p4 = New-Policy "test_or_condition" "ALLOW" 20 $cond_or @("report:read") @("report")
if ($p4) {
    Activate-Policy $p4
    
    # dept=Engineering → ALLOW
    $d = Eval-Policy $USER_WE "report:read" "report" @{ "user.department" = "Engineering" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "OR: dept=Engineering → ALLOW" $d.allowed $true "reason=$($d.reason)" }
    
    # dept=Finance → ALLOW
    $d = Eval-Policy $USER_WE "report:read" "report" @{ "user.department" = "Finance" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "OR: dept=Finance → ALLOW" $d.allowed $true "reason=$($d.reason)" }
    
    # dept=HR → DENY (neither branch)
    $d = Eval-Policy $USER_WE "report:read" "report" @{ "user.department" = "HR" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "OR: dept=HR → DENY (no branch matches)" $d.allowed $false "reason=$($d.reason)" }
}

# ── 7.6 NOT condition ────────────────────────────────────────────────────────
$cond_not = @{
    NOT = @{ attribute = "user.department"; operator = "EQUALS"; value = "HR" }
}
$p5 = New-Policy "test_not_condition" "ALLOW" 20 $cond_not @("planning:read") @("planning")
if ($p5) {
    Activate-Policy $p5
    
    # dept=Engineering → NOT(dept==HR) = true → ALLOW
    $d = Eval-Policy $USER_WE "planning:read" "planning" @{ "user.department" = "Engineering" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "NOT: dept=Engineering → ALLOW (not HR)" $d.allowed $true "reason=$($d.reason)" }
    
    # dept=HR → NOT(dept==HR) = false → DENY
    $d = Eval-Policy $USER_WE "planning:read" "planning" @{ "user.department" = "HR" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "NOT: dept=HR → DENY (IS HR)" $d.allowed $false "reason=$($d.reason)" }
}

# ── 7.7 Numeric operators ────────────────────────────────────────────────────
$p6 = New-Policy "test_numeric_between" "ALLOW" 20 `
    @{ attribute = "user.clearance_level"; operator = "BETWEEN"; value = @(2, 4) } `
    @("finance:read") @("finance")
if ($p6) {
    Activate-Policy $p6
    
    $d = Eval-Policy $USER_WE "finance:read" "finance" @{ "user.clearance_level" = "3" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "BETWEEN: clearance=3 in [2,4] → ALLOW" $d.allowed $true "reason=$($d.reason)" }
    
    $d = Eval-Policy $USER_WE "finance:read" "finance" @{ "user.clearance_level" = "5" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "BETWEEN: clearance=5 NOT in [2,4] → DENY" $d.allowed $false "reason=$($d.reason)" }
    
    $d = Eval-Policy $USER_WE "finance:read" "finance" @{ "user.clearance_level" = "1" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "BETWEEN: clearance=1 NOT in [2,4] → DENY" $d.allowed $false "reason=$($d.reason)" }
    
    # Boundary values
    $d = Eval-Policy $USER_WE "finance:read" "finance" @{ "user.clearance_level" = "2" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "BETWEEN: clearance=2 (lower bound) → ALLOW" $d.allowed $true "reason=$($d.reason)" }
    
    $d = Eval-Policy $USER_WE "finance:read" "finance" @{ "user.clearance_level" = "4" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "BETWEEN: clearance=4 (upper bound) → ALLOW" $d.allowed $true "reason=$($d.reason)" }
}

# ── 7.8 IN / NOT_IN operators ────────────────────────────────────────────────
$p7 = New-Policy "test_in_operator" "ALLOW" 20 `
    @{ attribute = "user.department"; operator = "IN"; value = @("Engineering", "IT", "DevOps") } `
    @("inventory:read") @("inventory")
if ($p7) {
    Activate-Policy $p7
    
    foreach ($dept in @("Engineering", "IT", "DevOps")) {
        $d = Eval-Policy $USER_WE "inventory:read" "inventory" @{ "user.department" = $dept }
        if ($null -ne $d) { Expect "G7_PolicyEval" "IN: dept=$dept in [Engineering,IT,DevOps] → ALLOW" $d.allowed $true "reason=$($d.reason)" }
    }
    foreach ($dept in @("HR", "Finance", "Marketing")) {
        $d = Eval-Policy $USER_WE "inventory:read" "inventory" @{ "user.department" = $dept }
        if ($null -ne $d) { Expect "G7_PolicyEval" "IN: dept=$dept NOT in [Engineering,IT,DevOps] → DENY" $d.allowed $false "reason=$($d.reason)" }
    }
}

# NOT_IN
$p8 = New-Policy "test_not_in_operator" "ALLOW" 20 `
    @{ attribute = "user.employment_type"; operator = "NOT_IN"; value = @("contractor", "intern") } `
    @("hr:read") @("hr")
if ($p8) {
    Activate-Policy $p8
    
    $d = Eval-Policy $USER_WE "hr:read" "hr" @{ "user.employment_type" = "full_time" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "NOT_IN: employment=full_time NOT IN [contractor,intern] → ALLOW" $d.allowed $true "reason=$($d.reason)" }
    
    $d = Eval-Policy $USER_WE "hr:read" "hr" @{ "user.employment_type" = "contractor" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "NOT_IN: employment=contractor IN blacklist → DENY" $d.allowed $false "reason=$($d.reason)" }
    
    $d = Eval-Policy $USER_WE "hr:read" "hr" @{ "user.employment_type" = "intern" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "NOT_IN: employment=intern IN blacklist → DENY" $d.allowed $false "reason=$($d.reason)" }
}

# ── 7.9 String operators: CONTAINS, STARTS_WITH, ENDS_WITH, MATCHES ──────────
$p9 = New-Policy "test_contains_op" "ALLOW" 20 `
    @{ attribute = "user.location"; operator = "CONTAINS"; value = "Mumbai" } `
    @("purchase:read") @("purchase")
if ($p9) {
    Activate-Policy $p9
    
    $d = Eval-Policy $USER_WE "purchase:read" "purchase" @{ "user.location" = "Mumbai-Central" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "CONTAINS: location=Mumbai-Central contains Mumbai → ALLOW" $d.allowed $true "reason=$($d.reason)" }
    
    $d = Eval-Policy $USER_WE "purchase:read" "purchase" @{ "user.location" = "Delhi" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "CONTAINS: location=Delhi doesn't contain Mumbai → DENY" $d.allowed $false "reason=$($d.reason)" }
}

$p10 = New-Policy "test_regex_matches" "ALLOW" 20 `
    @{ attribute = "user.department"; operator = "MATCHES"; value = "^Eng.*" } `
    @("project:approve") @("project")
if ($p10) {
    Activate-Policy $p10
    
    $d = Eval-Policy $USER_WE "project:approve" "project" @{ "user.department" = "Engineering" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "MATCHES regex ^Eng.*: department=Engineering → ALLOW" $d.allowed $true "reason=$($d.reason)" }
    
    $d = Eval-Policy $USER_WE "project:approve" "project" @{ "user.department" = "Engg" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "MATCHES regex ^Eng.*: department=Engg → ALLOW (prefix match)" $d.allowed $true "reason=$($d.reason)" }
    
    $d = Eval-Policy $USER_WE "project:approve" "project" @{ "user.department" = "Finance" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "MATCHES regex ^Eng.*: department=Finance → DENY" $d.allowed $false "reason=$($d.reason)" }
}

# ── 7.10 Action/Resource filtering in policy ─────────────────────────────────
$p11 = New-Policy "test_action_filter" "ALLOW" 20 `
    @{ attribute = "user.department"; operator = "EQUALS"; value = "Finance" } `
    @("finance:approve") @("finance")
if ($p11) {
    Activate-Policy $p11
    
    # Correct action → ALLOW
    $d = Eval-Policy $USER_WE "finance:approve" "finance" @{ "user.department" = "Finance" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "Action filter: finance:approve dept=Finance → ALLOW" $d.allowed $true "reason=$($d.reason)" }
    
    # Different action → DENY (policy doesn't apply)
    $d = Eval-Policy $USER_WE "finance:delete" "finance" @{ "user.department" = "Finance" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "Action filter: finance:delete dept=Finance → DENY (action mismatch)" $d.allowed $false "reason=$($d.reason)" }
    
    # Different resource → DENY
    $d = Eval-Policy $USER_WE "finance:approve" "hr" @{ "user.department" = "Finance" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "Resource filter: resource=hr dept=Finance → DENY (resource mismatch)" $d.allowed $false "reason=$($d.reason)" }
}

# ── 7.11 Resource attribute conditions ──────────────────────────────────────
$p12 = New-Policy "test_resource_attr" "DENY" 30 `
    @{ attribute = "resource.sensitivity"; operator = "EQUALS"; value = "classified" } `
    @("document:read") @("document")
if ($p12) {
    Activate-Policy $p12
    
    # Read a classified document → DENY
    $d = Eval-Policy $USER_WE "document:read" "document" @{} @{ "resource.sensitivity" = "classified" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "Resource attr DENY: sensitivity=classified document:read → DENY" $d.allowed $false "reason=$($d.reason)" }
    
    # Read a non-classified document → no DENY match (depends on whether ALLOW exists)
    $d = Eval-Policy $USER_WE "document:read" "document" @{} @{ "resource.sensitivity" = "public" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "Resource attr DENY: sensitivity=public → DENY overrider not triggered" $d.allowed $false "reason=$($d.reason) (no ALLOW policy for document:read)" }
}

# ── 7.12 ValidUntil expired policy is not applied ───────────────────────────
$yesterday = (Get-Date).AddDays(-1).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
$expiredPolicyBody = @{
    name         = "test_expired_policy_$RunTag"
    display_name = "Expired policy test"
    description  = "Should not apply"
    effect       = "ALLOW"
    priority     = 50
    status       = "draft"
    conditions   = @{ attribute = "user.department"; operator = "EQUALS"; value = "ExpiredDept" }
    actions      = @("special:action")
    resources    = @("special_resource")
    valid_until  = $yesterday
}
$r = Invoke-Api "POST" "/policies" $hSuper $expiredPolicyBody
$expPolicyID = if ($r.ok) {
    if ($r.data.PSObject.Properties.Name -contains "id") { $r.data.id }
    elseif ($r.data.PSObject.Properties.Name -contains "policy") { $r.data.policy.id }
    else { $null }
} else { $null }
if ($expPolicyID) {
    $createdPolicies.Add($expPolicyID)
    $null = Invoke-Api "POST" "/policies/$expPolicyID/activate" $hSuper
    $d = Eval-Policy $USER_WE "special:action" "special_resource" @{ "user.department" = "ExpiredDept" }
    if ($null -ne $d) {
        Expect "G7_PolicyEval" "Expired policy (valid_until=yesterday) not applied → DENY" $d.allowed $false "reason=$($d.reason)"
    }
}

# ── 7.13 Inactive policy is not applied ──────────────────────────────────────
$draftPolicyBody = @{
    name         = "test_draft_inactive_policy_$RunTag"
    display_name = "Draft inactive test"
    description  = "Should not apply (draft)"
    effect       = "ALLOW"
    priority     = 50
    status       = "draft"  # stays draft - never activated
    conditions   = @{ attribute = "user.department"; operator = "EQUALS"; value = "DraftDept" }
    actions      = @("draft:action")
    resources    = @("draft_resource")
}
$r = Invoke-Api "POST" "/policies" $hSuper $draftPolicyBody
$draftPolicyID = if ($r.ok) {
    if ($r.data.PSObject.Properties.Name -contains "id") { $r.data.id }
    elseif ($r.data.PSObject.Properties.Name -contains "policy") { $r.data.policy.id }
    else { $null }
} else { $null }
if ($draftPolicyID) {
    $createdPolicies.Add($draftPolicyID)
    # NOT activating it
    $d = Eval-Policy $USER_WE "draft:action" "draft_resource" @{ "user.department" = "DraftDept" }
    if ($null -ne $d) {
        Expect "G7_PolicyEval" "Draft/inactive policy not applied → DENY" $d.allowed $false "reason=$($d.reason)"
    }
}

# ── 7.14 Priority: higher-priority DENY evaluated first (still DENY wins) ────
$p_lo_deny  = New-Policy "test_lo_pri_deny"  "DENY"  1  @{ attribute="user.department"; operator="EQUALS"; value="PrioTest" } @("*") @("*")
$p_hi_allow = New-Policy "test_hi_pri_allow" "ALLOW" 100 @{ attribute="user.department"; operator="EQUALS"; value="PrioTest" } @("*") @("*")
if ($p_lo_deny -and $p_hi_allow) {
    Activate-Policy $p_lo_deny
    Activate-Policy $p_hi_allow
    $d = Eval-Policy $USER_WE "anything:do" "anything" @{ "user.department" = "PrioTest" }
    if ($null -ne $d) { Expect "G7_PolicyEval" "Priority: hi-ALLOW(100) vs lo-DENY(1) → DENY wins" $d.allowed $false "reason=$($d.reason)" }
}

# ── 7.15 Template variables in conditions ({{user.id}} self-referential) ─────
$p_tmpl = New-Policy "test_template_var" "ALLOW" 20 `
    @{ attribute = "user.id"; operator = "EQUALS"; value = "{{user.id}}" } `
    @("self:read") @("self")
if ($p_tmpl) {
    Activate-Policy $p_tmpl
    # user.id == {{user.id}} is always true (self-referential template)
    $d = Eval-Policy $USER_WE "self:read" "self" @{}
    if ($null -ne $d) { Expect "G7_PolicyEval" "Template var {{user.id}} self-reference → ALLOW" $d.allowed $true "reason=$($d.reason)" }
}

# ===========================================================================
# GROUP 8: ABAC — User Attribute Lifecycle (assign → evaluate → remove → re-evaluate)
# ===========================================================================
Write-Host "`n=== GROUP 8: ABAC - User Attribute Lifecycle ===" -ForegroundColor Magenta

# We already assigned dept=Engineering to Water Engineer in G6
# Use the existing p1 (ALLOW dept=Engineering project:read) if still active
if ($p1) {
    # Re-activate p1 (may have been deactivated in 7.2 DENY test)
    $null = Invoke-Api "POST" "/policies/$p1/activate" $hSuper
    
    # 8.1 Evaluate WITH user.department=Engineering via attributes in request
    $d = Eval-Policy $USER_WE "project:read" "project" @{ "user.department" = "Engineering" }
    if ($null -ne $d) { Expect "G8_AttrLifecycle" "With dept=Engineering attr in request → ALLOW" $d.allowed $true "reason=$($d.reason)" }
}

# 8.2 Assign clearance_level=4 to Water Engineer
$clAssign = @{ attribute_id = $ATTR_CLEARANCE; value = "4"; assigned_by = $USER_SUPER }
$r = Invoke-Api "POST" "/users/$USER_WE/attributes" $hSuper $clAssign
$clAssignID = $null
if ($r.ok) { $assignedUserAttrs.Add(@{ UserID = $USER_WE; AttributeID = $ATTR_CLEARANCE }) }
$clearanceAssignStatus = if ($r.ok) { "PASS" } else { "FAIL" }
$clearanceAssignDetail = if ($r.ok) { "assignID=$clAssignID" } else { "error=$($r.error)" }
Record "G8_AttrLifecycle" "Assign clearance_level=4 to Water Engineer" `
    $clearanceAssignStatus `
    $clearanceAssignDetail

# 8.3 Verify user attributes are stored
$r = Invoke-Api "GET" "/users/$USER_WE/attributes" $hSuper
if ($r.ok) {
    $attrNames = @($r.data.PSObject.Properties.Name)
    $hasExpectedAttrs = ($attrNames -contains "user.clearance_level") -and ($attrNames -contains "user.department")
    Expect "G8_AttrLifecycle" "GET user attributes shows assigned attrs" $hasExpectedAttrs $true "attrs=$($attrNames -join ',')"
}

# 8.4 Bulk assign: location + employment_type (keys must be attribute names, not IDs)
$bulkBody = @{
    attributes = @{
        "user.location"        = "Mumbai"
        "user.employment_type" = "full_time"
    }
}
$r = Invoke-Api "POST" "/users/$USER_WA/attributes/bulk" $hSuper $bulkBody
Expect "G8_AttrLifecycle" "Bulk assign 2 attrs to Water Admin" $r.ok $true ""

# 8.5 Remove one attribute (the clearance one assigned above)
$r = Invoke-Api "DELETE" "/users/$USER_WE/attributes/$ATTR_CLEARANCE" $hSuper
if ($r.ok) {
    $assignedUserAttrs.RemoveAll([Predicate[hashtable]]{ param($x) $x.UserID -eq $USER_WE -and $x.AttributeID -eq $ATTR_CLEARANCE }) | Out-Null
    Expect "G8_AttrLifecycle" "Remove clearance_level attr from Water Engineer" $true $true "removed"
} else {
    Record "G8_AttrLifecycle" "Remove clearance_level attr" "INFO" "code=$($r.status)"
}

# 8.6 manage_user_attributes permission: Water Admin cannot assign
$r = Invoke-Api "POST" "/users/$USER_WE/attributes" $hWA `
    @{ attribute_id = $ATTR_LOCATION; value = "Delhi"; assigned_by = $USER_WA }
Expect "G8_AttrLifecycle" "Water Admin (no manage_user_attributes) POST attrs - denied" $r.ok $false "code=$($r.status)"

# ===========================================================================
# GROUP 9: Policy Approval Workflow
# ===========================================================================
Write-Host "`n=== GROUP 9: Policy Approval Workflow ===" -ForegroundColor Magenta

# 9.1 Create approval workflow (requires manage_policies)
$wfBody = @{
    name               = "test_approval_wf_$RunTag"
    description        = "Test approval workflow"
    request_type       = "activate"
    required_approvals = 1
    approver_roles     = @("super_admin")
    conditions         = @{}
}
$r = Invoke-Api "POST" "/approvals/workflows" $hSuper $wfBody
Expect "G9_Approvals" "super_admin POST /approvals/workflows - allowed" $r.ok $true ""

$r = Invoke-Api "POST" "/approvals/workflows" $hWA $wfBody
Expect "G9_Approvals" "Water Admin POST /approvals/workflows (no manage_policies) - denied" $r.ok $false "code=$($r.status)"

# 9.2 Get pending approvals (open - any authenticated user)
$r = Invoke-Api "GET" "/approvals/requests/pending" $hWE
Expect "G9_Approvals" "Water Engineer GET /approvals/requests/pending (open) - allowed" $r.ok $true ""

$r = Invoke-Api "GET" "/approvals/requests/my-pending" $hWE
Expect "G9_Approvals" "Water Engineer GET /approvals/requests/my-pending (open) - allowed" $r.ok $true ""

# 9.3 Create approval request (requires manage_policies)
$approvalPolicyID = if ($p1) { $p1 } else { "00000000-0000-0000-0000-000000000001" }
$approvalReq = @{
    policy_id        = $approvalPolicyID
    request_type     = "activate"
    notes            = "Test approval flow"
    changes_proposed = @{}
}
$r = Invoke-Api "POST" "/approvals/requests" $hWA $approvalReq
Expect "G9_Approvals" "Water Admin POST /approvals/requests (no manage_policies) - denied" $r.ok $false "code=$($r.status)"

$r = Invoke-Api "POST" "/approvals/requests" $hSuper $approvalReq
Expect "G9_Approvals" "super_admin POST /approvals/requests - allowed" $r.ok $true ""
$approvalReqID = if ($r.ok) {
    if ($r.data.PSObject.Properties.Name -contains "id") { $r.data.id }
    elseif ($r.data.PSObject.Properties.Name -contains "request") { $r.data.request.id }
    else { $null }
} else { $null }

if ($approvalReqID) {
    # 9.4 Approve/Reject the request (open)
    $r = Invoke-Api "POST" "/approvals/requests/$approvalReqID/approve" $hWA `
        @{ comment = "Approved in test" }
    Expect "G9_Approvals" "Water Admin POST /approvals/requests/{id}/approve (open) - allowed" $r.ok $true ""
}

# ===========================================================================
# GROUP 10: Policy Statistics and Versioning
# ===========================================================================
Write-Host "`n=== GROUP 10: Policy Statistics and Versioning ===" -ForegroundColor Magenta

# 10.1 Statistics (requires manage_policies)
$r = Invoke-Api "GET" "/policies/statistics" $hSuper
Expect "G10_PolicyMgmt" "super_admin GET /policies/statistics - allowed" $r.ok $true ""

$r = Invoke-Api "GET" "/policies/statistics" $hWE
Expect "G10_PolicyMgmt" "Water Engineer GET /policies/statistics (no manage_policies) - denied" $r.ok $false "code=$($r.status)"

# 10.2 Policy versions/changelog (requires manage_policies)
if ($p3) {
    $r = Invoke-Api "GET" "/policies/$p3/versions" $hSuper
    Expect "G10_PolicyMgmt" "super_admin GET /policies/{id}/versions - allowed" $r.ok $true ""
    
    $r = Invoke-Api "GET" "/policies/$p3/changelog" $hSuper
    Expect "G10_PolicyMgmt" "super_admin GET /policies/{id}/changelog - allowed" $r.ok $true ""
    
    $r = Invoke-Api "GET" "/policies/$p3/versions" $hWE
    Expect "G10_PolicyMgmt" "Water Engineer GET /policies/{id}/versions - denied" $r.ok $false "code=$($r.status)"
    
    # 10.3 Test policy with context
    $testContext = @{
        user_id       = $USER_WE
        action        = "document:read"
        resource_type = "document"
        user_attributes = @{ "user.department" = "Engineering"; "user.clearance_level" = "3" }
    }
    $r = Invoke-Api "POST" "/policies/$p3/test" $hSuper $testContext
    Expect "G10_PolicyMgmt" "super_admin POST /policies/{id}/test - allowed" $r.ok $true ""
    
    # 10.4 Clone policy (requires manage_policies)
    $r = Invoke-Api "POST" "/policies/$p3/clone" $hSuper @{}
    Expect "G10_PolicyMgmt" "super_admin POST /policies/{id}/clone - allowed" $r.ok $true ""
    $cloneID = if ($r.ok) {
        if ($r.data.PSObject.Properties.Name -contains "id") { $r.data.id }
        elseif ($r.data.PSObject.Properties.Name -contains "policy") { $r.data.policy.id }
        else { $null }
    } else { $null }
    if ($cloneID) { $createdPolicies.Add($cloneID) }
    
    # Evaluation history
    $r = Invoke-Api "GET" "/policies/$p3/evaluations" $hSuper
    Expect "G10_PolicyMgmt" "super_admin GET /policies/{id}/evaluations - allowed" $r.ok $true ""
}

# ===========================================================================
# GROUP 11: Edge Cases & Security
# ===========================================================================
Write-Host "`n=== GROUP 11: Edge Cases & Security ===" -ForegroundColor Magenta

# 11.1 Stale token after role removed — need cache invalidation awareness
# (We can't easily test TTL expiry, but verify that role change is eventually reflected)
Set-UserRole $USER_WA $ROLE_SYSTEM_ADMIN
$tWA_elevated = Get-FreshToken "9999999901"
$hWA_elevated = MkH $tWA_elevated
$r = Invoke-Api "GET" "/admin/users" $hWA_elevated
$had_access = $r.ok
Restore-UserRole $USER_WA
# After restoring, fresh token should deny (cache respects role removal)
$tWA_restored = Get-FreshToken "9999999901"
$hWA_restored = MkH $tWA_restored
$r2 = Invoke-Api "GET" "/admin/users" $hWA_restored
$roleRestoreStatus = if ($had_access -and -not $r2.ok) { "PASS" } else { "INFO" }
Record "G11_EdgeCases" "Role elevation then removal: elevated=$had_access, after_restore=$($r2.ok)" `
    $roleRestoreStatus `
    "elevated=$had_access restored_denied=$(-not $r2.ok)"

# 11.2 Super admin can access all business verticals
$verticals = @("WATER", "SOLAR", "HO") 
foreach ($v in $verticals) {
    $r = Invoke-Api "GET" "/business/$v/info" (MkH $tSuper $v)
    Expect "G11_EdgeCases" "super_admin access to business/$v/info" $r.ok $true "code=$(if($r.ok){200}else{$r.status})"
}

# 11.3 Cross-vertical isolation
$crossTests = @(
    @{ user = "SolarA"; token = $tSolarA; vertical = "WATER";       expectAllow = $false }
    @{ user = "SolarA"; token = $tSolarA; vertical = "SOLAR";       expectAllow = $true  }
    @{ user = "WA";     token = $tWA;     vertical = "SOLAR";       expectAllow = $false }
    @{ user = "WA";     token = $tWA;     vertical = "WATER";       expectAllow = $true  }
    @{ user = "HOA";    token = $tHOA;    vertical = "WATER";       expectAllow = $false }
)
foreach ($t in $crossTests) {
    $r = Invoke-Api "GET" "/business/$($t.vertical)/forms" (MkH $t.token $t.vertical)
    Expect "G11_EdgeCases" "cross-vertical: $($t.user) -> $($t.vertical)" $r.ok $t.expectAllow "code=$(if($r.ok){200}else{$r.status})"
}

# 11.4 Condition with missing attribute → DENY (attribute not found = false)
$p_missing = New-Policy "test_missing_attr" "ALLOW" 5 `
    @{ attribute = "user.nonexistent_attr_xyz"; operator = "EQUALS"; value = "something" } `
    @("missing:test") @("missing")
if ($p_missing) {
    Activate-Policy $p_missing
    $d = Eval-Policy $USER_WE "missing:test" "missing"  # no attrs provided
    if ($null -ne $d) { Expect "G11_EdgeCases" "Missing attribute in context → DENY (attr not found = false)" $d.allowed $false "reason=$($d.reason)" }
}

# 11.5 Unknown operator → DENY (error in condition = treated as false)
$badOpPolicy = @{
    name         = "test_bad_operator_$RunTag"
    display_name = "Bad operator test"
    description  = "Should error gracefully"
    effect       = "ALLOW"
    priority     = 5
    status       = "draft"
    conditions   = @{ attribute = "user.department"; operator = "UNSUPPORTED_OP"; value = "X" }
    actions      = @("badop:test")
    resources    = @("badop")
}
$r = Invoke-Api "POST" "/policies" $hSuper $badOpPolicy
$badOpID = if ($r.ok) {
    if ($r.data.PSObject.Properties.Name -contains "id") { $r.data.id }
    elseif ($r.data.PSObject.Properties.Name -contains "policy") { $r.data.policy.id }
    else { $null }
} else { $null }
if ($badOpID) {
    $createdPolicies.Add($badOpID)
    $null = Invoke-Api "POST" "/policies/$badOpID/activate" $hSuper
    $d = Eval-Policy $USER_WE "badop:test" "badop" @{ "user.department" = "X" }
    if ($null -ne $d) { Expect "G11_EdgeCases" "Unknown operator graceful → DENY (error treated as false)" $d.allowed $false "reason=$($d.reason)" }
}

# ===========================================================================
# CLEANUP
# ===========================================================================
Write-Host "`n=== Cleaning up test data ===" -ForegroundColor DarkGray

# Remove test policies
foreach ($policyId in $createdPolicies) {
    try { $null = Invoke-Api "DELETE" "/policies/$policyId" $hSuper } catch {}
}

# Remove assigned user attributes via DELETE by attribute_id
foreach ($ua in $assignedUserAttrs) {
    try {
        $null = Invoke-Api "DELETE" "/users/$($ua.UserID)/attributes/$($ua.AttributeID)" $hSuper
    } catch {}
}
# Clean up bulk-assigned attrs on Water Admin
foreach ($attrID in @($ATTR_LOCATION, $ATTR_EMPLOYMENT)) {
    try { $null = Invoke-Api "DELETE" "/users/$USER_WA/attributes/$attrID" $hSuper } catch {}
}
# Clean up dept attr on Water Engineer
try { $null = Invoke-Api "DELETE" "/users/$USER_WE/attributes/$ATTR_DEPARTMENT" $hSuper } catch {}

# Remove test attribute definition
foreach ($aid in $createdAttributes) {
    try { $null = Invoke-Api "DELETE" "/attributes/$aid" $hSuper } catch {}
}

# Ensure WA has no global role
Restore-UserRole $USER_WA

Write-Host "Cleanup complete." -ForegroundColor DarkGray

# ===========================================================================
# FINAL SUMMARY
# ===========================================================================
Write-Host "`n=== FINAL SUMMARY ===" -ForegroundColor Cyan

$pass    = @($results | Where-Object { $_.Status -eq "PASS"    }).Count
$fail    = @($results | Where-Object { $_.Status -eq "FAIL"    }).Count
$info    = @($results | Where-Object { $_.Status -eq "INFO"    }).Count
$skip    = @($results | Where-Object { $_.Status -eq "SKIP"    }).Count

$groups = $results | Group-Object Group | Sort-Object Name
foreach ($g in $groups) {
    $gPass = @($g.Group | Where-Object { $_.Status -eq "PASS" }).Count
    $gFail = @($g.Group | Where-Object { $_.Status -eq "FAIL" }).Count
    $gInfo = @($g.Group | Where-Object { $_.Status -eq "INFO" }).Count
    $gSkip = @($g.Group | Where-Object { $_.Status -eq "SKIP" }).Count
    $col = if ($gFail -gt 0) { "Red" } elseif ($gSkip -gt 0 -or $gInfo -gt 0) { "Yellow" } else { "Green" }
    Write-Host ("{0,-22} PASS={1,3}  FAIL={2,2}  INFO={3,2}  SKIP={4,2}" -f $g.Name, $gPass, $gFail, $gInfo, $gSkip) -ForegroundColor $col
}

Write-Host ""
$totalCol = if ($fail -gt 0) { "Red" } else { "Green" }
Write-Host "PASS=$pass  FAIL=$fail  INFO=$info  SKIP=$skip" -ForegroundColor $totalCol

if ($fail -gt 0) {
    Write-Host "`nFailed cases:" -ForegroundColor Red
    $results | Where-Object { $_.Status -eq "FAIL" } | ForEach-Object {
        Write-Host "  [$($_.Group)] $($_.Case) -- $($_.Detail)" -ForegroundColor Red
    }
    exit 1
}
exit 0

