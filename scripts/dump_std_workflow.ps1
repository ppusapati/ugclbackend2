$api='http://localhost:8080/api/v1'
$key='87339ea3-1add-4689-ae57-3128ebd03c4f'
$login = Invoke-RestMethod -Uri "$api/login" -Method POST -ContentType 'application/json' -Headers @{ 'x-api-key'=$key } -Body '{"phone":"9999999999","password":"Welcome@123"}'
$h=@{'x-api-key'=$key; 'Authorization'="Bearer $($login.token)"}
$wfs=(Invoke-RestMethod -Uri "$api/admin/workflows" -Headers $h).workflows
$std=$wfs | Where-Object { $_.code -eq 'standard_approval' }
$std | ConvertTo-Json -Depth 20
