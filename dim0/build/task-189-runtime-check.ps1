[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('WaitPersistence', 'WaitBackend', 'WaitWebUi', 'AssertFinalServices', 'AssertIsolation', 'RecordPersistenceImages')]
    [string]$Action,

    [string]$BaselineEnv,
    [string]$ComposeStatePath,
    [string]$IsolationStatePath,
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
$ServiceContainers = [ordered]@{
    'postgres-test' = 'dim0-task189-postgres'
    'qdrant-test' = 'dim0-task189-qdrant'
    'redis-test' = 'dim0-task189-redis'
    'backend-test' = 'dim0-task189-backend'
    'webui-test' = 'dim0-task189-webui'
}
$NetworkName = 'dim0-task189_default'
$QdrantReadyScript = "exec 3<>/dev/tcp/127.0.0.1/6333; printf 'GET /readyz HTTP/1.0\r\nHost: localhost\r\n\r\n' >&3; grep -q 'all shards are ready' <&3"
$BackendProbe = "import json, urllib.request; p=urllib.request.urlopen('http://backend-test:8082/utils/ping', timeout=5); assert 200 <= p.status < 300; m=urllib.request.urlopen('http://backend-test:8082/ai/models', timeout=5); data=json.load(m); models=data.get('data', {}).get('llm', []); assert 200 <= m.status < 300 and len(models) > 0; print(json.dumps({'pingStatus': p.status, 'modelsStatus': m.status, 'modelCount': len(models)}, separators=(',', ':')))"
$WebUiProbe = "import json, urllib.request; request=urllib.request.Request('http://webui-test/', headers={'Host': 'localhost'}); r=urllib.request.urlopen(request, timeout=5); body=r.read(); assert 200 <= r.status < 300 and len(body) > 0; print(json.dumps({'webUiStatus': r.status, 'bodyBytes': len(body)}, separators=(',', ':')))"

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

function Invoke-DockerProbe([string[]]$Arguments) {
    $previousErrorActionPreference = $ErrorActionPreference
    try {
        # A bounded readiness poll expects transient native stderr (for example,
        # connection refused) and decides from the native exit code.
        $ErrorActionPreference = 'Continue'
        $output = @(& docker @Arguments 2>&1 | ForEach-Object { $_.ToString() })
        $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
        return [pscustomobject]@{ ExitCode = $exitCode; Output = $output }
    }
    finally {
        $ErrorActionPreference = $previousErrorActionPreference
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

function Get-BoundPortEntries([object]$Bindings) {
    if ($null -eq $Bindings) { return @() }
    $entries = @()
    foreach ($property in @($Bindings.PSObject.Properties)) {
        $values = @($property.Value | Where-Object { $null -ne $_ })
        if ($values.Count) {
            $entries += "$($property.Name)=$($values | ConvertTo-Json -Compress)"
        }
    }
    return $entries
}

function Assert-IsolationState([object]$State) {
    if (-not $State.network.internal) {
        throw "Compose network is not internal: $($State.network.name)"
    }
    if ([string]$State.network.name -ne $NetworkName) {
        throw "Unexpected Compose network: $($State.network.name)"
    }
    $rows = @($State.containers)
    $actual = @($rows.service | Sort-Object)
    $expected = @($ExpectedServices | Sort-Object)
    if ($rows.Count -ne $ExpectedServices.Count -or ($actual -join ',') -ne ($expected -join ',')) {
        throw "Isolation state must contain exactly the five baseline services; found: $($actual -join ', ')"
    }
    foreach ($row in $rows) {
        $networks = @($row.networks)
        if ($networks.Count -ne 1 -or [string]$networks[0] -ne $NetworkName) {
            throw "Baseline service must use only the internal network: $($row.service) networks=$($networks -join ',')"
        }
        if (@($row.hostPortBindings).Count -or @($row.effectiveHostMappings).Count) {
            throw "Baseline service has a host port mapping: $($row.service)"
        }
        if (@($row.defaultRoutes).Count) {
            throw "Baseline service has a default route usable for egress: $($row.service)"
        }
    }
    return $State
}

function Get-LiveIsolationState {
    $networkRows = @(docker network inspect $NetworkName | ConvertFrom-Json)
    if ($LASTEXITCODE -ne 0 -or $networkRows.Count -ne 1) {
        throw "Unable to inspect Compose network: $NetworkName"
    }
    $network = $networkRows[0]
    if ($network.Labels.'com.docker.compose.project' -ne 'dim0-task189') {
        throw "Network is not owned by Compose project dim0-task189: $NetworkName"
    }
    $containers = @()
    foreach ($service in $ServiceContainers.Keys) {
        $containerName = $ServiceContainers[$service]
        $containerRows = @(docker inspect $containerName | ConvertFrom-Json)
        if ($LASTEXITCODE -ne 0 -or $containerRows.Count -ne 1) {
            throw "Unable to inspect baseline container: $containerName"
        }
        $container = $containerRows[0]
        if ($container.Config.Labels.'com.docker.compose.project' -ne 'dim0-task189' -or
            $container.Config.Labels.'com.docker.compose.service' -ne $service) {
            throw "Container ownership/service label mismatch: $containerName"
        }
        $routeLines = @(& docker exec $containerName cat /proc/net/route 2>&1 | ForEach-Object { $_.ToString() })
        if ($LASTEXITCODE -ne 0) { throw "Unable to inspect container routes: $containerName" }
        $defaultRoutes = @($routeLines | Where-Object { $_ -match '^\S+\s+00000000\s+[0-9A-Fa-f]{8}\s+' })
        $containers += [ordered]@{
            service = $service
            container = $containerName
            networks = @($container.NetworkSettings.Networks.PSObject.Properties.Name)
            hostPortBindings = @(Get-BoundPortEntries $container.HostConfig.PortBindings)
            effectiveHostMappings = @(Get-BoundPortEntries $container.NetworkSettings.Ports)
            defaultRoutes = $defaultRoutes
        }
    }
    return [ordered]@{
        network = [ordered]@{ name = $NetworkName; internal = [bool]$network.Internal }
        containers = $containers
    }
}

switch ($Action) {
    'WaitPersistence' {
        $compose = Get-ComposeArguments
        $deadline = [DateTimeOffset]::Now.AddSeconds($TimeoutSeconds)
        do {
            $postgres = Invoke-DockerProbe ($compose + @('exec', '-T', 'postgres-test', 'pg_isready', '-U', 'topix', '-d', 'topix'))
            $postgresReady = $postgres.ExitCode -eq 0
            $redis = Invoke-DockerProbe ($compose + @('exec', '-T', 'redis-test', 'redis-cli', 'ping'))
            $redisReady = $redis.ExitCode -eq 0
            $qdrant = Invoke-DockerProbe ($compose + @('exec', '-T', 'qdrant-test', 'bash', '-ec', $QdrantReadyScript))
            $qdrantReady = $qdrant.ExitCode -eq 0
            $qdrantInspection = Invoke-DockerProbe @('inspect', '--format', '{{.State.Health.Status}}', $PersistenceContainers['qdrant-test'])
            $qdrantHealth = @($qdrantInspection.Output)[0]
            $qdrantHealthy = $qdrantInspection.ExitCode -eq 0 -and $qdrantHealth -eq 'healthy'
            if ($postgresReady -and $qdrantReady -and $qdrantHealthy -and $redisReady) {
                [ordered]@{
                    postgres = 'ready'; qdrantReadyz = 'ready'; qdrantHealth = $qdrantHealth; redis = 'ready'
                } | ConvertTo-Json -Compress | Write-Output
                return
            }
            Start-Sleep -Seconds 2
        } while ([DateTimeOffset]::Now -lt $deadline)
        throw 'Persistence readiness deadline exceeded.'
    }

    'WaitBackend' {
        $compose = Get-ComposeArguments
        $deadline = [DateTimeOffset]::Now.AddSeconds($TimeoutSeconds)
        do {
            $probe = Invoke-DockerProbe ($compose + @('exec', '-T', 'backend-test', 'python', '-c', $BackendProbe))
            if ($probe.ExitCode -eq 0 -and @($probe.Output).Count) { $probe.Output | Write-Output; return }
            Start-Sleep -Seconds 2
        } while ([DateTimeOffset]::Now -lt $deadline)
        throw 'Post-restart backend ping/model readiness deadline exceeded.'
    }

    'WaitWebUi' {
        $compose = Get-ComposeArguments
        $deadline = [DateTimeOffset]::Now.AddSeconds($TimeoutSeconds)
        do {
            $probe = Invoke-DockerProbe ($compose + @('exec', '-T', 'backend-test', 'python', '-c', $WebUiProbe))
            if ($probe.ExitCode -eq 0 -and @($probe.Output).Count) { $probe.Output | Write-Output; return }
            Start-Sleep -Seconds 2
        } while ([DateTimeOffset]::Now -lt $deadline)
        throw 'Web UI internal HTTP readiness deadline exceeded.'
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

    'AssertIsolation' {
        if ($IsolationStatePath) {
            $state = Get-Content -Raw -LiteralPath $IsolationStatePath | ConvertFrom-Json
        }
        else {
            $state = Get-LiveIsolationState
        }
        Assert-IsolationState $state | ConvertTo-Json -Depth 6 | Write-Output
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
