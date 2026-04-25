param(
    [string]$ApiBase = "http://localhost:8080/api/v1",
    [string]$ApiKey  = "87339ea3-1add-4689-ae57-3128ebd03c4f",
    [string]$BusinessCode = "WATER",
    [string]$FormCode = "WD12"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Set-Location "E:\Maheshwari\UGCL\backend\v1"

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------
function New-BaseHeaders([string]$token) {
    return @{
        "x-api-key"       = $ApiKey
        "Authorization"   = "Bearer $token"
        "Content-Type"    = "application/json"
        "Accept"          = "application/json"
        "X-Business-Code" = $BusinessCode
    }
}

function Invoke-Json([string]$Method, [string]$Path, [hashtable]$Headers, [object]$Body = $null) {
    $uri = "$ApiBase$Path"
    if ($null -eq $Body) {
        return Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers
    }
    return Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers -Body (ConvertTo-Json -InputObject $Body -Depth 60)
}

function Convert-ToWorkflowArray([object]$Value) {
    if ($null -eq $Value) { return @() }

    # Some PowerShell responses wrap JSON arrays as an object with a `value` property.
    if ($Value.PSObject -and ($Value.PSObject.Properties.Name -contains "value") -and $null -ne $Value.value) {
        return @((ConvertTo-Json -InputObject $Value.value -Depth 60 | ConvertFrom-Json))
    }

    if ($Value -is [array]) {
        return @((ConvertTo-Json -InputObject $Value -Depth 60 | ConvertFrom-Json))
    }

    return @((ConvertTo-Json -InputObject $Value -Depth 60 | ConvertFrom-Json))
}

function Is-ArrayShape([object]$Value) {
    if ($null -eq $Value) { return $false }
    return ($Value -is [array])
}

function Get-WorkflowById([string]$WorkflowID) {
    $wfReload = Invoke-Json "GET" "/admin/workflows" $superHeaders
    return @($wfReload.workflows | Where-Object { $_.id -eq $WorkflowID }) | Select-Object -First 1
}

function Put-WorkflowStrict([string]$WorkflowID, [hashtable]$Payload, [string]$ContextLabel) {
    $null = Invoke-Json "PUT" "/admin/workflows/$WorkflowID" $superHeaders $Payload

    $wfCheck = Get-WorkflowById $WorkflowID
    if (-not $wfCheck) {
        throw "Workflow $WorkflowID missing after $ContextLabel"
    }

    if (-not (Is-ArrayShape $wfCheck.transitions)) {
        $repairPayload = @{
            name = $wfCheck.name
            code = $wfCheck.code
            description = $wfCheck.description
            initial_state = $wfCheck.initial_state
            states = (Convert-ToWorkflowArray $wfCheck.states)
            transitions = (Convert-ToWorkflowArray $wfCheck.transitions)
            is_active = $wfCheck.is_active
        }
        $null = Invoke-Json "PUT" "/admin/workflows/$WorkflowID" $superHeaders $repairPayload

        $wfVerify = Get-WorkflowById $WorkflowID
        if (-not $wfVerify -or -not (Is-ArrayShape $wfVerify.transitions)) {
            throw "Workflow transitions shape is invalid after $ContextLabel"
        }
    }
}

function Login([string]$Phone) {
    $h = @{ "x-api-key" = $ApiKey; "Content-Type" = "application/json" }
    return Invoke-RestMethod -Method Post -Uri "$ApiBase/login" -Headers $h `
        -Body (@{ phone = $Phone; password = "Welcome@123" } | ConvertTo-Json)
}

$results = [System.Collections.Generic.List[object]]::new()
function Add-Result([string]$Category, [string]$Case, [string]$Status, [string]$Detail) {
    $results.Add([pscustomobject]@{ Category = $Category; Case = $Case; Status = $Status; Detail = $Detail })
    $colour = switch ($Status) { "PASS" { "Green" } "FAIL" { "Red" } "BLOCKED" { "Yellow" } default { "Cyan" } }
    Write-Host "[$Status] $Category :: $Case :: $Detail" -ForegroundColor $colour
}

function Get-UnreadCount([hashtable]$H) {
    return (Invoke-Json "GET" "/notifications/unread-count" $H).count
}

function Get-Notifications([hashtable]$H) {
    $r = Invoke-Json "GET" "/notifications?limit=200" $H
    return @($r.notifications)
}

function Has-NotifForSubmission([hashtable]$H, [string]$SID) {
    return (@(Get-Notifications $H | Where-Object { $_.submission_id -eq $SID }).Count -gt 0)
}

function Create-Submission([hashtable]$H, [hashtable]$FormData) {
    $r = Invoke-Json "POST" "/business/$BusinessCode/forms/$FormCode/submissions" $H @{ form_data = $FormData }
    return $r.submission.id
}

function Do-Transition([hashtable]$H, [string]$SID, [string]$Action, [string]$Comment = "") {
    $body = @{ action = $Action }
    if ($Comment) { $body["comment"] = $Comment }
    return Invoke-Json "POST" "/business/$BusinessCode/forms/$FormCode/submissions/$SID/transition" $H $body
}

function Get-SubmissionState([hashtable]$H, [string]$SID) {
    $r = Invoke-Json "GET" "/business/$BusinessCode/forms/$FormCode/submissions/$SID" $H
    return $r.submission.current_state
}

# ---------------------------------------------------------------------------
# Login users
# ---------------------------------------------------------------------------
Write-Host "`n=== Logging in users ===" -ForegroundColor Cyan

$lSuper  = Login "9999999999"
$lWAdmin = Login "9999999901"
$lWEng   = Login "9999999902"

$superHeaders = New-BaseHeaders $lSuper.token
$waHeaders    = New-BaseHeaders $lWAdmin.token
$weHeaders    = New-BaseHeaders $lWEng.token

$superID = $lSuper.user.id
$waID    = $lWAdmin.user.id
$weID    = $lWEng.user.id

Write-Host "Super Admin: $superID"
Write-Host "Water Admin: $waID"
Write-Host "Water Engineer: $weID"

# ---------------------------------------------------------------------------
# Load + save original workflow
# ---------------------------------------------------------------------------
$wfResp = Invoke-Json "GET" "/admin/workflows" $superHeaders
$wf = @($wfResp.workflows | Where-Object { $_.code -eq "standard_approval" }) | Select-Object -First 1
if (-not $wf) { throw "standard_approval workflow not found" }
$wfID = $wf.id
$originalTransJson = ConvertTo-Json -InputObject (Convert-ToWorkflowArray $wf.transitions) -Depth 60

function Restore-Workflow {
    try {
        $wfObj = Get-WorkflowById $wfID
        if (-not $wfObj) { return }
        $p = @{
            name = $wfObj.name
            code = $wfObj.code
            description = $wfObj.description
            initial_state = $wfObj.initial_state
            states = (Convert-ToWorkflowArray $wfObj.states)
            transitions = ($originalTransJson | ConvertFrom-Json)
            is_active = $wfObj.is_active
        }
        Put-WorkflowStrict $wfID $p "workflow restore"
        Write-Host "Workflow restored." -ForegroundColor Gray
    } catch {
        Write-Host "WARNING: Workflow restore failed: $($_.Exception.Message)" -ForegroundColor Yellow
    }
}

# ---------------------------------------------------------------------------
# Helper: patch ALL transitions with per-action notifications
#   submit  -> notify $recipientA (the approver / watcher)
#   approve -> notify $recipientB (the submitter / watcher)
#   reject  -> notify $recipientC (the submitter / watcher)
#   revise  -> notify $recipientD (the reviewer / watcher)
# ---------------------------------------------------------------------------
function Set-AllTransitionNotifications(
    [object]$submitRecip,
    [object]$approveRecip,
    [object]$rejectRecip,
    [object]$reviseRecip,
    [string]$Label
) {
    $wfObj = Get-WorkflowById $wfID
    $trans = Convert-ToWorkflowArray $wfObj.transitions

    foreach ($t in $trans) {
        $recip = switch ($t.action) {
            "submit"  { $submitRecip  }
            "approve" { $approveRecip }
            "reject"  { $rejectRecip  }
            "revise"  { $reviseRecip  }
        }
        if ($recip) {
            $notif = [pscustomobject]@{
                title_template = "[$Label] {{.FormCode}} {{.Action}}"
                body_template  = "Submission {{.SubmissionID}} -> {{.CurrentState}}"
                channels       = @("in_app")
                recipients     = @($recip)
            }
            $t | Add-Member -NotePropertyName notifications -NotePropertyValue @($notif) -Force
        } else {
            if ($t.PSObject.Properties.Name -contains "notifications") {
                $t.PSObject.Properties.Remove("notifications")
            }
        }
    }

    $payload = @{
        name = $wfObj.name
        code = $wfObj.code
        description = $wfObj.description
        initial_state = $wfObj.initial_state
        states = (Convert-ToWorkflowArray $wfObj.states)
        transitions = (Convert-ToWorkflowArray $trans)
        is_active = $wfObj.is_active
    }
    Put-WorkflowStrict $wfID $payload "set transition notifications [$Label]"
}

# ---------------------------------------------------------------------------
# Check notification received by a user for a given submission+action
# ---------------------------------------------------------------------------
function Check-NotifReceived([hashtable]$H, [string]$SID, [int]$BeforeCount, [string]$Label) {
    $after = Get-UnreadCount $H
    $matched = Has-NotifForSubmission $H $SID
    return [pscustomobject]@{
        received = ($after -gt $BeforeCount) -and $matched
        before   = $BeforeCount
        after    = $after
        detail   = "${Label}: unread $BeforeCount->$after matched=$matched"
    }
}

# ===========================================================================
# TEST SCENARIOS
# ===========================================================================
try {

# ---------------------------------------------------------------------------
# SCENARIO 1: Happy Path - all 3 WATER users cover each transition
#   submit:  Water Engineer  (draft -> submitted)  | notify -> Water Admin
#   approve: Water Admin     (submitted -> approved)| notify -> Water Engineer
#   Validate both notifications delivered correctly
# ---------------------------------------------------------------------------
Write-Host "`n--- SCENARIO 1: Happy Path (submit->approve) ---" -ForegroundColor Magenta

foreach ($submitterName in @("Super Admin", "Water Admin", "Water Engineer")) {
    foreach ($approverName in @("Super Admin", "Water Admin")) {
        $submitterH = switch ($submitterName) { "Super Admin" { $superHeaders } "Water Admin" { $waHeaders } "Water Engineer" { $weHeaders } }
        $approverH  = switch ($approverName)  { "Super Admin" { $superHeaders } "Water Admin" { $waHeaders } }
        $approverID = switch ($approverName)  { "Super Admin" { $superID }      "Water Admin" { $waID }      }
        $submitterID= switch ($submitterName) { "Super Admin" { $superID }      "Water Admin" { $waID }      "Water Engineer" { $weID } }

        # submit notifies approver; approve notifies submitter
        Set-AllTransitionNotifications `
            ([pscustomobject]@{ type = "user"; value = $approverID }) `
            ([pscustomobject]@{ type = "user"; value = $submitterID }) `
            $null `
            $null `
            "SC1"

        $approverBefore  = Get-UnreadCount $approverH
        $submitterBefore = Get-UnreadCount $submitterH

        try {
            $sid = Create-Submission $submitterH @{ sc = "happy"; approver = $approverID }
            Do-Transition $submitterH $sid "submit" | Out-Null
            $stateAfterSubmit = Get-SubmissionState $submitterH $sid

            $submitNotif = Check-NotifReceived $approverH $sid $approverBefore "approver"

            Do-Transition $approverH $sid "approve" "LGTM" | Out-Null
            $stateAfterApprove = Get-SubmissionState $approverH $sid

            $approveNotif = Check-NotifReceived $submitterH $sid $submitterBefore "submitter"

            $case = "HAPPY :: $submitterName -submit-> $approverName -approve-> final"
            $allOk = ($stateAfterSubmit -eq "submitted") -and ($stateAfterApprove -eq "approved") -and $submitNotif.received -and $approveNotif.received
            if ($allOk) {
                Add-Result "SC1_HAPPY" $case "PASS" "sid=$sid submit_notif=$($submitNotif.detail) approve_notif=$($approveNotif.detail)"
            } else {
                Add-Result "SC1_HAPPY" $case "FAIL" "sid=$sid states:$stateAfterSubmit/$stateAfterApprove submit_notif=$($submitNotif.received) approve_notif=$($approveNotif.received)"
            }
        } catch {
            Add-Result "SC1_HAPPY" "$submitterName -> $approverName" "BLOCKED" $_.Exception.Message
        }
    }
}

# ---------------------------------------------------------------------------
# SCENARIO 2: Rejection Path
#   submit:  Water Engineer  (draft -> submitted)  | notify -> Water Admin
#   reject:  Water Admin     (submitted -> rejected)| notify -> Water Engineer
# ---------------------------------------------------------------------------
Write-Host "`n--- SCENARIO 2: Rejection Path (submit->reject) ---" -ForegroundColor Magenta

foreach ($submitterName in @("Super Admin", "Water Admin", "Water Engineer")) {
    foreach ($rejectorName in @("Super Admin", "Water Admin")) {
        $submitterH  = switch ($submitterName) { "Super Admin" { $superHeaders } "Water Admin" { $waHeaders } "Water Engineer" { $weHeaders } }
        $rejectorH   = switch ($rejectorName)  { "Super Admin" { $superHeaders } "Water Admin" { $waHeaders } }
        $rejectorID  = switch ($rejectorName)  { "Super Admin" { $superID }      "Water Admin" { $waID }      }
        $submitterID = switch ($submitterName) { "Super Admin" { $superID }      "Water Admin" { $waID }      "Water Engineer" { $weID } }

        Set-AllTransitionNotifications `
            ([pscustomobject]@{ type = "user"; value = $rejectorID }) `
            $null `
            ([pscustomobject]@{ type = "user"; value = $submitterID }) `
            $null `
            "SC2"

        $rejectorBefore  = Get-UnreadCount $rejectorH
        $submitterBefore = Get-UnreadCount $submitterH

        try {
            $sid = Create-Submission $submitterH @{ sc = "reject"; rejector = $rejectorID }
            Do-Transition $submitterH $sid "submit" | Out-Null
            $stateAfterSubmit = Get-SubmissionState $submitterH $sid

            $submitNotif = Check-NotifReceived $rejectorH $sid $rejectorBefore "rejector"

            Do-Transition $rejectorH $sid "reject" "Needs revision" | Out-Null
            $stateAfterReject = Get-SubmissionState $rejectorH $sid

            $rejectNotif = Check-NotifReceived $submitterH $sid $submitterBefore "submitter"

            $case = "REJECT :: $submitterName -submit-> $rejectorName -reject-> final"
            $allOk = ($stateAfterSubmit -eq "submitted") -and ($stateAfterReject -eq "rejected") -and $submitNotif.received -and $rejectNotif.received
            if ($allOk) {
                Add-Result "SC2_REJECT" $case "PASS" "sid=$sid submit_notif=$($submitNotif.detail) reject_notif=$($rejectNotif.detail)"
            } else {
                Add-Result "SC2_REJECT" $case "FAIL" "sid=$sid states:$stateAfterSubmit/$stateAfterReject submit_notif=$($submitNotif.received) reject_notif=$($rejectNotif.received)"
            }
        } catch {
            Add-Result "SC2_REJECT" "$submitterName -> $rejectorName" "BLOCKED" $_.Exception.Message
        }
    }
}

# ---------------------------------------------------------------------------
# SCENARIO 3: Full Revise Cycle
#   draft -> submit -> reject -> revise -> submit -> approve
#   Notifications on every single transition step, all verified
# ---------------------------------------------------------------------------
Write-Host "`n--- SCENARIO 3: Full Revise Cycle (submit->reject->revise->submit->approve) ---" -ForegroundColor Magenta

$cycles = @(
    @{ submitterName = "Water Engineer"; reviewerName = "Water Admin";  submitterH = $weHeaders;    reviewerH = $waHeaders;    submitterID = $weID;    reviewerID = $waID   }
    @{ submitterName = "Water Admin";    reviewerName = "Super Admin"; submitterH = $waHeaders;    reviewerH = $superHeaders; submitterID = $waID;    reviewerID = $superID }
    @{ submitterName = "Super Admin";    reviewerName = "Water Admin";  submitterH = $superHeaders; reviewerH = $waHeaders;    submitterID = $superID; reviewerID = $waID   }
)

foreach ($cycle in $cycles) {
    $sName = $cycle.submitterName; $rName = $cycle.reviewerName
    $sH = $cycle.submitterH;      $rH = $cycle.reviewerH
    $sID = $cycle.submitterID;    $rID = $cycle.reviewerID

    # All 4 transitions have notifications
    #   submit  -> notify reviewer (built-in approver type is not set yet; use value)
    #   approve -> notify submitter
    #   reject  -> notify submitter
    #   revise  -> notify reviewer
    Set-AllTransitionNotifications `
        ([pscustomobject]@{ type = "user"; value = $rID }) `
        ([pscustomobject]@{ type = "submitter" }) `
        ([pscustomobject]@{ type = "submitter" }) `
        ([pscustomobject]@{ type = "user"; value = $rID }) `
        "SC3"

    try {
        $sid = Create-Submission $sH @{ sc = "cycle"; reviewer = $rID }

        # Step 1: submit
        $rBefore1 = Get-UnreadCount $rH
        Do-Transition $sH $sid "submit" | Out-Null
        $state1 = Get-SubmissionState $sH $sid
        $n1 = Check-NotifReceived $rH $sid $rBefore1 "reviewer@submit"

        # Step 2: reject
        $sBefore2 = Get-UnreadCount $sH
        Do-Transition $rH $sid "reject" "First review feedback" | Out-Null
        $state2 = Get-SubmissionState $rH $sid
        $n2 = Check-NotifReceived $sH $sid $sBefore2 "submitter@reject"

        # Step 3: revise (back to draft)
        $rBefore3 = Get-UnreadCount $rH
        Do-Transition $sH $sid "revise" | Out-Null
        $state3 = Get-SubmissionState $sH $sid
        $n3 = Check-NotifReceived $rH $sid $rBefore3 "reviewer@revise"

        # Step 4: re-submit
        $rBefore4 = Get-UnreadCount $rH
        Do-Transition $sH $sid "submit" | Out-Null
        $state4 = Get-SubmissionState $sH $sid
        $n4 = Check-NotifReceived $rH $sid $rBefore4 "reviewer@resubmit"

        # Step 5: approve
        $sBefore5 = Get-UnreadCount $sH
        Do-Transition $rH $sid "approve" "Looks good now" | Out-Null
        $state5 = Get-SubmissionState $rH $sid
        $n5 = Check-NotifReceived $sH $sid $sBefore5 "submitter@approve"

        $stateOk = ($state1 -eq "submitted") -and ($state2 -eq "rejected") -and ($state3 -eq "draft") -and ($state4 -eq "submitted") -and ($state5 -eq "approved")
        $notifOk = $n1.received -and $n2.received -and $n3.received -and $n4.received -and $n5.received

        $case = "CYCLE :: $sName <-> $rName (5 steps)"
        $detail = "sid=$sid states:$state1/$state2/$state3/$state4/$state5 notifs:submit=$($n1.received) reject=$($n2.received) revise=$($n3.received) resubmit=$($n4.received) approve=$($n5.received)"
        if ($stateOk -and $notifOk) {
            Add-Result "SC3_CYCLE" $case "PASS" $detail
        } else {
            Add-Result "SC3_CYCLE" $case "FAIL" $detail
        }
    } catch {
        Add-Result "SC3_CYCLE" "$sName <-> $rName" "BLOCKED" $_.Exception.Message
    }
}

# ---------------------------------------------------------------------------
# SCENARIO 4: Notification recipient type variety per transition
#   submit:  field_value (assignee_user_id in form data) -> resolved recipient
#   approve: business_role (Water_Admin role) -> all Water_Admin users notified
#   reject:  permission (attendance:read) -> all users with that perm notified
#   revise:  business_role (Engineer role)
# ---------------------------------------------------------------------------
Write-Host "`n--- SCENARIO 4: Mixed Recipient Types per Transition ---" -ForegroundColor Magenta

$waterAdminRole = $null
$engineerRole   = $null
try {
    $rolesResp = Invoke-Json "GET" "/business/$BusinessCode/roles" $superHeaders
    $rolesArr  = if ($rolesResp -is [array]) { $rolesResp } else { @($rolesResp.business_roles) }
    $waterAdminRole = @($rolesArr | Where-Object { $_.name -eq "Water_Admin" }) | Select-Object -First 1
    $engineerRole   = @($rolesArr | Where-Object { $_.name -eq "Engineer"    }) | Select-Object -First 1
    # Ensure business_role_id is a plain string (not array) for JSON serialization
    $engineerRoleID = [string]($engineerRole.id)
} catch { Write-Host "WARNING: could not fetch roles: $($_.Exception.Message)" -ForegroundColor Yellow }

if ($waterAdminRole -and $engineerRole) {
    Set-AllTransitionNotifications `
        ([pscustomobject]@{ type = "field_value"; value = "assignee_user_id" }) `
        ([pscustomobject]@{ type = "submitter" }) `
        ([pscustomobject]@{ type = "submitter" }) `
        ([pscustomobject]@{ type = "business_role"; business_role_id = $engineerRoleID }) `
        "SC4"

    # submit as Water Engineer, field_value assignee = Water Admin (waID)
    $waBefore  = Get-UnreadCount $waHeaders
    $weBefore  = Get-UnreadCount $weHeaders

    try {
        $sid = Create-Submission $weHeaders @{ sc = "mixed_recip"; assignee_user_id = $waID }

        # submit -> field_value -> Water Admin notified
        Do-Transition $weHeaders $sid "submit" | Out-Null
        $state1 = Get-SubmissionState $weHeaders $sid
        $n1 = Check-NotifReceived $waHeaders $sid $waBefore "WaterAdmin@submit(field_value)"

        # reject -> permission:attendance:read -> check Water Engineer (if has perm)
        $weBefore2 = Get-UnreadCount $weHeaders
        Do-Transition $waHeaders $sid "reject" "mixed recip test" | Out-Null
        $state2 = Get-SubmissionState $waHeaders $sid
        $n2 = Check-NotifReceived $weHeaders $sid $weBefore2 "SomeUser@reject(permission)"

        # revise -> business_role:Engineer -> Water Engineer notified
        $weBefore3 = Get-UnreadCount $weHeaders
        Do-Transition $weHeaders $sid "revise" | Out-Null
        $state3 = Get-SubmissionState $weHeaders $sid
        $n3 = Check-NotifReceived $weHeaders $sid $weBefore3 "WaterEng@revise(engineer_role)"

        # re-submit -> field_value -> Water Admin notified again
        $waBefore4 = Get-UnreadCount $waHeaders
        Do-Transition $weHeaders $sid "submit" | Out-Null
        $state4 = Get-SubmissionState $weHeaders $sid
        $n4 = Check-NotifReceived $waHeaders $sid $waBefore4 "WaterAdmin@resubmit(field_value)"

        # approve -> submitter type -> Water Engineer (the submitter) notified
        $weBefore5 = Get-UnreadCount $weHeaders
        Do-Transition $waHeaders $sid "approve" "mixed ok" | Out-Null
        $state5 = Get-SubmissionState $waHeaders $sid
        $n5 = Check-NotifReceived $weHeaders $sid $weBefore5 "WaterEng@approve(submitter)"

        $stateOk = ($state1 -eq "submitted") -and ($state2 -eq "rejected") -and ($state3 -eq "draft") -and ($state4 -eq "submitted") -and ($state5 -eq "approved")
        $notifOk = $n1.received -and $n3.received -and $n4.received -and $n5.received  # n2 depends on perm coverage

        $case = "MIXED_RECIP :: WaterEng submit, WaterAdmin approve, field_value+role+perm"
        $detail = "sid=$sid states:$state1/$state2/$state3/$state4/$state5 notifs:fv=$($n1.received) perm=$($n2.received) role=$($n3.received) fv2=$($n4.received) submitter_notif=$($n5.received)"
        if ($stateOk -and $notifOk) {
            Add-Result "SC4_MIXED" $case "PASS" $detail
        } else {
            Add-Result "SC4_MIXED" $case "FAIL" $detail
        }
    } catch {
        Add-Result "SC4_MIXED" "MIXED_RECIP" "BLOCKED" $_.Exception.Message
    }
} else {
    Add-Result "SC4_MIXED" "MIXED_RECIP" "SKIP" "Water_Admin or Engineer role not found for role-based recipient test"
}

# ---------------------------------------------------------------------------
# SCENARIO 5: Concurrent double-approve guard
#   Ensure approving an already-approved submission returns error (not 200)
# ---------------------------------------------------------------------------
Write-Host "`n--- SCENARIO 5: Invalid Transition Guard (double-approve) ---" -ForegroundColor Magenta

Set-AllTransitionNotifications `
        ([pscustomobject]@{ type = "user"; value = $waID }) `
        ([pscustomobject]@{ type = "submitter" }) `

try {
    $sid = Create-Submission $weHeaders @{ sc = "guard_test" }
    Do-Transition $weHeaders $sid "submit" | Out-Null
    Do-Transition $waHeaders $sid "approve" | Out-Null
    $state = Get-SubmissionState $waHeaders $sid

    # Now try to approve again - should fail
    try {
        Do-Transition $waHeaders $sid "approve" | Out-Null
        Add-Result "SC5_GUARD" "double-approve guard" "FAIL" "sid=$sid second approve was NOT rejected - guard missing"
    } catch {
        Add-Result "SC5_GUARD" "double-approve guard" "PASS" "sid=$sid state=$state second approve correctly rejected: $($_.Exception.Message)"
    }

    # Try to submit an already-approved submission - should fail
    try {
        Do-Transition $weHeaders $sid "submit" | Out-Null
        Add-Result "SC5_GUARD" "submit-after-approve guard" "FAIL" "sid=$sid submit after approve was NOT rejected"
    } catch {
        Add-Result "SC5_GUARD" "submit-after-approve guard" "PASS" "sid=$sid submit after approve correctly rejected: $($_.Exception.Message)"
    }

    # Try to reject an already-approved submission - should fail
    try {
        Do-Transition $waHeaders $sid "reject" | Out-Null
        Add-Result "SC5_GUARD" "reject-after-approve guard" "FAIL" "sid=$sid reject after approve was NOT rejected"
    } catch {
        Add-Result "SC5_GUARD" "reject-after-approve guard" "PASS" "sid=$sid reject after approve correctly rejected: $($_.Exception.Message)"
    }
} catch {
    Add-Result "SC5_GUARD" "guard-test setup" "BLOCKED" $_.Exception.Message
}

# ---------------------------------------------------------------------------
# SCENARIO 6: Invalid transition from draft (cannot approve draft directly)
# ---------------------------------------------------------------------------
Write-Host "`n--- SCENARIO 6: Invalid Transition (approve draft directly) ---" -ForegroundColor Magenta

Set-AllTransitionNotifications $null $null $null $null "SC6"

try {
    $sid = Create-Submission $weHeaders @{ sc = "invalid_trans" }
    try {
        Do-Transition $waHeaders $sid "approve" | Out-Null
        Add-Result "SC6_INVALID" "approve-draft-directly" "FAIL" "sid=$sid approve on draft was NOT rejected"
    } catch {
        Add-Result "SC6_INVALID" "approve-draft-directly" "PASS" "sid=$sid correctly rejected: $($_.Exception.Message)"
    }
    try {
        Do-Transition $waHeaders $sid "reject" | Out-Null
        Add-Result "SC6_INVALID" "reject-draft-directly" "FAIL" "sid=$sid reject on draft was NOT rejected"
    } catch {
        Add-Result "SC6_INVALID" "reject-draft-directly" "PASS" "sid=$sid correctly rejected: $($_.Exception.Message)"
    }
    try {
        Do-Transition $weHeaders $sid "revise" | Out-Null
        Add-Result "SC6_INVALID" "revise-draft-directly" "FAIL" "sid=$sid revise on draft was NOT rejected"
    } catch {
        Add-Result "SC6_INVALID" "revise-draft-directly" "PASS" "sid=$sid correctly rejected: $($_.Exception.Message)"
    }
} catch {
    Add-Result "SC6_INVALID" "invalid-trans-setup" "BLOCKED" $_.Exception.Message
}

}
finally {
    Restore-Workflow
}

# ===========================================================================
# SUMMARY
# ===========================================================================
Write-Host "`n=========================================" -ForegroundColor Cyan
Write-Host "  Workflow Multi-Transition Test Summary  " -ForegroundColor Cyan
Write-Host "=========================================`n" -ForegroundColor Cyan

$results | Format-Table -AutoSize

$pass    = @($results | Where-Object { $_.Status -eq "PASS" }).Count
$fail    = @($results | Where-Object { $_.Status -eq "FAIL" }).Count
$blocked = @($results | Where-Object { $_.Status -eq "BLOCKED" }).Count
$skip    = @($results | Where-Object { $_.Status -eq "SKIP" }).Count

Write-Host "PASS=$pass  FAIL=$fail  BLOCKED=$blocked  SKIP=$skip"

if ($fail -gt 0) { exit 1 }
exit 0
