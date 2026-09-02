[CmdletBinding()]
param(
  [string]$ServerURL = "https://jazzcake-home.tail87e929.ts.net/api"
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
  throw "Close the existing Codex Desktop/CLI sessions before switching Baley MCP transport. The current registration was left unchanged to prevent concurrent credential-store writes."
}
if (Get-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue) {
  Stop-ScheduledTask -TaskName $taskName -ErrorAction SilentlyContinue
}
if (Test-Path -LiteralPath $gatewayPidFile -PathType Leaf) {
  $gatewayPid = [int](Get-Content -Raw -LiteralPath $gatewayPidFile)
  $gatewayProcess = Get-CimInstance Win32_Process -Filter "ProcessId=$gatewayPid" -ErrorAction SilentlyContinue
  if ($null -ne $gatewayProcess -and $gatewayProcess.CommandLine -like "*baley-mcp.exe*serve-http*") {
    Stop-Process -Id $gatewayPid -Force
    Wait-Process -Id $gatewayPid -Timeout 10 -ErrorAction SilentlyContinue
  }
  Remove-Item -LiteralPath $gatewayPidFile -Force -ErrorAction SilentlyContinue
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

# `codex mcp add` replaces the named registration atomically. Codex speaks
# Streamable HTTP to the local-only Gateway; no environment variable or
# plaintext Authorization header is retained in config.toml.
codex mcp add baley --url $loopbackURL
if ($LASTEXITCODE -ne 0) { throw "Codex MCP registration failed" }
Write-Output "Baley MCP is registered through the tokenless loopback Gateway at $loopbackURL. Restart Codex Desktop or begin a new CLI session."
