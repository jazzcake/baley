[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('Initialize', 'Preflight', 'Run', 'ValidateTripwires', 'Finalize')]
    [string]$Action,

    [string]$EvidenceRoot = 'C:\ProgramData\Dim0\validation\task-189',
    [string]$RunDirectory,
    [string]$Name,
    [string]$CommandText,
    [string]$WorkingDirectory,
    [ValidateRange(50, 10000)]
    [int]$MaxLogLines = 1000
)

$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Dim0Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$ExpectedProject = 'dim0-task189'
$CounterKeys = @('llm', 'embedding', 'search', 'fetch', 'ocr', 'image', 'daytona')

function Resolve-ExternalRoot([string]$Path) {
    $resolved = [IO.Path]::GetFullPath($Path)
    $volume = [IO.Path]::GetPathRoot($resolved)
    if ($resolved.StartsWith($RepositoryRoot, [StringComparison]::OrdinalIgnoreCase)) {
        throw 'Task 189 evidence must be stored outside the Git worktree.'
    }
    if ($resolved.TrimEnd('\') -eq $volume.TrimEnd('\')) {
        throw 'Task 189 evidence root cannot be a filesystem volume root.'
    }
    return $resolved
}

function Resolve-RunDirectory {
    if (-not $RunDirectory) { throw '-RunDirectory is required for this action.' }
    $resolved = [IO.Path]::GetFullPath($RunDirectory)
    $root = Resolve-ExternalRoot $EvidenceRoot
    if (-not $resolved.StartsWith(($root.TrimEnd('\') + '\'), [StringComparison]::OrdinalIgnoreCase)) {
        throw "Run directory must be a child of evidence root: $root"
    }
    if (-not (Test-Path -LiteralPath $resolved -PathType Container)) {
        throw "Run directory does not exist: $resolved"
    }
    if (Test-Path -LiteralPath (Join-Path $resolved 'finalized.json')) {
        throw "Run directory is finalized and immutable through this helper: $resolved"
    }
    return $resolved
}

function Write-NewUtf8([string]$Path, [string]$Content) {
    if (Test-Path -LiteralPath $Path) { throw "Refusing to overwrite evidence: $Path" }
    [IO.File]::WriteAllText($Path, $Content, [Text.UTF8Encoding]::new($false))
}

function Add-CommandRecord([string]$Directory, [hashtable]$Record) {
    $recordPath = Join-Path $Directory 'commands.jsonl'
    $line = ($Record | ConvertTo-Json -Compress -Depth 8) + [Environment]::NewLine
    [IO.File]::AppendAllText($recordPath, $line, [Text.UTF8Encoding]::new($false))
}

function Invoke-RecordedCommand {
    param(
        [Parameter(Mandatory = $true)] [string]$Directory,
        [Parameter(Mandatory = $true)] [string]$RecordName,
        [Parameter(Mandatory = $true)] [string]$Text,
        [Parameter(Mandatory = $true)] [string]$Cwd,
        [switch]$AllowFailure
    )

    if ($RecordName -notmatch '^\d{2}-[a-z0-9][a-z0-9-]*$') {
        throw "Evidence name must match NN-lowercase-kebab-case: $RecordName"
    }
    $outputPath = Join-Path $Directory "$RecordName.txt"
    $exitPath = Join-Path $Directory "$RecordName.exit.txt"
    if ((Test-Path $outputPath) -or (Test-Path $exitPath)) {
        throw "Refusing to overwrite command evidence: $RecordName"
    }

    $started = [DateTimeOffset]::Now
    $previous = Get-Location
    $global:LASTEXITCODE = 0
    try {
        Set-Location -LiteralPath $Cwd
        $lines = @(& ([scriptblock]::Create($Text)) 2>&1 | ForEach-Object { $_.ToString() })
        $exitCode = if ($null -eq $LASTEXITCODE) { 0 } else { [int]$LASTEXITCODE }
    }
    catch {
        $lines = @($_.Exception.ToString())
        $exitCode = 1
    }
    finally {
        Set-Location $previous
    }
    $ended = [DateTimeOffset]::Now
    $truncated = $lines.Count -gt $MaxLogLines
    if ($truncated) {
        $lines = @("[truncated: retained final $MaxLogLines lines]") + @($lines | Select-Object -Last $MaxLogLines)
    }
    Write-NewUtf8 $outputPath (($lines -join [Environment]::NewLine) + [Environment]::NewLine)
    Write-NewUtf8 $exitPath ("$exitCode`n")
    Add-CommandRecord $Directory @{
        name = $RecordName; command = $Text; workingDirectory = $Cwd
        startedAt = $started.ToString('o'); endedAt = $ended.ToString('o')
        durationMs = [math]::Round(($ended - $started).TotalMilliseconds)
        exitCode = $exitCode; logLines = $lines.Count; truncated = $truncated
    }
    $lines | Write-Output
    if ($exitCode -ne 0 -and -not $AllowFailure) {
        throw "Evidence command '$RecordName' failed with exit code $exitCode."
    }
}

function Assert-ComposeOwnership([string]$Directory) {
    $expectedContainers = @(
        'dim0-task189-postgres', 'dim0-task189-qdrant', 'dim0-task189-redis',
        'dim0-task189-backend', 'dim0-task189-webui'
    )
    foreach ($container in $expectedContainers) {
        $id = docker ps -aq --filter "name=^/$container$"
        if ($LASTEXITCODE -ne 0) { throw 'docker ps failed during ownership preflight.' }
        if ($id) {
            $owner = docker inspect --format '{{ index .Config.Labels "com.docker.compose.project" }}' $id
            if ($LASTEXITCODE -ne 0 -or $owner -ne $ExpectedProject) {
                throw "Container name '$container' is not owned by Compose project '$ExpectedProject'."
            }
        }
    }

    $portDefaults = @{
        BASELINE_POSTGRES_PORT = 15434; BASELINE_QDRANT_PORT = 16335
        BASELINE_REDIS_PORT = 16381; BASELINE_API_PORT = 18082
        BASELINE_APP_PORT = 15175; BASELINE_MINI_APP_PORT = 15182
    }
    $configuredPorts = @{}
    $baselineEnvPath = Join-Path $Directory 'baseline.env'
    foreach ($line in Get-Content -LiteralPath $baselineEnvPath) {
        if ($line -match '^(BASELINE_[A-Z_]+)=(\d+)$') {
            $configuredPorts[$Matches[1]] = [int]$Matches[2]
        }
    }
    $ports = @($portDefaults.GetEnumerator() | ForEach-Object {
        $configured = [Environment]::GetEnvironmentVariable($_.Key)
        if ($configured) { [int]$configured }
        elseif ($configuredPorts.ContainsKey($_.Key)) { [int]$configuredPorts[$_.Key] }
        else { [int]$_.Value }
    })
    $ownedPorts = @()
    foreach ($container in $expectedContainers) {
        $published = docker port $container 2>$null
        if ($LASTEXITCODE -eq 0) {
            $ownedPorts += @($published | ForEach-Object { if ($_ -match ':(\d+)$') { [int]$Matches[1] } })
        }
    }
    foreach ($port in $ports) {
        $listener = Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue
        if ($listener -and $port -notin $ownedPorts) {
            throw "Host port $port is occupied by a process outside '$ExpectedProject'."
        }
    }
    Write-NewUtf8 (Join-Path $Directory '05-compose-ownership-preflight.json') ((@{
        checkedAt = [DateTimeOffset]::Now.ToString('o'); project = $ExpectedProject
        containers = $expectedContainers; hostPorts = $ports; result = 'clear'
    } | ConvertTo-Json -Depth 4) + "`n")
}

function Assert-TripwireFiles([string]$Directory) {
    foreach ($file in @('provider-invocations.json', 'provider-constructions.json')) {
        $path = Join-Path $Directory $file
        if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing exported tripwire file: $file" }
        $value = Get-Content -Raw -LiteralPath $path | ConvertFrom-Json
        $properties = @($value.PSObject.Properties)
        if (@($properties.Name | Sort-Object) -join ',' -ne @($CounterKeys | Sort-Object) -join ',') {
            throw "Unexpected tripwire schema in $file"
        }
        if (@($properties.Value | Where-Object { $_ -ne 0 }).Count) {
            throw "Provider tripwire is non-zero in $file; the baseline is invalid."
        }
    }
}

function Protect-Secrets([string]$Directory) {
    $patterns = @(
        '(?i)(bearer\s+)[a-z0-9._~+/=-]{12,}',
        '(?i)(sk-(?:proj-|or-v1-)?)[a-z0-9_-]{12,}',
        '(?i)(["'']?[a-z0-9_]*(?:api[_-]?key|token|password|secret)[a-z0-9_]*["'']?\s*[:=]\s*["'']?)(?!\s|["'']?$)[^\s,"'']+'
    )
    $extensions = @('.txt', '.json', '.jsonl', '.yml', '.yaml', '.env', '.har', '.log', '.md')
    $redactions = @()
    foreach ($file in Get-ChildItem -LiteralPath $Directory -File) {
        if ($file.Extension.ToLowerInvariant() -notin $extensions) { continue }
        $content = Get-Content -Raw -LiteralPath $file.FullName
        $count = 0
        foreach ($pattern in $patterns) {
            $content = [regex]::Replace($content, $pattern, {
                param($match)
                $script:matchCount++
                if ($match.Groups.Count -gt 1) { return $match.Groups[1].Value + '<redacted>' }
                return '<redacted>'
            })
            $count += $script:matchCount
            $script:matchCount = 0
        }
        if ($count) {
            [IO.File]::WriteAllText($file.FullName, $content, [Text.UTF8Encoding]::new($false))
            $redactions += @{ file = $file.Name; replacements = $count }
        }
    }
    Write-NewUtf8 (Join-Path $Directory 'secret-screening.json') ((@{
        screenedAt = [DateTimeOffset]::Now.ToString('o'); filesRedacted = $redactions
        result = if ($redactions.Count) { 'redacted' } else { 'clear' }
    } | ConvertTo-Json -Depth 5) + "`n")
}

switch ($Action) {
    'Initialize' {
        $root = Resolve-ExternalRoot $EvidenceRoot
        New-Item -ItemType Directory -Force -Path $root | Out-Null
        $stamp = [DateTimeOffset]::Now.ToString('yyyyMMdd-HHmmss-fff')
        $suffix = [Guid]::NewGuid().ToString('N').Substring(0, 8)
        $directory = Join-Path $root "run-$stamp-$suffix"
        New-Item -ItemType Directory -Path $directory | Out-Null
        $provenance = @{
            schemaVersion = 1; runId = Split-Path $directory -Leaf
            createdAt = [DateTimeOffset]::Now.ToString('o'); repositoryRoot = $RepositoryRoot
            dim0Root = $Dim0Root; host = $env:COMPUTERNAME; user = $env:USERNAME
            powershell = $PSVersionTable.PSVersion.ToString(); project = $ExpectedProject
        }
        Write-NewUtf8 (Join-Path $directory 'run-provenance.json') (($provenance | ConvertTo-Json -Depth 4) + "`n")
        $envFile = Join-Path $directory 'baseline.env'
        $composeRun = $directory.Replace('\', '/')
        $composeEnv = $envFile.Replace('\', '/')
        $nonSecretEnvironment = @(
            "BASELINE_RUN_DIR=$composeRun", "BASELINE_ENV_FILE=$composeEnv",
            'BASELINE_POSTGRES_PORT=15434', 'BASELINE_QDRANT_PORT=16335',
            'BASELINE_REDIS_PORT=16381', 'BASELINE_API_PORT=18082',
            'BASELINE_APP_PORT=15175', 'BASELINE_MINI_APP_PORT=15182',
            'DOPPLER_TOKEN=', 'API_PORT=8082', 'APP_PORT=5175', 'MINI_APP_PORT=5182',
            'API_ORIGIN=http://localhost:18082', 'VITE_API_URL=http://localhost:18082',
            'VITE_HOST_ORIGIN=http://localhost:15175',
            'VITE_MINI_APP_ORIGIN=http://localhost:15182',
            'EMAIL_VERIFICATION_ENABLED=false', 'PASSWORD_RESET_ENABLED=false',
            'GOOGLE_CONNECT_ENABLED=false', 'OPENAI_AGENTS_DISABLE_TRACING=1',
            'OPENAI_AGENTS_DONT_LOG_MODEL_DATA=1', 'OPENAI_AGENTS_DONT_LOG_TOOL_DATA=1',
            'DIM0_BASELINE_PROVIDER_TRIPWIRE=1',
            'DIM0_BASELINE_FAKE_EMBEDDING_DIMENSION=512',
            'OPENAI_API_KEY=', 'ANTHROPIC_API_KEY=', 'OPENROUTER_API_KEY=',
            'MISTRAL_API_KEY=', 'PERPLEXITY_API_KEY=', 'TAVILY_API_KEY=',
            'LINKUP_API_KEY=', 'EXA_API_KEY=', 'DAYTONA_API_KEY='
        )
        Write-NewUtf8 $envFile (($nonSecretEnvironment -join [Environment]::NewLine) + [Environment]::NewLine)
        Invoke-RecordedCommand $directory '01-git-head' 'git rev-parse HEAD' $RepositoryRoot | Out-Null
        Invoke-RecordedCommand $directory '02-git-status-before' 'git status --short -- dim0' $RepositoryRoot | Out-Null
        Invoke-RecordedCommand $directory '03-docker-version' 'docker version' $RepositoryRoot | Out-Null
        Invoke-RecordedCommand $directory '04-compose-version' 'docker compose version' $RepositoryRoot | Out-Null
        @{ runDirectory = $directory; baselineEnv = $envFile } | ConvertTo-Json -Compress
        break
    }
    'Preflight' {
        $directory = Resolve-RunDirectory
        Assert-ComposeOwnership $directory
        Write-Output 'Compose ownership and port preflight passed.'
        break
    }
    'Run' {
        $directory = Resolve-RunDirectory
        if (-not $Name -or -not $CommandText) { throw '-Name and -CommandText are required for Run.' }
        $cwd = if ($WorkingDirectory) { (Resolve-Path $WorkingDirectory).Path } else { $RepositoryRoot }
        Invoke-RecordedCommand $directory $Name $CommandText $cwd
        break
    }
    'ValidateTripwires' {
        $directory = Resolve-RunDirectory
        Assert-TripwireFiles $directory
        Write-Output 'Tripwire counter files are exported, schema-valid, and invocation-free.'
        break
    }
    'Finalize' {
        $directory = Resolve-RunDirectory
        Assert-TripwireFiles $directory
        Protect-Secrets $directory
        $manifest = Join-Path $directory 'manifest.sha256'
        $entries = Get-ChildItem -LiteralPath $directory -File |
            Where-Object Name -notin @('manifest.sha256', 'finalized.json') |
            Sort-Object Name |
            Get-FileHash -Algorithm SHA256 |
            ForEach-Object { "$($_.Hash.ToLowerInvariant())  $(Split-Path $_.Path -Leaf)" }
        Write-NewUtf8 $manifest (($entries -join [Environment]::NewLine) + [Environment]::NewLine)
        Write-NewUtf8 (Join-Path $directory 'finalized.json') ((@{
            finalizedAt = [DateTimeOffset]::Now.ToString('o'); manifest = 'manifest.sha256'
            fileCount = @($entries).Count; overwritePolicy = 'helper-refuses-finalized-run'
        } | ConvertTo-Json) + "`n")
        Write-Output "Finalized non-overwriteable evidence run: $directory"
        break
    }
}
