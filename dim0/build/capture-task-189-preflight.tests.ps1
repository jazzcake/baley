[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$Helper = (Resolve-Path (Join-Path $PSScriptRoot 'capture-task-189-evidence.ps1')).Path
$TempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$TestRoot = Join-Path $TempRoot ("dim0-task189-preflight-test-" + [Guid]::NewGuid().ToString('N'))
$ExpectedContainers = @(
    'dim0-task189-postgres', 'dim0-task189-qdrant', 'dim0-task189-redis',
    'dim0-task189-backend', 'dim0-task189-webui'
)
$ExpectedPorts = @(15434, 16335, 16381, 18082, 15175, 15182)
$PortEnvironment = @{
    BASELINE_POSTGRES_PORT = 15434; BASELINE_QDRANT_PORT = 16335
    BASELINE_REDIS_PORT = 16381; BASELINE_API_PORT = 18082
    BASELINE_APP_PORT = 15175; BASELINE_MINI_APP_PORT = 15182
}
$PreviousPortEnvironment = @{}
$global:Dim0Task189PreflightMock = @{
    Containers = @{}
    PublishedPorts = @{}
    OccupiedPorts = @()
    DockerFailure = $null
    DockerCalls = @()
}

# Fail the focused test with a descriptive assertion message.
function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
}

# Create the minimum external evidence directory accepted by the Preflight action.
function New-PreflightFixture([string]$Name) {
    $directory = Join-Path $TestRoot $Name
    New-Item -ItemType Directory -Path $directory | Out-Null
    $baselineEnvironment = $PortEnvironment.GetEnumerator() | ForEach-Object { "$($_.Key)=$($_.Value)" }
    [IO.File]::WriteAllText(
        (Join-Path $directory 'baseline.env'),
        (($baselineEnvironment -join [Environment]::NewLine) + [Environment]::NewLine),
        [Text.UTF8Encoding]::new($false)
    )
    return $directory
}

# Configure deterministic Docker and host-listener responses for one test case.
function Set-PreflightScenario {
    param(
        [hashtable]$Containers = @{},
        [hashtable]$PublishedPorts = @{},
        [int[]]$OccupiedPorts = @(),
        [string]$DockerFailure
    )

    $global:Dim0Task189PreflightMock.Containers = $Containers
    $global:Dim0Task189PreflightMock.PublishedPorts = $PublishedPorts
    $global:Dim0Task189PreflightMock.OccupiedPorts = $OccupiedPorts
    $global:Dim0Task189PreflightMock.DockerFailure = $DockerFailure
    $global:Dim0Task189PreflightMock.DockerCalls = @()
}

# Verify a rejected preflight leaves no success evidence behind.
function Assert-PreflightFails([string]$Directory, [string]$ExpectedMessage) {
    $failed = $false
    try {
        & $Helper -Action Preflight -EvidenceRoot $TestRoot -RunDirectory $Directory | Out-Null
    }
    catch {
        $failed = $true
        Assert-True ($_.Exception.Message.Contains($ExpectedMessage)) "Unexpected preflight error: $($_.Exception.Message)"
    }
    Assert-True $failed "Expected preflight to fail: $ExpectedMessage"
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $Directory '05-compose-ownership-preflight.json'))) 'Failed preflight wrote success evidence.'
}

# Emulate the Docker commands used by the ownership preflight.
function docker {
    $dockerArguments = @($args | ForEach-Object { $_.ToString() })
    $global:Dim0Task189PreflightMock.DockerCalls += ,$dockerArguments
    $command = $dockerArguments[0]
    if ($global:Dim0Task189PreflightMock.DockerFailure -eq $command) {
        $global:LASTEXITCODE = 1
        return
    }

    $global:LASTEXITCODE = 0
    switch ($command) {
        'ps' {
            $filter = $dockerArguments[-1]
            if ($filter -match '^name=\^/(.+)\$$' -and $global:Dim0Task189PreflightMock.Containers.ContainsKey($Matches[1])) {
                return $global:Dim0Task189PreflightMock.Containers[$Matches[1]].id
            }
        }
        'inspect' {
            $id = $dockerArguments[-1]
            return @($global:Dim0Task189PreflightMock.Containers.Values | Where-Object id -eq $id)[0].owner
        }
        'port' {
            $container = $dockerArguments[1]
            if ($global:Dim0Task189PreflightMock.PublishedPorts.ContainsKey($container)) {
                return @($global:Dim0Task189PreflightMock.PublishedPorts[$container])
            }
        }
        default { throw "Unexpected docker command in test: $($dockerArguments -join ' ')" }
    }
}

# Emulate host listeners without consulting or changing the real machine state.
function Get-NetTCPConnection {
    param(
        [string]$State,
        [int]$LocalPort,
        [System.Management.Automation.ActionPreference]$ErrorAction
    )

    if ($LocalPort -in $global:Dim0Task189PreflightMock.OccupiedPorts) {
        return [pscustomobject]@{ State = $State; LocalPort = $LocalPort }
    }
}

try {
    New-Item -ItemType Directory -Path $TestRoot | Out-Null
    foreach ($entry in $PortEnvironment.GetEnumerator()) {
        $PreviousPortEnvironment[$entry.Key] = [Environment]::GetEnvironmentVariable($entry.Key)
        [Environment]::SetEnvironmentVariable($entry.Key, $entry.Value.ToString())
    }

    $clean = New-PreflightFixture 'zero-containers'
    Set-PreflightScenario
    & $Helper -Action Preflight -EvidenceRoot $TestRoot -RunDirectory $clean | Out-Null
    Assert-True (Test-Path -LiteralPath (Join-Path $clean '05-compose-ownership-preflight.json')) 'Clean baseline did not produce preflight evidence.'
    Assert-True (@($global:Dim0Task189PreflightMock.DockerCalls | Where-Object { $_[0] -eq 'port' }).Count -eq 0) 'Clean baseline queried ports for absent containers.'

    $owned = New-PreflightFixture 'owned-containers'
    $ownedContainers = @{}
    for ($index = 0; $index -lt $ExpectedContainers.Count; $index++) {
        $ownedContainers[$ExpectedContainers[$index]] = @{ id = "owned-$index"; owner = 'dim0-task189' }
    }
    $ownedPublishedPorts = @{
        'dim0-task189-postgres' = '5432/tcp -> 0.0.0.0:15434'
        'dim0-task189-qdrant' = '6333/tcp -> 0.0.0.0:16335'
        'dim0-task189-redis' = '6379/tcp -> 0.0.0.0:16381'
        'dim0-task189-backend' = '8082/tcp -> 0.0.0.0:18082'
        'dim0-task189-webui' = @('5175/tcp -> 0.0.0.0:15175', '5182/tcp -> 0.0.0.0:15182')
    }
    Set-PreflightScenario -Containers $ownedContainers -PublishedPorts $ownedPublishedPorts -OccupiedPorts $ExpectedPorts
    & $Helper -Action Preflight -EvidenceRoot $TestRoot -RunDirectory $owned | Out-Null
    Assert-True (@($global:Dim0Task189PreflightMock.DockerCalls | Where-Object { $_[0] -eq 'port' }).Count -eq $ExpectedContainers.Count) 'Owned containers were not all queried for published ports.'

    $foreign = New-PreflightFixture 'foreign-name-owner'
    Set-PreflightScenario -Containers @{
        'dim0-task189-postgres' = @{ id = 'foreign-postgres'; owner = 'another-project' }
    }
    Assert-PreflightFails $foreign "Container name 'dim0-task189-postgres' is not owned"

    $collision = New-PreflightFixture 'foreign-port-collision'
    Set-PreflightScenario -OccupiedPorts @(15434)
    Assert-PreflightFails $collision "Host port 15434 is occupied by a process outside 'dim0-task189'."

    $dockerFailure = New-PreflightFixture 'docker-command-failure'
    Set-PreflightScenario -DockerFailure 'ps'
    Assert-PreflightFails $dockerFailure 'docker ps failed during ownership preflight.'

    $dockerPortFailure = New-PreflightFixture 'docker-port-failure'
    Set-PreflightScenario -Containers @{
        'dim0-task189-postgres' = @{ id = 'owned-postgres'; owner = 'dim0-task189' }
    } -DockerFailure 'port'
    Assert-PreflightFails $dockerPortFailure "docker port failed during ownership preflight for 'dim0-task189-postgres'."

    Write-Output 'Task 189 ownership and port preflight regression tests passed.'
}
finally {
    foreach ($entry in $PreviousPortEnvironment.GetEnumerator()) {
        [Environment]::SetEnvironmentVariable($entry.Key, $entry.Value)
    }
    $resolvedTestRoot = [IO.Path]::GetFullPath($TestRoot)
    if ($resolvedTestRoot.StartsWith($TempRoot, [StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedTestRoot)) {
        Remove-Item -LiteralPath $resolvedTestRoot -Recurse -Force
    }
    Remove-Variable -Name Dim0Task189PreflightMock -Scope Global -ErrorAction SilentlyContinue
}
