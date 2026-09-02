[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
. (Join-Path $PSScriptRoot "codex-cli.ps1")
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

$mcpLoopbackRegistration = $false
$mcpTokenFree = $false
$codexCLI = $null
try { $codexCLI = Resolve-CodexCLI } catch {}
if ($null -ne $codexCLI) {
  try {
    $mcpConfigJSON = (& $codexCLI mcp get baley --json 2>$null | Out-String)
    if ($LASTEXITCODE -eq 0) {
      $mcpConfig = $mcpConfigJSON | ConvertFrom-Json
      $topProperties = @($mcpConfig.PSObject.Properties.Name)
      $transportProperties = @($mcpConfig.transport.PSObject.Properties.Name)
      $mcpLoopbackRegistration = $mcpConfig.transport.type -eq "streamable_http" -and $mcpConfig.transport.url -eq "http://127.0.0.1:8090/mcp" -and -not ($topProperties -contains "command") -and -not ($topProperties -contains "env")
      $mcpTokenFree = $mcpConfig.transport.bearer_token_env_var -eq $null -and $mcpConfig.transport.http_headers -eq $null -and $mcpConfig.transport.env_http_headers -eq $null -and -not ($transportProperties -contains "authorization")
    }
  } catch {}
}
$checks += Add-Check "codex-mcp-loopback-gateway" $mcpLoopbackRegistration "streamable HTTP at http://127.0.0.1:8090/mcp with no command registration"
$checks += Add-Check "codex-mcp-no-token-or-header" $mcpTokenFree "no bearer-token environment variable or HTTP Authorization header"

$baleyPluginInstalled = $false
$baleyPluginSkills = $false
if ($null -ne $codexCLI) {
  try {
    $pluginList = (& $codexCLI plugin list --json 2>$null | Out-String) | ConvertFrom-Json
    $baleyPlugin = @($pluginList.installed | Where-Object { $_.pluginId -eq "baley@personal" -and $_.enabled }) | Select-Object -First 1
    if ($null -ne $baleyPlugin) {
      $baleyPluginInstalled = $true
      $pluginCache = Join-Path $env:USERPROFILE ".codex\plugins\cache\personal\baley\$($baleyPlugin.version)\skills"
      $baleyPluginSkills = (Test-Path -LiteralPath (Join-Path $pluginCache "baley-manage-work\SKILL.md") -PathType Leaf) -and (Test-Path -LiteralPath (Join-Path $pluginCache "baley-adopt-project\SKILL.md") -PathType Leaf)
    }
  } catch {}
}
$checks += Add-Check "baley-plugin-installed" $baleyPluginInstalled "baley@personal enabled"
$checks += Add-Check "baley-plugin-skills" $baleyPluginSkills "baley:baley-manage-work and baley:baley-adopt-project"

$checks | Format-Table -AutoSize
Write-Output "MANUAL: sign in as the Owner, verify the Viewer graph, then make one tokenless Codex MCP read and inspect baley_mcp_diagnostics."
if ($failures.Count -gt 0) { throw "Local Pilot preflight failed: $($failures -join ', ')" }
Write-Output "Automatic Local Pilot preflight passed."
