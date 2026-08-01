[CmdletBinding()]
param(
  [string]$Version,
  [string]$OutputRoot,
  [switch]$AllowDirty
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$createdReleaseRoot = $false
if ([string]::IsNullOrWhiteSpace($OutputRoot)) {
  $OutputRoot = Join-Path $repoRoot ".tmp\hosted-pilot-releases"
}

function Invoke-Checked {
  & $args[0] $args[1..($args.Count - 1)]
  if ($LASTEXITCODE -ne 0) {
    throw "$($args[0]) failed with exit code $LASTEXITCODE"
  }
}

Push-Location $repoRoot
try {
  $commit = (& git rev-parse HEAD).Trim()
  if ($LASTEXITCODE -ne 0 -or $commit -notmatch "^[0-9a-f]{40}$") {
    throw "Unable to resolve the Git commit."
  }
  $dirty = @(& git status --porcelain --untracked-files=normal)
  if ($LASTEXITCODE -ne 0) {
    throw "Unable to inspect the Git working tree."
  }
  if ($dirty.Count -gt 0 -and -not $AllowDirty) {
    throw "The working tree is dirty. Commit the release inputs or pass -AllowDirty for a non-production build."
  }
  if ([string]::IsNullOrWhiteSpace($Version)) {
    $Version = $commit.Substring(0, 12)
    if ($dirty.Count -gt 0) { $Version += ".dirty" }
  }
  if ($Version -notmatch "^[0-9A-Za-z][0-9A-Za-z._-]{0,63}$") {
    throw "Version must contain only letters, numbers, dot, underscore, and hyphen."
  }

  $releaseName = "baley-$Version-linux-amd64"
  $releaseRoot = [IO.Path]::GetFullPath((Join-Path $OutputRoot $releaseName))
  if (Test-Path -LiteralPath $releaseRoot) {
    throw "Release output already exists: $releaseRoot"
  }
  New-Item -ItemType Directory -Path $releaseRoot | Out-Null
  $createdReleaseRoot = $true
  New-Item -ItemType Directory -Path (Join-Path $releaseRoot "viewer") | Out-Null
  New-Item -ItemType Directory -Path (Join-Path $releaseRoot "migrations") | Out-Null

  $builtAt = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
  $previousAPIURL = [Environment]::GetEnvironmentVariable("VITE_BALEY_API_URL", "Process")
  $previousGOOS = [Environment]::GetEnvironmentVariable("GOOS", "Process")
  $previousGOARCH = [Environment]::GetEnvironmentVariable("GOARCH", "Process")
  $previousCGO = [Environment]::GetEnvironmentVariable("CGO_ENABLED", "Process")
  try {
    $env:VITE_BALEY_API_URL = "/api"
    Invoke-Checked npm.cmd run build
    Copy-Item -Path (Join-Path $repoRoot "dist\*") -Destination (Join-Path $releaseRoot "viewer") -Recurse

    $env:GOOS = "linux"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"
    $ldflags = "-s -w -X main.buildVersion=$Version -X main.buildCommit=$commit -X main.buildTime=$builtAt"
    Push-Location (Join-Path $repoRoot "server")
    try {
      Invoke-Checked go build -trimpath -ldflags $ldflags -o (Join-Path $releaseRoot "baley-server") ./cmd/baley-server
    } finally {
      Pop-Location
    }
  } finally {
    [Environment]::SetEnvironmentVariable("VITE_BALEY_API_URL", $previousAPIURL, "Process")
    [Environment]::SetEnvironmentVariable("GOOS", $previousGOOS, "Process")
    [Environment]::SetEnvironmentVariable("GOARCH", $previousGOARCH, "Process")
    [Environment]::SetEnvironmentVariable("CGO_ENABLED", $previousCGO, "Process")
  }

  Copy-Item -Path (Join-Path $repoRoot "server\migrations\*.sql") -Destination (Join-Path $releaseRoot "migrations")
  @{
    version = $Version
    commit = $commit
    builtAt = $builtAt
    platform = "linux/amd64"
    dirty = $dirty.Count -gt 0
    viewerApiBase = "/api"
  } | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $releaseRoot "release.json") -Encoding utf8

  $releasePrefix = $releaseRoot.TrimEnd("\") + "\"
  $checksumLines = Get-ChildItem -LiteralPath $releaseRoot -File -Recurse |
    Sort-Object FullName |
    ForEach-Object {
      if (-not $_.FullName.StartsWith($releasePrefix, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Release file escaped the output root: $($_.FullName)"
      }
      $relative = $_.FullName.Substring($releasePrefix.Length).Replace("\", "/")
      $hash = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
      "$hash  $relative"
    }
  $checksumLines | Set-Content -LiteralPath (Join-Path $releaseRoot "SHA256SUMS") -Encoding ascii

  Write-Output "Hosted Pilot release created: $releaseRoot"
  Write-Output "Commit: $commit"
  Write-Output "Verify SHA256SUMS again after transfer and before activation."
} catch {
  if ($createdReleaseRoot -and $releaseRoot -and (Test-Path -LiteralPath $releaseRoot)) {
    Remove-Item -LiteralPath $releaseRoot -Recurse -Force
  }
  throw
} finally {
  Pop-Location
}
