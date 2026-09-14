[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('WaitPersistence', 'WaitBackend', 'AssertFinalServices', 'RecordPersistenceImages')]
    [string]$Action,

    [string]$BaselineEnv,
    [string]$ComposeStatePath,
    [ValidateRange(1, 900)]
    [int]$TimeoutSeconds = 180
)

$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$ExpectedServices = @('postgres-test', 'qdrant-test', 'redis-test', 'backend-test', 'webui-test')
$PersistenceContainers = [ordered]@{
    'postgres-test' = 'dim0-task189-postgres'
    'qdrant-test' = 'dim0-task189-qdrant'
    'redis-test' = 'dim0-task189-redis'
}

function Get-BaselineValue([string]$Name, [string]$Default) {
    if (-not $BaselineEnv) { return $Default }
    foreach ($line in Get-Content -LiteralPath $BaselineEnv) {
        if ($line -match "^$([regex]::Escape($Name))=(.*)$") { return $Matches[1] }
    }
    return $Default
}

function Get-ComposeArguments {
    if (-not $BaselineEnv) { throw '-BaselineEnv is required for live runtime checks.' }
    $resolvedEnv = (Resolve-Path -LiteralPath $BaselineEnv).Path
    return @(
        'compose', '-p', 'dim0-task189',
        '-f', (Join-Path $RepositoryRoot 'dim0\build\docker-compose.yml'),
        '-f', (Join-Path $RepositoryRoot 'dim0\build\docker-compose.baseline.yml'),
        '--env-file', $resolvedEnv, '--profile', 'test'
    )
}

function Convert-ComposeState([string]$Raw) {
    if (-not $Raw.Trim()) { throw 'Compose returned no service state.' }
    try {
        return @($Raw | ConvertFrom-Json)
    }
    catch {
        $rows = @()
        foreach ($line in $Raw -split '\r?\n') {
            if ($line.Trim()) { $rows += $line | ConvertFrom-Json }
        }
        return $rows
    }
}

function Assert-FinalServices([object[]]$Rows) {
    $serviceRows = @($Rows | Where-Object { $_.Service -in $ExpectedServices })
    $allServices = @($Rows.Service | Sort-Object -Unique)
    $actual = @($serviceRows.Service | Sort-Object -Unique)
    $expected = @($ExpectedServices | Sort-Object)
    if (($actual -join ',') -ne ($expected -join ',') -or
        $serviceRows.Count -ne $ExpectedServices.Count -or $Rows.Count -ne $ExpectedServices.Count) {
        throw "Final Compose state must contain exactly the five baseline services; found: $($allServices -join ', ')"
    }
    foreach ($row in $serviceRows) {
        if ([string]$row.State -ne 'running') {
            throw "Final service is not running: $($row.Service) state=$($row.State)"
        }
        $health = [string]$row.Health
        if ($health -and $health -ne 'healthy') {
            throw "Final service health is not healthy: $($row.Service) health=$health"
        }
    }
    return $serviceRows
}

switch ($Action) {
    'WaitPersistence' {
        $compose = Get-ComposeArguments
        $qdrantPort = Get-BaselineValue 'BASELINE_QDRANT_PORT' '16335'
        $deadline = [DateTimeOffset]::Now.AddSeconds($TimeoutSeconds)
        do {
            & docker @compose exec -T postgres-test pg_isready -U topix -d topix 2>$null | Out-Null
            $postgresReady = $LASTEXITCODE -eq 0
            & docker @compose exec -T redis-test redis-cli ping 2>$null | Out-Null
            $redisReady = $LASTEXITCODE -eq 0
            try {
                $qdrantStatus = (Invoke-WebRequest "http://localhost:$qdrantPort/readyz" -UseBasicParsing -TimeoutSec 5).StatusCode
                $qdrantReady = $qdrantStatus -ge 200 -and $qdrantStatus -lt 300
            }
            catch { $qdrantReady = $false }
            if ($postgresReady -and $qdrantReady -and $redisReady) {
                Write-Output 'PostgreSQL, Qdrant, and Redis are ready.'
                return
            }
            Start-Sleep -Seconds 2
        } while ([DateTimeOffset]::Now -lt $deadline)
        throw 'Persistence readiness deadline exceeded.'
    }

    'WaitBackend' {
        $apiPort = Get-BaselineValue 'BASELINE_API_PORT' '18082'
        $deadline = [DateTimeOffset]::Now.AddSeconds($TimeoutSeconds)
        do {
            try {
                $ping = (Invoke-WebRequest "http://localhost:$apiPort/utils/ping" -UseBasicParsing -TimeoutSec 5).StatusCode
                $catalogResponse = Invoke-RestMethod "http://localhost:$apiPort/ai/models" -TimeoutSec 5
                $models = @($catalogResponse.data.llm)
                if ($ping -ge 200 -and $ping -lt 300 -and $models.Count -gt 0) {
                    [ordered]@{ pingStatus = $ping; modelsStatus = 200; modelCount = $models.Count } |
                        ConvertTo-Json -Compress | Write-Output
                    return
                }
            }
            catch {}
            Start-Sleep -Seconds 2
        } while ([DateTimeOffset]::Now -lt $deadline)
        throw 'Post-restart backend ping/model readiness deadline exceeded.'
    }

    'AssertFinalServices' {
        if ($ComposeStatePath) {
            $raw = Get-Content -Raw -LiteralPath $ComposeStatePath
        }
        else {
            $compose = Get-ComposeArguments
            $lines = @(& docker @compose ps --all --format json 2>&1 | ForEach-Object { $_.ToString() })
            if ($LASTEXITCODE -ne 0) { throw 'docker compose ps failed during final-state assertion.' }
            $raw = $lines -join [Environment]::NewLine
        }
        $rows = Assert-FinalServices (Convert-ComposeState $raw)
        $rows | Select-Object Service, State, Health | ConvertTo-Json -Depth 3 | Write-Output
    }

    'RecordPersistenceImages' {
        $identities = @()
        foreach ($service in $PersistenceContainers.Keys) {
            $containerName = $PersistenceContainers[$service]
            $container = docker inspect $containerName | ConvertFrom-Json
            if ($LASTEXITCODE -ne 0 -or @($container).Count -ne 1) {
                throw "Unable to inspect persistence container: $containerName"
            }
            if ($container[0].Config.Labels.'com.docker.compose.project' -ne 'dim0-task189') {
                throw "Persistence container is not owned by Compose project dim0-task189: $containerName"
            }
            if (-not $container[0].State.Running) {
                throw "Persistence container is not running: $containerName"
            }
            $imageId = [string]$container[0].Image
            if ($imageId -notmatch '^sha256:[0-9a-f]{64}$') {
                throw "Persistence container has no immutable image ID: $containerName"
            }
            $image = docker image inspect $imageId | ConvertFrom-Json
            if ($LASTEXITCODE -ne 0 -or @($image).Count -ne 1) {
                throw "Unable to inspect persistence image: $imageId"
            }
            $identities += [ordered]@{
                service = $service
                configuredReference = [string]$container[0].Config.Image
                imageId = $imageId
                repoDigests = @($image[0].RepoDigests)
            }
        }
        $identities | ConvertTo-Json -Depth 4 | Write-Output
    }
}
