[CmdletBinding()]
param()

$ErrorActionPreference = 'Stop'
$Helper = (Resolve-Path (Join-Path $PSScriptRoot 'capture-task-189-evidence.ps1')).Path
$TempRoot = [IO.Path]::GetFullPath([IO.Path]::GetTempPath())
$TestRoot = Join-Path $TempRoot ("dim0-task189-evidence-test-" + [Guid]::NewGuid().ToString('N'))

function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) { throw $Message }
}

function New-EvidenceFixture([string]$Name) {
    $directory = Join-Path $TestRoot $Name
    New-Item -ItemType Directory -Path $directory | Out-Null
    [IO.File]::WriteAllText((Join-Path $directory 'run-provenance.json'), '{"schemaVersion":1}', [Text.UTF8Encoding]::new($false))
    $baselineEnvironment = @(
        'DOPPLER_TOKEN=', 'OPENAI_API_KEY=', 'ANTHROPIC_API_KEY=',
        'API_ORIGIN=http://localhost:18082', 'DIM0_BASELINE_PROVIDER_TRIPWIRE=1'
    )
    [IO.File]::WriteAllText((Join-Path $directory 'baseline.env'), (($baselineEnvironment -join [Environment]::NewLine) + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
    $counters = '{"llm":0,"embedding":0,"search":0,"fetch":0,"ocr":0,"image":0,"daytona":0}'
    [IO.File]::WriteAllText((Join-Path $directory 'provider-invocations.json'), $counters, [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $directory 'provider-constructions.json'), $counters, [Text.UTF8Encoding]::new($false))
    return $directory
}

function Assert-FinalizeFails([string]$Directory, [string]$ExpectedMessage) {
    $failed = $false
    try {
        & $Helper -Action Finalize -EvidenceRoot $TestRoot -RunDirectory $Directory | Out-Null
    }
    catch {
        $failed = $true
        Assert-True ($_.Exception.Message.Contains($ExpectedMessage)) "Unexpected finalization error: $($_.Exception.Message)"
    }
    Assert-True $failed "Expected finalization to fail: $ExpectedMessage"
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $Directory 'finalized.json'))) 'Failed finalization wrote a finalized marker.'
    foreach ($generated in @('finalization-policy.json', 'integrity-metadata.json', 'secret-screening.json', 'manifest.sha256')) {
        Assert-True (-not (Test-Path -LiteralPath (Join-Path $Directory $generated))) "Failed finalization left retry-blocking artifact: $generated"
    }
}

try {
    New-Item -ItemType Directory -Path $TestRoot | Out-Null

    $valid = New-EvidenceFixture 'valid'
    $benignLog = @(
        'passed',
        'HTTPException in reset_password: Password reset is disabled',
        'HTTPException in refresh_access_token: Refresh token revoked'
    ) -join [Environment]::NewLine
    [IO.File]::WriteAllText((Join-Path $valid '10-focused-check.txt'), ($benignLog + [Environment]::NewLine), [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $valid '10-focused-check.exit.txt'), "0`n", [Text.UTF8Encoding]::new($false))
    & $Helper -Action Finalize -EvidenceRoot $TestRoot -RunDirectory $valid | Out-Null

    $manifest = Get-Content -Raw -LiteralPath (Join-Path $valid 'manifest.sha256')
    foreach ($covered in @('finalization-policy.json', 'integrity-metadata.json', 'secret-screening.json')) {
        Assert-True ($manifest.Contains($covered)) "Manifest does not cover $covered."
    }
    $marker = Get-Content -Raw -LiteralPath (Join-Path $valid 'finalized.json') | ConvertFrom-Json
    $actualManifestHash = (Get-FileHash -LiteralPath (Join-Path $valid 'manifest.sha256') -Algorithm SHA256).Hash.ToLowerInvariant()
    Assert-True ($marker.manifestSha256 -eq $actualManifestHash) 'Finalized marker does not bind the manifest hash.'
    $manifestEntries = @($manifest -split '\r?\n' | Where-Object { $_ })
    Assert-True ($marker.fileCount -eq $manifestEntries.Count) 'Finalized marker file count does not match the manifest.'
    foreach ($entry in $manifestEntries) {
        Assert-True ($entry -match '^([0-9a-f]{64})  (.+)$') "Malformed manifest entry: $entry"
        $actualFileHash = (Get-FileHash -LiteralPath (Join-Path $valid $Matches[2]) -Algorithm SHA256).Hash.ToLowerInvariant()
        Assert-True ($Matches[1] -eq $actualFileHash) "Manifest hash mismatch for $($Matches[2])."
    }
    Assert-True ($marker.filesystemImmutable -eq $false) 'Finalization incorrectly claims filesystem immutability.'

    $correctedRetry = New-EvidenceFixture 'corrected-retry'
    [IO.File]::WriteAllText((Join-Path $correctedRetry 'baseline.env'), "OPENAI_API_KEY=definitely-not-a-real-credential`n", [Text.UTF8Encoding]::new($false))
    Assert-FinalizeFails $correctedRetry 'credential-field'
    [IO.File]::WriteAllText((Join-Path $correctedRetry 'baseline.env'), "OPENAI_API_KEY=`n", [Text.UTF8Encoding]::new($false))
    & $Helper -Action Finalize -EvidenceRoot $TestRoot -RunDirectory $correctedRetry | Out-Null
    Assert-True (Test-Path -LiteralPath (Join-Path $correctedRetry 'finalized.json')) 'Corrected finalization retry did not succeed.'

    $structuredCredential = New-EvidenceFixture 'structured-credential'
    [IO.File]::WriteAllText((Join-Path $structuredCredential 'browser-console.json'), '{"client_secret":"definitely-not-a-real-credential"}', [Text.UTF8Encoding]::new($false))
    Assert-FinalizeFails $structuredCredential 'credential-field'

    $commandCredential = New-EvidenceFixture 'command-credential'
    [IO.File]::WriteAllText((Join-Path $commandCredential '20-browser-check.txt'), 'docker run -e OPENAI_API_KEY=definitely-not-a-real-credential', [Text.UTF8Encoding]::new($false))
    Assert-FinalizeFails $commandCredential 'credential-field'

    $unsupported = New-EvidenceFixture 'unsupported'
    [IO.File]::WriteAllBytes((Join-Path $unsupported 'screen.png'), [byte[]](1, 2, 3))
    Assert-FinalizeFails $unsupported 'Unclassified or unsupported evidence artifact'

    $malformed = New-EvidenceFixture 'malformed'
    [IO.File]::WriteAllText((Join-Path $malformed 'browser-console.json'), '{not-json}', [Text.UTF8Encoding]::new($false))
    Assert-FinalizeFails $malformed 'Unsupported encoding or malformed structured evidence artifact'

    $credentialUrl = New-EvidenceFixture 'credential-url'
    [IO.File]::WriteAllText((Join-Path $credentialUrl '20-browser-check.txt'), "https://user:password@example.test/`n", [Text.UTF8Encoding]::new($false))
    Assert-FinalizeFails $credentialUrl 'url-userinfo'

    $cookie = New-EvidenceFixture 'cookie'
    [IO.File]::WriteAllText((Join-Path $cookie 'browser-network.har'), '{"log":{"entries":[{"request":{"headers":[{"name":"Cookie","value":"sid=value"}]}}]}}', [Text.UTF8Encoding]::new($false))
    Assert-FinalizeFails $cookie 'har-sensitive-header'

    $session = New-EvidenceFixture 'session'
    [IO.File]::WriteAllText((Join-Path $session 'browser-console.json'), '{"sessionId":"browser-session-value"}', [Text.UTF8Encoding]::new($false))
    Assert-FinalizeFails $session 'session-field'

    Write-Output 'Task 189 evidence finalization self-tests passed.'
}
finally {
    $resolvedTestRoot = [IO.Path]::GetFullPath($TestRoot)
    if ($resolvedTestRoot.StartsWith($TempRoot, [StringComparison]::OrdinalIgnoreCase) -and (Test-Path -LiteralPath $resolvedTestRoot)) {
        Remove-Item -LiteralPath $resolvedTestRoot -Recurse -Force
    }
}
