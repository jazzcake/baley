[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [ValidateSet('Initialize', 'Finalize')]
    [string]$Action,

    [string]$EvidenceRoot = 'C:\ProgramData\Dim0\validation\task-189'
)

$ErrorActionPreference = 'Stop'
$RepositoryRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$Dim0Root = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
$ResolvedEvidenceRoot = [IO.Path]::GetFullPath($EvidenceRoot)
$VolumeRoot = [IO.Path]::GetPathRoot($ResolvedEvidenceRoot)

if ($ResolvedEvidenceRoot.StartsWith($RepositoryRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Task 189 evidence must be stored outside the Git worktree.'
}
if ($ResolvedEvidenceRoot.TrimEnd('\') -eq $VolumeRoot.TrimEnd('\')) {
    throw 'Task 189 evidence root cannot be a filesystem volume root.'
}

New-Item -ItemType Directory -Force -Path $ResolvedEvidenceRoot | Out-Null

function Invoke-EvidenceCommand {
    param(
        [Parameter(Mandatory = $true)] [string]$Name,
        [Parameter(Mandatory = $true)] [scriptblock]$Command
    )

    $OutputPath = Join-Path $ResolvedEvidenceRoot "$Name.txt"
    & $Command 2>&1 | Tee-Object -FilePath $OutputPath
    $ExitCode = if ($null -eq $LASTEXITCODE) { 0 } else { $LASTEXITCODE }
    $ExitCode | Set-Content -Encoding utf8 (Join-Path $ResolvedEvidenceRoot "$Name.exit.txt")
    if ($ExitCode -ne 0) {
        throw "Evidence command '$Name' failed with exit code $ExitCode."
    }
}

if ($Action -eq 'Initialize') {
    Invoke-EvidenceCommand '01-git-head' { git -C $Dim0Root rev-parse HEAD }
    Invoke-EvidenceCommand '02-git-status-before' { git -C $RepositoryRoot status --short -- dim0 }
    Invoke-EvidenceCommand '03-docker-version' { docker version }
    Invoke-EvidenceCommand '04-compose-version' { docker compose version }
    Write-Output "Initialized external evidence directory: $ResolvedEvidenceRoot"
    exit 0
}

Invoke-EvidenceCommand '62-git-status-after' { git -C $RepositoryRoot status --short -- dim0 }
$Manifest = Join-Path $ResolvedEvidenceRoot 'manifest.sha256'
Get-ChildItem $ResolvedEvidenceRoot -File |
    Where-Object Name -ne 'manifest.sha256' |
    Sort-Object FullName |
    Get-FileHash -Algorithm SHA256 |
    ForEach-Object { "$($_.Hash.ToLowerInvariant())  $($_.FullName)" } |
    Set-Content -Encoding utf8 $Manifest
Write-Output "Wrote SHA-256 manifest: $Manifest"
