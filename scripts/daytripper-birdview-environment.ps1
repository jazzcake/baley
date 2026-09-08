[CmdletBinding()]
param(
  [Parameter(Position = 0)]
  [ValidateSet("reseed", "start", "stop", "status", "snapshot", "verify")]
  [string]$Action = "status"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$runtimeRoot = Join-Path $repoRoot ".tmp\daytripper-birdview"
$logRoot = Join-Path $runtimeRoot "logs"
$secretRoot = Join-Path $runtimeRoot "secrets"
$statePath = Join-Path $runtimeRoot "runtime.json"
$snapshotPath = Join-Path $runtimeRoot "daytripper-snapshot.json"
$manifestPath = Join-Path $runtimeRoot "clone-manifest.json"
$verificationPath = Join-Path $runtimeRoot "verification.json"
$reseedResultPath = Join-Path $runtimeRoot "reseed-result.json"
$leaseSecretPath = Join-Path $secretRoot "lease_token_secret"
$reviewPasswordPath = Join-Path $secretRoot "review_password"
$serverBinary = "C:\dev-bin\baley\daytripper-birdview\baley-server.exe"

$sourceContainer = "local-dev-postgres"
$sourceDatabase = "baley"
$sourceUser = "local_admin"
$targetContainer = "baley-bird-view-v2-test"
$preservedDatabase = "baley_v2_test"
$targetDatabase = "baley_daytripper_birdview_test"
$disposableMarker = "BALEY_DISPOSABLE_DAYTRIPPER_BIRDVIEW_CLONE_V1"
$workspaceID = "410f335e-ddb2-443f-be3c-7d1d18ccd534"
$workspaceName = "DayTripper"
$reviewActorID = "17800000-0000-4000-8000-000000000001"
$reviewLoginID = "daytripper-review"
$reviewDisplayName = "DayTripper Bird View Review"
$birdViewID = "17800000-0000-4000-8000-000000000178"
$apiPort = 8182
$viewerPort = 5276
$tailnetViewerPort = 8449
$tailnetAPIPort = 8450
$apiLoopbackURL = "http://127.0.0.1:$apiPort"
$viewerLoopbackURL = "http://127.0.0.1:$viewerPort"

# Task #179 semantic curation contract. Public IDs make the review set readable;
# stable internal target IDs are resolved from the guarded imported Workspace.
$curatedFocusContracts = @(
  [ordered]@{ NodeID="17800000-0000-4000-8000-000000000011"; Title="Intake and canonical foundations"; PhaseIDs=@("intake"); GateIDs=@("integration-ready"); GatePublicIDs=@(1); TaskPublicIDs=@(13,16,17,18,19,21); Dependencies=@("18>16","21>16","16>19","19>17","17>13") },
  [ordered]@{ NodeID="17800000-0000-4000-8000-000000000012"; Title="Foundation Proof delivery"; PhaseIDs=@("foundation-proof"); GateIDs=@("foundation-evidence-ready"); GatePublicIDs=@(2); TaskPublicIDs=@(2,3,7,8,9,13,15,23,29,31,39,40,41,42); Dependencies=@("13>2","15>3","7>8","8>9","23>29","29>31","39>40","40>41","40>42") },
  [ordered]@{ NodeID="17800000-0000-4000-8000-000000000013"; Title="PlaceMatch transition"; PhaseIDs=@("foundation-proof"); GateIDs=@(); GatePublicIDs=@(); TaskPublicIDs=@(43,44,45,46,47,48,49,50,51,52); Dependencies=@("43>44","44>45","45>46","46>47","47>48","48>49","49>50","50>51","51>52") },
  [ordered]@{ NodeID="17800000-0000-4000-8000-000000000014"; Title="Jeju Map v1"; PhaseIDs=@("jeju-map-v1"); GateIDs=@("jeju-map-v1-reviewed"); GatePublicIDs=@(3); TaskPublicIDs=@(2,3,4,5,9,10,11,12,14); Dependencies=@("2>4","3>5","4>5","9>10","10>11","11>14") },
  [ordered]@{ NodeID="17800000-0000-4000-8000-000000000015"; Title="Curation and plan prototype"; PhaseIDs=@("curation-plan-prototype"); GateIDs=@("jeju-map-v1-reviewed"); GatePublicIDs=@(3); TaskPublicIDs=@(5,6); Dependencies=@("5>6") }
)
$curatedOverlapTaskPublicIDs = @(2,3,5,9,13)

function Invoke-Checked {
  param([string]$FilePath, [string[]]$Arguments, [string]$FailureMessage)
  $previousPreference = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  try {
    $output = @(& $FilePath @Arguments 2>&1)
    $exitCode = $LASTEXITCODE
  } finally {
    $ErrorActionPreference = $previousPreference
  }
  if ($exitCode -ne 0) {
    throw "$FailureMessage (exit $exitCode): $($output -join [Environment]::NewLine)"
  }
  return $output
}

function Invoke-DockerText {
  param([string[]]$Arguments)
  return ((Invoke-Checked docker $Arguments "docker command failed") -join [Environment]::NewLine).Trim()
}

function Get-CuratedTaskIDMap {
  $allPublicIDs = @($curatedFocusContracts | ForEach-Object { $_.TaskPublicIDs } | Sort-Object -Unique)
  $query = "SELECT public_id,id FROM tasks WHERE workspace_id='$workspaceID' AND public_id IN ($($allPublicIDs -join ',')) ORDER BY public_id"
  $rows = Invoke-DockerText @("exec", $targetContainer, "psql", "-X", "-q", "-A", "-t", "-F", "|", "-U", (Get-TargetConnection).User, "-d", $targetDatabase, "-v", "ON_ERROR_STOP=1", "-c", $query)
  $map = @{}
  foreach ($line in @($rows -split "`r?`n")) {
    if ($line -notmatch '^([0-9]+)\|(.+)$') { throw "unexpected curated Task lookup row: $line" }
    $map[[int]$Matches[1]] = $Matches[2]
  }
  if ($map.Count -ne $allPublicIDs.Count) { throw "curated Task lookup found $($map.Count) of $($allPublicIDs.Count) Tasks" }
  return $map
}

function Assert-CuratedContract {
  $counts = @{}
  foreach ($contract in $curatedFocusContracts) {
    if (@($contract.TaskPublicIDs).Count -eq 0) { throw "curated focus is empty: $($contract.Title)" }
    foreach ($publicID in $contract.TaskPublicIDs) { $counts[$publicID] = 1 + [int]($counts[$publicID]) }
  }
  $overlaps = @($counts.Keys | Where-Object { $counts[$_] -gt 1 } | ForEach-Object { [int]$_ } | Sort-Object)
  if (($overlaps -join ',') -ne ($curatedOverlapTaskPublicIDs -join ',')) { throw "curated overlap set changed: $($overlaps -join ',')" }
  if ($counts.Count -ne 36) { throw "curated coverage changed from 36 unique Tasks to $($counts.Count)" }
}

function Assert-ContainerRunning {
  param([string]$Name)
  $running = Invoke-DockerText @("inspect", "--format", "{{.State.Running}}", $Name)
  if ($running -ne "true") { throw "required container is not running: $Name" }
}

function Get-TargetConnection {
  Assert-ContainerRunning $targetContainer
  $port = Invoke-DockerText @("port", $targetContainer, "5432/tcp")
  if ($port -notmatch '^127\.0\.0\.1:([0-9]+)$') {
    throw "target PostgreSQL must be published on loopback only; observed: $port"
  }
  $hostPort = [int]$Matches[1]
  $rawEnvironment = Invoke-DockerText @("inspect", "--format", "{{range .Config.Env}}{{println .}}{{end}}", $targetContainer)
  $environment = @($rawEnvironment -split "`r?`n")
  $userLine = $environment | Where-Object { $_ -like "POSTGRES_USER=*" } | Select-Object -First 1
  $passwordLine = $environment | Where-Object { $_ -like "POSTGRES_PASSWORD=*" } | Select-Object -First 1
  if ([string]::IsNullOrWhiteSpace($userLine) -or [string]::IsNullOrWhiteSpace($passwordLine)) {
    throw "target container does not expose its local bootstrap user/password"
  }
  $user = $userLine.Substring("POSTGRES_USER=".Length)
  $password = $passwordLine.Substring("POSTGRES_PASSWORD=".Length)
  return [pscustomobject]@{
    User = $user
    Password = $password
    HostPort = $hostPort
    URL = "postgres://$([uri]::EscapeDataString($user)):$([uri]::EscapeDataString($password))@127.0.0.1:$hostPort/$targetDatabase`?sslmode=disable"
  }
}

function Get-TailnetURLs {
  $status = (Invoke-Checked tailscale @("status", "--json") "tailscale status failed") -join [Environment]::NewLine | ConvertFrom-Json
  $dnsName = [string]$status.Self.DNSName
  if ([string]::IsNullOrWhiteSpace($dnsName)) { throw "Tailscale DNS name is unavailable" }
  $dnsName = $dnsName.TrimEnd('.')
  return [pscustomobject]@{
    DNSName = $dnsName
    Viewer = "https://${dnsName}:$tailnetViewerPort"
    API = "https://${dnsName}:$tailnetAPIPort"
  }
}

function Set-ChildEnvironment {
  param([hashtable]$Values)
  $previous = @{}
  foreach ($name in $Values.Keys) {
    $previous[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
    [Environment]::SetEnvironmentVariable($name, $Values[$name], "Process")
  }
  return $previous
}

function Restore-ChildEnvironment {
  param([hashtable]$Previous)
  foreach ($name in $Previous.Keys) {
    [Environment]::SetEnvironmentVariable($name, $Previous[$name], "Process")
  }
}

function New-RandomSecret {
  param([int]$Bytes = 32)
  $value = [byte[]]::new($Bytes)
  $generator = [Security.Cryptography.RandomNumberGenerator]::Create()
  try { $generator.GetBytes($value) } finally { $generator.Dispose() }
  return [Convert]::ToBase64String($value).TrimEnd('=').Replace('+', '-').Replace('/', '_')
}

function Export-DayTripperSnapshot {
  Assert-ContainerRunning $sourceContainer
  New-Item -ItemType Directory -Force -Path $runtimeRoot | Out-Null
  $sql = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot "daytripper-birdview-source-snapshot.sql")
  $previousPreference = $ErrorActionPreference
  $ErrorActionPreference = "Continue"
  try {
    $output = @($sql | & docker exec -i $sourceContainer psql -X -q -A -t -U $sourceUser -d $sourceDatabase 2>&1)
    $exitCode = $LASTEXITCODE
  } finally { $ErrorActionPreference = $previousPreference }
  if ($exitCode -ne 0) { throw "DayTripper read-only snapshot failed: $($output -join [Environment]::NewLine)" }
  if ($output.Count -ne 1) { throw "snapshot exporter returned $($output.Count) lines instead of one manifest document" }
  $snapshot = $output[0] | ConvertFrom-Json
  if ($snapshot.source.workspaceId -ne $workspaceID -or $snapshot.source.workspaceName -ne $workspaceName -or [int]$snapshot.source.schemaVersion -ne 25) {
    throw "snapshot identity or schema guard failed"
  }
  [IO.File]::WriteAllText($snapshotPath, $output[0] + "`n", [Text.UTF8Encoding]::new($false))
  $counts = [ordered]@{}
  foreach ($property in $snapshot.tables.psobject.Properties) { $counts[$property.Name] = @($property.Value).Count }
  $snapshotHash = (Get-FileHash -LiteralPath $snapshotPath -Algorithm SHA256).Hash.ToLowerInvariant()
  [ordered]@{
    formatVersion = 1
    snapshotSHA256 = $snapshotHash
    capturedAt = $snapshot.capturedAt
    source = $snapshot.source
    policy = $snapshot.policy
    counts = $counts
  } | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $manifestPath -Encoding utf8
  return Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
}

function Invoke-SourceSummary {
  $sql = @"
SET default_transaction_read_only=on;
SELECT jsonb_build_object(
  'workspaceRevision',(SELECT revision FROM workspaces WHERE id='$workspaceID' AND name='$workspaceName'),
  'schemaVersion',(SELECT max(version_id) FILTER (WHERE is_applied) FROM goose_db_version),
  'phases',(SELECT count(*) FROM phases WHERE workspace_id='$workspaceID'),
  'lanes',(SELECT count(*) FROM lanes WHERE workspace_id='$workspaceID'),
  'tasks',(SELECT count(*) FROM tasks WHERE workspace_id='$workspaceID'),
  'task_dependencies',(SELECT count(*) FROM task_dependencies WHERE workspace_id='$workspaceID'),
  'gates',(SELECT count(*) FROM gates WHERE workspace_id='$workspaceID'),
  'gate_tasks',(SELECT count(*) FROM gate_tasks WHERE workspace_id='$workspaceID'),
  'backlog_items',(SELECT count(*) FROM backlog_items WHERE workspace_id='$workspaceID'),
  'runs',(SELECT count(*) FROM runs WHERE workspace_id='$workspaceID'),
  'task_record_indexes',(SELECT count(*) FROM task_record_indexes WHERE workspace_id='$workspaceID')
)::text;
"@
  $value = Invoke-DockerText @("exec", $sourceContainer, "psql", "-X", "-q", "-A", "-t", "-U", $sourceUser, "-d", $sourceDatabase, "-v", "ON_ERROR_STOP=1", "-c", $sql)
  return $value | ConvertFrom-Json
}

function Invoke-PreservedDatabaseSummary {
  $target = Get-TargetConnection
  $sql = "SELECT jsonb_build_object('schemaVersion',(SELECT max(version_id) FILTER (WHERE is_applied) FROM goose_db_version),'workspaces',(SELECT count(*) FROM workspaces),'workspaceRevisionSum',(SELECT coalesce(sum(revision),0) FROM workspaces),'tasks',(SELECT count(*) FROM tasks),'birdViews',(SELECT count(*) FROM bird_views))::text;"
  $value = Invoke-DockerText @("exec", $targetContainer, "psql", "-X", "-q", "-A", "-t", "-U", $target.User, "-d", $preservedDatabase, "-v", "ON_ERROR_STOP=1", "-c", $sql)
  return $value | ConvertFrom-Json
}

function Recreate-TargetDatabase {
  if ($sourceContainer -eq $targetContainer -or $sourceDatabase -eq $targetDatabase -or $targetDatabase -ne "baley_daytripper_birdview_test") {
    throw "source and target isolation guard failed"
  }
  $target = Get-TargetConnection
  $exists = Invoke-DockerText @("exec", $targetContainer, "psql", "-X", "-q", "-A", "-t", "-U", $target.User, "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "SELECT count(*) FROM pg_database WHERE datname='$targetDatabase';")
  if ($exists -eq "1") {
    $marker = Invoke-DockerText @("exec", $targetContainer, "psql", "-X", "-q", "-A", "-t", "-U", $target.User, "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "SELECT coalesce(shobj_description(oid,'pg_database'),'') FROM pg_database WHERE datname='$targetDatabase';")
    if ($marker -ne $disposableMarker) {
      throw "refusing to replace unmarked target database $targetDatabase"
    }
    Invoke-DockerText @("exec", $targetContainer, "dropdb", "--force", "-U", $target.User, $targetDatabase) | Out-Null
  }
  Invoke-DockerText @("exec", $targetContainer, "createdb", "-U", $target.User, "-T", "template0", "--encoding=UTF8", $targetDatabase) | Out-Null
  Invoke-DockerText @("exec", $targetContainer, "psql", "-X", "-q", "-U", $target.User, "-d", "postgres", "-v", "ON_ERROR_STOP=1", "-c", "COMMENT ON DATABASE $targetDatabase IS '$disposableMarker';") | Out-Null
}

function Build-Server {
  $binaryRoot = Split-Path -Parent $serverBinary
  New-Item -ItemType Directory -Force -Path $binaryRoot | Out-Null
  $commit = (Invoke-Checked git @("-C", $repoRoot, "rev-parse", "HEAD") "Git commit could not be resolved" | Select-Object -First 1).Trim()
  $builtAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
  $ldflags = "-s -w -X main.buildVersion=daytripper-$($commit.Substring(0,12)) -X main.buildCommit=$commit -X main.buildTime=$builtAt"
  Push-Location (Join-Path $repoRoot "server")
  try { Invoke-Checked go @("build", "-trimpath", "-ldflags", $ldflags, "-o", $serverBinary, "./cmd/baley-server") "Baley server build failed" | Out-Null }
  finally { Pop-Location }
}

function Migrate-TargetDatabase {
  $target = Get-TargetConnection
  $previous = Set-ChildEnvironment @{
    BALEY_DATABASE_URL = $target.URL
    BALEY_DATABASE_URL_FILE = $null
    BALEY_MIGRATIONS_DIR = (Join-Path $repoRoot "server\migrations")
  }
  try { Invoke-Checked $serverBinary @("migrate", "up") "target migration failed" | Out-Null }
  finally { Restore-ChildEnvironment $previous }
  $version = Invoke-DockerText @("exec", $targetContainer, "psql", "-X", "-q", "-A", "-t", "-U", $target.User, "-d", $targetDatabase, "-v", "ON_ERROR_STOP=1", "-c", "SELECT max(version_id) FILTER (WHERE is_applied) FROM goose_db_version;")
  if ($version -ne "27") { throw "target schema version is $version instead of 27" }
}

function Import-TargetSnapshot {
  if (!(Test-Path -LiteralPath $snapshotPath -PathType Leaf)) { throw "snapshot file is missing" }
  $target = Get-TargetConnection
  $remoteSnapshot = "/tmp/daytripper-birdview-$([guid]::NewGuid().ToString('N')).json"
  if ($remoteSnapshot -notmatch '^/tmp/daytripper-birdview-[0-9a-f]{32}\.json$') { throw "unsafe remote snapshot path" }
  try {
    Invoke-DockerText @("cp", $snapshotPath, "${targetContainer}:$remoteSnapshot") | Out-Null
    $sql = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot "daytripper-birdview-import.sql")
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
      $output = @($sql | & docker exec -i $targetContainer psql -X -q -U $target.User -d $targetDatabase -v ON_ERROR_STOP=1 -v "snapshot_path=$remoteSnapshot" -v "expected_workspace_id=$workspaceID" -v "expected_workspace_name=$workspaceName" -v "review_actor_id=$reviewActorID" -v "review_display_name=$reviewDisplayName" 2>&1)
      $exitCode = $LASTEXITCODE
    } finally { $ErrorActionPreference = $previousPreference }
    if ($exitCode -ne 0) { throw "target import failed: $($output -join [Environment]::NewLine)" }
  } finally {
    try { Invoke-DockerText @("exec", $targetContainer, "rm", "-f", "--", $remoteSnapshot) | Out-Null } catch {}
  }
}

function Bootstrap-ReviewAccount {
  New-Item -ItemType Directory -Force -Path $secretRoot | Out-Null
  $password = New-RandomSecret 27
  [IO.File]::WriteAllText($reviewPasswordPath, $password + "`n", [Text.UTF8Encoding]::new($false))
  $target = Get-TargetConnection
  $previous = Set-ChildEnvironment @{ BALEY_DATABASE_URL = $target.URL; BALEY_DATABASE_URL_FILE = $null }
  $bootstrapInputPath = Join-Path $secretRoot "account-bootstrap-input.txt"
  $bootstrapOutputPath = Join-Path $logRoot "account-bootstrap.stdout.log"
  $bootstrapErrorPath = Join-Path $logRoot "account-bootstrap.stderr.log"
  try {
    New-Item -ItemType Directory -Force -Path $logRoot | Out-Null
    [IO.File]::WriteAllText($bootstrapInputPath, "$password`n$password`n", [Text.UTF8Encoding]::new($false))
    $process = Start-Process -FilePath $serverBinary -ArgumentList @("account-bootstrap", $workspaceID, $reviewActorID, $reviewLoginID, $reviewDisplayName) -WindowStyle Hidden -PassThru -Wait -RedirectStandardInput $bootstrapInputPath -RedirectStandardOutput $bootstrapOutputPath -RedirectStandardError $bootstrapErrorPath
    if ($process.ExitCode -ne 0) {
      throw "review account bootstrap failed: $(Get-Content -Raw -LiteralPath $bootstrapOutputPath) $(Get-Content -Raw -LiteralPath $bootstrapErrorPath)"
    }
  } finally {
    $password = $null
    if (Test-Path -LiteralPath $bootstrapInputPath) { Remove-Item -LiteralPath $bootstrapInputPath -Force }
    Restore-ChildEnvironment $previous
  }
}

function Get-Listener {
  param([int]$Port)
  return @(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue)
}

function Assert-PortFree {
  param([int]$Port)
  $listeners = @(Get-Listener $Port)
  if ($listeners.Count -gt 0) {
    throw "port $Port is already owned by process $(($listeners | Select-Object -ExpandProperty OwningProcess -Unique) -join ',')"
  }
}

function Wait-HTTP {
  param([string]$URL, [int]$Attempts = 60)
  for ($attempt = 1; $attempt -le $Attempts; $attempt++) {
    try {
      $response = Invoke-WebRequest -UseBasicParsing -Uri $URL -TimeoutSec 2
      if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300) { return }
    } catch {
      if ($attempt -eq $Attempts) { throw }
    }
    Start-Sleep -Milliseconds 500
  }
}

function Read-State {
  if (!(Test-Path -LiteralPath $statePath -PathType Leaf)) { return $null }
  return Get-Content -Raw -LiteralPath $statePath | ConvertFrom-Json
}

function Stop-RecordedProcess {
  param([int]$ProcessID, [ValidateSet("server", "viewer")][string]$Kind)
  $process = Get-CimInstance Win32_Process -Filter "ProcessId=$ProcessID" -ErrorAction SilentlyContinue
  if ($null -eq $process) { return }
  $actual = if ($Kind -eq "server") { [string]$process.ExecutablePath } else { [string]$process.CommandLine }
  $expected = if ($Kind -eq "server") { $serverBinary } else { $repoRoot }
  $expectedPort = if ($Kind -eq "server") { $apiPort } else { $viewerPort }
  $matches = ![string]::IsNullOrWhiteSpace($actual) -and $actual.IndexOf($expected, [StringComparison]::OrdinalIgnoreCase) -ge 0
  $ownsPort = @((Get-Listener $expectedPort) | Where-Object { $_.OwningProcess -eq $ProcessID }).Count -gt 0
  if (!$matches -or !$ownsPort) { throw "refusing to stop PID $ProcessID because it is not the recorded $Kind runtime" }
  Stop-Process -Id $ProcessID -Force
}

function Stop-Runtime {
  $state = Read-State
  if ($null -eq $state) { Write-Output "DayTripper Bird View runtime is not managed by this script."; return }
  Stop-RecordedProcess ([int]$state.viewerProcessId) "viewer"
  Stop-RecordedProcess ([int]$state.serverProcessId) "server"
  Remove-Item -LiteralPath $statePath -Force
  Write-Output "Stopped DayTripper Bird View Viewer and API; databases and Tailscale routes remain intact."
}

function Invoke-BirdViewCommand {
  param(
    [Microsoft.PowerShell.Commands.WebRequestSession]$Session,
    [string]$CSRFToken,
    [string]$Name,
    [hashtable]$Arguments,
    [long]$ExpectedRevision,
    [string]$IdempotencyKey
  )
  $body = @{
    name = $Name
    arguments = $Arguments
    envelope = @{
      idempotencyKey = $IdempotencyKey
      expectedBirdViewRevision = $ExpectedRevision
      executedByActorId = $reviewActorID
    }
  } | ConvertTo-Json -Depth 12 -Compress
  return Invoke-RestMethod -Uri "$apiLoopbackURL/v1/commands/execute" -Method Post -ContentType "application/json" -Headers @{ Origin = $viewerLoopbackURL; "X-Baley-CSRF" = $CSRFToken } -WebSession $Session -Body $body
}

function Seed-BirdView {
  Assert-CuratedContract
  $password = (Get-Content -Raw -LiteralPath $reviewPasswordPath).Trim()
  $session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
  $loginBody = @{ loginId = $reviewLoginID; password = $password } | ConvertTo-Json -Compress
  $login = Invoke-RestMethod -Uri "$apiLoopbackURL/v1/auth/login" -Method Post -ContentType "application/json" -Headers @{ Origin = $viewerLoopbackURL } -WebSession $session -Body $loginBody
  $password = $null
  $csrf = [string]$login.csrfToken
  if ([string]::IsNullOrWhiteSpace($csrf)) { throw "isolated login did not return a CSRF token" }
  $revision = 0L
  $created = Invoke-BirdViewCommand $session $csrf "bird_view.create" @{ birdViewId=$birdViewID; title="DayTripper delivery map"; description="A realistic Bird View sample derived from the DayTripper phase and delivery graph." } $revision "daytripper-create-v1"
  $revision = [long]$created.birdViewRevision

  $nodes = @(
    @{ id="17800000-0000-4000-8000-000000000011"; title="Intake and canonical foundations"; summary="Confirmed intake decisions and canonical data contracts"; content="Derived from the completed Intake phase and G#1."; x=40; y=120 },
    @{ id="17800000-0000-4000-8000-000000000012"; title="Foundation Proof delivery"; summary="The active proof phase across data, web, and FeedGrid"; content="Derived from the active Foundation Proof phase and outgoing G#2."; x=420; y=40 },
    @{ id="17800000-0000-4000-8000-000000000013"; title="PlaceMatch transition"; summary="A focused ten-task migration stream inside Foundation Proof"; content="Derived from Task #43; undirected dependency closure contains ten Tasks."; x=460; y=330 },
    @{ id="17800000-0000-4000-8000-000000000014"; title="Jeju Map v1"; summary="Planned map release outcomes and the G#3 review boundary"; content="Derived from the Jeju Map v1 phase and its outgoing Gate."; x=850; y=120 },
    @{ id="17800000-0000-4000-8000-000000000015"; title="Curation and plan prototype"; summary="The downstream planning experience after map validation"; content="Derived from the final planned phase."; x=1240; y=120 }
  )
  foreach ($node in $nodes) {
    $result = Invoke-BirdViewCommand $session $csrf "bird_view.node.create" @{ birdViewId=$birdViewID; nodeId=$node.id; title=$node.title; summary=$node.summary; content=$node.content; positionX=$node.x; positionY=$node.y } $revision "daytripper-node-$($node.id)-v1"
    $revision = [long]$result.birdViewRevision
  }
  $edges = @(
    @{ id="17800000-0000-4000-8000-000000000101"; from=$nodes[0].id; to=$nodes[1].id; label="G#1 passed" },
    @{ id="17800000-0000-4000-8000-000000000102"; from=$nodes[1].id; to=$nodes[2].id; label="focused stream" },
    @{ id="17800000-0000-4000-8000-000000000103"; from=$nodes[1].id; to=$nodes[3].id; label="G#2" },
    @{ id="17800000-0000-4000-8000-000000000104"; from=$nodes[3].id; to=$nodes[4].id; label="G#3" }
  )
  foreach ($edge in $edges) {
    $result = Invoke-BirdViewCommand $session $csrf "bird_view.edge.connect" @{ birdViewId=$birdViewID; edgeId=$edge.id; fromNodeId=$edge.from; toNodeId=$edge.to; label=$edge.label } $revision "daytripper-edge-$($edge.id)-v1"
    $revision = [long]$result.birdViewRevision
  }
  $taskIDMap = Get-CuratedTaskIDMap
  foreach ($contract in $curatedFocusContracts) {
    $bindings = @()
    foreach ($phaseID in $contract.PhaseIDs) { $bindings += @{targetType="phase";workspaceId=$workspaceID;targetId=$phaseID} }
    foreach ($gateID in $contract.GateIDs) { $bindings += @{targetType="gate";workspaceId=$workspaceID;targetId=$gateID} }
    foreach ($publicID in $contract.TaskPublicIDs) { $bindings += @{targetType="task";workspaceId=$workspaceID;targetId=$taskIDMap[[int]$publicID]} }
    $result = Invoke-BirdViewCommand $session $csrf "bird_view.overlay.replace" @{ birdViewId=$birdViewID; nodeId=$contract.NodeID; bindings=$bindings } $revision "daytripper-overlay-$($contract.NodeID)-v2"
    $revision = [long]$result.birdViewRevision
  }
  if ($revision -ne 15) { throw "seeded Bird View revision is $revision instead of 15" }
  return [pscustomobject]@{ Revision=$revision; AccountID=[string]$login.account.id; NodeIDs=@($nodes.id) }
}

function Assert-TailnetRouteAvailable {
  param([string]$DNSName, [int]$Port, [string]$Target)
  $status = ((Invoke-Checked tailscale @("serve", "status", "--json") "tailscale serve status failed") -join [Environment]::NewLine) | ConvertFrom-Json
  $key = "${DNSName}:$Port"
  $entry = $status.Web.psobject.Properties[$key]
  if ($null -eq $entry) { return }
  $handler = $entry.Value.Handlers.psobject.Properties['/']
  $proxy = if ($null -eq $handler) { "" } else { [string]$handler.Value.Proxy }
  if ($proxy -ne $Target) { throw "Tailnet HTTPS port $Port is already routed to $proxy" }
}

function Configure-TailnetRoutes {
  param([pscustomobject]$Tailnet)
  Assert-TailnetRouteAvailable $Tailnet.DNSName $tailnetViewerPort $viewerLoopbackURL
  Assert-TailnetRouteAvailable $Tailnet.DNSName $tailnetAPIPort $apiLoopbackURL
  Invoke-Checked tailscale @("serve", "--https=$tailnetViewerPort", "--bg", "--yes", $viewerLoopbackURL) "Tailscale Viewer route failed" | Out-Null
  Invoke-Checked tailscale @("serve", "--https=$tailnetAPIPort", "--bg", "--yes", $apiLoopbackURL) "Tailscale API route failed" | Out-Null
}

function Start-Runtime {
  param([bool]$SeedView)
  if (Test-Path -LiteralPath $statePath) { throw "managed runtime state already exists; run stop or status first" }
  Assert-PortFree $apiPort
  Assert-PortFree $viewerPort
  if (!(Test-Path -LiteralPath $reviewPasswordPath -PathType Leaf)) { throw "review password is missing; run reseed" }
  New-Item -ItemType Directory -Force -Path $logRoot, $secretRoot | Out-Null
  if (!(Test-Path -LiteralPath $leaseSecretPath -PathType Leaf)) {
    [IO.File]::WriteAllText($leaseSecretPath, (New-RandomSecret 32), [Text.UTF8Encoding]::new($false))
  }
  $target = Get-TargetConnection
  $tailnet = Get-TailnetURLs
  $serverEnvironment = @{
    BALEY_DATABASE_URL = $target.URL
    BALEY_DATABASE_URL_FILE = $null
    BALEY_LEASE_TOKEN_SECRET = $null
    BALEY_LEASE_TOKEN_SECRET_FILE = $leaseSecretPath
    BALEY_ENV = "development"
    BALEY_AUTH_MODE = "enforced"
    BALEY_COOKIE_SECURE = "false"
    BALEY_HTTP_ADDR = "127.0.0.1:$apiPort"
    BALEY_VIEWER_ORIGINS = "$viewerLoopbackURL,$($tailnet.Viewer)"
    BALEY_GOOGLE_OIDC_CLIENT_ID = $null
    BALEY_GOOGLE_OIDC_CLIENT_SECRET = $null
    BALEY_GOOGLE_OIDC_CLIENT_SECRET_FILE = $null
    BALEY_GOOGLE_OIDC_REDIRECT_URL = $null
    BALEY_OIDC_PROVIDERS = $null
    BALEY_OIDC_POST_LOGIN_URL = $null
    BALEY_MIGRATIONS_DIR = (Join-Path $repoRoot "server\migrations")
    VITE_BALEY_API_URL = $tailnet.API
    VITE_BALEY_AUTH_MODE = "enforced"
    VITE_BALEY_LOCAL_REVIEW_LOGIN = "enabled"
  }
  $previous = Set-ChildEnvironment $serverEnvironment
  $serverProcess = $null
  $viewerProcess = $null
  try {
    $serverProcess = Start-Process -FilePath $serverBinary -ArgumentList "serve" -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $logRoot "server.stdout.log") -RedirectStandardError (Join-Path $logRoot "server.stderr.log")
    Wait-HTTP "$apiLoopbackURL/readyz"
    $seed = if ($SeedView) { Seed-BirdView } else { [pscustomobject]@{ Revision=15; AccountID="existing"; NodeIDs=@() } }
    $node = (Get-Command node.exe -ErrorAction Stop).Source
    $vite = Join-Path $repoRoot "node_modules\vite\bin\vite.js"
    $viewerProcess = Start-Process -FilePath $node -ArgumentList @($vite, "--force", "--host", "127.0.0.1", "--port", "$viewerPort", "--strictPort") -WorkingDirectory $repoRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $logRoot "viewer.stdout.log") -RedirectStandardError (Join-Path $logRoot "viewer.stderr.log")
    Wait-HTTP $viewerLoopbackURL
    Configure-TailnetRoutes $tailnet
    [ordered]@{
      startedAt = (Get-Date).ToUniversalTime().ToString("o")
      targetContainer = $targetContainer
      targetDatabase = $targetDatabase
      preservedDatabase = $preservedDatabase
      serverProcessId = $serverProcess.Id
      viewerProcessId = $viewerProcess.Id
      apiLoopbackURL = $apiLoopbackURL
      viewerLoopbackURL = $viewerLoopbackURL
      tailnetAPIURL = $tailnet.API
      tailnetViewerURL = $tailnet.Viewer
      workspaceID = $workspaceID
      birdViewID = $birdViewID
      birdViewRevision = $seed.Revision
      reviewLoginID = $reviewLoginID
      snapshotSHA256 = (Get-FileHash -LiteralPath $snapshotPath -Algorithm SHA256).Hash.ToLowerInvariant()
    } | ConvertTo-Json | Set-Content -LiteralPath $statePath -Encoding utf8
  } catch {
    if ($null -ne $viewerProcess) { Stop-Process -Id $viewerProcess.Id -Force -ErrorAction SilentlyContinue }
    if ($null -ne $serverProcess) { Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue }
    throw
  } finally {
    Restore-ChildEnvironment $previous
  }
  return Get-Content -Raw -LiteralPath $statePath | ConvertFrom-Json
}

function Get-TargetVerification {
  $target = Get-TargetConnection
  $sql = @"
SELECT jsonb_build_object(
  'schemaVersion',(SELECT max(version_id) FILTER (WHERE is_applied) FROM goose_db_version),
  'workspaceId',(SELECT id FROM workspaces),
  'workspaceName',(SELECT name FROM workspaces),
  'workspaceRevision',(SELECT revision FROM workspaces),
  'workspace_counters',(SELECT count(*) FROM workspace_counters),
  'phases',(SELECT count(*) FROM phases),'lanes',(SELECT count(*) FROM lanes),
  'evidence_profiles',(SELECT count(*) FROM evidence_profiles),
  'workspace_acceptance_policies',(SELECT count(*) FROM workspace_acceptance_policies),
  'tasks',(SELECT count(*) FROM tasks),'task_dependencies',(SELECT count(*) FROM task_dependencies),
  'gates',(SELECT count(*) FROM gates),'gate_tasks',(SELECT count(*) FROM gate_tasks),
  'gate_entry_tasks',(SELECT count(*) FROM gate_entry_tasks),'backlog_items',(SELECT count(*) FROM backlog_items),
  'repositories',(SELECT count(*) FROM repositories),'runs',(SELECT count(*) FROM runs),
  'run_git_observations',(SELECT count(*) FROM run_git_observations),'commit_references',(SELECT count(*) FROM commit_references),
  'task_record_indexes',(SELECT count(*) FROM task_record_indexes),
  'task_acceptance_assignments',(SELECT count(*) FROM task_acceptance_assignments),
  'task_acceptance_evidence',(SELECT count(*) FROM task_acceptance_evidence),
  'accounts',(SELECT count(*) FROM accounts),'credentials',(SELECT count(*) FROM account_credentials),
  'memberships',(SELECT count(*) FROM workspace_memberships),'sessions',(SELECT count(*) FROM account_sessions),
  'birdViews',(SELECT count(*) FROM bird_views),'birdViewNodes',(SELECT count(*) FROM bird_view_nodes),
  'birdViewEdges',(SELECT count(*) FROM bird_view_edges),'birdViewBindings',(
    (SELECT count(*) FROM bird_view_phase_bindings)+(SELECT count(*) FROM bird_view_gate_bindings)+
    (SELECT count(*) FROM bird_view_task_bindings)+(SELECT count(*) FROM bird_view_backlog_bindings)),
  'birdViewRevision',(SELECT revision FROM bird_views WHERE id='$birdViewID'),
  'forbiddenRows',(
    (SELECT count(*) FROM account_external_identities)+(SELECT count(*) FROM agent_tokens)+
    (SELECT count(*) FROM approval_grants)+(SELECT count(*) FROM human_approval_attestations)+
    (SELECT count(*) FROM mcp_connection_requests)+(SELECT count(*) FROM mcp_gateway_registrations)+
    (SELECT count(*) FROM oidc_authorization_flows)+(SELECT count(*) FROM auth_login_limits)+
    (SELECT count(*) FROM commands)+
    (SELECT count(*) FROM events)+(SELECT count(*) FROM mutation_attempts)),
  'unsanitizedRuns',(SELECT count(*) FROM runs WHERE session_ref IS NOT NULL OR lease_token_hash NOT LIKE 'redacted:daytripper-snapshot:%'),
  'unexpectedSecurityEvents',(SELECT count(*) FROM security_events
    WHERE event_type NOT IN ('workspace.owner_bootstrapped','authentication.login_succeeded')
       OR actor_id <> '$reviewActorID'
       OR account_id IS DISTINCT FROM (SELECT id FROM accounts)
       OR (event_type='workspace.owner_bootstrapped' AND workspace_id IS DISTINCT FROM '$workspaceID')
       OR (event_type='authentication.login_succeeded' AND workspace_id IS NOT NULL)),
  'unexpectedActors',(SELECT count(*) FROM actors WHERE id <> '$reviewActorID'),
  'unexpectedAccounts',(SELECT count(*) FROM accounts WHERE actor_id <> '$reviewActorID' OR login_id <> '$reviewLoginID' OR normalized_login_id <> '$reviewLoginID'),
  'unexpectedMemberships',(SELECT count(*) FROM workspace_memberships WHERE workspace_id <> '$workspaceID' OR actor_id <> '$reviewActorID' OR role <> 'owner' OR NOT active)
)::text;
"@
  $value = Invoke-DockerText @("exec", $targetContainer, "psql", "-X", "-q", "-A", "-t", "-U", $target.User, "-d", $targetDatabase, "-v", "ON_ERROR_STOP=1", "-c", $sql)
  return $value | ConvertFrom-Json
}

function Verify-Environment {
  $manifest = Get-Content -Raw -LiteralPath $manifestPath | ConvertFrom-Json
  $target = Get-TargetVerification
  $state = Read-State
  $sourceAfter = Invoke-SourceSummary
  $preservedAfter = Invoke-PreservedDatabaseSummary
  $expectedPairs = @{
    workspace_counters="workspace_counters"; phases="phases"; lanes="lanes";
    evidence_profiles="evidence_profiles"; workspace_acceptance_policies="workspace_acceptance_policies";
    tasks="tasks"; task_dependencies="task_dependencies";
    gates="gates"; gate_tasks="gate_tasks"; gate_entry_tasks="gate_entry_tasks"; backlog_items="backlog_items";
    repositories="repositories"; runs="runs"; run_git_observations="run_git_observations";
    commit_references="commit_references"; task_record_indexes="task_record_indexes";
    task_acceptance_assignments="task_acceptance_assignments"; task_acceptance_evidence="task_acceptance_evidence"
  }
  foreach ($key in $expectedPairs.Keys) {
    if ([long]$target.$key -ne [long]$manifest.counts.($expectedPairs[$key])) { throw "target count mismatch for $key" }
  }
  if ([long]$target.schemaVersion -ne 27 -or $target.workspaceId -ne $workspaceID -or $target.workspaceName -ne $workspaceName -or [long]$target.forbiddenRows -ne 0 -or [long]$target.unsanitizedRuns -ne 0 -or [long]$target.unexpectedSecurityEvents -ne 0 -or [long]$target.unexpectedActors -ne 0 -or [long]$target.unexpectedAccounts -ne 0 -or [long]$target.unexpectedMemberships -ne 0) {
    throw "target identity, schema, or security assertions failed"
  }
  if ([long]$target.birdViews -ne 1 -or [long]$target.birdViewNodes -ne 5 -or [long]$target.birdViewEdges -ne 4 -or [long]$target.birdViewBindings -ne 50 -or [long]$target.birdViewRevision -ne 15) {
    throw "Bird View seed assertions failed"
  }
  $apiProof = $null
  if ($null -ne $state) {
    $password = (Get-Content -Raw -LiteralPath $reviewPasswordPath).Trim()
    $webSession = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
    $login = Invoke-RestMethod -Uri "$apiLoopbackURL/v1/auth/login" -Method Post -ContentType "application/json" -Headers @{ Origin=$viewerLoopbackURL } -WebSession $webSession -Body (@{loginId=$reviewLoginID;password=$password}|ConvertTo-Json -Compress)
    $password = $null
    $workspaces = Invoke-RestMethod -Uri "$apiLoopbackURL/v1/workspaces" -WebSession $webSession
    $graph = Invoke-RestMethod -Uri "$apiLoopbackURL/v1/bird-views/$birdViewID/graph" -WebSession $webSession
    $focusProofs = @()
    foreach ($contract in $curatedFocusContracts) {
      $focus = Invoke-RestMethod -Uri "$apiLoopbackURL/v1/bird-views/$birdViewID/nodes/$($contract.NodeID)/focus" -WebSession $webSession
      if (@($focus.workspaces).Count -ne 1) { throw "curated focus Workspace missing: $($contract.Title)" }
      $focusedWorkspace = $focus.workspaces[0]
      $taskPublicByID = @{}
      foreach ($task in @($focusedWorkspace.tasks)) { $taskPublicByID[[string]$task.id] = [int]$task.publicId }
      $actualTasks = @($focusedWorkspace.tasks | ForEach-Object { [int]$_.publicId } | Sort-Object)
      $expectedTasks = @($contract.TaskPublicIDs | Sort-Object)
      $actualGates = @($focusedWorkspace.gates | ForEach-Object { [int]$_.publicId } | Sort-Object)
      $actualDependencies = @($focusedWorkspace.dependencies | ForEach-Object { "$($taskPublicByID[[string]$_.fromTaskId])>$($taskPublicByID[[string]$_.toTaskId])" } | Sort-Object)
      if (($actualTasks -join ',') -ne ($expectedTasks -join ',')) { throw "curated Task mismatch for $($contract.Title): $($actualTasks -join ',')" }
      if (($actualGates -join ',') -ne (@($contract.GatePublicIDs | Sort-Object) -join ',')) { throw "curated Gate mismatch for $($contract.Title): $($actualGates -join ',')" }
      if (($actualDependencies -join ',') -ne (@($contract.Dependencies | Sort-Object) -join ',')) { throw "curated dependency mismatch for $($contract.Title): $($actualDependencies -join ',')" }
      $focusProofs += [ordered]@{ node=$contract.Title; tasks=$actualTasks; gates=$actualGates; dependencies=$actualDependencies; phases=@($focusedWorkspace.phases | ForEach-Object {$_.name}); lanes=@($focusedWorkspace.lanes | ForEach-Object {$_.name}) }
    }
    $apiProof = [ordered]@{
      accountID = $login.account.id
      workspaceCount = @($workspaces.items).Count
      graphNodeCount = @($graph.nodes).Count
      graphEdgeCount = @($graph.edges).Count
      focuses = $focusProofs
    }
    if ($apiProof.workspaceCount -ne 1 -or $apiProof.graphNodeCount -ne 5 -or $apiProof.graphEdgeCount -ne 4 -or @($apiProof.focuses).Count -ne 5) {
      throw "authenticated API proof failed"
    }
  }
  [ordered]@{
    verifiedAt = (Get-Date).ToUniversalTime().ToString("o")
    manifestSHA256 = $manifest.snapshotSHA256
    sourceAfter = $sourceAfter
    preservedDatabaseAfter = $preservedAfter
    target = $target
    api = $apiProof
  } | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $verificationPath -Encoding utf8
  return Get-Content -Raw -LiteralPath $verificationPath | ConvertFrom-Json
}

function Show-Status {
  $state = Read-State
  $apiReady = try { (Invoke-WebRequest -UseBasicParsing "$apiLoopbackURL/readyz" -TimeoutSec 2).StatusCode -eq 200 } catch { $false }
  $viewerReady = try { (Invoke-WebRequest -UseBasicParsing $viewerLoopbackURL -TimeoutSec 2).StatusCode -eq 200 } catch { $false }
  [pscustomobject]@{
    Managed = $null -ne $state
    APIReady = $apiReady
    ViewerReady = $viewerReady
    TargetContainer = $targetContainer
    TargetDatabase = $targetDatabase
    PreservedDatabase = $preservedDatabase
    Viewer = if ($null -eq $state) { $viewerLoopbackURL } else { $state.tailnetViewerURL }
    API = if ($null -eq $state) { $apiLoopbackURL } else { $state.tailnetAPIURL }
    WorkspaceID = $workspaceID
    BirdViewID = $birdViewID
    ReviewLoginID = $reviewLoginID
    PasswordFile = $reviewPasswordPath
  }
}

switch ($Action) {
  "snapshot" { Export-DayTripperSnapshot }
  "reseed" {
    if (Test-Path -LiteralPath $statePath) { Stop-Runtime | Out-Host }
    $preservedBefore = Invoke-PreservedDatabaseSummary
    $sourceBefore = Invoke-SourceSummary
    $manifest = Export-DayTripperSnapshot
    Recreate-TargetDatabase
    Build-Server
    Migrate-TargetDatabase
    Import-TargetSnapshot
    Bootstrap-ReviewAccount
    $runtime = Start-Runtime $true
    $verification = Verify-Environment
    if (($sourceBefore | ConvertTo-Json -Compress) -ne ($verification.sourceAfter | ConvertTo-Json -Compress)) {
      throw "operating source summary changed during the clone window"
    }
    if (($preservedBefore | ConvertTo-Json -Compress) -ne ($verification.preservedDatabaseAfter | ConvertTo-Json -Compress)) {
      throw "preserved $preservedDatabase summary changed during the clone window"
    }
    $result = [ordered]@{
      sourceBefore = $sourceBefore
      sourceAfter = $verification.sourceAfter
      preservedDatabaseBefore = $preservedBefore
      preservedDatabaseAfter = $verification.preservedDatabaseAfter
      manifest = $manifestPath
      verification = $verificationPath
      runtime = $runtime
    }
    $result | ConvertTo-Json -Depth 10 | Set-Content -LiteralPath $reseedResultPath -Encoding utf8
    $result
  }
  "start" { Start-Runtime $false }
  "stop" { Stop-Runtime }
  "verify" { Verify-Environment }
  "status" { Show-Status }
}
