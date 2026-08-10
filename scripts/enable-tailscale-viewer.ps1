param(
  [string]$Target = "http://127.0.0.1:5174"
)

$ErrorActionPreference = "Stop"
& tailscale serve --http=80 --bg --yes $Target
if ($LASTEXITCODE -ne 0) { throw "tailscale serve failed" }
& tailscale serve status
