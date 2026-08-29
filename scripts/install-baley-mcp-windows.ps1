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
$worktreeChanges = git -C $repoRoot status --porcelain
if ($LASTEXITCODE -ne 0) { throw "Unable to inspect the Baley MCP source worktree" }
if ($worktreeChanges) { throw "Commit or stash Baley MCP source changes before creating a release install" }
$releaseID = (git -C $repoRoot rev-parse --short=12 HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($releaseID)) { throw "Unable to determine the Baley MCP release ID" }
$binary = Join-Path (Join-Path (Join-Path $installRoot "releases") $releaseID) "baley-mcp.exe"
$credentialStore = Join-Path $installRoot "credentials.json"
New-Item -ItemType Directory -Force (Split-Path -Parent $binary) | Out-Null
# Desktop/CLI starts this binary per stdio session. Strip DWARF and symbol
# tables from the release artifact; this changes neither the tokenless
# transport nor its Windows Credential Manager integration.
if (!(Test-Path -LiteralPath $binary -PathType Leaf)) {
  go -C (Join-Path $repoRoot "server") build -trimpath -ldflags "-s -w" -o $binary ./cmd/baley-mcp
  if ($LASTEXITCODE -ne 0) { throw "Baley MCP build failed" }
}
# `codex mcp add` replaces the named registration atomically. Do not remove
# the working registration first: a failed replacement must leave the prior
# Desktop/CLI configuration intact for the next session.
codex mcp add baley --env "BALEY_SERVER_URL=$($ServerURL.TrimEnd('/'))" --env "BALEY_MCP_CREDENTIAL_STORE=$credentialStore" -- $binary
if ($LASTEXITCODE -ne 0) { throw "Codex MCP registration failed" }
Write-Output "Baley MCP is registered as tokenless stdio. Restart Codex Desktop or begin a new CLI session."
