Set-StrictMode -Version Latest

function Resolve-CodexCLI {
  [CmdletBinding()]
  param()

  $pathCommand = Get-Command codex.exe -CommandType Application -ErrorAction SilentlyContinue |
    Select-Object -First 1
  if ($null -ne $pathCommand -and (Test-Path -LiteralPath $pathCommand.Source -PathType Leaf)) {
    return [IO.Path]::GetFullPath($pathCommand.Source)
  }

  $candidates = [Collections.Generic.List[string]]::new()
  if (![string]::IsNullOrWhiteSpace($env:CODEX_CLI_PATH)) {
    $candidates.Add($env:CODEX_CLI_PATH)
  }

  $localAppData = [Environment]::GetFolderPath([Environment+SpecialFolder]::LocalApplicationData)
  $userRoot = [Environment]::GetFolderPath([Environment+SpecialFolder]::UserProfile)
  $appBinRoot = Join-Path $localAppData "OpenAI\Codex\bin"
  if (Test-Path -LiteralPath $appBinRoot -PathType Container) {
    Get-ChildItem -LiteralPath $appBinRoot -Recurse -Filter codex.exe -File -ErrorAction SilentlyContinue |
      Sort-Object LastWriteTimeUtc -Descending |
      ForEach-Object { $candidates.Add($_.FullName) }
  }

  $extensionsRoot = Join-Path $userRoot ".vscode\extensions"
  if (Test-Path -LiteralPath $extensionsRoot -PathType Container) {
    Get-ChildItem -Path (Join-Path $extensionsRoot "openai.chatgpt-*") -Directory -ErrorAction SilentlyContinue |
      Sort-Object LastWriteTimeUtc -Descending |
      ForEach-Object {
        Get-ChildItem -Path (Join-Path $_.FullName "bin\windows-*\codex.exe") -File -ErrorAction SilentlyContinue |
          ForEach-Object { $candidates.Add($_.FullName) }
      }
  }

  foreach ($candidate in $candidates) {
    if (![string]::IsNullOrWhiteSpace($candidate) -and (Test-Path -LiteralPath $candidate -PathType Leaf)) {
      return [IO.Path]::GetFullPath($candidate)
    }
  }

  throw "Codex CLI was not found in PATH, CODEX_CLI_PATH, the Codex app, or the VS Code OpenAI extension. Install or update Codex first."
}
