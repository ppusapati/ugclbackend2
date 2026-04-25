param(
    [string]$ApiRoot = "http://localhost:8080",
    [string]$ApiBase = "http://localhost:8080/api/v1",
    [string]$ApiKey = "87339ea3-1add-4689-ae57-3128ebd03c4f",
    [string]$BusinessCode = "WATER",
    [string]$FormCode = "WD12"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

Set-Location "E:\Maheshwari\UGCL\backend\v1"

function New-Headers([string]$token) {
    return @{
        "x-api-key" = $ApiKey
        "Authorization" = "Bearer $token"
        "Content-Type" = "application/json"
        "Accept" = "application/json"
        "X-Business-Code" = $BusinessCode
    }
}

function Get-Token([string]$phone) {
    $h = @{ "x-api-key" = $ApiKey; "Content-Type" = "application/json" }
    $body = @{ phone = $phone; password = "Welcome@123" } | ConvertTo-Json
    $resp = Invoke-RestMethod -Method Post -Uri "$ApiRoot/login" -Headers $h -Body $body
    return $resp
}

function Invoke-Json([string]$method, [string]$url, [hashtable]$headers, [object]$body = $null) {
    if ($null -eq $body) {
        return Invoke-RestMethod -Method $method -Uri $url -Headers $headers
    }
    if ($body -is [string]) {
        return Invoke-RestMethod -Method $method -Uri $url -Headers $headers -Body $body
    }
    return Invoke-RestMethod -Method $method -Uri $url -Headers $headers -Body (ConvertTo-Json -InputObject $body -Depth 40)
}

function Convert-ToWorkflowArray([object]$Value) {
    if ($null -eq $Value) { return @() }

    if ($Value.PSObject -and ($Value.PSObject.Properties.Name -contains "value") -and $null -ne $Value.value) {
        return @((ConvertTo-Json -InputObject $Value.value -Depth 40 | ConvertFrom-Json))
    }

    if ($Value -is [array]) {
        return @((ConvertTo-Json -InputObject $Value -Depth 40 | ConvertFrom-Json))
    }

    return @((ConvertTo-Json -InputObject $Value -Depth 40 | ConvertFrom-Json))
}

$results = [System.Collections.Generic.List[object]]::new()
function Add-Case([string]$name, [string]$status, [string]$detail) {
    $results.Add([pscustomobject]@{ Name = $name; Status = $status; Detail = $detail })
    Write-Host ("[{0}] {1} - {2}" -f $status, $name, $detail)
}

# Login users
$super = Get-Token "9999999999"
$waterAdmin = Get-Token "9999999901"
$waterEngineer = Get-Token "9999999902"

$superHeaders = New-Headers $super.token
$waterAdminHeaders = New-Headers $waterAdmin.token
$waterEngineerHeaders = New-Headers $waterEngineer.token

# Smoke script matrix
function Run-SmokeCase([string]$name, [string]$token, [string[]]$extraArgs, [bool]$expectPass) {
    $cmdArgs = @("-NoProfile", "-File", ".\\scripts\\workflow_notification_smoke.ps1", "-Token", $token) + $extraArgs
    $null = & pwsh @cmdArgs
    $exit = $LASTEXITCODE
    $passed = ($exit -eq 0)

    if ($expectPass -and $passed) {
        Add-Case $name "PASS" "exit=$exit"
    } elseif ($expectPass -and -not $passed) {
        Add-Case $name "FAIL" "expected pass, exit=$exit"
    } elseif (-not $expectPass -and -not $passed) {
        Add-Case $name "PASS" "expected fail, exit=$exit"
    } else {
        Add-Case $name "FAIL" "expected fail but passed"
    }
}

Run-SmokeCase -name "Smoke super-admin full" -token $super.token -extraArgs @("-Mode", "full") -expectPass $true
Run-SmokeCase -name "Smoke super-admin readonly" -token $super.token -extraArgs @("-Mode", "readonly") -expectPass $true
Run-SmokeCase -name "Smoke water-admin full (admin-gated expected fail)" -token $waterAdmin.token -extraArgs @("-Mode", "full") -expectPass $false
Run-SmokeCase -name "Smoke water-admin full SkipAdmin" -token $waterAdmin.token -extraArgs @("-Mode", "full", "-SkipAdmin") -expectPass $true
Run-SmokeCase -name "Smoke water-admin readonly SkipAdmin SkipSse" -token $waterAdmin.token -extraArgs @("-Mode", "readonly", "-SkipAdmin", "-SkipSse") -expectPass $true
Run-SmokeCase -name "Smoke water-engineer full SkipAdmin" -token $waterEngineer.token -extraArgs @("-Mode", "full", "-SkipAdmin") -expectPass $true

# Cross-user notification matrix with transition patching
$wfResp = Invoke-Json "GET" "$ApiBase/admin/workflows" $superHeaders
$workflow = @($wfResp.workflows | Where-Object { $_.code -eq "standard_approval" }) | Select-Object -First 1
if (-not $workflow) { throw "standard_approval workflow not found" }
$workflowId = $workflow.id
$originalTransitions = (Convert-ToWorkflowArray $workflow.transitions) | ConvertTo-Json -Depth 40

# Discover water engineer business role id for business_role recipient test
$waterRolesResp = Invoke-Json "GET" "$ApiBase/business/$BusinessCode/roles" $superHeaders
$rolesList = @()
if ($waterRolesResp -is [array]) {
    $rolesList = @($waterRolesResp)
} elseif ($waterRolesResp.PSObject.Properties.Name -contains "roles") {
    $rolesList = @($waterRolesResp.roles)
}
$engineerRole = @($rolesList | Where-Object { $_.name -eq "Engineer" }) | Select-Object -First 1
$engineerRoleId = $null
if ($engineerRole) { $engineerRoleId = $engineerRole.id }

function Set-SubmitNotification([object]$recipientDef) {
    $wfReload = Invoke-Json "GET" "$ApiBase/admin/workflows" $superHeaders
    $wfObj = @($wfReload.workflows | Where-Object { $_.id -eq $workflowId }) | Select-Object -First 1
    if (-not $wfObj) { throw "workflow not found for patch" }

    $transitions = Convert-ToWorkflowArray $wfObj.transitions
    foreach ($t in $transitions) {
        if ($t.action -eq "submit") {
            $notif = [pscustomobject]@{
                title_template = "Matrix: {{.FormCode}} submitted"
                body_template  = "Submission {{.SubmissionID}} moved to {{.CurrentState}}"
                channels       = @("in_app")
                recipients     = @($recipientDef)
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
    $null = Invoke-Json "PUT" "$ApiBase/admin/workflows/$workflowId" $superHeaders $payload
}

function Restore-WorkflowTransitions() {
    $wfReload = Invoke-Json "GET" "$ApiBase/admin/workflows" $superHeaders
    $wfObj = @($wfReload.workflows | Where-Object { $_.id -eq $workflowId }) | Select-Object -First 1
    if (-not $wfObj) { throw "workflow not found for restore" }

    $payload = @{
        name = $wfObj.name
        code = $wfObj.code
        description = $wfObj.description
        initial_state = $wfObj.initial_state
        states = (Convert-ToWorkflowArray $wfObj.states)
        transitions = ($originalTransitions | ConvertFrom-Json)
        is_active = $wfObj.is_active
    }
    $null = Invoke-Json "PUT" "$ApiBase/admin/workflows/$workflowId" $superHeaders $payload
}

function Run-CrossUserCase([string]$name, [object]$recipientDef, [string]$recipientToken, [hashtable]$submitterHeaders, [hashtable]$recipientHeaders, [hashtable]$formData) {
    try {
        Set-SubmitNotification $recipientDef

        $before = (Invoke-Json "GET" "$ApiBase/notifications/unread-count" $recipientHeaders).count

        $payload = @{ form_data = $formData }
        $create = Invoke-Json "POST" "$ApiBase/business/$BusinessCode/forms/$FormCode/submissions" $submitterHeaders $payload
        $sid = $create.submission.id
        $null = Invoke-Json "POST" "$ApiBase/business/$BusinessCode/forms/$FormCode/submissions/$sid/transition" $submitterHeaders (@{ action = "submit" })

        $after = (Invoke-Json "GET" "$ApiBase/notifications/unread-count" $recipientHeaders).count
        $list = Invoke-Json "GET" "$ApiBase/notifications?limit=20" $recipientHeaders
        $matched = @($list.notifications | Where-Object { $_.submission_id -eq $sid })

        if (($after -gt $before) -and ($matched.Count -gt 0)) {
            Add-Case $name "PASS" ("submission={0} unread {1}->{2}" -f $sid, $before, $after)
        } else {
            Add-Case $name "FAIL" ("submission={0} unread {1}->{2}, matched={3}" -f $sid, $before, $after, $matched.Count)
        }
    }
    catch {
        Add-Case $name "FAIL" $_.Exception.Message
    }
}

try {
    $engUUID = $waterEngineer.user.id
    $engEmail = $waterEngineer.user.email
    $engPhone = $waterEngineer.user.phone

    Run-CrossUserCase -name "Cross-user recipient type=user value=phone" `
        -recipientDef ([pscustomobject]@{ type = "user"; value = $engPhone }) `
        -recipientToken $waterEngineer.token -submitterHeaders $waterAdminHeaders -recipientHeaders $waterEngineerHeaders `
        -formData @{ smoke_test = $true; target = $engPhone; mode = "user_phone"; smoke_ref = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() }

    Run-CrossUserCase -name "Cross-user recipient type=user value=email" `
        -recipientDef ([pscustomobject]@{ type = "user"; value = $engEmail }) `
        -recipientToken $waterEngineer.token -submitterHeaders $waterAdminHeaders -recipientHeaders $waterEngineerHeaders `
        -formData @{ smoke_test = $true; target = $engEmail; mode = "user_email"; smoke_ref = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() }

    Run-CrossUserCase -name "Cross-user recipient type=user value=uuid" `
        -recipientDef ([pscustomobject]@{ type = "user"; value = $engUUID }) `
        -recipientToken $waterEngineer.token -submitterHeaders $waterAdminHeaders -recipientHeaders $waterEngineerHeaders `
        -formData @{ smoke_test = $true; target = $engUUID; mode = "user_uuid"; smoke_ref = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() }

    Run-CrossUserCase -name "Cross-user recipient type=field_value phone" `
        -recipientDef ([pscustomobject]@{ type = "field_value"; value = "assignee_phone" }) `
        -recipientToken $waterEngineer.token -submitterHeaders $waterAdminHeaders -recipientHeaders $waterEngineerHeaders `
        -formData @{ smoke_test = $true; assignee_phone = $engPhone; mode = "field_value_phone"; smoke_ref = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() }

    Run-CrossUserCase -name "Cross-user recipient type=field_value object" `
        -recipientDef ([pscustomobject]@{ type = "field_value"; value = "assignee_obj" }) `
        -recipientToken $waterEngineer.token -submitterHeaders $waterAdminHeaders -recipientHeaders $waterEngineerHeaders `
        -formData @{ smoke_test = $true; assignee_obj = @{ phone = $engPhone; email = $engEmail }; mode = "field_value_object"; smoke_ref = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() }

    if ($engineerRoleId) {
        Run-CrossUserCase -name "Cross-user recipient type=business_role Engineer" `
            -recipientDef ([pscustomobject]@{ type = "business_role"; business_role_id = $engineerRoleId }) `
            -recipientToken $waterEngineer.token -submitterHeaders $waterAdminHeaders -recipientHeaders $waterEngineerHeaders `
            -formData @{ smoke_test = $true; mode = "business_role"; smoke_ref = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() }
    } else {
        Add-Case "Cross-user recipient type=business_role Engineer" "FAIL" "Engineer role id not found"
    }

    Run-CrossUserCase -name "Cross-user recipient type=permission attendance:checkin" `
        -recipientDef ([pscustomobject]@{ type = "permission"; permission_code = "attendance:checkin" }) `
        -recipientToken $waterEngineer.token -submitterHeaders $waterAdminHeaders -recipientHeaders $waterEngineerHeaders `
        -formData @{ smoke_test = $true; mode = "permission"; smoke_ref = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds() }
}
finally {
    Restore-WorkflowTransitions
}

Write-Host ""
Write-Host "=== Workflow + Notification Usecase Matrix Summary ==="
$results | Format-Table -AutoSize

$failCount = @($results | Where-Object { $_.Status -eq "FAIL" }).Count
if ($failCount -gt 0) {
    Write-Host "FAILED cases: $failCount"
    exit 1
}

Write-Host "All matrix cases passed"
exit 0
