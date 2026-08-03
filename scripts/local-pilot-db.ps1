[CmdletBinding()]
param(
  [Parameter(Position = 0)]
  [ValidateSet("status", "backup", "verify")]
  [string]$Action = "status",
  [Parameter(Position = 1)]
  [string]$BackupPath
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$composeFile = Join-Path $repoRoot "docker-compose.yml"
$defaultBackupRoot = Join-Path $repoRoot ".tmp\local-pilot\backups"

function Invoke-DockerText {
  param([string[]]$Arguments)
  $output = @(& docker @Arguments 2>&1)
  if ($LASTEXITCODE -ne 0) {
    throw "docker command failed: $($output -join [Environment]::NewLine)"
  }
  return ($output -join [Environment]::NewLine).Trim()
}

function Get-ContainerID {
  $containerID = Invoke-DockerText @("compose", "-f", $composeFile, "ps", "-q", "postgres")
  if ([string]::IsNullOrWhiteSpace($containerID)) { throw "local PostgreSQL container is not running" }
  $health = Invoke-DockerText @("inspect", "--format", "{{.State.Health.Status}}", $containerID)
  if ($health -ne "healthy") { throw "local PostgreSQL is not healthy: $health" }
  return $containerID
}

function Invoke-DatabaseScalar {
  param([string]$Database, [string]$SQL)
  return Invoke-DockerText @("compose", "-f", $composeFile, "exec", "-T", "postgres", "psql", "-U", "baley", "-d", $Database, "-At", "-v", "ON_ERROR_STOP=1", "-c", $SQL)
}

function Get-DatabaseSummary {
  param([string]$Database = "baley")
  $values = (Invoke-DatabaseScalar $Database "SELECT (SELECT count(*) FROM workspaces),(SELECT count(*) FROM tasks),(SELECT count(*) FROM events),(SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1);").Split("|")
  if ($values.Count -ne 4) { throw "unexpected database summary" }
  return [pscustomobject]@{
    Database = $Database
    Workspaces = [int64]$values[0]
    Tasks = [int64]$values[1]
    Events = [int64]$values[2]
    SchemaVersion = [int64]$values[3]
  }
}

function New-Backup {
  $containerID = Get-ContainerID
  $target = if ([string]::IsNullOrWhiteSpace($BackupPath)) {
    Join-Path $defaultBackupRoot ("baley-" + (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ"))
  } else {
    [IO.Path]::GetFullPath($BackupPath)
  }
  if (Test-Path -LiteralPath $target) { throw "backup target already exists: $target" }
  New-Item -ItemType Directory -Path $target | Out-Null

  $remoteDump = "/tmp/baley-local-backup-$([guid]::NewGuid().ToString('N')).dump"
  $dumpPath = Join-Path $target "database.dump"
  try {
    Invoke-DockerText @("compose", "-f", $composeFile, "exec", "-T", "postgres", "pg_dump", "-U", "baley", "-d", "baley", "--format=custom", "--no-owner", "--no-privileges", "--file=$remoteDump") | Out-Null
    Invoke-DockerText @("cp", "${containerID}:$remoteDump", $dumpPath) | Out-Null
  } finally {
    try { Invoke-DockerText @("compose", "-f", $composeFile, "exec", "-T", "postgres", "rm", "-f", "--", $remoteDump) | Out-Null } catch {}
  }

  $summary = Get-DatabaseSummary
  $hash = (Get-FileHash -LiteralPath $dumpPath -Algorithm SHA256).Hash.ToLowerInvariant()
  $commit = (& git -C $repoRoot rev-parse HEAD).Trim()
  @{
    createdAt = (Get-Date).ToUniversalTime().ToString("o")
    gitCommit = $commit
    sha256 = $hash
    database = $summary.Database
    workspaces = $summary.Workspaces
    tasks = $summary.Tasks
    events = $summary.Events
    schemaVersion = $summary.SchemaVersion
  } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $target "backup.json") -Encoding utf8
  [IO.File]::WriteAllText((Join-Path $target "SHA256SUMS"), "$hash  database.dump`n", [Text.Encoding]::ASCII)
  Write-Output $target
}

function Verify-Backup {
  if ([string]::IsNullOrWhiteSpace($BackupPath)) { throw "verify requires a backup directory" }
  $backupDirectory = [IO.Path]::GetFullPath($BackupPath)
  $dumpPath = Join-Path $backupDirectory "database.dump"
  $metadataPath = Join-Path $backupDirectory "backup.json"
  if (!(Test-Path -LiteralPath $dumpPath -PathType Leaf) -or !(Test-Path -LiteralPath $metadataPath -PathType Leaf)) {
    throw "backup directory is incomplete: $backupDirectory"
  }
  $metadata = Get-Content -Raw -LiteralPath $metadataPath | ConvertFrom-Json
  $actualHash = (Get-FileHash -LiteralPath $dumpPath -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($actualHash -ne [string]$metadata.sha256) { throw "backup checksum mismatch" }

  $containerID = Get-ContainerID
  $verifyDatabase = "baley_restore_verify_$((Get-Date).ToUniversalTime().ToString('yyyyMMddHHmmss'))_$([guid]::NewGuid().ToString('N').Substring(0,8))"
  if ($verifyDatabase -notmatch "^baley_restore_verify_[0-9]{14}_[0-9a-f]{8}$") { throw "unsafe verification database name" }
  $remoteDump = "/tmp/$verifyDatabase.dump"
  $created = $false
  try {
    Invoke-DockerText @("cp", $dumpPath, "${containerID}:$remoteDump") | Out-Null
    Invoke-DockerText @("compose", "-f", $composeFile, "exec", "-T", "postgres", "createdb", "-U", "baley", $verifyDatabase) | Out-Null
    $created = $true
    Invoke-DockerText @("compose", "-f", $composeFile, "exec", "-T", "postgres", "pg_restore", "-U", "baley", "-d", $verifyDatabase, "--no-owner", "--no-privileges", "--exit-on-error", "--single-transaction", $remoteDump) | Out-Null
    $summary = Get-DatabaseSummary $verifyDatabase
    if ($summary.Workspaces -ne [int64]$metadata.workspaces -or $summary.Tasks -ne [int64]$metadata.tasks -or $summary.Events -ne [int64]$metadata.events -or $summary.SchemaVersion -ne [int64]$metadata.schemaVersion) {
      throw "restored database summary does not match backup metadata"
    }
    @{
      verifiedAt = (Get-Date).ToUniversalTime().ToString("o")
      sha256 = $actualHash
      workspaces = $summary.Workspaces
      tasks = $summary.Tasks
      events = $summary.Events
      schemaVersion = $summary.SchemaVersion
    } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $backupDirectory "verification.json") -Encoding utf8
    Write-Output "Verified isolated restore: $($summary.Workspaces) Workspaces, $($summary.Tasks) Tasks, $($summary.Events) Events, schema $($summary.SchemaVersion)"
  } finally {
    if ($created) {
      try { Invoke-DockerText @("compose", "-f", $composeFile, "exec", "-T", "postgres", "dropdb", "--if-exists", "--force", "-U", "baley", $verifyDatabase) | Out-Null } catch {}
    }
    try { Invoke-DockerText @("compose", "-f", $composeFile, "exec", "-T", "postgres", "rm", "-f", "--", $remoteDump) | Out-Null } catch {}
  }
}

switch ($Action) {
  "status" { Get-ContainerID | Out-Null; Get-DatabaseSummary }
  "backup" { New-Backup }
  "verify" { Verify-Backup }
}
