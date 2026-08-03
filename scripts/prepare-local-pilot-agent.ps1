[CmdletBinding()]
param(
  [string]$LoginID = "jazzc",
  [string]$AgentActorID = "00000000-0000-4000-8000-000000000003",
  [string]$TokenName = "local-day-tripper-onboarding"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$serverURL = "http://127.0.0.1:8080"
$origin = "http://127.0.0.1:5174"
$workspaceID = "00000000-0000-4000-8000-000000000001"
$repoRoot = Split-Path -Parent $PSScriptRoot
$environmentPath = Join-Path $repoRoot ".env.baley-mcp.local"
. (Join-Path $PSScriptRoot "baley-mcp-env.ps1")
$tokenNameBase = $TokenName.Trim()
if ([string]::IsNullOrWhiteSpace($tokenNameBase)) {
  $tokenNameBase = "local-day-tripper-onboarding"
}
$tokenNameSuffix = (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ") + "-" + [guid]::NewGuid().ToString("N").Substring(0, 8)
$effectiveTokenName = "$tokenNameBase-$tokenNameSuffix"

$existing = $null
if (Test-Path -LiteralPath $environmentPath -PathType Leaf) {
  try {
    $existingEnvironment = Read-BaleyMCPEnvironment -Path $environmentPath
    $existing = Invoke-RestMethod -Uri "$serverURL/v1/workspaces/$workspaceID" -Headers @{ Authorization = "Bearer $($existingEnvironment.BALEY_AGENT_TOKEN)" }
    $existingEnvironment = $null
    if ($existing.id -eq $workspaceID) {
      Write-Output "The existing local Baley MCP environment passes Workspace read verification."
      Write-Output "Open a new Codex thread so the MCP launcher reloads the file."
      return
    }
  } catch {
    $existingEnvironment = $null
  }
}

$securePassword = Read-Host "Password for $LoginID" -AsSecureString
$pointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($securePassword)
$plainPassword = $null
$loginBody = $null
try {
  $plainPassword = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($pointer)
  $loginBody = @{ loginId = $LoginID; password = $plainPassword } | ConvertTo-Json
  $login = Invoke-RestMethod -Method Post -Uri "$serverURL/v1/auth/login" -Headers @{ Origin = $origin } `
    -ContentType "application/json" -Body $loginBody -SessionVariable session
} finally {
  if ($pointer -ne [IntPtr]::Zero) { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($pointer) }
  $plainPassword = $null
  $loginBody = $null
}

$body = @{
  actorId = $AgentActorID
  name = $effectiveTokenName
  scopes = @("workspace:read", "workspace:operate", "run:operate", "record:operate")
} | ConvertTo-Json
$issued = Invoke-RestMethod -Method Post -Uri "$serverURL/v1/workspaces/$workspaceID/agent-tokens" `
  -Headers @{ Origin = $origin; "X-Baley-CSRF" = $login.csrfToken } -WebSession $session `
  -ContentType "application/json" -Body $body

$verified = Invoke-RestMethod -Uri "$serverURL/v1/workspaces/$workspaceID" -Headers @{ Authorization = "Bearer $($issued.token)" }
if ($verified.id -ne $workspaceID) { throw "issued Agent token did not pass Workspace read verification" }
Write-BaleyMCPEnvironment -Path $environmentPath -ServerURL $serverURL -AgentToken $issued.token
$issued.token = $null

Write-Output "Issued and verified Agent token $($issued.id) (prefix $($issued.prefix))."
Write-Output "Token audit name: $effectiveTokenName"
Write-Output "Stored it in the Git-ignored local Baley MCP environment file."
Write-Output "Open a new Codex thread so the MCP launcher reloads the file."
