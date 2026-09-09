[CmdletBinding()]
param(
  [Parameter(Mandatory)]
  [ValidateSet('Preflight', 'Backup', 'VerifyRestore', 'Migrate', 'Verify', 'Rollback')]
  [string]$Action,
  [string]$PostgresContainer = 'local-dev-postgres',
  [string]$Database = 'baley',
  [string]$DatabaseUser = 'local_admin',
  [string]$BackupDirectory,
  [string]$BackupFile,
  [string]$RestoreDatabase,
  [string]$DeploySha,
  [string]$RollbackApiImage,
  [string]$RollbackViewerImage
)

$ErrorActionPreference = 'Stop'

function Assert-ExactIdentifier([string]$Value, [string]$Pattern, [string]$Label) {
  if ([string]::IsNullOrWhiteSpace($Value) -or $Value -notmatch $Pattern) {
    throw "$Label '$Value' does not match the required safe form"
  }
}

function Invoke-PsqlScalar([string]$TargetDatabase, [string]$Sql) {
  $result = docker exec $PostgresContainer psql -X -v ON_ERROR_STOP=1 -U $DatabaseUser -d $TargetDatabase -At -c $Sql
  if ($LASTEXITCODE -ne 0) { throw "psql failed for database $TargetDatabase" }
  return ($result | Out-String).Trim()
}

function Get-SchemaVersion([string]$TargetDatabase) {
  return [int](Invoke-PsqlScalar $TargetDatabase "SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1")
}

function Get-TableCounts([string]$TargetDatabase) {
  return Invoke-PsqlScalar $TargetDatabase "SELECT jsonb_build_object('workspaces',(SELECT count(*) FROM workspaces),'tasks',(SELECT count(*) FROM tasks),'commands',(SELECT count(*) FROM commands),'events',(SELECT count(*) FROM events))::text"
}

Assert-ExactIdentifier $PostgresContainer '^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$' 'container name'
Assert-ExactIdentifier $Database '^[A-Za-z][A-Za-z0-9_]{0,62}$' 'database name'

switch ($Action) {
  'Preflight' {
    $status = docker inspect $PostgresContainer --format '{{.State.Status}}'
    if ($LASTEXITCODE -ne 0 -or $status.Trim() -ne 'running') { throw 'PostgreSQL container is not running' }
    $schema = Get-SchemaVersion $Database
    if ($schema -notin 25, 27) { throw "expected schema 25 before rollout or 27 after rollout, got $schema" }
    git diff --quiet --exit-code
    if ($LASTEXITCODE -ne 0) { throw 'tracked worktree changes must be reviewed before rollout' }
    [pscustomobject]@{ action = $Action; database = $Database; schemaVersion = $schema; tableCounts = (Get-TableCounts $Database); commit = (git rev-parse HEAD).Trim() }
  }
  'Backup' {
    if ((Get-SchemaVersion $Database) -ne 25) { throw 'pre-migration backup requires schema 25' }
    if ([string]::IsNullOrWhiteSpace($BackupDirectory)) { throw 'BackupDirectory is required' }
    $resolvedParent = [IO.Path]::GetFullPath($BackupDirectory)
    if (Test-Path $resolvedParent) { throw 'BackupDirectory must be a new directory' }
    New-Item -ItemType Directory -Path $resolvedParent | Out-Null
    $remoteFile = "/tmp/baley-task184-$([guid]::NewGuid().ToString('N')).dump"
    $localFile = Join-Path $resolvedParent 'baley-schema25.dump'
    try {
      docker exec $PostgresContainer pg_dump -U $DatabaseUser -d $Database --format=custom --no-owner --no-privileges --file=$remoteFile
      if ($LASTEXITCODE -ne 0) { throw 'pg_dump failed' }
      docker cp "${PostgresContainer}:$remoteFile" $localFile
      if ($LASTEXITCODE -ne 0) { throw 'docker cp failed' }
    } finally {
      docker exec $PostgresContainer rm -f -- $remoteFile | Out-Null
    }
    $metadata = [ordered]@{ createdAt = [DateTimeOffset]::UtcNow.ToString('O'); schemaVersion = 25; database = $Database; deploySha = (git rev-parse HEAD).Trim(); sha256 = (Get-FileHash $localFile -Algorithm SHA256).Hash.ToLowerInvariant(); tableCounts = (Get-TableCounts $Database | ConvertFrom-Json) }
    $metadata | ConvertTo-Json -Depth 5 | Set-Content -Encoding utf8 (Join-Path $resolvedParent 'backup.json')
    $metadata
  }
  'VerifyRestore' {
    if (-not (Test-Path -LiteralPath $BackupFile -PathType Leaf)) { throw 'BackupFile must name an existing exact file' }
    Assert-ExactIdentifier $RestoreDatabase '^baley_task184_restore_[0-9]{14}_[a-f0-9]{8}$' 'restore database'
    $remoteFile = "/tmp/$RestoreDatabase.dump"
    try {
      docker exec $PostgresContainer createdb -U $DatabaseUser $RestoreDatabase
      if ($LASTEXITCODE -ne 0) { throw 'createdb failed' }
      docker cp $BackupFile "${PostgresContainer}:$remoteFile"
      docker exec $PostgresContainer pg_restore -U $DatabaseUser -d $RestoreDatabase --no-owner --no-privileges --exit-on-error --single-transaction $remoteFile
      if ($LASTEXITCODE -ne 0) { throw 'pg_restore failed' }
      if ((Get-SchemaVersion $RestoreDatabase) -ne 25) { throw 'restored database is not schema 25' }
      [pscustomobject]@{ action = $Action; database = $RestoreDatabase; schemaVersion = 25; tableCounts = (Get-TableCounts $RestoreDatabase) }
    } finally {
      docker exec $PostgresContainer rm -f -- $remoteFile | Out-Null
      docker exec $PostgresContainer dropdb -U $DatabaseUser --if-exists --force $RestoreDatabase | Out-Null
    }
  }
  'Migrate' {
    if ((Get-SchemaVersion $Database) -ne 25) { throw 'migration requires schema 25' }
    if ($DeploySha -notmatch '^[0-9a-f]{40}$' -or $DeploySha -ne (git rev-parse HEAD).Trim()) { throw 'DeploySha must equal the checked-out commit' }
    $env:BALEY_BUILD_COMMIT = $DeploySha
    $env:BALEY_BUILD_TIME = [DateTimeOffset]::UtcNow.ToString('O')
    docker compose run --rm --no-deps --entrypoint /app/baley-server api migrate up
    if ($LASTEXITCODE -ne 0) { throw 'one-shot migration failed' }
    if ((Get-SchemaVersion $Database) -ne 27) { throw 'migration did not reach schema 27' }
  }
  'Verify' {
    if ((Get-SchemaVersion $Database) -ne 27) { throw 'verification requires schema 27' }
    $invalid = Invoke-PsqlScalar $Database "SELECT count(*) FROM task_journal_entries j LEFT JOIN events e ON e.workspace_id=j.workspace_id AND e.id=j.event_id LEFT JOIN commands c ON c.workspace_id=j.workspace_id AND c.id=j.command_id WHERE e.id IS NULL OR c.id IS NULL OR j.recorded_at<>j.occurred_at"
    if ([int]$invalid -ne 0) { throw "$invalid invalid backfill provenance rows" }
    $task183 = Invoke-PsqlScalar $Database "SELECT count(*) FROM task_journal_entries j JOIN tasks t ON t.workspace_id=j.workspace_id AND t.id=j.task_id WHERE t.public_id=183 AND j.lifecycle_stage IN ('created','run_started','implemented','confirmed')"
    if ([int]$task183 -ne 4) { throw "Task #183 expected 4 historical rows, got $task183" }
    [pscustomobject]@{ action = $Action; schemaVersion = 27; task183Rows = [int]$task183; invalidProvenanceRows = [int]$invalid }
  }
  'Rollback' {
    if ($RollbackApiImage -notmatch '^sha256:[0-9a-f]{64}$' -or $RollbackViewerImage -notmatch '^sha256:[0-9a-f]{64}$') { throw 'rollback images must be exact sha256 image IDs' }
    docker image tag $RollbackApiImage baley-api:latest
    docker image tag $RollbackViewerImage baley-viewer:latest
    docker compose up -d --no-deps api viewer
    if ($LASTEXITCODE -ne 0) { throw 'application rollback failed' }
    Write-Warning 'Schema 27 and append-only backfilled rows remain by design; do not run migration down as an application rollback.'
  }
}
