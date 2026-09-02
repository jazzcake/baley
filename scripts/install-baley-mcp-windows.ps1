[CmdletBinding()]
param(
  [string]$ServerURL = "https://jazzcake-home.tail87e929.ts.net/api",
  [string[]]$AdditionalCodexHomes = @()
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
if ($ServerURL.TrimEnd('/') -ne "https://jazzcake-home.tail87e929.ts.net/api") {
  throw "ServerURL must be an approved HTTPS Baley API URL"
}
$installRoot = Join-Path $env:LOCALAPPDATA "Baley\mcp"
$buildRoot = "C:\dev-bin\baley"
$worktreeChanges = git -C $repoRoot status --porcelain
if ($LASTEXITCODE -ne 0) { throw "Unable to inspect the Baley MCP source worktree" }
if ($worktreeChanges) { throw "Commit or stash Baley MCP source changes before creating a release install" }
$releaseID = (git -C $repoRoot rev-parse --short=12 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($releaseID)) { throw "Unable to determine the Baley MCP release ID" }
$binary = Join-Path (Join-Path (Join-Path $buildRoot "releases") $releaseID) "baley-mcp.exe"
$credentialStore = Join-Path $installRoot "credentials.json"
$loopbackAddress = "127.0.0.1:8090"
$loopbackURL = "http://$loopbackAddress/mcp"
$launcher = Join-Path $installRoot "start-loopback-gateway.ps1"
$gatewayPidFile = Join-Path $installRoot "loopback-gateway.pid"
$legacyGatewayPidFile = Join-Path $buildRoot "loopback-gateway.pid"
$taskName = "Baley MCP Gateway"
New-Item -ItemType Directory -Force (Split-Path -Parent $binary) | Out-Null
# Desktop/CLI starts this binary per stdio session. Strip DWARF and symbol
# tables from the release artifact; this changes neither the tokenless
# transport nor its Windows Credential Manager integration.
if (!(Test-Path -LiteralPath $binary -PathType Leaf)) {
  go -C (Join-Path $repoRoot "server") build -trimpath -ldflags "-s -w" -o $binary ./cmd/baley-mcp
  if ($LASTEXITCODE -ne 0) { throw "Baley MCP build failed" }
}
# A per-user scheduled task owns one long-lived loopback Gateway. The launcher
# contains routing metadata only; device credentials stay in Windows Credential
# Manager and no bearer token is written to Codex configuration.
$launcherContent = @"
`$env:BALEY_SERVER_URL = '$($ServerURL.TrimEnd('/'))'
`$env:BALEY_MCP_CREDENTIAL_STORE = '$credentialStore'
`$env:BALEY_MCP_HTTP_ADDR = '$loopbackAddress'
`$gateway = Start-Process -FilePath '$binary' -ArgumentList 'serve-http' -WindowStyle Hidden -PassThru
`$gateway.Id | Set-Content -LiteralPath '$gatewayPidFile' -NoNewline -Encoding ascii
try { Wait-Process -Id `$gateway.Id } finally { Remove-Item -LiteralPath '$gatewayPidFile' -Force -ErrorAction SilentlyContinue }
"@
Set-Content -LiteralPath $launcher -Value $launcherContent -Encoding utf8
$powershell = Join-Path $env:SystemRoot "System32\WindowsPowerShell\v1.0\powershell.exe"
$action = New-ScheduledTaskAction -Execute $powershell -Argument "-NoProfile -WindowStyle Hidden -ExecutionPolicy Bypass -File `"$launcher`""
$trigger = New-ScheduledTaskTrigger -AtLogOn -User $env:USERNAME
$principal = New-ScheduledTaskPrincipal -UserId "$env:USERDOMAIN\$env:USERNAME" -LogonType Interactive -RunLevel Limited
$legacyStdio = @(Get-CimInstance Win32_Process -Filter "Name='baley-mcp.exe'" -ErrorAction SilentlyContinue | Where-Object { $_.CommandLine -notlike "*serve-http*" })
if ($legacyStdio.Count -gt 0) {
  Write-Warning "$($legacyStdio.Count) existing Codex session(s) still use stdio Baley MCP. They may finish naturally; new sessions will use the single loopback Gateway. Cross-process credential-store locking remains enforced during the transition."
}
if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) {
  Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
}
$allowedBuildRoot = [IO.Path]::GetFullPath($buildRoot).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
$allowedInstallRoot = [IO.Path]::GetFullPath($installRoot).TrimEnd([IO.Path]::DirectorySeparatorChar) + [IO.Path]::DirectorySeparatorChar
foreach ($pidFile in @($gatewayPidFile, $legacyGatewayPidFile)) {
  if (Test-Path -LiteralPath $pidFile -PathType Leaf) {
    $gatewayPid = 0
    $gatewayPidText = [string](Get-Content -Raw -LiteralPath $pidFile)
    $gatewayPidText = $gatewayPidText.Trim()
    if ([int]::TryParse($gatewayPidText, [ref]$gatewayPid) -and $gatewayPid -gt 0) {
      $gatewayProcess = Get-CimInstance Win32_Process -Filter "ProcessId=$gatewayPid" -ErrorAction SilentlyContinue
      $gatewayListener = Get-NetTCPConnection -LocalAddress '127.0.0.1' -LocalPort 8090 -State Listen -ErrorAction SilentlyContinue |
        Where-Object { $_.OwningProcess -eq $gatewayPid } |
        Select-Object -First 1
      $gatewayPath = if ($null -ne $gatewayProcess -and ![string]::IsNullOrWhiteSpace($gatewayProcess.ExecutablePath)) {
        [IO.Path]::GetFullPath($gatewayProcess.ExecutablePath)
      } else {
        ""
      }
      $gatewayPathAllowed = [IO.Path]::GetFileName($gatewayPath) -ieq 'baley-mcp.exe' -and (
        $gatewayPath.StartsWith($allowedBuildRoot, [StringComparison]::OrdinalIgnoreCase) -or
        $gatewayPath.StartsWith($allowedInstallRoot, [StringComparison]::OrdinalIgnoreCase)
      )
      $isGatewayCommand = $null -ne $gatewayProcess -and $gatewayProcess.CommandLine -match '(?i)(?:^|\s)serve-http(?:\s|$)'
      if ($gatewayPathAllowed -and $isGatewayCommand -and $null -ne $gatewayListener) {
        Stop-Process -Id $gatewayPid -Force
        Wait-Process -Id $gatewayPid -Timeout 10 -ErrorAction SilentlyContinue
      }
    }
    Remove-Item -LiteralPath $pidFile -Force -ErrorAction SilentlyContinue
  }
}
$listener = Get-NetTCPConnection -LocalAddress '127.0.0.1' -LocalPort 8090 -State Listen -ErrorAction SilentlyContinue | Select-Object -First 1
if ($null -ne $listener) {
  $listenerProcess = Get-CimInstance Win32_Process -Filter "ProcessId=$($listener.OwningProcess)" -ErrorAction SilentlyContinue
  $listenerPath = if ($null -ne $listenerProcess -and ![string]::IsNullOrWhiteSpace($listenerProcess.ExecutablePath)) {
    [IO.Path]::GetFullPath($listenerProcess.ExecutablePath)
  } else {
    ""
  }
  $listenerPathAllowed = [IO.Path]::GetFileName($listenerPath) -ieq 'baley-mcp.exe' -and (
    $listenerPath.StartsWith($allowedBuildRoot, [StringComparison]::OrdinalIgnoreCase) -or
    $listenerPath.StartsWith($allowedInstallRoot, [StringComparison]::OrdinalIgnoreCase)
  )
  $isListenerGatewayCommand = $null -ne $listenerProcess -and $listenerProcess.CommandLine -match '(?i)(?:^|\s)serve-http(?:\s|$)'
  if ($null -eq $listenerProcess -or !$listenerPathAllowed -or !$isListenerGatewayCommand) {
    throw "Port 8090 is owned by an unexpected process; refusing to stop it"
  }
  Stop-Process -Id $listener.OwningProcess -Force
  Wait-Process -Id $listener.OwningProcess -Timeout 10 -ErrorAction SilentlyContinue
}
$stopDeadline = (Get-Date).AddSeconds(10)
while ((Get-Date) -lt $stopDeadline -and (Get-NetTCPConnection -LocalPort 8090 -State Listen -ErrorAction SilentlyContinue)) {
  Start-Sleep -Milliseconds 200
}
if (Get-NetTCPConnection -LocalPort 8090 -State Listen -ErrorAction SilentlyContinue) { throw "Existing Baley loopback Gateway did not stop" }
Register-ScheduledTask -TaskName $taskName -Action $action -Trigger $trigger -Principal $principal -Description "Baley tokenless loopback MCP Gateway" -Force | Out-Null
Start-ScheduledTask -TaskName $taskName
$readyDeadline = (Get-Date).AddSeconds(15)
$ready = $false
while ((Get-Date) -lt $readyDeadline) {
  try {
    $response = Invoke-WebRequest -UseBasicParsing -Uri $loopbackURL -Method Post -ContentType "application/json" -Headers @{ Accept = "application/json, text/event-stream" } -Body '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"baley-installer","version":"1"}}}' -TimeoutSec 3
    if ($response.StatusCode -eq 200 -and $response.Content -match '"serverInfo"') { $ready = $true; break }
  } catch {}
  Start-Sleep -Milliseconds 250
}
if (!$ready) { throw "Baley loopback Gateway did not become ready; existing Codex MCP registration was left unchanged" }

# `codex mcp add` replaces the named registration atomically. Orca supplies a
# private CODEX_HOME to its sessions, while standalone Codex Desktop/CLI use the
# default profile. Register both homes so a reboot or restored Desktop session
# cannot fall back to the retired per-session stdio binary.
$originalCodexHome = [Environment]::GetEnvironmentVariable("CODEX_HOME", "Process")
$defaultCodexHome = Join-Path $env:USERPROFILE ".codex"
$orcaCodexHome = Join-Path $env:APPDATA "orca\codex-runtime-home\home"
$codexHomes = [System.Collections.Generic.List[string]]::new()
foreach ($candidate in @($originalCodexHome, $defaultCodexHome, $(if (Test-Path -LiteralPath $orcaCodexHome -PathType Container) { $orcaCodexHome })) + $AdditionalCodexHomes) {
  if ([string]::IsNullOrWhiteSpace($candidate)) { continue }
  $resolvedCandidate = [IO.Path]::GetFullPath($candidate)
  if (!($codexHomes | Where-Object { $_.Equals($resolvedCandidate, [StringComparison]::OrdinalIgnoreCase) })) {
    $codexHomes.Add($resolvedCandidate)
  }
}
try {
  foreach ($codexHome in $codexHomes) {
    New-Item -ItemType Directory -Force $codexHome | Out-Null
    $env:CODEX_HOME = $codexHome
    codex mcp add baley --url $loopbackURL
    if ($LASTEXITCODE -ne 0) { throw "Codex MCP registration failed for $codexHome" }
    $registration = codex mcp get baley 2>&1
    $registrationText = $registration | Out-String
    if ($LASTEXITCODE -ne 0 -or $registrationText -notmatch [regex]::Escape($loopbackURL) -or $registrationText -notmatch 'transport:\s+streamable_http') {
      throw "Codex MCP registration verification failed for $codexHome"
    }
  }
} finally {
  if ([string]::IsNullOrWhiteSpace($originalCodexHome)) {
    Remove-Item Env:CODEX_HOME -ErrorAction SilentlyContinue
  } else {
    $env:CODEX_HOME = $originalCodexHome
  }
}
Write-Output "Baley MCP is registered and verified through the tokenless loopback Gateway at $loopbackURL for: $($codexHomes -join ', '). Restart Codex Desktop or begin a new CLI session."
