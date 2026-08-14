[CmdletBinding()]
param(
  [string]$EnvironmentFile
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($EnvironmentFile)) {
  $EnvironmentFile = Join-Path $repoRoot ".env.baley-mcp.local"
}

. (Join-Path $PSScriptRoot "baley-mcp-env.ps1")
$values = Read-BaleyMCPEnvironment -Path $EnvironmentFile
try {
  # Codex Streamable HTTP registration reads only an environment-variable name;
  # never place this bearer token in its URL or config.toml.
  [Environment]::SetEnvironmentVariable("BALEY_AGENT_TOKEN", $values.BALEY_AGENT_TOKEN, "User")
  $env:BALEY_AGENT_TOKEN = $values.BALEY_AGENT_TOKEN
  Write-Output "Baley HTTP MCP token was synchronized to the current and User environment. Restart Codex to reload it."
} finally {
  $values = $null
}
