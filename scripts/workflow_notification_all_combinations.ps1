param(
    [string]$ApiBase = "http://localhost:8080/api/v1",
    [string]$ApiKey = "87339ea3-1add-4689-ae57-3128ebd03c4f",
    [string]$BusinessCode = "WATER",
    [string]$FormCode = "WD12"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Set-Location "E:\Maheshwari\UGCL\backend\v1"

function New-BaseHeaders([string]$token) {
    return @{
        "x-api-key" = $ApiKey
        "Authorization" = "Bearer $token"
        "Content-Type" = "application/json"
        "Accept" = "application/json"
        "X-Business-Code" = $BusinessCode
    }
}

function Invoke-Json([string]$Method, [string]$Path, [hashtable]$Headers, [object]$Body = $null) {
    $uri = "$ApiBase$Path"
    if ($null -eq $Body) {
        return Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers
    }
    if ($Body -is [string]) {
        return Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers -Body $Body
    }
    return Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers -Body (ConvertTo-Json -InputObject $Body -Depth 60)
}

function Convert-ToWorkflowArray([object]$Value) {
    if ($null -eq $Value) { return @() }

    if ($Value.PSObject -and ($Value.PSObject.Properties.Name -contains "value") -and $null -ne $Value.value) {
        return @((ConvertTo-Json -InputObject $Value.value -Depth 60 | ConvertFrom-Json))
    }

    if ($Value -is [array]) {
        return @((ConvertTo-Json -InputObject $Value -Depth 60 | ConvertFrom-Json))
    }

    return @((ConvertTo-Json -InputObject $Value -Depth 60 | ConvertFrom-Json))
}

function Login([string]$phone) {
    $headers = @{ "x-api-key" = $ApiKey; "Content-Type" = "application/json" }
    return Invoke-RestMethod -Method Post -Uri "$ApiBase/login" -Headers $headers -Body (@{ phone = $phone; password = "Welcome@123" } | ConvertTo-Json)
}

$results = [System.Collections.Generic.List[object]]::new()
function Add-Result([string]$Category, [string]$Case, [string]$Status, [string]$Detail) {
    $results.Add([pscustomobject]@{ Category = $Category; Case = $Case; Status = $Status; Detail = $Detail })
    Write-Host "[$Status] $Category :: $Case :: $Detail"
}

function Get-UnreadCount([hashtable]$Headers) {
    return (Invoke-Json "GET" "/notifications/unread-count" $Headers).count
}

function Create-And-Submit([hashtable]$Headers, [hashtable]$FormData) {
    $create = Invoke-Json "POST" "/business/$BusinessCode/forms/$FormCode/submissions" $Headers @{ form_data = $FormData }
    $sid = $create.submission.id
    $null = Invoke-Json "POST" "/business/$BusinessCode/forms/$FormCode/submissions/$sid/transition" $Headers @{ action = "submit" }
    return $sid
}

function Has-NotificationForSubmission([hashtable]$Headers, [string]$SubmissionID) {
    $list = Invoke-Json "GET" "/notifications?limit=200" $Headers
    $items = @($list.notifications)
    $matches = @($items | Where-Object { $_.submission_id -eq $SubmissionID })
    return ($matches.Count -gt 0)
}

function Ensure-RoleHasAtLeastOneUser([object]$Role, [object[]]$AllUsers, [hashtable]$Sessions, [hashtable]$Headers) {
    if (($Role.user_count -as [int]) -gt 0) { return $true }

    $candidate = @($AllUsers | Where-Object { $Sessions.ContainsKey($_.id) } | Select-Object -First 1)
    if (-not $candidate) { return $false }

    try {
        $null = Invoke-Json "POST" "/business/$BusinessCode/users/assign" $Headers @{
            user_id = $candidate.id
            business_role_id = $Role.id
        }
        $Role.user_count = 1
        Add-Result "ROLE_SETUP" $Role.name "PASS" "assigned fallback user: $($candidate.name)"
        return $true
    } catch {
        Add-Result "ROLE_SETUP" $Role.name "FAIL" $_.Exception.Message
        return $false
    }
}

function Get-BusinessUsers([hashtable]$Headers) {
    $resp = Invoke-Json "GET" "/business/$BusinessCode/users?page=1&limit=500" $Headers
    if ($resp -and ($resp.PSObject.Properties.Name -contains "data")) {
        return @($resp.data)
    }
    return @()
}

# Login and discover users
$super = Login "9999999999"
$superHeaders = New-BaseHeaders $super.token

$usersResp = Invoke-Json "GET" "/admin/users" $superHeaders
$users = @($usersResp.data)

$userSessions = @{}
foreach ($u in $users) {
    try {
        $session = Login $u.phone
        $userSessions[$u.id] = [pscustomobject]@{
            user = $u
            token = $session.token
            headers = (New-BaseHeaders $session.token)
        }
    } catch {
        Add-Result "LOGIN" $u.phone "FAIL" $_.Exception.Message
    }
}

# Get workflow + save original transitions
$wfResp = Invoke-Json "GET" "/admin/workflows" $superHeaders
$workflow = @($wfResp.workflows | Where-Object { $_.code -eq "standard_approval" }) | Select-Object -First 1
if (-not $workflow) { throw "standard_approval workflow not found" }
$workflowId = $workflow.id
$originalTransitionsJson = (Convert-ToWorkflowArray $workflow.transitions) | ConvertTo-Json -Depth 60

function Set-SubmitRecipient([object]$recipientDef, [string]$titlePrefix) {
    $wfReload = Invoke-Json "GET" "/admin/workflows" $superHeaders
    $wfObj = @($wfReload.workflows | Where-Object { $_.id -eq $workflowId }) | Select-Object -First 1
    if (-not $wfObj) { throw "workflow reload failed" }

    $transitions = Convert-ToWorkflowArray $wfObj.transitions
    foreach ($t in $transitions) {
        if ($t.action -eq "submit") {
            $notif = [pscustomobject]@{
                title_template = "$titlePrefix {{.FormCode}} submitted"
                body_template = "Submission {{.SubmissionID}} moved to {{.CurrentState}}"
                channels = @("in_app")
                recipients = @($recipientDef)
            }
            $t | Add-Member -NotePropertyName notifications -NotePropertyValue @($notif) -Force
        }
    }

    $payload = @{
        name = $wfObj.name
        code = $wfObj.code
        description = $wfObj.description
        initial_state = $wfObj.initial_state
        states = (Convert-ToWorkflowArray $wfObj.states)
        transitions = $transitions
        is_active = $wfObj.is_active
    }
    $null = Invoke-Json "PUT" "/admin/workflows/$workflowId" $superHeaders $payload
}

function Restore-Workflow {
    $wfReload = Invoke-Json "GET" "/admin/workflows" $superHeaders
    $wfObj = @($wfReload.workflows | Where-Object { $_.id -eq $workflowId }) | Select-Object -First 1
    if (-not $wfObj) { return }
    $payload = @{
        name = $wfObj.name
        code = $wfObj.code
        description = $wfObj.description
        initial_state = $wfObj.initial_state
        states = (Convert-ToWorkflowArray $wfObj.states)
        transitions = ($originalTransitionsJson | ConvertFrom-Json)
        is_active = $wfObj.is_active
    }
    $null = Invoke-Json "PUT" "/admin/workflows/$workflowId" $superHeaders $payload
}

try {
    # 1) USER-PAIR combinations using field_value recipient by UUID
    Set-SubmitRecipient ([pscustomobject]@{ type = "field_value"; value = "assignee_user_id" }) "Pair-UUID"
    foreach ($submitter in $users) {
        foreach ($recipient in $users) {
            $case = "$($submitter.name) -> $($recipient.name)"
            if (-not $userSessions.ContainsKey($submitter.id) -or -not $userSessions.ContainsKey($recipient.id)) {
                Add-Result "PAIR_UUID" $case "SKIP" "login unavailable"
                continue
            }

            $submitterHeaders = $userSessions[$submitter.id].headers
            $recipientHeaders = $userSessions[$recipient.id].headers
            $before = Get-UnreadCount $recipientHeaders

            try {
                $sid = Create-And-Submit $submitterHeaders @{
                    smoke_test = $true
                    matrix = "pair_uuid"
                    assignee_user_id = $recipient.id
                    smoke_ref = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
                }
                $after = Get-UnreadCount $recipientHeaders
                $matched = Has-NotificationForSubmission $recipientHeaders $sid
                if (($after -gt $before) -and $matched) {
                    Add-Result "PAIR_UUID" $case "PASS" "submission=$sid unread $before->$after"
                } else {
                    Add-Result "PAIR_UUID" $case "FAIL" "submission=$sid unread $before->$after matched=$matched"
                }
            } catch {
                Add-Result "PAIR_UUID" $case "BLOCKED" $_.Exception.Message
            }
        }
    }

    # 2) ROLE combinations (all WATER roles with assigned users)
    $rolesRaw = Invoke-Json "GET" "/business/$BusinessCode/roles" $superHeaders
    $roles = @($rolesRaw)

    $businessUsers = Get-BusinessUsers $superHeaders

    foreach ($role in $roles) {
        if (($role.user_count -as [int]) -le 0) {
            $ready = Ensure-RoleHasAtLeastOneUser $role $users $userSessions $superHeaders
            if (-not $ready) {
                Add-Result "ROLE" $role.name "SKIP" "no assigned users and auto-assign failed"
                continue
            }
            # Refresh role/user mapping after assignment.
            $businessUsers = Get-BusinessUsers $superHeaders
        }

        # pick a probe recipient in this role from discovered users
        $probe = @($businessUsers | Where-Object {
            if (-not ($_.PSObject.Properties.Name -contains "roles")) { return $false }
            $rolesForUser = @($_.roles)
            if ($rolesForUser.Count -eq 0) { return $false }
            if (-not $userSessions.ContainsKey($_.id)) { return $false }
            return (@($rolesForUser | Where-Object { $_.id -eq $role.id }).Count -gt 0)
        }) | Select-Object -First 1

        if (-not $probe) {
            Add-Result "ROLE" $role.name "SKIP" "unable to map user for role"
            continue
        }

        Set-SubmitRecipient ([pscustomobject]@{ type = "business_role"; business_role_id = $role.id }) "Role"

        foreach ($submitter in $users) {
            $case = "$($submitter.name) -> role:$($role.name) probe:$($probe.name)"
            if (-not $userSessions.ContainsKey($submitter.id) -or -not $userSessions.ContainsKey($probe.id)) {
                Add-Result "ROLE" $case "SKIP" "login unavailable"
                continue
            }

            $submitterHeaders = $userSessions[$submitter.id].headers
            $probeHeaders = $userSessions[$probe.id].headers
            $before = Get-UnreadCount $probeHeaders

            try {
                $sid = Create-And-Submit $submitterHeaders @{
                    smoke_test = $true
                    matrix = "role"
                    target_role = $role.name
                    smoke_ref = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
                }
                $after = Get-UnreadCount $probeHeaders
                $matched = Has-NotificationForSubmission $probeHeaders $sid
                if (($after -gt $before) -and $matched) {
                    Add-Result "ROLE" $case "PASS" "submission=$sid unread $before->$after"
                } else {
                    Add-Result "ROLE" $case "FAIL" "submission=$sid unread $before->$after matched=$matched"
                }
            } catch {
                Add-Result "ROLE" $case "BLOCKED" $_.Exception.Message
            }
        }
    }

    # 3) PERMISSION combinations for all permissions assigned to active WATER roles with users
    $rolePermNames = New-Object System.Collections.Generic.HashSet[string]
    foreach ($role in $roles) {
        if (($role.user_count -as [int]) -le 0) { continue }
        foreach ($p in @($role.permissions)) {
            if ($p.name) { $null = $rolePermNames.Add([string]$p.name) }
        }
    }

    $permissionNames = @($rolePermNames)
    $permissionNames = $permissionNames | Sort-Object

    # Use Water Admin as submitter baseline if available
    $submitter = @($users | Where-Object { $_.phone -eq "9999999901" }) | Select-Object -First 1
    if (-not $submitter) { $submitter = $users[0] }

    foreach ($permName in $permissionNames) {
        $case = "permission:$permName"
        Set-SubmitRecipient ([pscustomobject]@{ type = "permission"; permission_code = $permName }) "Perm"

        if (-not $userSessions.ContainsKey($submitter.id)) {
            Add-Result "PERMISSION" $case "SKIP" "submitter login unavailable"
            continue
        }

        $submitterHeaders = $userSessions[$submitter.id].headers

        $beforeMap = @{}
        foreach ($u in $users) {
            if ($userSessions.ContainsKey($u.id)) {
                $beforeMap[$u.id] = Get-UnreadCount $userSessions[$u.id].headers
            }
        }

        try {
            $sid = Create-And-Submit $submitterHeaders @{
                smoke_test = $true
                matrix = "permission"
                permission_code = $permName
                smoke_ref = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
            }

            $delivered = @()
            foreach ($u in $users) {
                if (-not $userSessions.ContainsKey($u.id)) { continue }
                $after = Get-UnreadCount $userSessions[$u.id].headers
                if ($after -gt $beforeMap[$u.id]) {
                    $delivered += $u.name
                }
            }

            if ($delivered.Count -gt 0) {
                Add-Result "PERMISSION" $case "PASS" ("submission=$sid delivered={0}" -f ($delivered -join ","))
            } else {
                Add-Result "PERMISSION" $case "FAIL" "submission=$sid delivered=none"
            }
        } catch {
            Add-Result "PERMISSION" $case "BLOCKED" $_.Exception.Message
        }
    }
}
finally {
    Restore-Workflow
}

Write-Host ""
Write-Host "=== Workflow + Notification All-Combination Summary ==="
$results | Format-Table -AutoSize

$failCount = @($results | Where-Object { $_.Status -eq "FAIL" }).Count
$blockedCount = @($results | Where-Object { $_.Status -eq "BLOCKED" }).Count
Write-Host "FAIL count: $failCount"
Write-Host "BLOCKED count: $blockedCount"

if ($failCount -gt 0 -or $blockedCount -gt 0) {
    exit 1
}
exit 0
