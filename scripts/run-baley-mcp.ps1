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
  $env:BALEY_SERVER_URL = $values.BALEY_SERVER_URL
  $env:BALEY_AGENT_TOKEN = $values.BALEY_AGENT_TOKEN
  $values = $null
  & go -C (Join-Path $repoRoot "server") run ./cmd/baley-mcp
  exit $LASTEXITCODE
} finally {
  $env:BALEY_AGENT_TOKEN = $null
  $values = $null
}
