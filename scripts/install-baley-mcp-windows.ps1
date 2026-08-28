[CmdletBinding()]
param(
  [string]$ServerURL = "https://jazzcake-home.tail87e929.ts.net/api"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
if (![Uri]::IsWellFormedUriString($ServerURL, [UriKind]::Absolute) -or !$ServerURL.StartsWith("https://", [StringComparison]::OrdinalIgnoreCase)) {
  throw "ServerURL must be an approved HTTPS Baley API URL"
}
$installRoot = Join-Path $env:LOCALAPPDATA "Baley\mcp"
$binary = Join-Path $installRoot "baley-mcp.exe"
$credentialStore = Join-Path $installRoot "credentials.json"
New-Item -ItemType Directory -Force $installRoot | Out-Null
go -C (Join-Path $repoRoot "server") build -o $binary ./cmd/baley-mcp
codex mcp remove baley 2>$null
codex mcp add baley --env "BALEY_SERVER_URL=$($ServerURL.TrimEnd('/'))" --env "BALEY_MCP_CREDENTIAL_STORE=$credentialStore" -- $binary
Write-Output "Baley MCP is registered as tokenless stdio. Restart Codex Desktop or begin a new CLI session."
