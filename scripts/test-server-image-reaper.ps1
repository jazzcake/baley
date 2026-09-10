[CmdletBinding()]
param(
  [Parameter(Mandatory)]
  [ValidatePattern('^[A-Za-z0-9][A-Za-z0-9._:/@-]{0,255}$')]
  [string]$Image
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$entrypoint = (docker image inspect $Image --format '{{json .Config.Entrypoint}}' | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $entrypoint -notmatch 'tini' -or $entrypoint -notmatch 'baley-server-entrypoint') {
  throw 'server image must run its Baley entrypoint below tini'
}

$probe = @'
for iteration in 1 2 3; do
  rm -f /tmp/baley-reaper-child
  sh -c 'sleep 30 & echo $! > /tmp/baley-reaper-child; wait' &
  parent=$!
  wait_count=0
  while [ ! -s /tmp/baley-reaper-child ] && [ "$wait_count" -lt 100 ]; do sleep 0.02; wait_count=$((wait_count + 1)); done
  if [ ! -s /tmp/baley-reaper-child ]; then echo 'transport child PID was not recorded' >&2; exit 1; fi
  child=$(cat /tmp/baley-reaper-child)
  kill -KILL "$child" "$parent" 2>/dev/null || true
  wait "$parent" 2>/dev/null || true
  reap_count=0
  while [ -e "/proc/$child/stat" ] && [ "$reap_count" -lt 100 ]; do sleep 0.02; reap_count=$((reap_count + 1)); done
  if [ -e "/proc/$child/stat" ]; then
    echo "unreaped transport descendant $child after iteration $iteration" >&2
    exit 1
  fi
done
'@

$encodedProbe = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($probe))
$runner = "echo '$encodedProbe' | base64 -d | /bin/sh"
docker run --rm $Image /bin/sh -ec $runner
if ($LASTEXITCODE -ne 0) { throw 'server image PID-1 reaper smoke test failed' }
Write-Output 'PASS server image reaped adversarial transport descendants across 3 timeouts.'
