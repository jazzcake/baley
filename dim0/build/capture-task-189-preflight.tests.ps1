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
$global:Dim0Task189PreflightMock = @{
    Containers = @{}
    PublishedPorts = @{}
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
    [IO.File]::WriteAllText(
        (Join-Path $directory 'baseline.env'),
        "API_ORIGIN=http://backend-test:8082`n",
        [Text.UTF8Encoding]::new($false)
    )
    return $directory
}

# Configure deterministic Docker and host-listener responses for one test case.
function Set-PreflightScenario {
    param(
        [hashtable]$Containers = @{},
        [hashtable]$PublishedPorts = @{},
        [string]$DockerFailure
    )

    $global:Dim0Task189PreflightMock.Containers = $Containers
    $global:Dim0Task189PreflightMock.PublishedPorts = $PublishedPorts
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
            $owner = @($global:Dim0Task189PreflightMock.Containers.Values | Where-Object id -eq $id)[0].owner
            return @{ Config = @{ Labels = @{ 'com.docker.compose.project' = $owner } } } | ConvertTo-Json -Compress
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

try {
    New-Item -ItemType Directory -Path $TestRoot | Out-Null

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
    Set-PreflightScenario -Containers $ownedContainers
    & $Helper -Action Preflight -EvidenceRoot $TestRoot -RunDirectory $owned | Out-Null
    Assert-True (@($global:Dim0Task189PreflightMock.DockerCalls | Where-Object { $_[0] -eq 'port' }).Count -eq $ExpectedContainers.Count) 'Owned containers were not all queried for published ports.'
    $ownedEvidence = Get-Content -Raw -LiteralPath (Join-Path $owned '05-compose-ownership-preflight.json') | ConvertFrom-Json
    Assert-True (@($ownedEvidence.expectedHostPorts).Count -eq 0) 'Preflight still reserves host ports.'

    $foreign = New-PreflightFixture 'foreign-name-owner'
    Set-PreflightScenario -Containers @{
        'dim0-task189-postgres' = @{ id = 'foreign-postgres'; owner = 'another-project' }
    }
    Assert-PreflightFails $foreign "Container name 'dim0-task189-postgres' is not owned"

    $published = New-PreflightFixture 'owned-published-port'
    Set-PreflightScenario -Containers $ownedContainers -PublishedPorts @{
        'dim0-task189-backend' = '8082/tcp -> 127.0.0.1:18082'
    }
    Assert-PreflightFails $published "must not publish host ports: 'dim0-task189-backend'"

    $dockerFailure = New-PreflightFixture 'docker-command-failure'
    Set-PreflightScenario -DockerFailure 'ps'
    Assert-PreflightFails $dockerFailure 'docker ps failed during ownership preflight.'

    $dockerPortFailure = New-PreflightFixture 'docker-port-failure'
    Set-PreflightScenario -Containers @{
        'dim0-task189-postgres' = @{ id = 'owned-postgres'; owner = 'dim0-task189' }
    } -DockerFailure 'port'
    Assert-PreflightFails $dockerPortFailure "docker port failed during ownership preflight for 'dim0-task189-postgres'."

    Write-Output 'Task 189 ownership and no-publication preflight regression tests passed.'
}
finally {
    $resolvedTestRoot = [IO.Path]::GetFullPath($TestRoot)
    if ($resolvedTestRoot.StartsWith($TempRoot, [StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedTestRoot)) {
        Remove-Item -LiteralPath $resolvedTestRoot -Recurse -Force
    }
    Remove-Variable -Name Dim0Task189PreflightMock -Scope Global -ErrorAction SilentlyContinue
}
