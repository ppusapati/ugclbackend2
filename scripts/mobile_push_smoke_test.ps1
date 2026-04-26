param(
    [string]$ApiBase = "http://localhost:8080/api/v1",
    [string]$ApiKey = "87339ea3-1add-4689-ae57-3128ebd03c4f",
    [string]$Phone = "9999999999",
    [string]$Password = "Welcome@123",
    [string]$Title = "UGCL Mobile Push Smoke Test",
    [string]$Body = "Triggered from backend smoke test script.",
    [string]$ActionUrl = "/chat"
)

$ErrorActionPreference = 'Stop'

function Write-Step {
    param([string]$Message)
    Write-Host "[mobile-push-smoke] $Message"
}

$headers = @{
    'x-api-key' = $ApiKey
    'Content-Type' = 'application/json'
}

Write-Step "Logging in as $Phone"
$loginResponse = Invoke-RestMethod -Method Post -Uri "$ApiBase/login" -Headers $headers -Body (@{
    phone = $Phone
    password = $Password
} | ConvertTo-Json)

if (-not $loginResponse.token) {
    throw "Login failed: token missing from response"
}

$authHeaders = @{
    'x-api-key' = $ApiKey
    'Content-Type' = 'application/json'
    'Authorization' = "Bearer $($loginResponse.token)"
}

Write-Step "Loading registered mobile push tokens"
$tokensResponse = Invoke-RestMethod -Method Get -Uri "$ApiBase/notifications/push/mobile/tokens" -Headers $authHeaders
$tokenCount = [int]($tokensResponse.count)
Write-Step "Registered active tokens: $tokenCount"

if ($tokenCount -le 0) {
    throw "No active mobile push tokens are registered for this user. Open the mobile app, log in, and allow push permission first."
}

Write-Step "Dispatching test mobile push"
$testResponse = Invoke-RestMethod -Method Post -Uri "$ApiBase/notifications/push/mobile/test" -Headers $authHeaders -Body (@{
    title = $Title
    body = $Body
    url = $ActionUrl
} | ConvertTo-Json)

Write-Step ("Dispatch result: " + ($testResponse | ConvertTo-Json -Depth 5 -Compress))
Write-Step "Smoke test completed. Confirm the push arrives on the registered Android/iOS device."
