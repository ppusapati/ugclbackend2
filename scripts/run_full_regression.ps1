param(
    [switch]$AutoRepairWorkflowShape
)

$ErrorActionPreference = "Stop"
Set-Location "E:\Maheshwari\UGCL\backend\v1"

function Run-Step {
    param(
        [string]$Name,
        [scriptblock]$Action
    )

    Write-Host "`n=== $Name ===" -ForegroundColor Cyan
    try {
        & $Action
        Write-Host "[PASS] $Name" -ForegroundColor Green
        return $true
    } catch {
        Write-Host "[FAIL] $Name :: $($_.Exception.Message)" -ForegroundColor Red
        return $false
    }
}

$allOk = $true

$ok = Run-Step -Name "Preflight workflow shape validation" -Action {
    powershell -ExecutionPolicy Bypass -File ".\scripts\validate_workflow_transition_shapes.ps1"
    if ($LASTEXITCODE -ne 0) { throw "validate_workflow_transition_shapes.ps1 exited $LASTEXITCODE" }
}

if (-not $ok -and $AutoRepairWorkflowShape) {
    $okRepair = Run-Step -Name "Auto-repair workflow transitions JSON" -Action {
        powershell -ExecutionPolicy Bypass -File ".\scripts\repair_workflow_transitions_json.ps1"
        if ($LASTEXITCODE -ne 0) { throw "repair_workflow_transitions_json.ps1 exited $LASTEXITCODE" }
    }
    if (-not $okRepair) { $allOk = $false }

    $okRevalidate = Run-Step -Name "Revalidate workflow shape after repair" -Action {
        powershell -ExecutionPolicy Bypass -File ".\scripts\validate_workflow_transition_shapes.ps1"
        if ($LASTEXITCODE -ne 0) { throw "validate_workflow_transition_shapes.ps1 exited $LASTEXITCODE after repair" }
    }
    if (-not $okRevalidate) { $allOk = $false }
} elseif (-not $ok) {
    $allOk = $false
}

$okAuth = Run-Step -Name "Auth exhaustive regression" -Action {
    powershell -ExecutionPolicy Bypass -File ".\scripts\auth_exhaustive_test.ps1" *>&1 | Tee-Object -FilePath ".\tmp\auth-test-latest.txt"
    if ($LASTEXITCODE -ne 0) { throw "auth_exhaustive_test.ps1 exited $LASTEXITCODE" }
}
if (-not $okAuth) { $allOk = $false }

$okWorkflow = Run-Step -Name "Workflow transition + notification regression" -Action {
    powershell -ExecutionPolicy Bypass -File ".\scripts\workflow_transition_notification_test.ps1" *>&1 | Tee-Object -FilePath ".\tmp\workflow-notif-test-latest.txt"
    if ($LASTEXITCODE -ne 0) { throw "workflow_transition_notification_test.ps1 exited $LASTEXITCODE" }
}
if (-not $okWorkflow) { $allOk = $false }

$okPost = Run-Step -Name "Post-run workflow shape validation" -Action {
    powershell -ExecutionPolicy Bypass -File ".\scripts\validate_workflow_transition_shapes.ps1"
    if ($LASTEXITCODE -ne 0) { throw "workflow transitions shape is invalid after regression run" }
}
if (-not $okPost) { $allOk = $false }

Write-Host "`n=== Regression Summary ===" -ForegroundColor Cyan
if ($allOk) {
    Write-Host "All checks passed." -ForegroundColor Green
    exit 0
}

Write-Host "One or more checks failed." -ForegroundColor Red
exit 1
