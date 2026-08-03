[CmdletBinding()]
param()

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$userRoot = [Environment]::GetFolderPath([Environment+SpecialFolder]::UserProfile)
$pluginParent = Join-Path $userRoot "plugins"
$pluginRoot = Join-Path $pluginParent "baley"
$marketplacePath = Join-Path $userRoot ".agents\plugins\marketplace.json"
$pluginCreatorRoot = Join-Path $userRoot ".codex\skills\.system\plugin-creator"
$skillValidator = Join-Path $userRoot ".codex\skills\.system\skill-creator\scripts\quick_validate.py"
$manifestSource = Join-Path $repoRoot "codex-plugin\baley\.codex-plugin\plugin.json"
$skillNames = @("baley-manage-work", "baley-adopt-project")

foreach ($required in @(
  (Join-Path $pluginCreatorRoot "scripts\create_basic_plugin.py"),
  (Join-Path $pluginCreatorRoot "scripts\update_plugin_cachebuster.py"),
  (Join-Path $pluginCreatorRoot "scripts\validate_plugin.py"),
  (Join-Path $pluginCreatorRoot "scripts\read_marketplace_name.py"),
  $skillValidator,
  $manifestSource
)) {
  if (!(Test-Path -LiteralPath $required -PathType Leaf)) {
    throw "Required Codex plugin resource is missing: $required"
  }
}

$pluginFullPath = [IO.Path]::GetFullPath($pluginRoot)
$pluginParentFullPath = [IO.Path]::GetFullPath($pluginParent).TrimEnd([IO.Path]::DirectorySeparatorChar)
if (!$pluginFullPath.StartsWith($pluginParentFullPath + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase) -or
    [IO.Path]::GetFileName($pluginFullPath) -ne "baley") {
  throw "Refusing to update an unsafe plugin path: $pluginFullPath"
}

$marketplaceHasBaley = $false
if (Test-Path -LiteralPath $marketplacePath -PathType Leaf) {
  $marketplace = Get-Content -LiteralPath $marketplacePath -Raw | ConvertFrom-Json
  $marketplaceHasBaley = @($marketplace.plugins | Where-Object { $_.name -eq "baley" }).Count -eq 1
}

if (!(Test-Path -LiteralPath (Join-Path $pluginRoot ".codex-plugin\plugin.json") -PathType Leaf) -or !$marketplaceHasBaley) {
  $scaffoldArguments = @(
    (Join-Path $pluginCreatorRoot "scripts\create_basic_plugin.py"),
    "baley",
    "--path", $pluginParent,
    "--with-skills",
    "--with-marketplace"
  )
  if (Test-Path -LiteralPath $pluginRoot) {
    $scaffoldArguments += "--force"
  }
  & python @scaffoldArguments
  if ($LASTEXITCODE -ne 0) { throw "Baley plugin scaffold failed" }
}

$skillsRoot = Join-Path $pluginRoot "skills"
New-Item -ItemType Directory -Path $skillsRoot -Force | Out-Null
foreach ($skillName in $skillNames) {
  $source = Join-Path $repoRoot ".agents\skills\$skillName"
  $target = Join-Path $skillsRoot $skillName
  if (!(Test-Path -LiteralPath (Join-Path $source "SKILL.md") -PathType Leaf)) {
    throw "Baley skill source is missing: $source"
  }
  $targetFullPath = [IO.Path]::GetFullPath($target)
  if (!$targetFullPath.StartsWith($pluginFullPath + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to replace an unsafe skill target: $targetFullPath"
  }
  if (Test-Path -LiteralPath $target) {
    Remove-Item -LiteralPath $target -Recurse -Force
  }
  Copy-Item -LiteralPath $source -Destination $skillsRoot -Recurse -Force
}

$manifestTarget = Join-Path $pluginRoot ".codex-plugin\plugin.json"
Copy-Item -LiteralPath $manifestSource -Destination $manifestTarget -Force
& python (Join-Path $pluginCreatorRoot "scripts\update_plugin_cachebuster.py") $pluginRoot
if ($LASTEXITCODE -ne 0) { throw "Baley plugin cachebuster update failed" }

$previousPythonUTF8 = $env:PYTHONUTF8
try {
  $env:PYTHONUTF8 = "1"
  foreach ($skillName in $skillNames) {
    & python $skillValidator (Join-Path $skillsRoot $skillName)
    if ($LASTEXITCODE -ne 0) { throw "Baley skill validation failed: $skillName" }
  }
  & python (Join-Path $pluginCreatorRoot "scripts\validate_plugin.py") $pluginRoot
  if ($LASTEXITCODE -ne 0) { throw "Baley plugin validation failed" }
} finally {
  $env:PYTHONUTF8 = $previousPythonUTF8
}

$marketplaceName = (& python (Join-Path $pluginCreatorRoot "scripts\read_marketplace_name.py")).Trim()
if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($marketplaceName)) {
  throw "Personal Codex marketplace name could not be resolved"
}
& codex plugin add "baley@$marketplaceName"
if ($LASTEXITCODE -ne 0) { throw "Baley plugin installation failed" }

$pluginList = (& codex plugin list --json | ConvertFrom-Json)
$installed = @($pluginList.installed | Where-Object { $_.pluginId -eq "baley@$marketplaceName" -and $_.enabled })
if ($installed.Count -ne 1) {
  throw "Baley plugin was not found as an enabled Codex plugin after installation"
}

Write-Output "Baley Codex plugin installed: $($installed[0].pluginId) $($installed[0].version)"
Write-Output "Start a new Codex thread to load baley:baley-manage-work and baley:baley-adopt-project."
