[CmdletBinding()]
param(
  [string]$Binary
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Binary)) {
  $installRoot = Join-Path $env:LOCALAPPDATA "Baley\mcp"
  $latestRelease = Get-ChildItem -LiteralPath (Join-Path $installRoot "releases") -Filter "baley-mcp.exe" -Recurse -File -ErrorAction SilentlyContinue |
    Sort-Object LastWriteTimeUtc -Descending |
    Select-Object -First 1
  $Binary = if ($null -ne $latestRelease) { $latestRelease.FullName } else { Join-Path $installRoot "baley-mcp.exe" }
}

if (!(Test-Path -LiteralPath $Binary -PathType Leaf)) {
  throw "Baley MCP binary was not found: $Binary"
}

$file = Get-Item -LiteralPath $Binary
$processes = Get-Process -Name "baley-mcp" -ErrorAction SilentlyContinue |
  Select-Object Id, StartTime,
    @{ Name = "WorkingSetMB"; Expression = { [math]::Round($_.WorkingSet64 / 1MB, 2) } },
    @{ Name = "PrivateMB"; Expression = { [math]::Round($_.PrivateMemorySize64 / 1MB, 2) } }
$processList = @($processes)

[pscustomobject]@{
  Binary = $file.FullName
  BinaryMB = [math]::Round($file.Length / 1MB, 2)
  RunningSessionCount = $processList.Count
  TotalWorkingSetMB = [math]::Round(($processList | Measure-Object -Property WorkingSetMB -Sum).Sum, 2)
  TotalPrivateMB = [math]::Round(($processList | Measure-Object -Property PrivateMB -Sum).Sum, 2)
  AverageWorkingSetMB = if ($processList.Count) { [math]::Round((($processList | Measure-Object -Property WorkingSetMB -Average).Average), 2) } else { 0 }
  AveragePrivateMB = if ($processList.Count) { [math]::Round((($processList | Measure-Object -Property PrivateMB -Average).Average), 2) } else { 0 }
  Processes = $processList
} | ConvertTo-Json -Depth 4
