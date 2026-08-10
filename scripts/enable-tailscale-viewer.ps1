param(
  [string]$Target = "http://127.0.0.1:5174"
)

$ErrorActionPreference = "Stop"
& tailscale serve --https=443 --bg --yes $Target
if ($LASTEXITCODE -ne 0) { throw "tailscale serve failed" }
& tailscale serve --http=80 off
if ($LASTEXITCODE -ne 0) { throw "tailscale HTTP Serve disable failed" }
& tailscale serve status
