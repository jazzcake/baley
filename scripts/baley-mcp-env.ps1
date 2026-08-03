Set-StrictMode -Version Latest

function Read-BaleyMCPEnvironment {
  param(
    [Parameter(Mandatory)]
    [string]$Path
  )

  if (!(Test-Path -LiteralPath $Path -PathType Leaf)) {
    throw "Baley MCP environment file does not exist: $Path"
  }

  $allowed = @{
    BALEY_SERVER_URL = $true
    BALEY_AGENT_TOKEN = $true
  }
  $values = @{}
  $lineNumber = 0
  foreach ($line in [IO.File]::ReadAllLines([IO.Path]::GetFullPath($Path))) {
    $lineNumber++
    $trimmed = $line.Trim()
    if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) { continue }

    $separator = $trimmed.IndexOf("=")
    if ($separator -le 0) {
      throw "Invalid Baley MCP environment entry at line $lineNumber"
    }
    $name = $trimmed.Substring(0, $separator).Trim()
    $value = $trimmed.Substring($separator + 1).Trim()
    if (!$allowed.ContainsKey($name)) {
      throw "Unsupported Baley MCP environment name at line $lineNumber"
    }
    if ($values.ContainsKey($name)) {
      throw "Duplicate Baley MCP environment name at line $lineNumber"
    }
    if ($value.Length -ge 2) {
      $first = $value[0]
      $last = $value[$value.Length - 1]
      if (($first -eq '"' -and $last -eq '"') -or ($first -eq "'" -and $last -eq "'")) {
        $value = $value.Substring(1, $value.Length - 2)
      }
    }
    if ([string]::IsNullOrWhiteSpace($value) -or $value.Contains("`r") -or $value.Contains("`n")) {
      throw "Empty or multiline Baley MCP environment value at line $lineNumber"
    }
    $values[$name] = $value
  }

  foreach ($required in @("BALEY_SERVER_URL", "BALEY_AGENT_TOKEN")) {
    if (!$values.ContainsKey($required)) {
      throw "Required Baley MCP environment name is missing: $required"
    }
  }
  if (![Uri]::IsWellFormedUriString($values.BALEY_SERVER_URL, [UriKind]::Absolute)) {
    throw "BALEY_SERVER_URL must be an absolute URL"
  }
  return $values
}

function Write-BaleyMCPEnvironment {
  param(
    [Parameter(Mandatory)]
    [string]$Path,
    [Parameter(Mandatory)]
    [string]$ServerURL,
    [Parameter(Mandatory)]
    [string]$AgentToken
  )

  foreach ($value in @($ServerURL, $AgentToken)) {
    if ([string]::IsNullOrWhiteSpace($value) -or $value.Contains("`r") -or $value.Contains("`n")) {
      throw "Baley MCP environment values must be non-empty single-line strings"
    }
  }
  if (![Uri]::IsWellFormedUriString($ServerURL, [UriKind]::Absolute)) {
    throw "BALEY_SERVER_URL must be an absolute URL"
  }

  $resolved = [IO.Path]::GetFullPath($Path)
  $parent = Split-Path -Parent $resolved
  New-Item -ItemType Directory -Path $parent -Force | Out-Null
  $temporary = "$resolved.$PID.tmp"
  $content = "BALEY_SERVER_URL=$ServerURL`nBALEY_AGENT_TOKEN=$AgentToken`n"
  try {
    [IO.File]::WriteAllText($temporary, $content, [Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $temporary -Destination $resolved -Force
  } finally {
    if (Test-Path -LiteralPath $temporary) {
      Remove-Item -LiteralPath $temporary -Force
    }
    $content = $null
  }
}
