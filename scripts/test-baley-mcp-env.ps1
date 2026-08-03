[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$testRoot = Join-Path $repoRoot ".tmp\baley-mcp-env-tests\$([guid]::NewGuid().ToString('N'))"
$resolvedTestRoot = [IO.Path]::GetFullPath($testRoot)
$resolvedRepoRoot = [IO.Path]::GetFullPath($repoRoot)
if (!$resolvedTestRoot.StartsWith($resolvedRepoRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
  throw "Unsafe test directory: $resolvedTestRoot"
}

. (Join-Path $PSScriptRoot "baley-mcp-env.ps1")

function Assert-Throws {
  param([scriptblock]$Action, [string]$Name)
  try {
    & $Action
  } catch {
    return
  }
  throw "Expected failure was not raised: $Name"
}

New-Item -ItemType Directory -Path $resolvedTestRoot -Force | Out-Null
try {
  $validPath = Join-Path $resolvedTestRoot "valid.env"
  Write-BaleyMCPEnvironment -Path $validPath -ServerURL "http://127.0.0.1:8080" -AgentToken "test-token"
  $valid = Read-BaleyMCPEnvironment -Path $validPath
  if ($valid.BALEY_SERVER_URL -ne "http://127.0.0.1:8080" -or $valid.BALEY_AGENT_TOKEN -ne "test-token") {
    throw "Valid Baley MCP environment did not round-trip"
  }

  $unknownPath = Join-Path $resolvedTestRoot "unknown.env"
  [IO.File]::WriteAllText($unknownPath, "BALEY_SERVER_URL=http://127.0.0.1:8080`nUNEXPECTED=value`nBALEY_AGENT_TOKEN=token`n")
  Assert-Throws { Read-BaleyMCPEnvironment -Path $unknownPath } "unknown name"

  $duplicatePath = Join-Path $resolvedTestRoot "duplicate.env"
  [IO.File]::WriteAllText($duplicatePath, "BALEY_SERVER_URL=http://127.0.0.1:8080`nBALEY_AGENT_TOKEN=one`nBALEY_AGENT_TOKEN=two`n")
  Assert-Throws { Read-BaleyMCPEnvironment -Path $duplicatePath } "duplicate name"

  $missingPath = Join-Path $resolvedTestRoot "missing.env"
  [IO.File]::WriteAllText($missingPath, "BALEY_SERVER_URL=http://127.0.0.1:8080`n")
  Assert-Throws { Read-BaleyMCPEnvironment -Path $missingPath } "missing token"
} finally {
  if (Test-Path -LiteralPath $resolvedTestRoot) {
    Remove-Item -LiteralPath $resolvedTestRoot -Recurse -Force
  }
}

Write-Output "PASS Baley MCP environment parser"
