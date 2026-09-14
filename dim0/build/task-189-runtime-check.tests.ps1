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

    Write-Output 'Task 189 runtime assertion self-tests passed.'
}
finally {
    $resolved = [IO.Path]::GetFullPath($TestRoot)
    $temp = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
    if ($resolved.StartsWith($temp, [StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolved)) {
        Remove-Item -LiteralPath $resolved -Recurse -Force
    }
}
