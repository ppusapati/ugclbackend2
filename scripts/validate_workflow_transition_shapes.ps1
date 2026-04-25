param(
    [string]$ApiBase = "http://localhost:8080/api/v1",
    [string]$ApiKey  = "87339ea3-1add-4689-ae57-3128ebd03c4f"
)

$ErrorActionPreference = "Stop"

function Login-SuperAdmin {
    Invoke-RestMethod -Uri "$ApiBase/login" -Method POST -ContentType "application/json" `
        -Headers @{"x-api-key"=$ApiKey} `
        -Body '{"phone":"9999999999","password":"Welcome@123"}'
}

function Is-ArrayShape($Value) {
    if ($null -eq $Value) { return $false }
    if ($Value -is [array]) { return $true }

    # Detect PowerShell wrapper from malformed stored JSON object
    if ($Value.PSObject -and ($Value.PSObject.Properties.Name -contains "value") -and $null -ne $Value.value) {
        return $false
    }

    return $false
}

$login = Login-SuperAdmin
$headers = @{"x-api-key"=$ApiKey; "Authorization"="Bearer $($login.token)"}
$workflows = (Invoke-RestMethod -Uri "$ApiBase/admin/workflows" -Method GET -Headers $headers).workflows

$errors = New-Object System.Collections.Generic.List[string]

foreach ($wf in $workflows) {
    if (-not (Is-ArrayShape $wf.transitions)) {
        $errors.Add("workflow '$($wf.code)' has non-array transitions shape")
        continue
    }

    $idx = 0
    foreach ($t in @($wf.transitions)) {
        $idx++
        if (-not $t.action -or -not $t.from -or -not $t.to) {
            $errors.Add("workflow '$($wf.code)' transition #$idx missing one of: action/from/to")
        }

        if ($t.PSObject.Properties.Name -contains "notifications") {
            foreach ($n in @($t.notifications)) {
                if ($null -eq $n) {
                    $errors.Add("workflow '$($wf.code)' transition #$idx has null notification entry")
                    continue
                }
                if (-not $n.title_template -or -not $n.body_template) {
                    $errors.Add("workflow '$($wf.code)' transition #$idx notification missing title_template/body_template")
                }
                if (-not ($n.PSObject.Properties.Name -contains "recipients") -or @($n.recipients).Count -eq 0) {
                    $errors.Add("workflow '$($wf.code)' transition #$idx notification missing recipients")
                }
            }
        }
    }
}

if ($errors.Count -gt 0) {
    Write-Host "Workflow validation FAILED" -ForegroundColor Red
    foreach ($e in $errors) { Write-Host " - $e" -ForegroundColor Red }
    exit 1
}

Write-Host "Workflow validation PASSED: transitions are array-shaped and structurally valid" -ForegroundColor Green
exit 0
