param(
    [string]$ApiBase = "http://localhost:8080/api/v1",
    [string]$ApiKey  = "87339ea3-1add-4689-ae57-3128ebd03c4f"
)
$ErrorActionPreference = "Stop"

$loginResp = Invoke-RestMethod -Uri "$ApiBase/login" -Method POST -ContentType "application/json" -Headers @{"x-api-key"=$ApiKey} -Body '{"phone":"9999999999","password":"Welcome@123"}'
$tok = $loginResp.token
$H = @{"x-api-key"=$ApiKey; "Authorization"="Bearer $tok"; "Content-Type"="application/json"}
Write-Host "Logged in"

$workflows = (Invoke-RestMethod -Uri "$ApiBase/admin/workflows" -Method GET -Headers $H).workflows
Write-Host "Found $($workflows.Count) workflows"

function Convert-ToWorkflowArray([object]$Value) {
    if ($null -eq $Value) { return @() }

    if ($Value.PSObject -and ($Value.PSObject.Properties.Name -contains "value") -and $null -ne $Value.value) {
        return @((ConvertTo-Json -InputObject $Value.value -Depth 10 | ConvertFrom-Json))
    }

    if ($Value -is [array]) {
        return @((ConvertTo-Json -InputObject $Value -Depth 10 | ConvertFrom-Json))
    }

    return @((ConvertTo-Json -InputObject $Value -Depth 10 | ConvertFrom-Json))
}

$stdJson = '[{"from":"draft","to":"submitted","action":"submit","label":"Submit for Review","required_permission":"","notifications":[{"title_template":"Form submitted: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) has been submitted for review.","channels":["in_app"],"recipients":[{"type":"submitter"}]}]},{"from":"submitted","to":"approved","action":"approve","label":"Approve","required_permission":"workflow:approve","notifications":[{"title_template":"Form approved: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) has been approved by {{.ApproverName}}.","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]},{"from":"submitted","to":"rejected","action":"reject","label":"Reject","required_permission":"workflow:approve","notifications":[{"title_template":"Form rejected: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) was rejected by {{.ApproverName}}. Comment: {{.Comment}}","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]},{"from":"rejected","to":"draft","action":"revise","label":"Revise","required_permission":"","notifications":[{"title_template":"Revision requested: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) has been sent back for revision. Please update and resubmit.","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]}]'

$multiJson = '[{"from":"draft","to":"submitted","action":"submit","label":"Submit","required_permission":"","notifications":[{"title_template":"Form submitted: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) has been submitted for L1 review.","channels":["in_app"],"recipients":[{"type":"submitter"}]}]},{"from":"submitted","to":"l1_approved","action":"l1_approve","label":"L1 Approve","required_permission":"workflow:l1_approve","notifications":[{"title_template":"L1 Approved: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) passed L1 review by {{.ApproverName}}. Pending L2.","channels":["in_app"],"priority":"normal","recipients":[{"type":"submitter"}]}]},{"from":"submitted","to":"rejected","action":"reject","label":"Reject","required_permission":"workflow:l1_approve","notifications":[{"title_template":"Form rejected: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) was rejected by {{.ApproverName}}. Comment: {{.Comment}}","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]},{"from":"l1_approved","to":"l2_approved","action":"l2_approve","label":"L2 Approve","required_permission":"workflow:l2_approve","notifications":[{"title_template":"Form fully approved: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) has been fully approved by {{.ApproverName}}.","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]},{"from":"l1_approved","to":"rejected","action":"reject","label":"Reject","required_permission":"workflow:l2_approve","notifications":[{"title_template":"Form rejected: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) was rejected at L2 by {{.ApproverName}}. Comment: {{.Comment}}","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]},{"from":"rejected","to":"draft","action":"revise","label":"Revise","required_permission":"","notifications":[{"title_template":"Revision requested: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) has been sent back for revision.","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]}]'

$patchMap = @{ "standard_approval"=$stdJson; "multi_level_approval"=$multiJson }

foreach ($wf in $workflows) {
    $newJson = $patchMap[$wf.code]
    if ($null -eq $newJson) { Write-Host "Skip $($wf.code)"; continue }
    Write-Host "Patching $($wf.name) ..."
    $bodyObj = @{
        name = $wf.name
        code = $wf.code
        description = $wf.description
        initial_state = $wf.initial_state
        states = (Convert-ToWorkflowArray $wf.states)
        transitions = ($newJson | ConvertFrom-Json)
        is_active = $wf.is_active
    }
    $result = Invoke-RestMethod -Uri "$ApiBase/admin/workflows/$($wf.id)" -Method PUT -Headers $H -Body ($bodyObj|ConvertTo-Json -Depth 10)
    Write-Host "  -> $($result.message)"
}
Write-Host "Done."
