[CmdletBinding()]
param(
  [Parameter(Position = 0)]
  [ValidateSet("start", "stop", "restart", "status")]
  [string]$Action = "status"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$runtimeRoot = Join-Path $repoRoot ".tmp\local-pilot"
$binRoot = Join-Path $runtimeRoot "bin"
$logRoot = Join-Path $runtimeRoot "logs"
$secretRoot = Join-Path $runtimeRoot "secrets"
$statePath = Join-Path $runtimeRoot "runtime.json"
$serverBinary = Join-Path $binRoot "baley-server.exe"
$leaseSecretPath = Join-Path $secretRoot "lease_token_secret"
$databaseURL = "postgres://baley:baley@127.0.0.1:54329/baley?sslmode=disable"
$apiURL = "http://127.0.0.1:8080"
$viewerURL = "http://127.0.0.1:5174"

function Invoke-Checked {
  param([string]$FilePath, [string[]]$Arguments, [string]$FailureMessage)
  & $FilePath @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "$FailureMessage (exit $LASTEXITCODE)"
  }
}

function Get-Listener {
  param([int]$Port)
  @(Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction SilentlyContinue)
}

function Assert-PortFree {
  param([int]$Port)
  $listeners = @(Get-Listener $Port)
  if ($listeners.Count -gt 0) {
    $owners = ($listeners | Select-Object -ExpandProperty OwningProcess -Unique) -join ", "
    throw "port $Port is already owned by process $owners; stop the existing process explicitly"
  }
}

function Wait-HTTP {
  param([string]$URL, [int]$Attempts = 30)
  for ($attempt = 1; $attempt -le $Attempts; $attempt++) {
    try {
      $response = Invoke-WebRequest -UseBasicParsing -Uri $URL -TimeoutSec 2
      if ($response.StatusCode -ge 200 -and $response.StatusCode -lt 300) {
        return
      }
    } catch {
      if ($attempt -eq $Attempts) { throw }
    }
    Start-Sleep -Milliseconds 500
  }
  throw "endpoint did not become ready: $URL"
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

function Read-State {
  if (!(Test-Path -LiteralPath $statePath -PathType Leaf)) { return $null }
  return Get-Content -Raw -LiteralPath $statePath | ConvertFrom-Json
}

function Stop-RecordedProcess {
  param([int]$ProcessID, [ValidateSet("server", "viewer")][string]$Kind)
  $process = Get-CimInstance Win32_Process -Filter "ProcessId=$ProcessID" -ErrorAction SilentlyContinue
  if ($null -eq $process) { return }
  $expected = if ($Kind -eq "server") { $serverBinary } else { $repoRoot }
  $actual = if ($Kind -eq "server") { [string]$process.ExecutablePath } else { [string]$process.CommandLine }
  $commandMatches = ![string]::IsNullOrWhiteSpace($actual) -and
    $actual.IndexOf($expected, [StringComparison]::OrdinalIgnoreCase) -ge 0
  if (!$commandMatches) {
    # Some Windows launch contexts hide ExecutablePath and CommandLine even for
    # same-user children. Fall back only when the recorded PID still has the
    # exact expected process name and owns the managed loopback listener.
    $runtimeProcess = Get-Process -Id $ProcessID -ErrorAction SilentlyContinue
    $expectedName = if ($Kind -eq "server") { "baley-server" } else { "node" }
    $expectedPort = if ($Kind -eq "server") { 8080 } else { 5174 }
    $ownsExpectedListener = @(
      Get-Listener $expectedPort | Where-Object { $_.OwningProcess -eq $ProcessID }
    ).Count -gt 0
    if ($null -eq $runtimeProcess -or $runtimeProcess.ProcessName -ne $expectedName -or !$ownsExpectedListener) {
      throw "refusing to stop PID $ProcessID because it no longer matches the recorded $Kind process"
    }
  }
  Stop-Process -Id $ProcessID -Force
}

function Stop-Runtime {
  $state = Read-State
  if ($null -eq $state) {
    Write-Output "Local Pilot runtime is not managed by this script."
    return
  }
  Stop-RecordedProcess -ProcessID ([int]$state.viewerProcessId) -Kind viewer
  Stop-RecordedProcess -ProcessID ([int]$state.serverProcessId) -Kind server
  Remove-Item -LiteralPath $statePath -Force
  Write-Output "Stopped Local Pilot Viewer and API. PostgreSQL remains running."
}

function Start-Runtime {
  if (Test-Path -LiteralPath $statePath) {
    throw "managed runtime state already exists; run status or stop first"
  }
  Assert-PortFree 8080
  Assert-PortFree 5174

  Invoke-Checked docker @("compose", "-f", (Join-Path $repoRoot "docker-compose.yml"), "up", "-d", "postgres") "PostgreSQL start failed"
  $containerID = ([string](& docker compose -f (Join-Path $repoRoot "docker-compose.yml") ps -q postgres)).Trim()
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($containerID)) {
    throw "PostgreSQL container was not found"
  }
  for ($attempt = 1; $attempt -le 30; $attempt++) {
    $health = (& docker inspect --format "{{.State.Health.Status}}" $containerID).Trim()
    if ($health -eq "healthy") { break }
    if ($attempt -eq 30) { throw "PostgreSQL did not become healthy" }
    Start-Sleep -Milliseconds 500
  }
  $unsafeListeners = Get-Listener 54329 | Where-Object { $_.LocalAddress -notin @("127.0.0.1", "::1") }
  if (@($unsafeListeners).Count -gt 0) {
    throw "PostgreSQL port 54329 is not loopback-only"
  }

  New-Item -ItemType Directory -Force -Path $binRoot, $logRoot, $secretRoot | Out-Null
  if (!(Test-Path -LiteralPath $leaseSecretPath -PathType Leaf)) {
    $secretBytes = [byte[]]::new(32)
    $generator = [Security.Cryptography.RandomNumberGenerator]::Create()
    try {
      $generator.GetBytes($secretBytes)
    } finally {
      $generator.Dispose()
    }
    [IO.File]::WriteAllText($leaseSecretPath, [Convert]::ToBase64String($secretBytes), [Text.UTF8Encoding]::new($false))
  }

  $commit = (& git -C $repoRoot rev-parse HEAD).Trim()
  if ($LASTEXITCODE -ne 0 -or $commit -notmatch "^[0-9a-f]{40}$") { throw "Git commit could not be resolved" }
  $builtAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
  $ldflags = "-s -w -X main.buildVersion=local-$($commit.Substring(0,12)) -X main.buildCommit=$commit -X main.buildTime=$builtAt"
  Push-Location (Join-Path $repoRoot "server")
  try {
    Invoke-Checked go @("build", "-trimpath", "-ldflags", $ldflags, "-o", $serverBinary, "./cmd/baley-server") "Baley server build failed"
  } finally {
    Pop-Location
  }

  $serverEnvironment = @{
    BALEY_DATABASE_URL = $databaseURL
    BALEY_DATABASE_URL_FILE = $null
    BALEY_LEASE_TOKEN_SECRET = $null
    BALEY_LEASE_TOKEN_SECRET_FILE = $leaseSecretPath
    BALEY_ENV = "development"
    BALEY_AUTH_MODE = "enforced"
    BALEY_COOKIE_SECURE = "false"
    BALEY_HTTP_ADDR = "127.0.0.1:8080"
    BALEY_VIEWER_ORIGINS = $viewerURL
    BALEY_MIGRATIONS_DIR = (Join-Path $repoRoot "server\migrations")
    VITE_BALEY_API_URL = $apiURL
    VITE_BALEY_AUTH_MODE = "enforced"
  }
  $previous = Set-ChildEnvironment $serverEnvironment
  $serverProcess = $null
  $viewerProcess = $null
  try {
    Invoke-Checked $serverBinary @("migrate", "up") "Database migration failed"
    $serverProcess = Start-Process -FilePath $serverBinary -ArgumentList "serve" -WindowStyle Hidden -PassThru `
      -RedirectStandardOutput (Join-Path $logRoot "server.stdout.log") `
      -RedirectStandardError (Join-Path $logRoot "server.stderr.log")
    Wait-HTTP "$apiURL/readyz"

    $node = (Get-Command node.exe -ErrorAction Stop).Source
    $vite = Join-Path $repoRoot "node_modules\vite\bin\vite.js"
    $viewerProcess = Start-Process -FilePath $node -ArgumentList @($vite, "--force", "--host", "127.0.0.1", "--port", "5174", "--strictPort") `
      -WorkingDirectory $repoRoot -WindowStyle Hidden -PassThru `
      -RedirectStandardOutput (Join-Path $logRoot "viewer.stdout.log") `
      -RedirectStandardError (Join-Path $logRoot "viewer.stderr.log")
    Wait-HTTP $viewerURL
  } catch {
    if ($null -ne $viewerProcess) { Stop-Process -Id $viewerProcess.Id -Force -ErrorAction SilentlyContinue }
    if ($null -ne $serverProcess) { Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue }
    throw
  } finally {
    Restore-ChildEnvironment $previous
  }

  @{
    commit = $commit
    startedAt = (Get-Date).ToUniversalTime().ToString("o")
    serverProcessId = $serverProcess.Id
    viewerProcessId = $viewerProcess.Id
    apiURL = $apiURL
    viewerURL = $viewerURL
  } | ConvertTo-Json | Set-Content -LiteralPath $statePath -Encoding utf8
  Write-Output "Local Pilot is ready: $viewerURL (API $apiURL)"
}

function Show-Status {
  $state = Read-State
  $databaseContainer = ([string](& docker compose -f (Join-Path $repoRoot "docker-compose.yml") ps -q postgres)).Trim()
  $databaseHealth = if ($databaseContainer) { (& docker inspect --format "{{.State.Health.Status}}" $databaseContainer).Trim() } else { "stopped" }
  $apiReady = $false
  $viewerReady = $false
  try { $apiReady = (Invoke-WebRequest -UseBasicParsing "$apiURL/readyz" -TimeoutSec 2).StatusCode -eq 200 } catch {}
  try { $viewerReady = (Invoke-WebRequest -UseBasicParsing $viewerURL -TimeoutSec 2).StatusCode -eq 200 } catch {}
  [pscustomobject]@{
    Managed = $null -ne $state
    DatabaseHealth = $databaseHealth
    DatabaseLoopbackOnly = @((Get-Listener 54329 | Where-Object { $_.LocalAddress -notin @("127.0.0.1", "::1") })).Count -eq 0
    APIReady = $apiReady
    ViewerReady = $viewerReady
    API = $apiURL
    Viewer = $viewerURL
    Commit = if ($null -ne $state) { $state.commit } else { "unmanaged" }
  }
}

switch ($Action) {
  "start" { Start-Runtime }
  "stop" { Stop-Runtime }
  "restart" { Stop-Runtime; Start-Runtime }
  "status" { Show-Status }
}
