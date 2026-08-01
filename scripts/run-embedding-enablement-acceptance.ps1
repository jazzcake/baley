param(
  [string]$DatabaseUrl = $env:BALEY_TEST_DATABASE_URL
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$serverRoot = Join-Path $repoRoot "server"
$skillRoot = Join-Path $repoRoot ".agents/skills/baley-adopt-project"

function Invoke-Checked {
  & $args[0] $args[1..($args.Count - 1)]
  if ($LASTEXITCODE -ne 0) {
    throw "$($args[0]) failed with exit code $LASTEXITCODE"
  }
}

if ([string]::IsNullOrWhiteSpace($DatabaseUrl)) {
  throw "BALEY_TEST_DATABASE_URL is required and must name a disposable local test database."
}
$uri = [Uri]$DatabaseUrl
if ($uri.Host -notin @("127.0.0.1", "::1")) {
  throw "Acceptance database must use a literal loopback host."
}
if ($uri.AbsolutePath -notmatch "(^|_)(test|testing|acceptance)(_|$)") {
  throw "Acceptance database name must contain a standalone test, testing, or acceptance marker."
}
$allowedQueryKeys = @("sslmode", "connect_timeout", "application_name")
foreach ($part in $uri.Query.TrimStart("?").Split("&", [System.StringSplitOptions]::RemoveEmptyEntries)) {
  $key = [Uri]::UnescapeDataString($part.Split("=", 2)[0]).ToLowerInvariant()
  if ($key -notin $allowedQueryKeys) {
    throw "Acceptance database URL query parameter '$key' is not allowed."
  }
}

$previousTestDatabaseUrl = [Environment]::GetEnvironmentVariable("BALEY_TEST_DATABASE_URL", "Process")
$previousDatabaseUrl = [Environment]::GetEnvironmentVariable("BALEY_DATABASE_URL", "Process")
$previousLeaseTokenSecret = [Environment]::GetEnvironmentVariable("BALEY_LEASE_TOKEN_SECRET", "Process")

try {
  $env:BALEY_TEST_DATABASE_URL = $DatabaseUrl
  $env:BALEY_DATABASE_URL = $DatabaseUrl
  if ([string]::IsNullOrWhiteSpace($env:BALEY_LEASE_TOKEN_SECRET)) {
    $env:BALEY_LEASE_TOKEN_SECRET = "embedding-enablement-acceptance-local-only"
  }

  Push-Location $serverRoot
  try {
    Invoke-Checked go test ./integration -run "TestValidateDisposableDatabase(Connection|URL)$" -count=1
    Invoke-Checked go run ./cmd/baley-server migrate up
    Invoke-Checked go test ./internal/projectinit ./internal/domain ./internal/application
    $projectRoot = Join-Path ([IO.Path]::GetTempPath()) ("baley-adoption-" + [guid]::NewGuid().ToString("N"))
    $inputPath = Join-Path ([IO.Path]::GetTempPath()) ("baley-adoption-input-" + [guid]::NewGuid().ToString("N") + ".json")
    New-Item -ItemType Directory -Path $projectRoot | Out-Null
    try {
      @{
        ClientProjectID = "6279cb62-d52f-4642-942c-15e7bd72c911"
        Server = "http://127.0.0.1:8080"
        WorkspaceID = "6279cb62-d52f-4642-942c-15e7bd72c912"
        WorkspaceName = "Acceptance"
        RepositoryID = "6279cb62-d52f-4642-942c-15e7bd72c913"
        RepositoryName = "Acceptance"
        RemoteURL = "https://example.test/acceptance.git"
        RecordRepositoryID = "6279cb62-d52f-4642-942c-15e7bd72c913"
        TaskRecordsRoot = "task-records"
        InitiatedByActorID = "human"
        ExecutedByActorID = "operator"
        BootstrapCompleted = $true
      } | ConvertTo-Json | Set-Content -LiteralPath $inputPath -Encoding utf8
      Invoke-Checked go run ./cmd/baley-project-init --project-root $projectRoot --input $inputPath --apply
      $rerun = & go run ./cmd/baley-project-init --project-root $projectRoot --input $inputPath
      if ($LASTEXITCODE -ne 0) { throw "baley-project-init rerun failed" }
      $rerunPlan = ($rerun -join "`n") | ConvertFrom-Json
      if ($rerunPlan.files.Where({ $_.action -ne "keep" }).Count -ne 0) {
        throw "baley-project-init rerun did not converge to keep"
      }
    } finally {
      if (Test-Path -LiteralPath $inputPath) { Remove-Item -LiteralPath $inputPath -Force }
      if (Test-Path -LiteralPath $projectRoot) { Remove-Item -LiteralPath $projectRoot -Recurse -Force }
    }
  Invoke-Checked go test ./integration -run "Test(EmbeddingEnablementSingleRepositoryScenarioAgainstPostgres|EmbeddingEnablementCrossFeatureAcceptanceAgainstPostgres|AccountWorkspaceAccessAndAuthenticatedApprovalAgainstPostgres|PilotMeasurementRecordRegisterAgainstPostgres|Migration16AddsPilotMeasurementRecordType|LaneBacklogVerticalSliceAgainstPostgres|GateEntryTaskExplicitBindingAndAutomaticFallback|GatePublicIDsSerializeAndRollbackWithoutGaps|RunLifecycleAgainstPostgres|DelegatedAcceptanceAutoConfirmsOnlyEligibleTasks|MutationAttemptsAreWorkspaceScopedAppendOnlyAndRedacted)$" -count=1
    Invoke-Checked go test ./...
    Invoke-Checked go vet ./...
  } finally {
    Pop-Location
  }

  $pythonCommand = if (Get-Command python -ErrorAction SilentlyContinue) {
    "python"
  } elseif (Get-Command python3 -ErrorAction SilentlyContinue) {
    "python3"
  } else {
    throw "Python is required (python or python3)."
  }
  Invoke-Checked $pythonCommand (Join-Path $skillRoot "scripts/test_validate_pilot_measurement.py")
  Invoke-Checked $pythonCommand (Join-Path $skillRoot "scripts/validate_pilot_measurement.py") (Join-Path $repoRoot "task-records/embedding-enablement-acceptance/pilot-measurement-01.md")
  Push-Location $repoRoot
  try {
    $npmCommand = if (Get-Command npm.cmd -ErrorAction SilentlyContinue) { "npm.cmd" } else { "npm" }
    Invoke-Checked $npmCommand test
    Invoke-Checked $npmCommand run build
  } finally {
    Pop-Location
  }

  Write-Output "PASS embedding-enablement acceptance aggregate"
} finally {
  [Environment]::SetEnvironmentVariable("BALEY_TEST_DATABASE_URL", $previousTestDatabaseUrl, "Process")
  [Environment]::SetEnvironmentVariable("BALEY_DATABASE_URL", $previousDatabaseUrl, "Process")
  [Environment]::SetEnvironmentVariable("BALEY_LEASE_TOKEN_SECRET", $previousLeaseTokenSecret, "Process")
}
