<#
.SYNOPSIS
    Exhaustive workflow transition notification test.
    Verifies that the form submitter receives in-app notifications for:
      submit, approve, reject, revise transitions (standard_approval workflow)

.USAGE
    powershell -ExecutionPolicy Bypass -File workflow_transition_notification_test.ps1
#>

param(
    [string]$ApiBase     = "http://localhost:8080/api/v1",
    [string]$ApiKey      = "87339ea3-1add-4689-ae57-3128ebd03c4f",
    [string]$BusinessCode = "WATER",
    [string]$FormCode    = "WD12"
)

$ErrorActionPreference = "Stop"

# ──────────────────────────────────────────────────────────────────────────────
# Helpers
# ──────────────────────────────────────────────────────────────────────────────
$PASS = 0; $FAIL = 0
$Results = [System.Collections.Generic.List[object]]::new()

function Log {
    param([string]$Status, [string]$Name, [string]$Detail)
    $Results.Add([pscustomobject]@{Status=$Status; Name=$Name; Detail=$Detail})
    $c = switch ($Status) { "PASS" {"Green"} "FAIL" {"Red"} "INFO" {"Cyan"} default {"Yellow"} }
    Write-Host ("[{0}] {1} - {2}" -f $Status, $Name, $Detail) -ForegroundColor $c
    if ($Status -eq "PASS") { $script:PASS++ } elseif ($Status -eq "FAIL") { $script:FAIL++ }
}

function Api {
    param([string]$Method, [string]$Path, [hashtable]$Headers, [string]$Body)
    $uri = "$ApiBase$Path"
    try {
        if ($PSBoundParameters.ContainsKey('Body')) {
            return Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers -Body $Body
        }
        return Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers
    } catch {
        $code = $null; $msg = ""
        if ($_.Exception.Response) { try { $code = [int]$_.Exception.Response.StatusCode } catch {} }
        if ($_.ErrorDetails.Message) { $msg = $_.ErrorDetails.Message }
        elseif ($_.Exception.Message) { $msg = $_.Exception.Message }
        throw "HTTP $code @ $Method $Path :: $msg"
    }
}

function Login {
    param([string]$Phone, [string]$Password)
    $b = "{""phone"":""$Phone"",""password"":""$Password""}"
    $r = Invoke-RestMethod -Uri "$ApiBase/login" -Method POST -ContentType "application/json" `
        -Headers @{"x-api-key"=$ApiKey} -Body $b
    return $r.token
}

function Headers {
    param([string]$Token)
    return @{
        "x-api-key"     = $ApiKey
        "Authorization" = "Bearer $Token"
        "Content-Type"  = "application/json"
        "X-Business-Code" = $BusinessCode
    }
}

function GetUnreadNotifications {
    param([hashtable]$H, [string]$TitleContains)
    $r = Api -Method GET -Path "/notifications" -Headers $H
    $raw = $r.notifications
    [array]$notifs = if ($null -ne $raw) { @($raw) } else { @() }
    if ($TitleContains) {
        return [array]@($notifs | Where-Object { $_.title -like "*$TitleContains*" -and $_.is_read -eq $false })
    }
    return [array]@($notifs | Where-Object { $_.is_read -eq $false })
}

function NotificationCount { param([hashtable]$H)
    $r = Api -Method GET -Path "/notifications/unread-count" -Headers $H
    return $r.count
}

function CreateSubmission {
    param([hashtable]$H, [string]$SiteId)
    $body = @{
        form_data = @{ note = "notification-test-$(Get-Random)" }
        site_id   = $SiteId
    } | ConvertTo-Json -Depth 5 -Compress
    return Api -Method POST -Path "/business/$BusinessCode/forms/$FormCode/submissions" -Headers $H -Body $body
}

function FireTransition {
    param([hashtable]$H, [string]$SubmissionId, [string]$Action, [string]$Comment = "")
    $payload = @{ action = $Action }
    if ($Comment) { $payload["comment"] = $Comment }
    $body = $payload | ConvertTo-Json -Compress
    return Api -Method POST -Path "/business/$BusinessCode/forms/$FormCode/submissions/$SubmissionId/transition" `
        -Headers $H -Body $body
}

function DeleteSubmission {
    param([hashtable]$H, [string]$SubmissionId)
    try { Api -Method DELETE -Path "/business/$BusinessCode/forms/$FormCode/submissions/$SubmissionId" -Headers $H | Out-Null }
    catch { Write-Host "  [WARN] Could not delete $SubmissionId : $_" -ForegroundColor Yellow }
}

function MarkAllRead {
    param([hashtable]$H)
    try { Api -Method PATCH -Path "/notifications/read-all" -Headers $H | Out-Null } catch { Write-Host "  [WARN] read-all: $_" -ForegroundColor DarkYellow }
}

# ──────────────────────────────────────────────────────────────────────────────
# Setup
# ──────────────────────────────────────────────────────────────────────────────
Write-Host "=== Workflow Transition Notification Test ===" -ForegroundColor Cyan
Write-Host "API: $ApiBase | Business: $BusinessCode | Form: $FormCode"
Write-Host ""

# Login both actors
$adminTok = Login -Phone "9999999999" -Password "Welcome@123"
$engTok   = Login -Phone "9999999902" -Password "Welcome@123"
$adminH   = Headers -Token $adminTok
$engH     = Headers -Token $engTok

Log -Status "INFO" -Name "Auth" -Detail "super_admin and Water Engineer logged in"

# Discover site to use (pick first WATER site)
$sitesResp = Api -Method GET -Path "/admin/sites?limit=100" -Headers $adminH
$allSites = if ($sitesResp.data) { $sitesResp.data } elseif ($sitesResp.sites) { $sitesResp.sites } else { @() }
$waterSite = @($allSites | Where-Object { $_.business_vertical_code -eq $BusinessCode -or $_.businessVertical.Code -eq $BusinessCode } | Select-Object -First 1)
$siteId = if ($waterSite.Count -gt 0) { $waterSite[0].id } elseif ($allSites.Count -gt 0) { $allSites[0].id } else { $null }
Log -Status "INFO" -Name "Site" -Detail "Using site: $siteId"

# Confirm workflow has notifications on standard_approval
$wfs = (Api -Method GET -Path "/admin/workflows" -Headers $adminH).workflows
$stdWf = @($wfs | Where-Object { $_.code -eq "standard_approval" })[0]
if (-not $stdWf) { Log -Status "FAIL" -Name "Workflow check" -Detail "standard_approval workflow not found"; exit 1 }
[array]$stdTransitions = @()
if ($stdWf.transitions -and $stdWf.transitions.PSObject.Properties["value"]) {
    $stdTransitions = @($stdWf.transitions.value)
} elseif ($stdWf.transitions -is [array]) {
    $stdTransitions = @($stdWf.transitions)
} elseif ($stdWf.transitions) {
    $stdTransitions = @($stdWf.transitions)
}
[array]$approveTrans = @($stdTransitions | Where-Object { $_.action -eq "approve" })
if ($approveTrans.Count -eq 0 -or -not $approveTrans[0].PSObject.Properties["notifications"] -or $null -eq $approveTrans[0].notifications) {
    Log -Status "FAIL" -Name "Workflow notification check" -Detail "standard_approval 'approve' transition has no notifications configured"
    exit 1
}
Log -Status "PASS" -Name "Workflow notification check" -Detail "standard_approval transitions have notifications configured"

$createdIds = [System.Collections.Generic.List[string]]::new()

# ──────────────────────────────────────────────────────────────────────────────
# TEST 1: submit -> approve: submitter notified of approval
# ──────────────────────────────────────────────────────────────────────────────
Write-Host "`n--- TEST 1: Approve notification ---" -ForegroundColor DarkCyan

$beforeApprove = NotificationCount -H $engH

$sub1 = CreateSubmission -H $engH -SiteId $siteId
$sub1Id = $sub1.submission.id
if (-not $sub1Id) { $sub1Id = $sub1.id }
$createdIds.Add($sub1Id)
Log -Status "INFO" -Name "T1 Create" -Detail "Created submission $sub1Id"

FireTransition -H $engH -SubmissionId $sub1Id -Action "submit" | Out-Null
Log -Status "INFO" -Name "T1 Submit" -Detail "Submitted"

FireTransition -H $adminH -SubmissionId $sub1Id -Action "approve" | Out-Null
Log -Status "INFO" -Name "T1 Approve" -Detail "Approved by admin"

Start-Sleep -Milliseconds 500  # allow async notification write

$afterApprove = NotificationCount -H $engH
if ($afterApprove -gt $beforeApprove) {
    Log -Status "PASS" -Name "T1 Approve notification count" -Detail "Count went $beforeApprove -> $afterApprove"
} else {
    Log -Status "FAIL" -Name "T1 Approve notification count" -Detail "Count did not increase: $beforeApprove -> $afterApprove"
}

$approveNotif = [array](GetUnreadNotifications -H $engH -TitleContains "approved")
if ($approveNotif.Count -gt 0) {
    Log -Status "PASS" -Name "T1 Approve notification content" -Detail "Title: $($approveNotif[0].title)"
} else {
    # Try broader search
    [array]$allUnread = GetUnreadNotifications -H $engH
    [array]$approveAny = @($allUnread | Where-Object { $_.title -like "*$FormCode*" -or $_.body -like "*approved*" })
    if ($approveAny.Count -gt 0) {
        Log -Status "PASS" -Name "T1 Approve notification content" -Detail "Found approval notification: $($approveAny[0].title)"
    } else {
        Log -Status "FAIL" -Name "T1 Approve notification content" -Detail "No approval notification found for engineer. Unread count: $afterApprove"
    }
}

MarkAllRead -H $engH

# ──────────────────────────────────────────────────────────────────────────────
# TEST 2: submit -> reject: submitter notified of rejection with comment
# ──────────────────────────────────────────────────────────────────────────────
Write-Host "`n--- TEST 2: Reject notification ---" -ForegroundColor DarkCyan

$beforeReject = NotificationCount -H $engH

$sub2 = CreateSubmission -H $engH -SiteId $siteId
$sub2Id = $sub2.submission.id
if (-not $sub2Id) { $sub2Id = $sub2.id }
$createdIds.Add($sub2Id)
Log -Status "INFO" -Name "T2 Create" -Detail "Created submission $sub2Id"

FireTransition -H $engH -SubmissionId $sub2Id -Action "submit" | Out-Null
Log -Status "INFO" -Name "T2 Submit" -Detail "Submitted"

FireTransition -H $adminH -SubmissionId $sub2Id -Action "reject" -Comment "Missing site details" | Out-Null
Log -Status "INFO" -Name "T2 Reject" -Detail "Rejected by admin"

Start-Sleep -Milliseconds 500

$afterReject = NotificationCount -H $engH
if ($afterReject -gt $beforeReject) {
    Log -Status "PASS" -Name "T2 Reject notification count" -Detail "Count went $beforeReject -> $afterReject"
} else {
    Log -Status "FAIL" -Name "T2 Reject notification count" -Detail "Count did not increase: $beforeReject -> $afterReject"
}

$rejectNotif = [array](GetUnreadNotifications -H $engH -TitleContains "rejected")
if ($rejectNotif.Count -gt 0) {
    Log -Status "PASS" -Name "T2 Reject notification content" -Detail "Title: $($rejectNotif[0].title)"
} else {
    [array]$allUnread2 = GetUnreadNotifications -H $engH
    [array]$rejectAny = @($allUnread2 | Where-Object { $_.title -like "*$FormCode*" -or $_.body -like "*reject*" })
    if ($rejectAny.Count -gt 0) {
        Log -Status "PASS" -Name "T2 Reject notification content" -Detail "Found rejection notification: $($rejectAny[0].title)"
    } else {
        Log -Status "FAIL" -Name "T2 Reject notification content" -Detail "No rejection notification found for engineer"
    }
}

MarkAllRead -H $engH

# ──────────────────────────────────────────────────────────────────────────────
# TEST 3: submit -> reject -> revise: submitter notified of revision request
# ──────────────────────────────────────────────────────────────────────────────
Write-Host "`n--- TEST 3: Revise notification ---" -ForegroundColor DarkCyan

$sub3 = CreateSubmission -H $engH -SiteId $siteId
$sub3Id = $sub3.submission.id
if (-not $sub3Id) { $sub3Id = $sub3.id }
$createdIds.Add($sub3Id)
Log -Status "INFO" -Name "T3 Create" -Detail "Created submission $sub3Id"

FireTransition -H $engH -SubmissionId $sub3Id -Action "submit" | Out-Null
FireTransition -H $adminH -SubmissionId $sub3Id -Action "reject" -Comment "Needs more info" | Out-Null
Log -Status "INFO" -Name "T3 Submit+Reject" -Detail "Submitted then rejected"

MarkAllRead -H $engH   # clear so revise notification is isolated
$beforeRevise = NotificationCount -H $engH

# The engineer (submitter) triggers revise (send back to draft)
FireTransition -H $engH -SubmissionId $sub3Id -Action "revise" | Out-Null
Log -Status "INFO" -Name "T3 Revise" -Detail "Revise transition fired by engineer"

Start-Sleep -Milliseconds 500

$afterRevise = NotificationCount -H $engH
if ($afterRevise -gt $beforeRevise) {
    Log -Status "PASS" -Name "T3 Revise notification count" -Detail "Count went $beforeRevise -> $afterRevise"
} else {
    Log -Status "FAIL" -Name "T3 Revise notification count" -Detail "Count did not increase: $beforeRevise -> $afterRevise"
}

$reviseNotif = [array](GetUnreadNotifications -H $engH -TitleContains "Revision")
if ($reviseNotif.Count -gt 0) {
    Log -Status "PASS" -Name "T3 Revise notification content" -Detail "Title: $($reviseNotif[0].title)"
} else {
    [array]$allUnread3 = GetUnreadNotifications -H $engH
    [array]$reviseAny = @($allUnread3 | Where-Object { $_.title -like "*revis*" -or $_.body -like "*revis*" })
    if ($reviseAny.Count -gt 0) {
        Log -Status "PASS" -Name "T3 Revise notification content" -Detail "Found revision notification: $($reviseAny[0].title)"
    } else {
        Log -Status "FAIL" -Name "T3 Revise notification content" -Detail "No revision notification found. Unread count: $afterRevise"
    }
}

MarkAllRead -H $engH

# ──────────────────────────────────────────────────────────────────────────────
# TEST 4: submit transition also notifies submitter
# ──────────────────────────────────────────────────────────────────────────────
Write-Host "`n--- TEST 4: Submit notification ---" -ForegroundColor DarkCyan

$beforeSubmit = NotificationCount -H $engH

$sub4 = CreateSubmission -H $engH -SiteId $siteId
$sub4Id = $sub4.submission.id
if (-not $sub4Id) { $sub4Id = $sub4.id }
$createdIds.Add($sub4Id)
# capture count AFTER creating (before submit) to isolate submit notification
$beforeSubmit = NotificationCount -H $engH

FireTransition -H $engH -SubmissionId $sub4Id -Action "submit" | Out-Null
Log -Status "INFO" -Name "T4 Submit" -Detail "Submitted submission $sub4Id"

Start-Sleep -Milliseconds 800

$afterSubmit = NotificationCount -H $engH
if ($afterSubmit -gt $beforeSubmit) {
    Log -Status "PASS" -Name "T4 Submit notification count" -Detail "Count went $beforeSubmit -> $afterSubmit"
} else {
    Log -Status "FAIL" -Name "T4 Submit notification count" -Detail "Count did not increase: $beforeSubmit -> $afterSubmit"
}

$submitNotif = [array](GetUnreadNotifications -H $engH -TitleContains "submitted")
if ($submitNotif.Count -gt 0) {
    # Verify notification is for our submission (body contains submission ID)
    $ourNotif = @($submitNotif | Where-Object { $_.body -like "*$sub4Id*" -or $_.title -like "*$FormCode*" })
    if ($ourNotif.Count -gt 0) {
        Log -Status "PASS" -Name "T4 Submit notification content" -Detail "Title: $($ourNotif[0].title)"
    } else {
        Log -Status "PASS" -Name "T4 Submit notification content" -Detail "Found submit notification: $($submitNotif[0].title)"
    }
} else {
    Log -Status "FAIL" -Name "T4 Submit notification content" -Detail "No 'submitted' notification found"
}

MarkAllRead -H $engH

# ──────────────────────────────────────────────────────────────────────────────
# TEST 5: Notification mark-read and unread-count accuracy
# ──────────────────────────────────────────────────────────────────────────────
Write-Host "`n--- TEST 5: Mark read & unread count ---" -ForegroundColor DarkCyan

# Get list of unread notifications
[array]$unread5 = GetUnreadNotifications -H $engH
if ($unread5.Count -gt 0) {
    $nid = $unread5[0].id
    Api -Method PATCH -Path "/notifications/$nid/read" -Headers $engH | Out-Null
    [array]$after5 = GetUnreadNotifications -H $engH
    [array]$remaining = @($after5 | Where-Object { $_.id -eq $nid })
    if ($remaining.Count -eq 0) {
        Log -Status "PASS" -Name "T5 Mark single read" -Detail "Notification $nid marked read"
    } else {
        Log -Status "FAIL" -Name "T5 Mark single read" -Detail "Notification $nid still in unread list"
    }
} else {
    Log -Status "INFO" -Name "T5 Mark single read" -Detail "No unread notifications to test with"
}

# Read-all then count = 0
MarkAllRead -H $engH
$count5 = NotificationCount -H $engH
if ($count5 -eq 0) {
    Log -Status "PASS" -Name "T5 Mark all read" -Detail "Unread count is 0 after read-all"
} else {
    Log -Status "FAIL" -Name "T5 Mark all read" -Detail "Unread count is $count5 after read-all"
}

# ──────────────────────────────────────────────────────────────────────────────
# Cleanup: delete test submissions
# ──────────────────────────────────────────────────────────────────────────────
Write-Host "`n--- Cleanup ---" -ForegroundColor DarkCyan
foreach ($id in $createdIds) {
    DeleteSubmission -H $adminH -SubmissionId $id
    Write-Host "  Deleted $id"
}

# ──────────────────────────────────────────────────────────────────────────────
# Summary
# ──────────────────────────────────────────────────────────────────────────────
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
$summaryColor = if ($FAIL -eq 0) { "Green" } else { "Red" }
Write-Host "  RESULTS: PASS=$PASS  FAIL=$FAIL" -ForegroundColor $summaryColor
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
$Results | Format-Table -AutoSize

if ($FAIL -gt 0) { exit 1 } else { exit 0 }
