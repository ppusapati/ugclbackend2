param(
    [string]$ApiBase = "http://localhost:8080/api/v1",
    [string]$ApiKey  = "87339ea3-1add-4689-ae57-3128ebd03c4f"
)

$ErrorActionPreference = "Stop"

$login = Invoke-RestMethod -Uri "$ApiBase/login" -Method POST -ContentType "application/json" `
    -Headers @{"x-api-key"=$ApiKey} -Body '{"phone":"9999999999","password":"Welcome@123"}'
$h = @{"x-api-key"=$ApiKey; "Authorization"="Bearer $($login.token)"; "Content-Type"="application/json"}

$wfs = (Invoke-RestMethod -Uri "$ApiBase/admin/workflows" -Method GET -Headers $h).workflows

$std = $wfs | Where-Object { $_.code -eq "standard_approval" } | Select-Object -First 1
$ml  = $wfs | Where-Object { $_.code -eq "multi_level_approval" } | Select-Object -First 1

if (-not $std) { throw "standard_approval workflow not found" }
if (-not $ml)  { throw "multi_level_approval workflow not found" }

$stdStates = '[{"code":"draft","name":"Draft","description":"Initial draft state","color":"gray","is_final":false},{"code":"submitted","name":"Submitted","description":"Submitted for review","color":"blue","is_final":false},{"code":"approved","name":"Approved","description":"Approved by reviewer","color":"green","is_final":true},{"code":"rejected","name":"Rejected","description":"Rejected by reviewer","color":"red","is_final":true}]'
$stdTransitions = '[{"from":"draft","to":"submitted","action":"submit","label":"Submit for Review","required_permission":"","notifications":[{"title_template":"Form submitted: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) has been submitted for review.","channels":["in_app"],"recipients":[{"type":"submitter"}]}]},{"from":"submitted","to":"approved","action":"approve","label":"Approve","required_permission":"workflow:approve","notifications":[{"title_template":"Form approved: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) has been approved by {{.ApproverName}}.","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]},{"from":"submitted","to":"rejected","action":"reject","label":"Reject","required_permission":"workflow:approve","notifications":[{"title_template":"Form rejected: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) was rejected by {{.ApproverName}}. Comment: {{.Comment}}","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]},{"from":"rejected","to":"draft","action":"revise","label":"Revise","required_permission":"","notifications":[{"title_template":"Revision requested: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) has been sent back for revision. Please update and resubmit.","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]}]'

$mlStates = '[{"code":"draft","name":"Draft","description":"Initial draft state","color":"gray","is_final":false},{"code":"submitted","name":"Submitted","description":"Submitted for L1 approval","color":"blue","is_final":false},{"code":"l1_approved","name":"L1 Approved","description":"Approved by Level 1","color":"yellow","is_final":false},{"code":"l2_approved","name":"L2 Approved","description":"Final approval","color":"green","is_final":true},{"code":"rejected","name":"Rejected","description":"Rejected at any level","color":"red","is_final":true}]'
$mlTransitions = '[{"from":"draft","to":"submitted","action":"submit","label":"Submit","required_permission":"","notifications":[{"title_template":"Form submitted: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) has been submitted for L1 review.","channels":["in_app"],"recipients":[{"type":"submitter"}]}]},{"from":"submitted","to":"l1_approved","action":"l1_approve","label":"L1 Approve","required_permission":"workflow:l1_approve","notifications":[{"title_template":"L1 Approved: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) passed L1 review by {{.ApproverName}}. Pending L2 review.","channels":["in_app"],"priority":"normal","recipients":[{"type":"submitter"}]}]},{"from":"submitted","to":"rejected","action":"reject","label":"Reject","required_permission":"workflow:l1_approve","notifications":[{"title_template":"Form rejected: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) was rejected by {{.ApproverName}}. Comment: {{.Comment}}","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]},{"from":"l1_approved","to":"l2_approved","action":"l2_approve","label":"L2 Approve","required_permission":"workflow:l2_approve","notifications":[{"title_template":"Form fully approved: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) has been fully approved by {{.ApproverName}}.","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]},{"from":"l1_approved","to":"rejected","action":"reject","label":"Reject","required_permission":"workflow:l2_approve","notifications":[{"title_template":"Form rejected: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) was rejected at L2 by {{.ApproverName}}. Comment: {{.Comment}}","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]},{"from":"rejected","to":"draft","action":"revise","label":"Revise","required_permission":"","notifications":[{"title_template":"Revision requested: {{.FormCode}}","body_template":"Your {{.FormCode}} submission ({{.SubmissionID}}) has been sent back for revision. Please update and resubmit.","channels":["in_app"],"priority":"high","recipients":[{"type":"submitter"}]}]}]'

$stdBody = @"
{
  "name": "$($std.name)",
  "code": "$($std.code)",
  "description": "$($std.description)",
  "initial_state": "$($std.initial_state)",
  "states": $stdStates,
  "transitions": $stdTransitions,
  "is_active": true
}
"@

$mlBody = @"
{
  "name": "$($ml.name)",
  "code": "$($ml.code)",
  "description": "$($ml.description)",
  "initial_state": "$($ml.initial_state)",
  "states": $mlStates,
  "transitions": $mlTransitions,
  "is_active": true
}
"@

Invoke-RestMethod -Uri "$ApiBase/admin/workflows/$($std.id)" -Method PUT -Headers $h -Body $stdBody | Out-Null
Write-Host "Patched standard_approval with array transitions"

Invoke-RestMethod -Uri "$ApiBase/admin/workflows/$($ml.id)" -Method PUT -Headers $h -Body $mlBody | Out-Null
Write-Host "Patched multi_level_approval with array transitions"
