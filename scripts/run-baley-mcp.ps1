[CmdletBinding()]
param(
  [string]$EnvironmentFile,
  [string]$CredentialStore
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($EnvironmentFile)) {
  $EnvironmentFile = Join-Path $repoRoot ".env.baley-mcp.local"
}
if ([string]::IsNullOrWhiteSpace($CredentialStore)) {
  $CredentialStore = Join-Path $repoRoot ".tmp\baley-mcp\credentials.json"
}

$serverURL = "http://127.0.0.1:8080"
$agentToken = ""
if (Test-Path -LiteralPath $EnvironmentFile) {
  . (Join-Path $PSScriptRoot "baley-mcp-env.ps1")
  $values = Read-BaleyMCPEnvironment -Path $EnvironmentFile
  $serverURL = $values.BALEY_SERVER_URL
  $agentToken = $values.BALEY_AGENT_TOKEN
}
try {
  $env:BALEY_SERVER_URL = $serverURL
  $env:BALEY_AGENT_TOKEN = $agentToken
  $env:BALEY_MCP_CREDENTIAL_STORE = $CredentialStore
  $values = $null
  & go -C (Join-Path $repoRoot "server") run ./cmd/baley-mcp
  exit $LASTEXITCODE
} finally {
  $env:BALEY_AGENT_TOKEN = $null
  $env:BALEY_MCP_CREDENTIAL_STORE = $null
  $values = $null
}
