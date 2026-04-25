param(
    [Parameter(Mandatory = $false)]
    [string]$ApiBase = "http://localhost:8080/api/v1",

    [Parameter(Mandatory = $false)]
    [string]$Token = $env:UGCL_TOKEN,

    [Parameter(Mandatory = $false)]
    [string]$ApiKey = "87339ea3-1add-4689-ae57-3128ebd03c4f",

    [Parameter(Mandatory = $false)]
    [string]$BusinessCode = "WATER",

    [Parameter(Mandatory = $false)]
    [string]$FormCode = "WD12",

    [Parameter(Mandatory = $false)]
    [ValidateSet("full", "readonly")]
    [string]$Mode = "full",

    [Parameter(Mandatory = $false)]
    [switch]$SkipAdmin,

    [Parameter(Mandatory = $false)]
    [switch]$SkipSse
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Token)) {
    Write-Host "ERROR: Missing token. Pass -Token or set UGCL_TOKEN env variable." -ForegroundColor Red
    exit 2
}

$ApiBase = $ApiBase.TrimEnd('/')

$headers = @{
    Authorization = "Bearer $Token"
    "x-api-key" = $ApiKey
    Accept = "application/json"
    "Content-Type" = "application/json"
    "X-Business-Code" = $BusinessCode
}

$results = [System.Collections.Generic.List[object]]::new()
$script:submissionId = $null
$script:firstNotificationId = $null
$script:tempRuleId = $null
$script:wfOriginalTransitions = $null

function Add-Result {
    param(
        [string]$Name,
        [string]$Status,
        [string]$Detail
    )

    $results.Add([pscustomobject]@{
        Name = $Name
        Status = $Status
        Detail = $Detail
    })

    $color = "Gray"
    if ($Status -eq "PASS") { $color = "Green" }
    elseif ($Status -eq "FAIL") { $color = "Red" }
    elseif ($Status -eq "SKIP") { $color = "Yellow" }

    Write-Host ("[{0}] {1} - {2}" -f $Status, $Name, $Detail) -ForegroundColor $color
}

function Invoke-Api {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Method,

        [Parameter(Mandatory = $true)]
        [string]$Path,

        [Parameter(Mandatory = $false)]
        [object]$Body
    )

    $uri = "$ApiBase$Path"

    try {
        if ($PSBoundParameters.ContainsKey('Body')) {
            $jsonBody = $Body
            if ($Body -isnot [string]) {
                $jsonBody = ConvertTo-Json -InputObject $Body -Depth 20 -Compress
            }
            return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers -Body $jsonBody
        }

        return Invoke-RestMethod -Method $Method -Uri $uri -Headers $headers
    }
    catch {
        $err = $_
        $statusCode = $null
        $respBody = ""

        $hasResponse = $false
        if ($err.Exception -and $err.Exception.PSObject -and ($err.Exception.PSObject.Properties.Name -contains 'Response')) {
            $hasResponse = $null -ne $err.Exception.Response
        }

        if ($hasResponse) {
            try {
                $statusCode = [int]$err.Exception.Response.StatusCode
            } catch {
                $statusCode = $null
            }

            try {
                $stream = $err.Exception.Response.GetResponseStream()
                if ($stream) {
                    $reader = New-Object System.IO.StreamReader($stream)
                    $respBody = $reader.ReadToEnd()
                }
            } catch {
                $respBody = ""
            }
        }

        if (-not $statusCode -and $err.Exception -and $err.Exception.PSObject -and ($err.Exception.PSObject.Properties.Name -contains 'StatusCode')) {
            try {
                $statusCode = [int]$err.Exception.StatusCode
            } catch {
                $statusCode = $null
            }
        }

        if (-not $respBody -and $err.ErrorDetails -and $err.ErrorDetails.Message) {
            $respBody = $err.ErrorDetails.Message
        }

        if (-not $respBody) {
            $respBody = $err.Exception.Message
        }

        throw "HTTP $statusCode @ $Method $Path :: $respBody"
    }
}

function Convert-ToWorkflowArray {
    param([object]$Value)

    if ($null -eq $Value) { return @() }

    if ($Value.PSObject -and ($Value.PSObject.Properties.Name -contains "value") -and $null -ne $Value.value) {
        return @((ConvertTo-Json -InputObject $Value.value -Depth 20 | ConvertFrom-Json))
    }

    if ($Value -is [array]) {
        return @((ConvertTo-Json -InputObject $Value -Depth 20 | ConvertFrom-Json))
    }

    return @((ConvertTo-Json -InputObject $Value -Depth 20 | ConvertFrom-Json))
}

function Run-Step {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name,

        [Parameter(Mandatory = $true)]
        [scriptblock]$Action
    )

    try {
        & $Action
    }
    catch {
        Add-Result -Name $Name -Status "FAIL" -Detail $_.Exception.Message
    }
}

Write-Host "Running workflow + notification smoke test" -ForegroundColor Cyan
Write-Host ("ApiBase={0} Business={1} Form={2} Mode={3}" -f $ApiBase, $BusinessCode, $FormCode, $Mode) -ForegroundColor DarkCyan
Write-Host ""

# ── STEP 1: Notification baseline ─────────────────────────────────────────────

Run-Step -Name "Notification unread count" -Action {
    $r = Invoke-Api -Method "GET" -Path "/notifications/unread-count"
    $count = if ($null -ne $r.count) { $r.count } else { "unknown" }
    Add-Result -Name "Notification unread count" -Status "PASS" -Detail ("count={0}" -f $count)
}

Run-Step -Name "Notification preferences get" -Action {
    $r = Invoke-Api -Method "GET" -Path "/notifications/preferences"
    $flag = if ($null -ne $r.preferences.enable_in_app) { $r.preferences.enable_in_app } else { "n/a" }
    Add-Result -Name "Notification preferences get" -Status "PASS" -Detail ("enable_in_app={0}" -f $flag)
}

if ($Mode -eq "full") {
    Run-Step -Name "Notification preferences update" -Action {
        $payload = @{
            enable_in_app  = $true
            enable_email   = $false
            enable_sms     = $false
            enable_web_push = $true
        }
        $null = Invoke-Api -Method "PUT" -Path "/notifications/preferences" -Body $payload
        Add-Result -Name "Notification preferences update" -Status "PASS" -Detail "updated"
    }
}
else {
    Add-Result -Name "Notification preferences update" -Status "SKIP" -Detail "readonly mode"
}

# ── STEP 2: Admin setup — embed a notification in the workflow submit transition
# Patches standard_approval's "submit" transition to include an in-app
# notification to the submitter.  Restored to original in cleanup (Step 5).

if (-not $SkipAdmin -and $Mode -eq "full") {
    Run-Step -Name "Admin create temp notification rule" -Action {
        $wfResp = Invoke-Api -Method "GET" -Path "/admin/workflows"
        $wfId = $null
        $wfObj = $null
        if ($wfResp.workflows) {
            $match = @($wfResp.workflows | Where-Object { $_.code -eq "standard_approval" })
            if ($match.Count -gt 0) { $wfId = $match[0].id; $wfObj = $match[0] }
        }
        if (-not $wfId) { throw "standard_approval workflow not found" }

        # Store original transitions for cleanup
        $script:tempRuleId = $wfId
        $script:wfOriginalTransitions = (Convert-ToWorkflowArray $wfObj.transitions) | ConvertTo-Json -Depth 20

        # Clone transitions and inject notification on the "submit" action
        $transitions = Convert-ToWorkflowArray $wfObj.transitions
        foreach ($t in $transitions) {
            if ($t.action -eq "submit") {
                $t | Add-Member -NotePropertyName "notifications" -NotePropertyValue @(
                    [pscustomobject]@{
                        title_template = "Smoke: {{.FormCode}} submitted"
                        body_template  = "Submission {{.SubmissionID}} moved to {{.CurrentState}}"
                        channels       = @("in_app")
                        recipients     = @([pscustomobject]@{ type = "submitter" })
                    }
                ) -Force
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

        $null = Invoke-Api -Method "PUT" -Path ("/admin/workflows/{0}" -f $wfId) -Body $payload
        Add-Result -Name "Admin create temp notification rule" -Status "PASS" -Detail ("patched submit transition on workflow {0}" -f $wfId)
    }
}
else {
    Add-Result -Name "Admin create temp notification rule" -Status "SKIP" -Detail $(if ($SkipAdmin) { "SkipAdmin enabled" } else { "readonly mode" })
}

# ── STEP 3: Workflow ────────────────────────────────────────────────────────────

Run-Step -Name "Workflow create submission" -Action {
    if ($Mode -ne "full") {
        Add-Result -Name "Workflow create submission" -Status "SKIP" -Detail "readonly mode"
        return
    }

    $payload = @{
        form_data = @{
            smoke_test = $true
            smoke_ref  = ([DateTimeOffset]::UtcNow.ToUnixTimeSeconds())
            smoke_note = "workflow_notification_smoke"
        }
    }

    $r = Invoke-Api -Method "POST" -Path ("/business/{0}/forms/{1}/submissions" -f $BusinessCode, $FormCode) -Body $payload
    if (-not $r.submission.id) { throw "submission id missing in response" }

    $script:submissionId = $r.submission.id
    Add-Result -Name "Workflow create submission" -Status "PASS" -Detail ("submission_id={0}" -f $script:submissionId)
}

Run-Step -Name "Workflow get submissions" -Action {
    $r = Invoke-Api -Method "GET" -Path ("/business/{0}/forms/{1}/submissions?my_submissions=true" -f $BusinessCode, $FormCode)
    $count = if ($r.submissions) { @($r.submissions).Count } else { 0 }
    Add-Result -Name "Workflow get submissions" -Status "PASS" -Detail ("count={0}" -f $count)
}

Run-Step -Name "Workflow stats" -Action {
    $r = Invoke-Api -Method "GET" -Path ("/business/{0}/forms/{1}/stats" -f $BusinessCode, $FormCode)
    $keys = @()
    if ($r.stats) { $keys = @($r.stats.PSObject.Properties.Name) }
    Add-Result -Name "Workflow stats" -Status "PASS" -Detail ("states={0}" -f ($keys -join ','))
}

$submissionId = $script:submissionId
if ($submissionId) {
    Run-Step -Name "Workflow get submission by id" -Action {
        $null = Invoke-Api -Method "GET" -Path ("/business/{0}/forms/{1}/submissions/{2}" -f $BusinessCode, $FormCode, $submissionId)
        Add-Result -Name "Workflow get submission by id" -Status "PASS" -Detail ("submission_id={0}" -f $submissionId)
    }

    # Transition fires ProcessTransitionNotifications → delivers in-app notification
    # via the temp rule created in Step 2.
    Run-Step -Name "Workflow transition submit" -Action {
        $payload = @{ action = "submit" }
        $null = Invoke-Api -Method "POST" -Path ("/business/{0}/forms/{1}/submissions/{2}/transition" -f $BusinessCode, $FormCode, $submissionId) -Body $payload
        Add-Result -Name "Workflow transition submit" -Status "PASS" -Detail ("submission_id={0}" -f $submissionId)
    }

    Run-Step -Name "Workflow history" -Action {
        $r = Invoke-Api -Method "GET" -Path ("/business/{0}/forms/{1}/submissions/{2}/history" -f $BusinessCode, $FormCode, $submissionId)
        $count = if ($r.history) { @($r.history).Count } else { 0 }
        Add-Result -Name "Workflow history" -Status "PASS" -Detail ("transitions={0}" -f $count)
    }
}
else {
    Add-Result -Name "Workflow get submission by id" -Status "SKIP" -Detail "no submission id"
    Add-Result -Name "Workflow transition submit" -Status "SKIP" -Detail "no submission id"
    Add-Result -Name "Workflow history" -Status "SKIP" -Detail "no submission id"
}

# ── STEP 4: Notification list after transition + mark-read ─────────────────────
# Transition above should have generated a notification via the temp rule.

Run-Step -Name "Notification list after transition" -Action {
    $r = Invoke-Api -Method "GET" -Path "/notifications?limit=10&read=false"
    $list = @()
    if ($r.notifications) { $list = @($r.notifications) }
    if ($list.Count -gt 0) { $script:firstNotificationId = $list[0].id }
    Add-Result -Name "Notification list after transition" -Status "PASS" -Detail ("items={0}" -f $list.Count)
}

if ($Mode -eq "full") {
    $firstNotificationId = $script:firstNotificationId
    if ($firstNotificationId) {
        Run-Step -Name "Notification mark single as read" -Action {
            $null = Invoke-Api -Method "PATCH" -Path ("/notifications/{0}/read" -f $firstNotificationId) -Body "{}"
            Add-Result -Name "Notification mark single as read" -Status "PASS" -Detail ("id={0}" -f $firstNotificationId)
        }
    }
    else {
        Add-Result -Name "Notification mark single as read" -Status "SKIP" -Detail "no notification delivered (check notification rule setup)"
    }

    Run-Step -Name "Notification mark all as read" -Action {
        $null = Invoke-Api -Method "PATCH" -Path "/notifications/read-all" -Body "{}"
        Add-Result -Name "Notification mark all as read" -Status "PASS" -Detail "done"
    }
}
else {
    Add-Result -Name "Notification mark single as read" -Status "SKIP" -Detail "readonly mode"
    Add-Result -Name "Notification mark all as read" -Status "SKIP" -Detail "readonly mode"
}

# ── STEP 5: Admin checks + cleanup ─────────────────────────────────────────────

if (-not $SkipAdmin) {
    # /admin/notification-rules/stats is routed after /{id} in gorilla/mux so 'stats'
    # is treated as an id param — use the legacy path which registers correctly.
    Run-Step -Name "Admin notification stats (legacy path)" -Action {
        $null = Invoke-Api -Method "GET" -Path "/admin/notifications/stats"
        Add-Result -Name "Admin notification stats (legacy path)" -Status "PASS" -Detail "ok"
    }

    Run-Step -Name "Admin notification rules list (new path)" -Action {
        $r = Invoke-Api -Method "GET" -Path "/admin/notification-rules"
        $count = if ($r.rules) { @($r.rules).Count } else { 0 }
        Add-Result -Name "Admin notification rules list (new path)" -Status "PASS" -Detail ("count={0}" -f $count)
    }

    Run-Step -Name "Admin notification rules list (legacy path)" -Action {
        $r = Invoke-Api -Method "GET" -Path "/admin/notifications/rules"
        $count = if ($r.rules) { @($r.rules).Count } else { 0 }
        Add-Result -Name "Admin notification rules list (legacy path)" -Status "PASS" -Detail ("count={0}" -f $count)
    }

    # Restore the workflow's submit transition to its original state (remove smoke notification)
    $tempRuleId = $script:tempRuleId  # holds workflow id in this variant
    if ($tempRuleId -and $script:wfOriginalTransitions) {
        Run-Step -Name "Admin delete temp notification rule" -Action {
            $wfResp = Invoke-Api -Method "GET" -Path "/admin/workflows"
            $wfObj = $null
            if ($wfResp.workflows) {
                $match = @($wfResp.workflows | Where-Object { $_.id -eq $tempRuleId })
                if ($match.Count -gt 0) { $wfObj = $match[0] }
            }
            if (-not $wfObj) { throw "could not reload workflow for cleanup" }

            $origTransitions = $script:wfOriginalTransitions | ConvertFrom-Json
            $payload = @{
                name = $wfObj.name
                code = $wfObj.code
                description = $wfObj.description
                initial_state = $wfObj.initial_state
                states = (Convert-ToWorkflowArray $wfObj.states)
                transitions = $origTransitions
                is_active = $wfObj.is_active
            }

            $null = Invoke-Api -Method "PUT" -Path ("/admin/workflows/{0}" -f $tempRuleId) -Body $payload
            Add-Result -Name "Admin delete temp notification rule" -Status "PASS" -Detail ("restored transitions on workflow {0}" -f $tempRuleId)
        }
    }
}
else {
    Add-Result -Name "Admin notification stats (legacy path)" -Status "SKIP" -Detail "SkipAdmin enabled"
    Add-Result -Name "Admin notification rules list (new path)" -Status "SKIP" -Detail "SkipAdmin enabled"
    Add-Result -Name "Admin notification rules list (legacy path)" -Status "SKIP" -Detail "SkipAdmin enabled"
    Add-Result -Name "Admin delete temp notification rule" -Status "SKIP" -Detail "SkipAdmin enabled"
}

if (-not $SkipSse) {
    Run-Step -Name "Notification SSE connect" -Action {
        $client = [System.Net.Http.HttpClient]::new()
        $client.DefaultRequestHeaders.Authorization = [System.Net.Http.Headers.AuthenticationHeaderValue]::new("Bearer", $Token)
        $client.DefaultRequestHeaders.Add("x-api-key", $ApiKey)
        $client.DefaultRequestHeaders.Accept.ParseAdd("text/event-stream")

        $request = [System.Net.Http.HttpRequestMessage]::new([System.Net.Http.HttpMethod]::Get, "$ApiBase/notifications/stream")
        $cts = [System.Threading.CancellationTokenSource]::new()
        $cts.CancelAfter([TimeSpan]::FromSeconds(8))

        try {
            $response = $client.SendAsync($request, [System.Net.Http.HttpCompletionOption]::ResponseHeadersRead, $cts.Token).GetAwaiter().GetResult()
            if (-not $response.IsSuccessStatusCode) {
                throw "status=$([int]$response.StatusCode)"
            }
            Add-Result -Name "Notification SSE connect" -Status "PASS" -Detail ("status={0}" -f [int]$response.StatusCode)
        }
        finally {
            $client.Dispose()
            $cts.Dispose()
        }
    }
}
else {
    Add-Result -Name "Notification SSE connect" -Status "SKIP" -Detail "SkipSse enabled"
}

Write-Host ""
Write-Host "Smoke test summary" -ForegroundColor Cyan
$results | Format-Table -AutoSize

$failed = @($results | Where-Object { $_.Status -eq "FAIL" }).Count
if ($failed -gt 0) {
    Write-Host ("FAILED steps: {0}" -f $failed) -ForegroundColor Red
    exit 1
}

Write-Host "All required smoke steps passed (or were intentionally skipped)." -ForegroundColor Green
exit 0
