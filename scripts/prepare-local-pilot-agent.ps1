[CmdletBinding()]
param(
  [string]$LoginID = "jazzc",
  [ValidatePattern("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$")]
  [string]$WorkspaceID = "00000000-0000-4000-8000-000000000001",
  [string]$AgentActorID = "00000000-0000-4000-8000-000000000003",
  [string]$TokenName = "local-day-tripper-onboarding",
  [string]$EnvironmentFile = "",
  [string]$ServerURL = "http://127.0.0.1:8080",
  [string]$ViewerOrigin = "http://127.0.0.1:5174"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$apiBaseURL = $ServerURL.TrimEnd("/")
$origin = $ViewerOrigin.TrimEnd("/")
if ([string]::IsNullOrWhiteSpace($EnvironmentFile)) {
  $environmentPath = Join-Path $repoRoot ".env.baley-mcp.local"
} elseif ([IO.Path]::IsPathRooted($EnvironmentFile)) {
  $environmentPath = [IO.Path]::GetFullPath($EnvironmentFile)
} else {
  $environmentPath = [IO.Path]::GetFullPath((Join-Path $repoRoot $EnvironmentFile))
}
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
    $existing = Invoke-RestMethod -Uri "$apiBaseURL/v1/workspaces/$WorkspaceID" -Headers @{ Authorization = "Bearer $($existingEnvironment.BALEY_AGENT_TOKEN)" }
    $existingEnvironment = $null
    if ($existing.id -eq $WorkspaceID) {
      Write-Output "The existing local Baley MCP environment passes Workspace $WorkspaceID read verification."
      Write-Output "Environment file: $environmentPath"
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
  $login = Invoke-RestMethod -Method Post -Uri "$apiBaseURL/v1/auth/login" -Headers @{ Origin = $origin } `
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
$issued = Invoke-RestMethod -Method Post -Uri "$apiBaseURL/v1/workspaces/$WorkspaceID/agent-tokens" `
  -Headers @{ Origin = $origin; "X-Baley-CSRF" = $login.csrfToken } -WebSession $session `
  -ContentType "application/json" -Body $body

$verified = Invoke-RestMethod -Uri "$apiBaseURL/v1/workspaces/$WorkspaceID" -Headers @{ Authorization = "Bearer $($issued.token)" }
if ($verified.id -ne $WorkspaceID) { throw "issued Agent token did not pass Workspace read verification" }
Write-BaleyMCPEnvironment -Path $environmentPath -ServerURL $apiBaseURL -AgentToken $issued.token
$issued.token = $null

Write-Output "Issued and verified Agent token $($issued.id) (prefix $($issued.prefix))."
Write-Output "Token audit name: $effectiveTokenName"
Write-Output "Workspace: $WorkspaceID"
Write-Output "Stored it in the local Baley MCP environment file: $environmentPath"
Write-Output "Open a new Codex thread so the MCP launcher reloads the file."
