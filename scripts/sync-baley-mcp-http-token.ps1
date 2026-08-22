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
  # Kept for the legacy stdio rollback path only. Streamable HTTP uses an
  # independent local gateway token and obtains Workspace agent tokens through
  # the approved per-session connection flow.
  [Environment]::SetEnvironmentVariable("BALEY_AGENT_TOKEN", $values.BALEY_AGENT_TOKEN, "User")
  $env:BALEY_AGENT_TOKEN = $values.BALEY_AGENT_TOKEN
  $gatewayToken = [Environment]::GetEnvironmentVariable("BALEY_MCP_GATEWAY_TOKEN", "User")
  if ([string]::IsNullOrWhiteSpace($gatewayToken)) {
    $bytes = New-Object byte[] 32
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $gatewayToken = [Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+', '-').Replace('/', '_')
    [Environment]::SetEnvironmentVariable("BALEY_MCP_GATEWAY_TOKEN", $gatewayToken, "User")
  }
  $env:BALEY_MCP_GATEWAY_TOKEN = $gatewayToken
  Write-Output "Baley HTTP MCP gateway token was synchronized to the current and User environment. Restart Codex, then recreate the MCP container."
} finally {
  $values = $null
  $gatewayToken = $null
}
