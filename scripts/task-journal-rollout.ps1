[CmdletBinding()]
param(
  [Parameter(Mandatory)]
  [ValidateSet('Preflight', 'Backup', 'VerifyRestore', 'Migrate', 'Verify', 'Rollback')]
  [string]$Action,
  [string]$PostgresContainer = 'local-dev-postgres',
  [string]$Database = 'baley',
  [string]$DatabaseUser = 'local_admin',
  [string]$WorkspaceId,
  [string]$BackupDirectory,
  [string]$BackupFile,
  [string]$BackupMetadataFile,
  [string]$RestoreDatabase,
  [string]$DeploySha,
  [string]$RollbackApiImage,
  [string]$RollbackViewerImage,
  [string]$ApiBaseUrl = 'http://127.0.0.1:8080',
  [string]$ViewerBaseUrl = 'http://127.0.0.1:5174'
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

function Assert-ExactIdentifier([string]$Value, [string]$Pattern, [string]$Label) {
  if ([string]::IsNullOrWhiteSpace($Value) -or $Value -notmatch $Pattern) {
    throw "$Label '$Value' does not match the required safe form"
  }
}

function Assert-ExistingFile([string]$Path, [string]$Label) {
  if ([string]::IsNullOrWhiteSpace($Path) -or -not (Test-Path -LiteralPath $Path -PathType Leaf)) {
    throw "$Label must name an existing exact file"
  }
}

function Assert-WorkspaceId {
  Assert-ExactIdentifier $WorkspaceId '^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$' 'WorkspaceId'
}

function Invoke-PsqlScalar([string]$TargetDatabase, [string]$Sql) {
  $result = docker exec $PostgresContainer psql -X -v ON_ERROR_STOP=1 -U $DatabaseUser -d $TargetDatabase -At -c $Sql
  if ($LASTEXITCODE -ne 0) { throw "psql failed for database $TargetDatabase" }
  return ($result | Out-String).Trim()
}

function Get-SchemaVersion([string]$TargetDatabase) {
  return [int](Invoke-PsqlScalar $TargetDatabase "SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1")
}

function Get-AllTableCounts([string]$TargetDatabase) {
  $tableOutput = Invoke-PsqlScalar $TargetDatabase "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname='public' ORDER BY tablename"
  $tableNames = @($tableOutput -split "`r?`n" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
  if ($tableNames.Count -eq 0) { throw 'no public tables found while collecting backup metadata' }
  $counts = [ordered]@{}
  foreach ($tableName in $tableNames) {
    Assert-ExactIdentifier $tableName '^[a-z][a-z0-9_]{0,62}$' 'database table'
    $counts[$tableName] = [int64](Invoke-PsqlScalar $TargetDatabase "SELECT count(*) FROM public.`"$tableName`"")
  }
  return [pscustomobject]$counts
}

function Get-IntegritySnapshot([string]$TargetDatabase) {
  Assert-WorkspaceId
  $sql = @'
WITH selected_tasks AS (
  SELECT id, public_id, status
  FROM tasks
  WHERE workspace_id='__WORKSPACE_ID__' AND public_id IN (183,184)
), selected_events AS (
  SELECT e.id, e.command_id
  FROM events e
  WHERE e.workspace_id='__WORKSPACE_ID__'
    AND (
      (e.entity_type='task' AND e.entity_id IN (SELECT id FROM selected_tasks))
      OR (e.entity_type='run' AND e.entity_id IN (
        SELECT r.id FROM runs r WHERE r.workspace_id='__WORKSPACE_ID__' AND r.task_id IN (SELECT id FROM selected_tasks)
      ))
    )
)
SELECT jsonb_build_object(
  'workspace', (SELECT jsonb_build_object('id',w.id,'revision',w.revision) FROM workspaces w WHERE w.id='__WORKSPACE_ID__'),
  'tasks', COALESCE((SELECT jsonb_agg(jsonb_build_object('id',id,'publicId',public_id,'status',status) ORDER BY public_id) FROM selected_tasks),'[]'::jsonb),
  'eventIds', COALESCE((SELECT jsonb_agg(id ORDER BY id) FROM selected_events),'[]'::jsonb),
  'commandIds', COALESCE((SELECT jsonb_agg(command_id ORDER BY command_id) FROM (SELECT DISTINCT command_id FROM selected_events) c),'[]'::jsonb),
  'approvalIds', COALESCE((SELECT jsonb_agg(a.id ORDER BY a.id) FROM human_approval_attestations a WHERE a.workspace_id='__WORKSPACE_ID__' AND (a.entity_id IN (SELECT id FROM selected_tasks) OR a.executed_command_id IN (SELECT command_id FROM selected_events))),'[]'::jsonb)
)::text
'@
  $snapshot = (Invoke-PsqlScalar $TargetDatabase ($sql.Replace('__WORKSPACE_ID__', $WorkspaceId))) | ConvertFrom-Json
  if ($null -eq $snapshot.workspace) { throw "Workspace $WorkspaceId was not found" }
  if (@($snapshot.tasks).Count -ne 2 -or @($snapshot.tasks.publicId | Sort-Object) -join ',' -ne '183,184') {
    throw 'backup metadata requires exactly Task #183 and Task #184 in the selected Workspace'
  }
  if (@($snapshot.eventIds).Count -eq 0 -or @($snapshot.commandIds).Count -eq 0 -or @($snapshot.approvalIds).Count -eq 0) {
    throw 'backup metadata requires fixed Event, command, and approval IDs for Tasks #183/#184'
  }
  return $snapshot
}

function ConvertTo-ComparisonJson($Value) {
  return ($Value | ConvertTo-Json -Depth 20 -Compress)
}

function Assert-SameJson($Expected, $Actual, [string]$Label) {
  if ((ConvertTo-ComparisonJson $Expected) -cne (ConvertTo-ComparisonJson $Actual)) {
    throw "restored $Label does not match backup metadata"
  }
}

function Read-BackupMetadata {
  Assert-ExistingFile $BackupMetadataFile 'BackupMetadataFile'
  try { $metadata = Get-Content -Raw -LiteralPath $BackupMetadataFile | ConvertFrom-Json } catch { throw "BackupMetadataFile is not valid JSON: $($_.Exception.Message)" }
  foreach ($required in @('formatVersion','schemaVersion','workspaceId','workspaceRevision','dumpSha256','tableCounts','taskIdentity','eventIds','commandIds','approvalIds')) {
    if ($metadata.PSObject.Properties.Name -notcontains $required) { throw "BackupMetadataFile is missing $required" }
  }
  if ([int]$metadata.formatVersion -ne 2 -or [int]$metadata.schemaVersion -ne 25) { throw 'BackupMetadataFile must use format 2 for schema 25' }
  if ([string]$metadata.workspaceId -ne $WorkspaceId) { throw 'BackupMetadataFile Workspace does not match WorkspaceId' }
  return $metadata
}

function Get-ImageLabel([string]$Image, [string]$Label) {
  $rawLabels = docker image inspect $Image --format '{{json .Config.Labels}}'
  if ($LASTEXITCODE -ne 0) { throw "could not inspect image $Image" }
  try { $labels = ($rawLabels | Out-String).Trim() | ConvertFrom-Json } catch { throw "image $Image has invalid label metadata" }
  if ($null -eq $labels) { return '' }
  $property = $labels.PSObject.Properties[$Label]
  if ($null -eq $property) { return '' }
  return [string]$property.Value
}

function Get-ComposeContainerId([string]$Service) {
  $containerId = docker compose ps -q --all $Service
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace(($containerId | Out-String))) { throw "$Service container was not created" }
  return ($containerId | Out-String).Trim()
}

function Wait-HealthyContainer([string]$ContainerId, [string]$ServiceLabel) {
  $deadline = [DateTimeOffset]::UtcNow.AddSeconds(60)
  do {
    $health = docker inspect $ContainerId --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}'
    if ($LASTEXITCODE -ne 0) { throw "failed to inspect rollback $ServiceLabel container health" }
    $health = ($health | Out-String).Trim()
    if ($health -eq 'healthy') { return }
    if ($health -in 'unhealthy','exited','dead') { throw "rollback $ServiceLabel container is $health" }
    Start-Sleep -Seconds 2
  } while ([DateTimeOffset]::UtcNow -lt $deadline)
  throw "rollback $ServiceLabel container did not become healthy (last state: $health)"
}

function Read-SuccessEndpoint([string]$BaseUrl, [string]$Path, [string]$Label) {
  try { $response = Invoke-WebRequest -UseBasicParsing -Uri ($BaseUrl.TrimEnd('/') + $Path) -TimeoutSec 5 } catch { throw "$Label check failed: $($_.Exception.Message)" }
  if ($response.StatusCode -ne 200) { throw "$Label returned HTTP $($response.StatusCode)" }
  return $response
}

function Read-JsonEndpoint([string]$BaseUrl, [string]$Path, [string]$Label) {
  $response = Read-SuccessEndpoint $BaseUrl $Path $Label
  try { return $response.Content | ConvertFrom-Json } catch { throw "$Label did not return JSON" }
}

function Assert-LoopbackOrigin([string]$Value, [string]$Label) {
  try { $parsed = [uri]$Value } catch { throw "$Label must be an HTTP loopback origin" }
  if ($parsed.Scheme -ne 'http' -or $parsed.Host -notin '127.0.0.1','localhost','::1' -or $parsed.UserInfo -or $parsed.Query -or $parsed.Fragment -or $parsed.AbsolutePath -ne '/') {
    throw "$Label must be an HTTP loopback origin"
  }
}

Assert-ExactIdentifier $PostgresContainer '^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$' 'container name'
Assert-ExactIdentifier $Database '^[A-Za-z][A-Za-z0-9_]{0,62}$' 'database name'

switch ($Action) {
  'Preflight' {
    $status = docker inspect $PostgresContainer --format '{{.State.Status}}'
    if ($LASTEXITCODE -ne 0 -or ($status | Out-String).Trim() -ne 'running') { throw 'PostgreSQL container is not running' }
    $schema = Get-SchemaVersion $Database
    if ($schema -notin 25, 28) { throw "expected schema 25 before rollout or 28 after rollout, got $schema" }
    git diff --quiet --exit-code
    if ($LASTEXITCODE -ne 0) { throw 'tracked worktree changes must be reviewed before rollout' }
    [pscustomobject]@{ action = $Action; database = $Database; schemaVersion = $schema; tableCounts = (Get-AllTableCounts $Database); commit = (git rev-parse HEAD).Trim() }
  }
  'Backup' {
    Assert-WorkspaceId
    if ((Get-SchemaVersion $Database) -ne 25) { throw 'pre-migration backup requires schema 25' }
    if ([string]::IsNullOrWhiteSpace($BackupDirectory)) { throw 'BackupDirectory is required' }
    $resolvedParent = [IO.Path]::GetFullPath($BackupDirectory)
    if (Test-Path -LiteralPath $resolvedParent) { throw 'BackupDirectory must be a new directory' }
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
    $identity = Get-IntegritySnapshot $Database
    $metadata = [ordered]@{
      formatVersion = 2
      createdAt = [DateTimeOffset]::UtcNow.ToString('O')
      schemaVersion = 25
      database = $Database
      deploySha = (git rev-parse HEAD).Trim()
      workspaceId = $WorkspaceId
      workspaceRevision = [int64]$identity.workspace.revision
      dumpSha256 = (Get-FileHash -LiteralPath $localFile -Algorithm SHA256).Hash.ToLowerInvariant()
      tableCounts = Get-AllTableCounts $Database
      taskIdentity = $identity.tasks
      eventIds = $identity.eventIds
      commandIds = $identity.commandIds
      approvalIds = $identity.approvalIds
    }
    $metadataPath = Join-Path $resolvedParent 'backup.json'
    $metadata | ConvertTo-Json -Depth 20 | Set-Content -LiteralPath $metadataPath -Encoding utf8
    [pscustomobject]@{ backupFile = $localFile; metadataFile = $metadataPath; metadata = $metadata }
  }
  'VerifyRestore' {
    Assert-WorkspaceId
    Assert-ExistingFile $BackupFile 'BackupFile'
    Assert-ExactIdentifier $RestoreDatabase '^baley_task184_restore_[0-9]{14}_[a-f0-9]{8}$' 'restore database'
    $metadata = Read-BackupMetadata
    $actualHash = (Get-FileHash -LiteralPath $BackupFile -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($actualHash -cne ([string]$metadata.dumpSha256).ToLowerInvariant()) { throw 'backup dump SHA-256 does not match BackupMetadataFile' }
    $remoteFile = "/tmp/baley-task184-restore-$([guid]::NewGuid().ToString('N')).dump"
    $createdByInvocation = $false
    try {
      docker exec $PostgresContainer createdb -U $DatabaseUser $RestoreDatabase
      if ($LASTEXITCODE -ne 0) { throw 'createdb failed; the pre-existing database was left untouched' }
      $createdByInvocation = $true
      docker cp $BackupFile "${PostgresContainer}:$remoteFile"
      if ($LASTEXITCODE -ne 0) { throw 'docker cp failed' }
      docker exec $PostgresContainer pg_restore -U $DatabaseUser -d $RestoreDatabase --no-owner --no-privileges --exit-on-error --single-transaction $remoteFile
      if ($LASTEXITCODE -ne 0) { throw 'pg_restore failed' }
      $schemaVersion = Get-SchemaVersion $RestoreDatabase
      if ($schemaVersion -ne [int]$metadata.schemaVersion) { throw 'restored schema version does not match backup metadata' }
      $tableCounts = Get-AllTableCounts $RestoreDatabase
      Assert-SameJson $metadata.tableCounts $tableCounts 'table counts'
      $identity = Get-IntegritySnapshot $RestoreDatabase
      if ([int64]$identity.workspace.revision -ne [int64]$metadata.workspaceRevision) { throw 'restored Workspace revision does not match backup metadata' }
      Assert-SameJson $metadata.taskIdentity $identity.tasks 'Task #183/#184 identity and status'
      Assert-SameJson $metadata.eventIds $identity.eventIds 'fixed Event IDs'
      Assert-SameJson $metadata.commandIds $identity.commandIds 'fixed command IDs'
      Assert-SameJson $metadata.approvalIds $identity.approvalIds 'fixed approval IDs'
      [pscustomobject]@{ action = $Action; database = $RestoreDatabase; schemaVersion = $schemaVersion; dumpSha256 = $actualHash; tableCounts = $tableCounts; workspaceRevision = [int64]$identity.workspace.revision; taskIdentity = $identity.tasks; eventIds = $identity.eventIds; commandIds = $identity.commandIds; approvalIds = $identity.approvalIds }
    } finally {
      docker exec $PostgresContainer rm -f -- $remoteFile | Out-Null
      if ($createdByInvocation) {
        docker exec $PostgresContainer dropdb -U $DatabaseUser --if-exists --force $RestoreDatabase | Out-Null
      }
    }
  }
  'Migrate' {
    if ((Get-SchemaVersion $Database) -ne 25) { throw 'migration requires schema 25' }
    if ($DeploySha -notmatch '^[0-9a-f]{40}$' -or $DeploySha -ne (git rev-parse HEAD).Trim()) { throw 'DeploySha must equal the checked-out commit' }
    $env:BALEY_BUILD_COMMIT = $DeploySha
    $env:BALEY_BUILD_TIME = [DateTimeOffset]::UtcNow.ToString('O')
    docker compose run --rm --no-deps --entrypoint /app/baley-server api migrate up
    if ($LASTEXITCODE -ne 0) { throw 'one-shot migration failed' }
    if ((Get-SchemaVersion $Database) -ne 28) { throw 'migration did not reach schema 28' }
  }
  'Verify' {
    if ((Get-SchemaVersion $Database) -ne 28) { throw 'verification requires schema 28' }
    Assert-WorkspaceId
    $invalid = Invoke-PsqlScalar $Database "SELECT count(*) FROM task_journal_entries j LEFT JOIN events e ON e.workspace_id=j.workspace_id AND e.id=j.event_id LEFT JOIN commands c ON c.workspace_id=j.workspace_id AND c.id=j.command_id WHERE e.id IS NULL OR c.id IS NULL OR j.recorded_at<>j.occurred_at"
    if ([int]$invalid -ne 0) { throw "$invalid invalid backfill provenance rows" }
    $task183Projection = @"
WITH selected_task AS (
  SELECT id FROM tasks WHERE workspace_id='$WorkspaceId' AND public_id=183
), eligible AS (
  SELECT e.id AS event_id,e.command_id
  FROM events e CROSS JOIN selected_task t
  WHERE e.workspace_id='$WorkspaceId'
    AND e.event_type IN (
      'task.created','run.started','task.updated','task.rework_started',
      'task.blocked','task.unblocked','task.implemented_reported',
      'task.confirmed','task.discarded'
    )
    AND NOT (e.payload ? 'taskJournal')
    AND CASE WHEN e.event_type='task.created' THEN
      COALESCE(NULLIF(btrim(e.payload #>> '{task,id}'),''),NULLIF(btrim(e.payload #>> '{task,ID}'),''))
    ELSE NULLIF(btrim(e.payload ->> 'taskId'),'') END=t.id
), journal AS (
  SELECT j.event_id,j.command_id
  FROM task_journal_entries j CROSS JOIN selected_task t
  WHERE j.workspace_id='$WorkspaceId' AND j.task_id=t.id
)
"@
    $eligibleCount = Invoke-PsqlScalar $Database "$task183Projection SELECT count(*) FROM eligible"
    $journalCount = Invoke-PsqlScalar $Database "$task183Projection SELECT count(*) FROM journal"
    $setDifference = Invoke-PsqlScalar $Database "$task183Projection SELECT count(*) FROM ((SELECT event_id,command_id FROM eligible EXCEPT SELECT event_id,command_id FROM journal) UNION ALL (SELECT event_id,command_id FROM journal EXCEPT SELECT event_id,command_id FROM eligible)) differences"
    if ([int]$setDifference -ne 0 -or [int]$eligibleCount -ne [int]$journalCount) {
      throw "Task #183 historical Event/Journal projection mismatch: eligible=$eligibleCount journal=$journalCount setDifference=$setDifference"
    }
    [pscustomobject]@{ action = $Action; schemaVersion = 28; task183EligibleEvents = [int]$eligibleCount; task183JournalRows = [int]$journalCount; task183SetDifference = [int]$setDifference; invalidProvenanceRows = [int]$invalid }
  }
  'Rollback' {
    if ((Get-SchemaVersion $Database) -ne 28) { throw 'application rollback is allowed only while the database remains at schema 28' }
    if ($RollbackApiImage -notmatch '^sha256:[0-9a-f]{64}$' -or $RollbackViewerImage -notmatch '^sha256:[0-9a-f]{64}$') { throw 'rollback images must be exact sha256 image IDs' }
    $apiSchema = Get-ImageLabel $RollbackApiImage 'org.opencontainers.image.baley.schema-version'
    if ($apiSchema -ne '28') { throw 'pre-rollout API rollback is forbidden: the exact API image must declare schema 28 compatibility' }
    $apiRevision = Get-ImageLabel $RollbackApiImage 'org.opencontainers.image.revision'
    $viewerRevision = Get-ImageLabel $RollbackViewerImage 'org.opencontainers.image.revision'
    if ($apiRevision -notmatch '^[0-9a-f]{40}$' -or $viewerRevision -ne $apiRevision) { throw 'rollback API and Viewer must declare the same exact 40-character revision' }
    Assert-LoopbackOrigin $ApiBaseUrl 'ApiBaseUrl'
    Assert-LoopbackOrigin $ViewerBaseUrl 'ViewerBaseUrl'
    docker image tag $RollbackApiImage baley-api:latest
    if ($LASTEXITCODE -ne 0) { throw 'failed to tag rollback API image' }
    docker image tag $RollbackViewerImage baley-viewer:latest
    if ($LASTEXITCODE -ne 0) { throw 'failed to tag rollback Viewer image' }
    docker compose up -d --no-deps --no-build --force-recreate api viewer
    if ($LASTEXITCODE -ne 0) { throw 'application rollback failed' }
    $apiContainerId = Get-ComposeContainerId 'api'
    $viewerContainerId = Get-ComposeContainerId 'viewer'
    $apiContainerImage = (docker inspect $apiContainerId --format '{{.Image}}' | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $apiContainerImage -ne $RollbackApiImage) { throw 'rollback API container is not running the requested exact image ID' }
    $viewerContainerImage = (docker inspect $viewerContainerId --format '{{.Image}}' | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $viewerContainerImage -ne $RollbackViewerImage) { throw 'rollback Viewer container is not running the requested exact image ID' }
    Wait-HealthyContainer $apiContainerId 'API'
    Wait-HealthyContainer $viewerContainerId 'Viewer'
    $viewerRoot = Read-SuccessEndpoint $ViewerBaseUrl '/' 'rollback Viewer root'
    $viewerReady = Read-JsonEndpoint $ViewerBaseUrl '/api/readyz' 'rollback Viewer /api/readyz proxy'
    if ($viewerReady.status -ne 'ready' -or [int]$viewerReady.schemaVersion -ne 28) { throw 'rollback Viewer /api/readyz proxy did not report ready on schema 28' }
    $ready = Read-JsonEndpoint $ApiBaseUrl '/readyz' 'rollback API /readyz'
    if ($ready.status -ne 'ready' -or [int]$ready.schemaVersion -ne 28) { throw 'rollback /readyz did not report ready on schema 28' }
    $version = Read-JsonEndpoint $ApiBaseUrl '/versionz' 'rollback API /versionz'
    if ([int]$version.schemaVersion -ne 28 -or $version.commit -ne $apiRevision) { throw 'rollback /versionz does not match schema 28 and the rollback artifact revision' }
    [pscustomobject]@{ action = $Action; apiImage = $RollbackApiImage; viewerImage = $RollbackViewerImage; revision = $apiRevision; schemaVersion = 28; containerHealth = 'healthy'; apiContainerHealth = 'healthy'; viewerContainerHealth = 'healthy'; viewerRootStatus = [int]$viewerRoot.StatusCode; viewerReady = $viewerReady; ready = $ready; version = $version }
  }
}
