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

$existingToken = [Environment]::GetEnvironmentVariable("BALEY_AGENT_TOKEN", "User")
if (![string]::IsNullOrWhiteSpace($existingToken)) {
  try {
    $existing = Invoke-RestMethod -Uri "$serverURL/v1/workspaces/$workspaceID" -Headers @{ Authorization = "Bearer $existingToken" }
    if ($existing.id -eq $workspaceID) {
      Write-Output "An existing User-level Agent token already passes Workspace read verification."
      Write-Output "Open a new Codex thread if the current Baley MCP process started before that token was configured."
      return
    }
  } catch {}
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
  name = $TokenName
  scopes = @("workspace:read", "workspace:operate", "run:operate", "record:operate")
} | ConvertTo-Json
$issued = Invoke-RestMethod -Method Post -Uri "$serverURL/v1/workspaces/$workspaceID/agent-tokens" `
  -Headers @{ Origin = $origin; "X-Baley-CSRF" = $login.csrfToken } -WebSession $session `
  -ContentType "application/json" -Body $body

[Environment]::SetEnvironmentVariable("BALEY_SERVER_URL", $serverURL, "User")
[Environment]::SetEnvironmentVariable("BALEY_AGENT_TOKEN", $issued.token, "User")
$verified = Invoke-RestMethod -Uri "$serverURL/v1/workspaces/$workspaceID" -Headers @{ Authorization = "Bearer $($issued.token)" }
if ($verified.id -ne $workspaceID) { throw "issued Agent token did not pass Workspace read verification" }

Write-Output "Issued and verified Agent token $($issued.id) (prefix $($issued.prefix))."
Write-Output "Ensure the Codex Baley MCP registration whitelists BALEY_SERVER_URL and BALEY_AGENT_TOKEN through env_vars."
Write-Output "Completely exit and relaunch the Codex host, then open a new thread so the Baley MCP process receives the current User environment."
