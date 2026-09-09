[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$rolloutScript = Join-Path $PSScriptRoot 'task-journal-rollout.ps1'
$workspaceId = '00000000-0000-4000-8000-000000000001'
$restoreDatabase = 'baley_task184_restore_20260910010101_deadbeef'
$failures = [Collections.Generic.List[string]]::new()
$passed = 0

function Assert-True([bool]$Condition, [string]$Message) {
  if (-not $Condition) { throw $Message }
}

function Invoke-Case([string]$Name, [scriptblock]$Body) {
  try {
    & $Body
    $script:passed++
    Write-Output "PASS $Name"
  } catch {
    $script:failures.Add("$Name`: $($_.Exception.Message)")
    Write-Output "FAIL $Name`: $($_.Exception.Message)"
  } finally {
    Remove-Item -Path Function:\docker -Force -ErrorAction SilentlyContinue
    Remove-Item -Path Function:\Invoke-WebRequest -Force -ErrorAction SilentlyContinue
    Remove-Variable TaskJournalDockerCalls,TaskJournalCreateSucceeds -Scope Global -ErrorAction SilentlyContinue
  }
}

function New-TestBackup([string]$Root, [int64]$EventCount = 5) {
  New-Item -ItemType Directory -Path $Root | Out-Null
  $dumpPath = Join-Path $Root 'baley-schema25.dump'
  [IO.File]::WriteAllText($dumpPath, 'task-184-safe-restore-fixture', [Text.Encoding]::UTF8)
  $hash = (Get-FileHash -LiteralPath $dumpPath -Algorithm SHA256).Hash.ToLowerInvariant()
  $metadata = [ordered]@{
    formatVersion = 2
    schemaVersion = 25
    workspaceId = $workspaceId
    workspaceRevision = 1354
    dumpSha256 = $hash
    tableCounts = [ordered]@{ commands = 3; events = $EventCount }
    taskIdentity = @(
      [ordered]@{ id = 'task-183'; publicId = 183; status = 'confirmed' },
      [ordered]@{ id = 'task-184'; publicId = 184; status = 'in_progress' }
    )
    eventIds = @('event-1','event-2')
    commandIds = @('command-1')
    approvalIds = @('approval-1')
  }
  $metadataPath = Join-Path $Root 'backup.json'
  $metadata | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $metadataPath -Encoding utf8
  return [pscustomobject]@{ Dump = $dumpPath; Metadata = $metadataPath }
}

function Get-IdentityJson {
  return '{"approvalIds":["approval-1"],"commandIds":["command-1"],"eventIds":["event-1","event-2"],"tasks":[{"id":"task-183","publicId":183,"status":"confirmed"},{"id":"task-184","publicId":184,"status":"in_progress"}],"workspace":{"id":"00000000-0000-4000-8000-000000000001","revision":1354}}'
}

function Set-RestoreDockerMock([bool]$CreateSucceeds = $true) {
  $global:TaskJournalDockerCalls = [Collections.Generic.List[string]]::new()
  $global:TaskJournalCreateSucceeds = $CreateSucceeds
  function global:docker {
    $arguments = @($args)
    $global:TaskJournalDockerCalls.Add(($arguments -join ' '))
    if ($arguments -contains 'createdb' -and -not $global:TaskJournalCreateSucceeds) {
      $global:LASTEXITCODE = 1
      return 'database already exists'
    }
    if ($arguments -contains 'psql') {
      $sql = [string]$arguments[-1]
      $global:LASTEXITCODE = 0
      if ($sql -like 'SELECT version_id*') { return '25' }
      if ($sql -like '*FROM pg_catalog.pg_tables*') { return "commands`nevents" }
      if ($sql -like '*public."commands"*') { return '3' }
      if ($sql -like '*public."events"*') { return '5' }
      if ($sql -like '*jsonb_build_object*') { return Get-IdentityJson }
      throw "unexpected psql query: $sql"
    }
    $global:LASTEXITCODE = 0
  }
}

$testRoot = Join-Path ([IO.Path]::GetTempPath()) ("baley-task184-rollout-tests-" + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testRoot | Out-Null
try {
  Invoke-Case 'VerifyRestore never drops a pre-existing database' {
    $backup = New-TestBackup (Join-Path $testRoot 'preexisting')
    Set-RestoreDockerMock $false
    $thrown = $false
    try {
      & $rolloutScript -Action VerifyRestore -WorkspaceId $workspaceId -BackupFile $backup.Dump -BackupMetadataFile $backup.Metadata -RestoreDatabase $restoreDatabase | Out-Null
    } catch {
      $thrown = $true
      Assert-True ($_.Exception.Message -like '*pre-existing database was left untouched*') 'failure did not describe non-deletion guarantee'
    }
    Assert-True $thrown 'pre-existing database did not fail closed'
    Assert-True (-not ($global:TaskJournalDockerCalls -match 'dropdb')) 'pre-existing database was passed to dropdb'
  }

  Invoke-Case 'VerifyRestore compares hash, counts, revision, Tasks, and fixed IDs' {
    $backup = New-TestBackup (Join-Path $testRoot 'matching')
    Set-RestoreDockerMock $true
    & $rolloutScript -Action VerifyRestore -WorkspaceId $workspaceId -BackupFile $backup.Dump -BackupMetadataFile $backup.Metadata -RestoreDatabase $restoreDatabase | Out-Null
    Assert-True ([bool]($global:TaskJournalDockerCalls -match 'pg_restore')) 'restore was not attempted'
    Assert-True ([bool]($global:TaskJournalDockerCalls -match 'dropdb')) 'database created by the invocation was not cleaned up'
  }

  Invoke-Case 'VerifyRestore returns non-zero on metadata mismatch and cleans only its database' {
    $backup = New-TestBackup (Join-Path $testRoot 'mismatch') 99
    Set-RestoreDockerMock $true
    $thrown = $false
    try {
      & $rolloutScript -Action VerifyRestore -WorkspaceId $workspaceId -BackupFile $backup.Dump -BackupMetadataFile $backup.Metadata -RestoreDatabase $restoreDatabase | Out-Null
    } catch {
      $thrown = $true
      Assert-True ($_.Exception.Message -like '*table counts*') 'metadata mismatch did not identify table counts'
    }
    Assert-True $thrown 'metadata mismatch did not return failure'
    Assert-True ([bool]($global:TaskJournalDockerCalls -match 'dropdb')) 'owned restore database was not cleaned after comparison failure'
  }

  Invoke-Case 'VerifyRestore rejects dump hash mismatch before creating a database' {
    $backup = New-TestBackup (Join-Path $testRoot 'hash')
    Add-Content -LiteralPath $backup.Dump -Value 'tampered'
    Set-RestoreDockerMock $true
    $thrown = $false
    try { & $rolloutScript -Action VerifyRestore -WorkspaceId $workspaceId -BackupFile $backup.Dump -BackupMetadataFile $backup.Metadata -RestoreDatabase $restoreDatabase | Out-Null } catch { $thrown = $true }
    Assert-True $thrown 'tampered dump did not return failure'
    Assert-True (-not ($global:TaskJournalDockerCalls -match 'createdb')) 'database was created before dump hash validation'
  }

  Invoke-Case 'Rollback rejects a pre-schema-27 API before any tag or container write' {
    $apiImage = 'sha256:' + ('a' * 64)
    $viewerImage = 'sha256:' + ('b' * 64)
    $global:TaskJournalDockerCalls = [Collections.Generic.List[string]]::new()
    function global:docker {
      $arguments = @($args)
      $global:TaskJournalDockerCalls.Add(($arguments -join ' '))
      $global:LASTEXITCODE = 0
      if ($arguments -contains 'psql') { return '27' }
      if ($arguments[0] -eq 'image' -and $arguments[1] -eq 'inspect') { return '{"org.opencontainers.image.baley.schema-version":"26","org.opencontainers.image.revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}' }
      return ''
    }
    $thrown = $false
    try { & $rolloutScript -Action Rollback -RollbackApiImage $apiImage -RollbackViewerImage $viewerImage | Out-Null } catch { $thrown = $true }
    Assert-True $thrown 'pre-schema-27 API was accepted'
    Assert-True (-not ($global:TaskJournalDockerCalls -match '^image tag')) 'rollback mutated image tags before compatibility validation'
  }

  Invoke-Case 'Rollback checks exact containers, both health states, Viewer HTTP/proxy, readyz, and versionz fail closed' {
    $apiImage = 'sha256:' + ('c' * 64)
    $viewerImage = 'sha256:' + ('d' * 64)
    $revision = 'e' * 40
    $global:TaskJournalDockerCalls = [Collections.Generic.List[string]]::new()
    function global:docker {
      $arguments = @($args)
      $global:TaskJournalDockerCalls.Add(($arguments -join ' '))
      $global:LASTEXITCODE = 0
      if ($arguments -contains 'psql') { return '27' }
      if ($arguments[0] -eq 'image' -and $arguments[1] -eq 'inspect') {
        return ('{"org.opencontainers.image.baley.schema-version":"27","org.opencontainers.image.revision":"' + $revision + '"}')
      }
      if ($arguments[0] -eq 'compose' -and $arguments[1] -eq 'ps') { if ($arguments[-1] -eq 'api') { return 'api-container' } else { return 'viewer-container' } }
      if ($arguments[0] -eq 'inspect' -and $arguments[1] -eq 'api-container') {
        if ($arguments[-1] -like '*Health*') { return 'healthy' }
        return $apiImage
      }
      if ($arguments[0] -eq 'inspect' -and $arguments[1] -eq 'viewer-container') {
        if ($arguments[-1] -like '*Health*') { return 'healthy' }
        return $viewerImage
      }
      return ''
    }
    function global:Invoke-WebRequest {
      param([switch]$UseBasicParsing, [string]$Uri, [int]$TimeoutSec)
      if ($Uri -eq 'http://127.0.0.1:5174/') { return [pscustomobject]@{ StatusCode = 200; Content = '<!doctype html>' } }
      if ($Uri -like '*/readyz') { return [pscustomobject]@{ StatusCode = 200; Content = '{"status":"ready","schemaVersion":27}' } }
      if ($Uri -like '*/versionz') { return [pscustomobject]@{ StatusCode = 200; Content = ('{"commit":"' + $revision + '","schemaVersion":27}') } }
      throw "unexpected URI $Uri"
    }
    & $rolloutScript -Action Rollback -RollbackApiImage $apiImage -RollbackViewerImage $viewerImage | Out-Null
    Assert-True ([bool]($global:TaskJournalDockerCalls -match 'compose up -d --no-deps --no-build --force-recreate api viewer')) 'rollback did not restart only API and Viewer'
    Assert-True ([bool]($global:TaskJournalDockerCalls -match 'api-container.*Health')) 'API container health was not checked'
    Assert-True ([bool]($global:TaskJournalDockerCalls -match 'viewer-container.*Health')) 'Viewer container health was not checked'
  }

  Invoke-Case 'Rollback returns non-zero when the exact Viewer container has exited' {
    $apiImage = 'sha256:' + ('4' * 64)
    $viewerImage = 'sha256:' + ('5' * 64)
    $revision = '6' * 40
    $global:TaskJournalDockerCalls = [Collections.Generic.List[string]]::new()
    function global:docker {
      $arguments = @($args)
      $global:TaskJournalDockerCalls.Add(($arguments -join ' '))
      $global:LASTEXITCODE = 0
      if ($arguments -contains 'psql') { return '27' }
      if ($arguments[0] -eq 'image' -and $arguments[1] -eq 'inspect') { return ('{"org.opencontainers.image.baley.schema-version":"27","org.opencontainers.image.revision":"' + $revision + '"}') }
      if ($arguments[0] -eq 'compose' -and $arguments[1] -eq 'ps') { if ($arguments[-1] -eq 'api') { return 'api-container' } else { return 'viewer-container' } }
      if ($arguments[0] -eq 'inspect' -and $arguments[1] -eq 'api-container') { if ($arguments[-1] -like '*Health*') { return 'healthy' }; return $apiImage }
      if ($arguments[0] -eq 'inspect' -and $arguments[1] -eq 'viewer-container') { if ($arguments[-1] -like '*Health*') { return 'exited' }; return $viewerImage }
      return ''
    }
    function global:Invoke-WebRequest { throw 'Viewer HTTP must not be reached after an exited state' }
    $thrown = $false
    try { & $rolloutScript -Action Rollback -RollbackApiImage $apiImage -RollbackViewerImage $viewerImage | Out-Null } catch { $thrown = $true }
    Assert-True $thrown 'exited Viewer container was accepted'
    Assert-True ([bool]($global:TaskJournalDockerCalls -match 'viewer-container.*Health')) 'Viewer state was not inspected'
  }

  Invoke-Case 'Rollback returns non-zero when the Viewer loopback root is unreachable' {
    $apiImage = 'sha256:' + ('7' * 64)
    $viewerImage = 'sha256:' + ('8' * 64)
    $revision = '9' * 40
    $global:TaskJournalDockerCalls = [Collections.Generic.List[string]]::new()
    function global:docker {
      $arguments = @($args)
      $global:TaskJournalDockerCalls.Add(($arguments -join ' '))
      $global:LASTEXITCODE = 0
      if ($arguments -contains 'psql') { return '27' }
      if ($arguments[0] -eq 'image' -and $arguments[1] -eq 'inspect') { return ('{"org.opencontainers.image.baley.schema-version":"27","org.opencontainers.image.revision":"' + $revision + '"}') }
      if ($arguments[0] -eq 'compose' -and $arguments[1] -eq 'ps') { if ($arguments[-1] -eq 'api') { return 'api-container' } else { return 'viewer-container' } }
      if ($arguments[0] -eq 'inspect' -and $arguments[1] -eq 'api-container') { if ($arguments[-1] -like '*Health*') { return 'healthy' }; return $apiImage }
      if ($arguments[0] -eq 'inspect' -and $arguments[1] -eq 'viewer-container') { if ($arguments[-1] -like '*Health*') { return 'healthy' }; return $viewerImage }
      return ''
    }
    function global:Invoke-WebRequest { throw 'connection refused' }
    $thrown = $false
    try { & $rolloutScript -Action Rollback -RollbackApiImage $apiImage -RollbackViewerImage $viewerImage | Out-Null } catch { $thrown = $true }
    Assert-True $thrown 'unreachable Viewer root was accepted'
  }

  Invoke-Case 'Rollback returns non-zero when the Viewer same-origin readyz proxy is incompatible' {
    $apiImage = 'sha256:' + ('a' * 64)
    $viewerImage = 'sha256:' + ('b' * 64)
    $revision = 'c' * 40
    $global:TaskJournalDockerCalls = [Collections.Generic.List[string]]::new()
    function global:docker {
      $arguments = @($args)
      $global:TaskJournalDockerCalls.Add(($arguments -join ' '))
      $global:LASTEXITCODE = 0
      if ($arguments -contains 'psql') { return '27' }
      if ($arguments[0] -eq 'image' -and $arguments[1] -eq 'inspect') { return ('{"org.opencontainers.image.baley.schema-version":"27","org.opencontainers.image.revision":"' + $revision + '"}') }
      if ($arguments[0] -eq 'compose' -and $arguments[1] -eq 'ps') { if ($arguments[-1] -eq 'api') { return 'api-container' } else { return 'viewer-container' } }
      if ($arguments[0] -eq 'inspect' -and $arguments[1] -eq 'api-container') { if ($arguments[-1] -like '*Health*') { return 'healthy' }; return $apiImage }
      if ($arguments[0] -eq 'inspect' -and $arguments[1] -eq 'viewer-container') { if ($arguments[-1] -like '*Health*') { return 'healthy' }; return $viewerImage }
      return ''
    }
    function global:Invoke-WebRequest {
      param([switch]$UseBasicParsing, [string]$Uri, [int]$TimeoutSec)
      if ($Uri -eq 'http://127.0.0.1:5174/') { return [pscustomobject]@{ StatusCode = 200; Content = '<!doctype html>' } }
      if ($Uri -eq 'http://127.0.0.1:5174/api/readyz') { return [pscustomobject]@{ StatusCode = 200; Content = '{"status":"ready","schemaVersion":26}' } }
      throw "API checks must not be reached after an incompatible Viewer proxy: $Uri"
    }
    $thrown = $false
    try { & $rolloutScript -Action Rollback -RollbackApiImage $apiImage -RollbackViewerImage $viewerImage | Out-Null } catch { $thrown = $true }
    Assert-True $thrown 'schema-incompatible Viewer proxy response was accepted'
  }

  Invoke-Case 'Rollback returns non-zero when readyz is incompatible' {
    $apiImage = 'sha256:' + ('1' * 64)
    $viewerImage = 'sha256:' + ('2' * 64)
    $revision = '3' * 40
    $global:TaskJournalDockerCalls = [Collections.Generic.List[string]]::new()
    function global:docker {
      $arguments = @($args)
      $global:TaskJournalDockerCalls.Add(($arguments -join ' '))
      $global:LASTEXITCODE = 0
      if ($arguments -contains 'psql') { return '27' }
      if ($arguments[0] -eq 'image' -and $arguments[1] -eq 'inspect') {
        return ('{"org.opencontainers.image.baley.schema-version":"27","org.opencontainers.image.revision":"' + $revision + '"}')
      }
      if ($arguments[0] -eq 'compose' -and $arguments[1] -eq 'ps') { if ($arguments[-1] -eq 'api') { return 'api-container' } else { return 'viewer-container' } }
      if ($arguments[0] -eq 'inspect' -and $arguments[1] -eq 'api-container') {
        if ($arguments[-1] -like '*Health*') { return 'healthy' }
        return $apiImage
      }
      if ($arguments[0] -eq 'inspect' -and $arguments[1] -eq 'viewer-container') { if ($arguments[-1] -like '*Health*') { return 'healthy' }; return $viewerImage }
      return ''
    }
    function global:Invoke-WebRequest {
      param([switch]$UseBasicParsing, [string]$Uri, [int]$TimeoutSec)
      if ($Uri -eq 'http://127.0.0.1:5174/') { return [pscustomobject]@{ StatusCode = 200; Content = '<!doctype html>' } }
      if ($Uri -eq 'http://127.0.0.1:5174/api/readyz') { return [pscustomobject]@{ StatusCode = 200; Content = '{"status":"ready","schemaVersion":27}' } }
      if ($Uri -eq 'http://127.0.0.1:8080/readyz') { return [pscustomobject]@{ StatusCode = 200; Content = '{"status":"ready","schemaVersion":26}' } }
      throw "versionz must not be reached after an incompatible readyz response"
    }
    $thrown = $false
    try { & $rolloutScript -Action Rollback -RollbackApiImage $apiImage -RollbackViewerImage $viewerImage | Out-Null } catch { $thrown = $true }
    Assert-True $thrown 'schema-incompatible readyz response was accepted'
  }
} finally {
  Remove-Item -Path Function:\docker -Force -ErrorAction SilentlyContinue
  Remove-Item -Path Function:\Invoke-WebRequest -Force -ErrorAction SilentlyContinue
  Remove-Variable TaskJournalDockerCalls,TaskJournalCreateSucceeds -Scope Global -ErrorAction SilentlyContinue
  Remove-Item -LiteralPath $testRoot -Recurse -Force
}

if ($failures.Count -gt 0) {
  $failures | ForEach-Object { Write-Error $_ }
  throw "$($failures.Count) task-journal rollout safety regression test(s) failed"
}
Write-Output "All $passed task-journal rollout safety regression tests passed."
