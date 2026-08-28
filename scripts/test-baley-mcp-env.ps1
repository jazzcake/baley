[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$output = ""
try {
  . (Join-Path $PSScriptRoot "baley-mcp-env.ps1")
} catch {
  $output = $_.Exception.Message
}
if ($output -notmatch "retired") {
  throw "Retired plaintext MCP environment helper did not fail closed"
}
Write-Output "PASS retired plaintext Baley MCP environment helper fails closed"
