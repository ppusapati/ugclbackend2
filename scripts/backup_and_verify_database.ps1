[CmdletBinding()]
param(
    [string]$PostgresBin = "",
    [string]$BackupDirectory = "",
    [string]$SourceDsnEnv = "DB_DSN",
    [string]$RestoreDsnEnv = "RESTORE_DB_DSN"
)

$ErrorActionPreference = "Stop"

function Import-DotEnvValue {
    param([string]$Name)

    if (Test-Path "env:$Name") {
        return (Get-Item "env:$Name").Value
    }

    if (-not (Test-Path ".env")) {
        return ""
    }

    $line = Get-Content ".env" |
        Where-Object { $_ -match "^$([regex]::Escape($Name))=" } |
        Select-Object -First 1
    if (-not $line) {
        return ""
    }

    return $line.Substring($line.IndexOf("=") + 1).Trim().Trim('"').Trim("'")
}

function ConvertFrom-PostgresDsn {
    param([Parameter(Mandatory = $true)][string]$Dsn)

    $result = @{}
    if ($Dsn -match '^postgres(ql)?://') {
        $uri = [Uri]$Dsn
        $userInfo = $uri.UserInfo.Split(':', 2)
        $result.user = [Uri]::UnescapeDataString($userInfo[0])
        if ($userInfo.Count -gt 1) {
            $result.password = [Uri]::UnescapeDataString($userInfo[1])
        }
        $result.host = $uri.Host
        $result.port = if ($uri.IsDefaultPort) { "5432" } else { [string]$uri.Port }
        $result.dbname = $uri.AbsolutePath.TrimStart('/')

        if ($uri.Query) {
            foreach ($part in $uri.Query.TrimStart('?').Split('&')) {
                $pair = $part.Split('=', 2)
                if ($pair.Count -eq 2) {
                    $result[[Uri]::UnescapeDataString($pair[0])] = [Uri]::UnescapeDataString($pair[1])
                }
            }
        }
        return $result
    }

    $pattern = '(?<key>[A-Za-z_][A-Za-z0-9_]*)\s*=\s*(?:"(?<double>[^"]*)"|''(?<single>[^'']*)''|(?<plain>[^\s]+))'
    foreach ($match in [regex]::Matches($Dsn, $pattern)) {
        $value = if ($match.Groups['double'].Success) {
            $match.Groups['double'].Value
        } elseif ($match.Groups['single'].Success) {
            $match.Groups['single'].Value
        } else {
            $match.Groups['plain'].Value
        }
        $result[$match.Groups['key'].Value] = $value
    }
    return $result
}

function Resolve-PostgresTool {
    param([Parameter(Mandatory = $true)][string]$Name)

    if ($PostgresBin) {
        $candidate = Join-Path $PostgresBin "$Name.exe"
        if (Test-Path $candidate) {
            return $candidate
        }
    }

    $command = Get-Command $Name -ErrorAction SilentlyContinue
    if ($command) {
        return $command.Source
    }

    throw "$Name was not found. Install PostgreSQL client tools or pass -PostgresBin. No backup or migration was run."
}

function Assert-ConnectionFields {
    param([hashtable]$Connection, [string]$Label)

    foreach ($required in @('host', 'dbname', 'user')) {
        if (-not $Connection.ContainsKey($required) -or [string]::IsNullOrWhiteSpace($Connection[$required])) {
            throw "$Label DSN does not contain $required"
        }
    }
    if (-not $Connection.ContainsKey('port')) {
        $Connection.port = "5432"
    }
}

function Escape-PgPassValue {
    param([string]$Value)
    return $Value.Replace('\', '\\').Replace(':', '\:')
}

function Add-ServiceSection {
    param([System.Text.StringBuilder]$Builder, [string]$Name, [hashtable]$Connection)

    [void]$Builder.AppendLine("[$Name]")
    foreach ($key in @('host', 'port', 'dbname', 'user', 'sslmode', 'sslrootcert', 'sslcert', 'sslkey', 'connect_timeout')) {
        if ($Connection.ContainsKey($key) -and -not [string]::IsNullOrWhiteSpace($Connection[$key])) {
            [void]$Builder.AppendLine("$key=$($Connection[$key])")
        }
    }
    [void]$Builder.AppendLine()
}

$sourceDsn = Import-DotEnvValue $SourceDsnEnv
$restoreDsn = Import-DotEnvValue $RestoreDsnEnv
if ([string]::IsNullOrWhiteSpace($sourceDsn)) {
    throw "$SourceDsnEnv is not configured"
}
if ([string]::IsNullOrWhiteSpace($restoreDsn)) {
    throw "$RestoreDsnEnv is required and must point to a disposable restore database"
}

$source = ConvertFrom-PostgresDsn $sourceDsn
$restore = ConvertFrom-PostgresDsn $restoreDsn
Assert-ConnectionFields $source "Source"
Assert-ConnectionFields $restore "Restore"

$sameDatabase = $source.host -eq $restore.host -and
    $source.port -eq $restore.port -and
    $source.dbname -eq $restore.dbname
if ($sameDatabase) {
    throw "Restore database must differ from the source database"
}

$pgDump = Resolve-PostgresTool "pg_dump"
$pgRestore = Resolve-PostgresTool "pg_restore"

if ([string]::IsNullOrWhiteSpace($BackupDirectory)) {
    $ugclRoot = Split-Path (Split-Path $PSScriptRoot -Parent) -Parent
    $BackupDirectory = Join-Path $ugclRoot "backups\database"
}
New-Item -ItemType Directory -Force -Path $BackupDirectory | Out-Null

$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$backupPath = Join-Path $BackupDirectory "ugcl-$timestamp.dump"
$checksumPath = "$backupPath.sha256"
$sourceAuditPath = Join-Path $BackupDirectory "ugcl-$timestamp-source-rbac.json"
$restoreAuditPath = Join-Path $BackupDirectory "ugcl-$timestamp-restore-rbac.json"

$serviceFile = Join-Path $env:TEMP "ugcl-pg-service-$([guid]::NewGuid()).conf"
$passFile = Join-Path $env:TEMP "ugcl-pg-pass-$([guid]::NewGuid()).conf"

try {
    $serviceBuilder = [System.Text.StringBuilder]::new()
    Add-ServiceSection $serviceBuilder "ugcl_source" $source
    Add-ServiceSection $serviceBuilder "ugcl_restore" $restore
    [System.IO.File]::WriteAllText($serviceFile, $serviceBuilder.ToString())

    $sourcePassword = if ($source.ContainsKey('password')) { $source.password } else { "" }
    $restorePassword = if ($restore.ContainsKey('password')) { $restore.password } else { "" }
    $passLines = @(
        "$(Escape-PgPassValue $source.host):$(Escape-PgPassValue $source.port):$(Escape-PgPassValue $source.dbname):$(Escape-PgPassValue $source.user):$(Escape-PgPassValue $sourcePassword)",
        "$(Escape-PgPassValue $restore.host):$(Escape-PgPassValue $restore.port):$(Escape-PgPassValue $restore.dbname):$(Escape-PgPassValue $restore.user):$(Escape-PgPassValue $restorePassword)"
    )
    [System.IO.File]::WriteAllLines($passFile, $passLines)

    $env:PGSERVICEFILE = $serviceFile
    $env:PGPASSFILE = $passFile

    & $pgDump --dbname=service=ugcl_source --format=custom --no-owner --no-privileges --file=$backupPath
    if ($LASTEXITCODE -ne 0 -or -not (Test-Path $backupPath)) {
        throw "pg_dump failed; no migration may proceed"
    }

    $hash = (Get-FileHash -Algorithm SHA256 $backupPath).Hash.ToLowerInvariant()
    "$hash  $([System.IO.Path]::GetFileName($backupPath))" | Set-Content -Path $checksumPath -Encoding ascii

    & $pgRestore --clean --if-exists --no-owner --no-privileges --dbname=service=ugcl_restore $backupPath
    if ($LASTEXITCODE -ne 0) {
        throw "pg_restore failed; backup is not verified and no migration may proceed"
    }

    [Environment]::SetEnvironmentVariable($SourceDsnEnv, $sourceDsn, "Process")
    [Environment]::SetEnvironmentVariable($RestoreDsnEnv, $restoreDsn, "Process")
    go run ./scripts/rbac-audit -dsn-env $SourceDsnEnv -output $sourceAuditPath
    if ($LASTEXITCODE -ne 0) {
        throw "source RBAC audit failed"
    }
    go run ./scripts/rbac-audit -dsn-env $RestoreDsnEnv -output $restoreAuditPath
    if ($LASTEXITCODE -ne 0) {
        throw "restore RBAC audit failed"
    }

    $sourceAudit = Get-Content $sourceAuditPath -Raw | ConvertFrom-Json
    $restoreAudit = Get-Content $restoreAuditPath -Raw | ConvertFrom-Json
    foreach ($property in $sourceAudit.tables.PSObject.Properties) {
        $name = $property.Name
        $sourceTable = $property.Value
        $restoreTable = $restoreAudit.tables.$name
        if ($null -eq $restoreTable -or
            $sourceTable.count -ne $restoreTable.count -or
            $sourceTable.checksum -ne $restoreTable.checksum) {
            throw "restore verification failed for RBAC table $name"
        }
    }

    Write-Host "Verified backup: $backupPath"
    Write-Host "SHA-256: $hash"
    Write-Host "Restore audit matches the source RBAC audit"
} finally {
    Remove-Item Env:PGSERVICEFILE -ErrorAction SilentlyContinue
    Remove-Item Env:PGPASSFILE -ErrorAction SilentlyContinue
    Remove-Item $serviceFile -Force -ErrorAction SilentlyContinue
    Remove-Item $passFile -Force -ErrorAction SilentlyContinue
}