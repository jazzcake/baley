[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Write-Output "The shared HTTP MCP gateway is retired. Install the tokenless stdio MCP registration instead: scripts/install-baley-mcp-macos.sh or codex mcp add baley -- <baley-mcp>."
