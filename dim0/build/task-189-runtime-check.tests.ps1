[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$Check = (Resolve-Path (Join-Path $PSScriptRoot 'task-189-runtime-check.ps1')).Path
$TestRoot = Join-Path ([IO.Path]::GetTempPath()) ("dim0-task189-runtime-test-" + [Guid]::NewGuid().ToString('N'))

function Assert-Throws([scriptblock]$Body, [string]$Expected) {
    $threw = $false
    try { & $Body }
    catch {
        $threw = $true
        if (-not $_.Exception.Message.Contains($Expected)) {
            throw "Unexpected error: $($_.Exception.Message)"
        }
    }
    if (-not $threw) { throw "Expected failure containing: $Expected" }
}

function Write-State([string]$Name, [object[]]$Rows) {
    $path = Join-Path $TestRoot "$Name.json"
    [IO.File]::WriteAllText($path, ($Rows | ConvertTo-Json -Depth 4), [Text.UTF8Encoding]::new($false))
    return $path
}

function Write-IsolationState([string]$Name, [object]$State) {
    $path = Join-Path $TestRoot "$Name-isolation.json"
    [IO.File]::WriteAllText($path, ($State | ConvertTo-Json -Depth 8), [Text.UTF8Encoding]::new($false))
    return $path
}

try {
    New-Item -ItemType Directory -Path $TestRoot | Out-Null
    $goodRows = @(
        @{ Service = 'postgres-test'; State = 'running'; Health = 'healthy' },
        @{ Service = 'qdrant-test'; State = 'running'; Health = '' },
        @{ Service = 'redis-test'; State = 'running'; Health = 'healthy' },
        @{ Service = 'backend-test'; State = 'running'; Health = '' },
        @{ Service = 'webui-test'; State = 'running'; Health = '' }
    )
    $good = Write-State 'good' $goodRows
    & $Check -Action AssertFinalServices -ComposeStatePath $good | Out-Null

    $exitedRows = @($goodRows | ForEach-Object { @{} + $_ })
    $exitedRows[3].State = 'exited'
    $exited = Write-State 'backend-exited' $exitedRows
    Assert-Throws { & $Check -Action AssertFinalServices -ComposeStatePath $exited | Out-Null } 'backend-test state=exited'

    $unhealthyRows = @($goodRows | ForEach-Object { @{} + $_ })
    $unhealthyRows[0].Health = 'unhealthy'
    $unhealthy = Write-State 'postgres-unhealthy' $unhealthyRows
    Assert-Throws { & $Check -Action AssertFinalServices -ComposeStatePath $unhealthy | Out-Null } 'postgres-test health=unhealthy'

    $missing = Write-State 'missing-webui' @($goodRows | Where-Object Service -ne 'webui-test')
    Assert-Throws { & $Check -Action AssertFinalServices -ComposeStatePath $missing | Out-Null } 'exactly the five baseline services'

    $extra = Write-State 'extra-service' @($goodRows + @{ Service = 'provider-proxy'; State = 'running'; Health = '' })
    Assert-Throws { & $Check -Action AssertFinalServices -ComposeStatePath $extra | Out-Null } 'provider-proxy'

    $isolatedRows = @($goodRows | ForEach-Object {
        @{
            service = $_.Service
            container = "dim0-task189-$($_.Service -replace '-test$', '')"
            networks = @('dim0-task189_default')
            hostPortBindings = @()
            effectiveHostMappings = @()
            defaultRoutes = @()
        }
    })
    $isolatedState = @{ network = @{ name = 'dim0-task189_default'; internal = $true }; containers = $isolatedRows }
    $isolated = Write-IsolationState 'isolated' $isolatedState
    & $Check -Action AssertIsolation -IsolationStatePath $isolated | Out-Null

    $publishedState = $isolatedState | ConvertTo-Json -Depth 8 | ConvertFrom-Json
    $publishedState.containers[3].effectiveHostMappings = @('8082/tcp=127.0.0.1:18082')
    $published = Write-IsolationState 'published' $publishedState
    Assert-Throws { & $Check -Action AssertIsolation -IsolationStatePath $published | Out-Null } 'backend-test'

    $routedState = $isolatedState | ConvertTo-Json -Depth 8 | ConvertFrom-Json
    $routedState.containers[0].defaultRoutes = @('eth0 00000000 0100007F 0003')
    $routed = Write-IsolationState 'default-route' $routedState
    Assert-Throws { & $Check -Action AssertIsolation -IsolationStatePath $routed | Out-Null } 'default route usable for egress'

    $dualHomeState = $isolatedState | ConvertTo-Json -Depth 8 | ConvertFrom-Json
    $dualHomeState.containers[4].networks = @('dim0-task189_default', 'bridge')
    $dualHome = Write-IsolationState 'dual-home' $dualHomeState
    Assert-Throws { & $Check -Action AssertIsolation -IsolationStatePath $dualHome | Out-Null } 'only the internal network'

    $externalState = $isolatedState | ConvertTo-Json -Depth 8 | ConvertFrom-Json
    $externalState.network.internal = $false
    $external = Write-IsolationState 'external-network' $externalState
    Assert-Throws { & $Check -Action AssertIsolation -IsolationStatePath $external | Out-Null } 'not internal'

    $overlay = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot 'docker-compose.baseline.yml')
    $portResets = @([regex]::Matches($overlay, '(?m)^\s+ports:\s+!reset\s+\[\]\s*$'))
    if ($portResets.Count -ne 5) { throw "Baseline overlay must reset ports for exactly five services; found $($portResets.Count)." }
    if ($overlay -match 'BASELINE_(POSTGRES|QDRANT|REDIS|API|APP|MINI_APP)_PORT') {
        throw 'Baseline overlay still contains host-port environment variables.'
    }
    $healthyDependencies = @([regex]::Matches($overlay, '(?m)^\s+condition:\s+service_healthy\s*$'))
    if ($healthyDependencies.Count -ne 3) {
        throw "Backend baseline must wait for all three healthy persistence services; found $($healthyDependencies.Count)."
    }

    $planPath = Join-Path $PSScriptRoot '..\docs\plans\task-189-baseline-execution.md'
    $plan = Get-Content -Raw -LiteralPath $planPath
    $names = @([regex]::Matches($plan, "-Name '([^']+)'") | ForEach-Object { $_.Groups[1].Value })
    $duplicates = @($names | Group-Object | Where-Object Count -gt 1)
    if ($duplicates.Count) { throw "Execution plan contains duplicate evidence names: $($duplicates.Name -join ', ')" }
    $orderedRestart = @(
        '56-backend-stop-before-restart',
        '57-persistence-restart',
        '58-persistence-ready-after-restart',
        '59-backend-start-after-persistence',
        '60-backend-ready-after-restart',
        '64-runtime-isolation',
        '65-five-service-final-state'
    )
    $lastIndex = -1
    foreach ($name in $orderedRestart) {
        $index = $names.IndexOf($name)
        if ($index -le $lastIndex) { throw "Execution plan restart ordering is invalid at $name" }
        $lastIndex = $index
    }

    Write-Output 'Task 189 runtime assertion self-tests passed.'
}
finally {
    $resolved = [IO.Path]::GetFullPath($TestRoot)
    $temp = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if ($resolved.StartsWith($temp, [StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolved)) {
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
