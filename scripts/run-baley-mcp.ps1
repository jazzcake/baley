[CmdletBinding()]
param(
  [string]$ServerURL = "https://jazzcake-home.tail87e929.ts.net/api",
  [string]$CredentialStore
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
if ([string]::IsNullOrWhiteSpace($CredentialStore)) {
  $CredentialStore = Join-Path $repoRoot ".tmp\baley-mcp\credentials.json"
}
if (![Uri]::IsWellFormedUriString($ServerURL, [UriKind]::Absolute) -or !$ServerURL.StartsWith("https://", [StringComparison]::OrdinalIgnoreCase)) {
  throw "BALEY_SERVER_URL must be an approved HTTPS URL for tokenless stdio"
}

try {
  $env:BALEY_SERVER_URL = $ServerURL.TrimEnd("/")
  $env:BALEY_MCP_CREDENTIAL_STORE = $CredentialStore
  # Do not read or pass BALEY_AGENT_TOKEN / BALEY_MCP_GATEWAY_TOKEN. The local
  # binary obtains device-bound Workspace credentials from Windows Credential
  # Manager after Owner approval.
  Remove-Item Env:BALEY_AGENT_TOKEN -ErrorAction SilentlyContinue
  Remove-Item Env:BALEY_MCP_GATEWAY_TOKEN -ErrorAction SilentlyContinue
  & go -C (Join-Path $repoRoot "server") run ./cmd/baley-mcp
  exit $LASTEXITCODE
} finally {
  Remove-Item Env:BALEY_AGENT_TOKEN -ErrorAction SilentlyContinue
  Remove-Item Env:BALEY_MCP_GATEWAY_TOKEN -ErrorAction SilentlyContinue
  Remove-Item Env:BALEY_MCP_CREDENTIAL_STORE -ErrorAction SilentlyContinue
}
