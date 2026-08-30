[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
Write-Output "The former shared HTTP bearer-token flow is retired. On Windows use scripts/install-baley-mcp-windows.ps1 for the tokenless loopback Gateway; it never writes a bearer token to Codex configuration."
