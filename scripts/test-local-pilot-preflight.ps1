[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$failures = [Collections.Generic.List[string]]::new()

function Add-Check {
  param([string]$Name, [bool]$Passed, [string]$Detail)
  if (!$Passed) { $failures.Add($Name) }
  [pscustomobject]@{ Check = $Name; Result = if ($Passed) { "PASS" } else { "FAIL" }; Detail = $Detail }
}

$checks = @()
$runtime = & (Join-Path $PSScriptRoot "local-pilot-runtime.ps1") status
$checks += Add-Check "managed-runtime" ([bool]$runtime.Managed) "commit=$($runtime.Commit)"
$headCommit = (& git -C $repoRoot rev-parse HEAD).Trim()
$checks += Add-Check "runtime-current-commit" ($runtime.Commit -eq $headCommit) "HEAD=$headCommit"
$checks += Add-Check "database-health" ($runtime.DatabaseHealth -eq "healthy") $runtime.DatabaseHealth
$checks += Add-Check "database-loopback" ([bool]$runtime.DatabaseLoopbackOnly) "127.0.0.1:54329 only"
$checks += Add-Check "api-readiness" ([bool]$runtime.APIReady) $runtime.API
$checks += Add-Check "viewer-readiness" ([bool]$runtime.ViewerReady) $runtime.Viewer

$backupRoot = Join-Path $repoRoot ".tmp\local-pilot\backups"
$latestVerification = Get-ChildItem -LiteralPath $backupRoot -Filter verification.json -Recurse -File -ErrorAction SilentlyContinue |
  Sort-Object LastWriteTimeUtc -Descending | Select-Object -First 1
$recentVerification = $null -ne $latestVerification -and $latestVerification.LastWriteTimeUtc -ge (Get-Date).ToUniversalTime().AddDays(-7)
$checks += Add-Check "recent-restore-verification" $recentVerification $(if ($latestVerification) { $latestVerification.FullName } else { "none" })

$worktree = @(& git -C $repoRoot status --porcelain)
$checks += Add-Check "clean-worktree" ($worktree.Count -eq 0) $(if ($worktree.Count -eq 0) { "clean" } else { "$($worktree.Count) pending entries" })

$serverURL = "http://127.0.0.1:8080"
$workspaceID = "00000000-0000-4000-8000-000000000001"
$environmentPath = Join-Path $repoRoot ".env.baley-mcp.local"
. (Join-Path $PSScriptRoot "baley-mcp-env.ps1")
$agentToken = $null
try {
  $localEnvironment = Read-BaleyMCPEnvironment -Path $environmentPath
  $agentToken = $localEnvironment.BALEY_AGENT_TOKEN
  $localEnvironment = $null
} catch {}
$environmentIgnored = @(& git -C $repoRoot check-ignore -- ".env.baley-mcp.local").Count -eq 1
$checks += Add-Check "mcp-env-gitignored" $environmentIgnored ".env.baley-mcp.local"
$mcpEnvironmentLauncher = $false
$codexCommand = Get-Command codex -ErrorAction SilentlyContinue
if ($null -ne $codexCommand) {
  try {
    $mcpConfigJSON = (& codex mcp get baley --json 2>$null | Out-String)
    if ($LASTEXITCODE -eq 0) {
      $mcpConfig = $mcpConfigJSON | ConvertFrom-Json
      $staticEnvironmentNames = if ($null -eq $mcpConfig.transport.env) {
        @()
      } else {
        @($mcpConfig.transport.env.PSObject.Properties.Name)
      }
      $mcpEnvironmentLauncher =
        $mcpConfig.transport.command -match "(?i)powershell" -and
        @($mcpConfig.transport.args) -contains (Join-Path $PSScriptRoot "run-baley-mcp.ps1") -and
        $staticEnvironmentNames -notcontains "BALEY_AGENT_TOKEN"
    }
  } catch {}
}
$checks += Add-Check "codex-mcp-local-env-launcher" $mcpEnvironmentLauncher "Launcher reads the Git-ignored environment without static token config"
$agentRead = $false
$agentAdminDenied = $false
$workspace = $null
$task124 = $null
$gate4 = $null
if (![string]::IsNullOrWhiteSpace($agentToken)) {
  try {
    $headers = @{ Authorization = "Bearer $agentToken" }
    $workspace = Invoke-RestMethod -Uri "$serverURL/v1/workspaces/$workspaceID" -Headers $headers
    $task124 = Invoke-RestMethod -Uri "$serverURL/v1/workspaces/$workspaceID/tasks/124" -Headers $headers
    $gate4 = Invoke-RestMethod -Uri "$serverURL/v1/workspaces/$workspaceID/gates/G%234/status" -Headers $headers
    $agentRead = $workspace.id -eq $workspaceID
    try {
      Invoke-RestMethod -Uri "$serverURL/v1/workspaces/$workspaceID/members" -Headers $headers | Out-Null
    } catch {
      $agentAdminDenied = $_.Exception.Response.StatusCode.value__ -eq 403
    }
  } catch {}
}
$checks += Add-Check "agent-token-read" $agentRead "Workspace-scoped operator credential"
$checks += Add-Check "agent-admin-denied" $agentAdminDenied "members endpoint returns 403"
$checks += Add-Check "embedding-pilot-active" ($null -ne $workspace -and $workspace.activePhaseId -eq "embedding-pilot") $(if ($workspace) { $workspace.activePhaseId } else { "unavailable" })
$checks += Add-Check "task-124-confirmed" ($null -ne $task124 -and $task124.status -eq "confirmed") $(if ($task124) { $task124.status } else { "unavailable" })
$checks += Add-Check "gate-4-passed" ($null -ne $gate4 -and $gate4.status -eq "passed") $(if ($gate4) { $gate4.status } else { "unavailable" })
$agentToken = $null

$checks | Format-Table -AutoSize
Write-Output "MANUAL: sign in as the Owner and confirm the Baley Pilot graph renders before #125."
if ($failures.Count -gt 0) {
  throw "Local Pilot preflight failed: $($failures -join ', ')"
}
Write-Output "Automatic Local Pilot preflight passed."
