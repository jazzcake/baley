# Task #189: upstream Docker baseline execution

Status: **execution plan; no runtime implementation in this task**

Parent plan: [`codex-runtime-implementation-plan.md`](./codex-runtime-implementation-plan.md)

Normative spec: [`codex-runtime-and-persistence-spec.md`](./codex-runtime-and-persistence-spec.md)

Reviewed upstream baseline: `vcmf/dim0@c75cb32901aac30b1dd79f07d4a560d93b21750b`

## 1. Purpose and boundary

Establish a reproducible provider-free baseline for the existing Dim0 Docker stack before further Codex integration. The baseline proves what already builds, starts, persists, and renders; it does not prove provider generation, real embedding quality, or external search/OCR/code services.

Task #189 produces evidence only. Runtime, Compose, application, and test-harness code belongs to the later WP0 implementation commit. The operator must stop if a required provider tripwire cannot be installed: absence of an observed call is not proof that no call was possible.

Hard constraints:

- Do not submit an agent prompt or call a real LLM, embedding, search, fetch, OCR, image, or Daytona endpoint.
- Use deterministic fakes at all provider boundaries. A real provider check is a separate opt-in task and is not part of baseline acceptance.
- Never read, copy, print, or mount an existing developer `.env` or credential file.
- Keep environment files, logs, database dumps, screenshots, and Docker state outside Git.
- Use only the project name `dim0-task189`; never run Docker-wide prune or remove unrelated containers, networks, images, or volumes.
- Record facts as observed. Do not repair product code during the baseline run.

## 2. Harness prerequisites

WP0 must provide these test-only assets before the acceptance run:

- `backend/test/integration/baseline/test_provider_free_baseline.py`: exercises FastAPI lifespan plus board/note/link storage with live PostgreSQL, Qdrant, and Redis and a deterministic 512-dimensional fake embedder.
- `backend/test/integration/baseline/provider_tripwire.py`: replaces LLM, embedding, search, fetch, OCR, image, and Daytona network clients; it records construction and invocation separately and fails on any outbound invocation.
- `webui/src/features/agent/engine/__tests__/provider-free-baseline.test.ts`: asserts that page loading and basic non-agent canvas interactions construct no BYOK/provider LLM client instances.
- `build/docker-compose.baseline.yml`: a test-only overlay that adds provider-tripwire configuration without adding a provider service or weakening the upstream PostgreSQL/Qdrant/Redis topology.

The tripwire output schema is fixed:

```json
{
  "llm": 0,
  "embedding": 0,
  "search": 0,
  "fetch": 0,
  "ocr": 0,
  "image": 0,
  "daytona": 0
}
```

The harness may construct an embedding client only inside the isolated construction test. Every network invocation counter must remain zero throughout mandatory baseline execution.

## 3. Evidence layout

Use an external directory and one manifest. Commands below are PowerShell and start from the Baley repository root.

```powershell
$EvidenceRoot = 'C:\ProgramData\Dim0\validation\task-189'
$BaselineEnv = Join-Path $EvidenceRoot 'baseline.env'
$Manifest = Join-Path $EvidenceRoot 'manifest.sha256'
New-Item -ItemType Directory -Force -Path $EvidenceRoot | Out-Null
git -C dim0 rev-parse HEAD | Tee-Object (Join-Path $EvidenceRoot '01-git-head.txt')
git status --short -- dim0 | Tee-Object (Join-Path $EvidenceRoot '02-git-status-before.txt')
docker version | Tee-Object (Join-Path $EvidenceRoot '03-docker-version.txt')
docker compose version | Tee-Object (Join-Path $EvidenceRoot '04-compose-version.txt')
```

Create `baseline.env` with non-secret settings only:

```powershell
@'
DOPPLER_TOKEN=
API_PORT=8082
APP_PORT=5175
MINI_APP_PORT=5182
API_ORIGIN=http://localhost:8082
VITE_API_URL=http://localhost:8082
VITE_HOST_ORIGIN=http://localhost:5175
VITE_MINI_APP_ORIGIN=http://localhost:5182
EMAIL_VERIFICATION_ENABLED=false
PASSWORD_RESET_ENABLED=false
GOOGLE_CONNECT_ENABLED=false
OPENAI_AGENTS_DISABLE_TRACING=1
OPENAI_AGENTS_DONT_LOG_MODEL_DATA=1
OPENAI_AGENTS_DONT_LOG_TOOL_DATA=1
DIM0_BASELINE_PROVIDER_TRIPWIRE=1
DIM0_BASELINE_FAKE_EMBEDDING_DIMENSION=512
'@ | Set-Content -Encoding utf8 $BaselineEnv
$ComposeEnvPath = [IO.Path]::GetRelativePath((Resolve-Path 'dim0'), $BaselineEnv).Replace('\', '/')
```

The source Compose file resolves `ENVFILE` beneath `dim0/`; the relative path above allows its existing `../${ENVFILE}` mount to reach the external file without writing secrets or state into the worktree.

For every command, retain stdout/stderr and the exit code. The examples below show the canonical output file; the executor records `$LASTEXITCODE` in the adjacent `*.exit.txt` file before proceeding.

## 4. Stage A — static and build baseline

Run repository checks before starting services:

```powershell
Push-Location dim0
make lint-backend 2>&1 | Tee-Object (Join-Path $EvidenceRoot '10-lint-backend.txt')
$LASTEXITCODE | Set-Content (Join-Path $EvidenceRoot '10-lint-backend.exit.txt')
make test-backend 2>&1 | Tee-Object (Join-Path $EvidenceRoot '11-test-backend.txt')
$LASTEXITCODE | Set-Content (Join-Path $EvidenceRoot '11-test-backend.exit.txt')
make lint-ui 2>&1 | Tee-Object (Join-Path $EvidenceRoot '12-lint-ui.txt')
$LASTEXITCODE | Set-Content (Join-Path $EvidenceRoot '12-lint-ui.exit.txt')
make test-ui 2>&1 | Tee-Object (Join-Path $EvidenceRoot '13-test-ui.txt')
$LASTEXITCODE | Set-Content (Join-Path $EvidenceRoot '13-test-ui.exit.txt')
npm --prefix webui run build 2>&1 | Tee-Object (Join-Path $EvidenceRoot '14-webui-build.txt')
$LASTEXITCODE | Set-Content (Join-Path $EvidenceRoot '14-webui-build.exit.txt')
Pop-Location
```

Expected result: each exit file contains `0`; backend unit tests, frontend type/lint/tests, and the production Web UI build complete without a provider invocation. A dependency-install network fetch is environment setup, not a provider call, but it must be recorded separately from the acceptance run.

## 5. Stage B — Compose expansion and images

```powershell
$env:ENVFILE = $ComposeEnvPath
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv --profile test config 2>&1 | Tee-Object (Join-Path $EvidenceRoot '20-compose-config.yml')
$LASTEXITCODE | Set-Content (Join-Path $EvidenceRoot '20-compose-config.exit.txt')
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv --profile test config --services 2>&1 | Tee-Object (Join-Path $EvidenceRoot '21-compose-services.txt')
$LASTEXITCODE | Set-Content (Join-Path $EvidenceRoot '21-compose-services.exit.txt')
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv --profile test build backend-test webui-test 2>&1 | Tee-Object (Join-Path $EvidenceRoot '22-image-build.txt')
$LASTEXITCODE | Set-Content (Join-Path $EvidenceRoot '22-image-build.exit.txt')
```

Expected services are exactly `postgres-test`, `qdrant-test`, `redis-test`, `backend-test`, and `webui-test`. Expansion must show persistent PostgreSQL, Qdrant, and Redis volumes, no Codex profile, no real provider endpoint, and the external read-only `baseline.env` mount. Image build success is recorded independently from service startup.

## 6. Stage C — persistence services

```powershell
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv --profile test up -d postgres-test qdrant-test redis-test 2>&1 | Tee-Object (Join-Path $EvidenceRoot '30-persistence-up.txt')
$LASTEXITCODE | Set-Content (Join-Path $EvidenceRoot '30-persistence-up.exit.txt')
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv --profile test ps 2>&1 | Tee-Object (Join-Path $EvidenceRoot '31-persistence-ps.txt')
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv exec -T postgres-test pg_isready -U topix -d topix 2>&1 | Tee-Object (Join-Path $EvidenceRoot '32-postgres-health.txt')
Invoke-RestMethod http://localhost:6335/readyz | ConvertTo-Json -Compress | Set-Content (Join-Path $EvidenceRoot '33-qdrant-health.json')
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv exec -T redis-test redis-cli ping 2>&1 | Tee-Object (Join-Path $EvidenceRoot '34-redis-health.txt')
```

Expected results: PostgreSQL reports accepting connections for database/user `topix`; Qdrant `/readyz` succeeds; Redis prints `PONG`; all three containers are running and PostgreSQL/Redis are healthy. Capture `docker compose ... logs --no-color --tail 200` if any check fails.

## 7. Stage D — provider-free backend and storage contract

Run the live-storage test against the named Compose services. It must apply the PostgreSQL schema twice, create a Qdrant collection through the existing `GraphStore -> ContentStore` path, and use the deterministic fake embedder.

```powershell
Push-Location dim0/backend
$env:POSTGRES_HOST = 'localhost'
$env:POSTGRES_PORT = '5434'
$env:QDRANT_HOST = 'localhost'
$env:QDRANT_PORT = '6335'
$env:REDIS_HOST = 'localhost'
$env:REDIS_PORT = '6381'
$env:DIM0_BASELINE_PROVIDER_TRIPWIRE = '1'
$env:DIM0_BASELINE_FAKE_EMBEDDING_DIMENSION = '512'
uv run pytest -q test/integration/baseline/test_provider_free_baseline.py 2>&1 | Tee-Object (Join-Path $EvidenceRoot '40-storage-contract.txt')
$LASTEXITCODE | Set-Content (Join-Path $EvidenceRoot '40-storage-contract.exit.txt')
Pop-Location
```

The test records:

- PostgreSQL schema application succeeds twice and metadata/identity rows remain readable.
- Qdrant collection vector size is 512.
- Board creation, note creation, link creation, and retrieval succeed.
- Text create/update calls the fake embedder and performs the corresponding Qdrant write.
- Spatial/style-only note update calls neither the embedder nor vector update.
- A forced fake-embedding failure prevents Qdrant upsert/vector update; no zero vector is written.
- Redis ticket/sequence state is readable and incrementing.
- Provider invocation counters remain all zero.

## 8. Stage E — backend/Web UI smoke and restart persistence

```powershell
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv --profile test up -d backend-test webui-test 2>&1 | Tee-Object (Join-Path $EvidenceRoot '50-app-up.txt')
$LASTEXITCODE | Set-Content (Join-Path $EvidenceRoot '50-app-up.exit.txt')
Invoke-WebRequest http://localhost:8082/utils/ping -UseBasicParsing | Select-Object StatusCode | Out-File (Join-Path $EvidenceRoot '51-backend-ping.txt')
Invoke-WebRequest http://localhost:5175 -UseBasicParsing | Select-Object StatusCode | Out-File (Join-Path $EvidenceRoot '52-webui-http.txt')
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv --profile test ps 2>&1 | Tee-Object (Join-Path $EvidenceRoot '53-app-ps-before-restart.txt')
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv restart postgres-test qdrant-test redis-test backend-test 2>&1 | Tee-Object (Join-Path $EvidenceRoot '54-restart.txt')
Push-Location dim0/backend
uv run pytest -q test/integration/baseline/test_provider_free_baseline.py -k persisted_after_restart 2>&1 | Tee-Object (Join-Path $EvidenceRoot '55-persistence-after-restart.txt')
$LASTEXITCODE | Set-Content (Join-Path $EvidenceRoot '55-persistence-after-restart.exit.txt')
Pop-Location
```

Expected results: backend and Web UI return HTTP 2xx; after container restart, the seeded board, note, link, Qdrant payload/vector, and Redis-backed sequence contract remain readable. The persistence assertion must identify the records created before restart rather than creating replacements.

For the UI observation, load the existing board without opening the agent or external-tool controls. Capture one screenshot and a browser console/network export. The network export must contain no provider host and the frontend provider-construction assertion must remain zero.

## 9. Stage F — final evidence and cleanup

Before cleanup, record bounded logs, container/volume names, tripwire counters, and worktree state:

```powershell
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv --profile test logs --no-color --tail 300 2>&1 | Tee-Object (Join-Path $EvidenceRoot '60-compose-logs.txt')
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv --profile test ps --all 2>&1 | Tee-Object (Join-Path $EvidenceRoot '61-compose-ps-final.txt')
git status --short -- dim0 | Tee-Object (Join-Path $EvidenceRoot '62-git-status-after.txt')
Get-ChildItem $EvidenceRoot -File | Where-Object Name -ne 'manifest.sha256' | Get-FileHash -Algorithm SHA256 | ForEach-Object { "$($_.Hash.ToLower())  $($_.Path)" } | Set-Content $Manifest
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file $BaselineEnv --profile test down --remove-orphans 2>&1 | Tee-Object (Join-Path $EvidenceRoot '63-compose-down.txt')
Remove-Item Env:ENVFILE,Env:POSTGRES_HOST,Env:POSTGRES_PORT,Env:QDRANT_HOST,Env:QDRANT_PORT,Env:REDIS_HOST,Env:REDIS_PORT,Env:DIM0_BASELINE_PROVIDER_TRIPWIRE,Env:DIM0_BASELINE_FAKE_EMBEDDING_DIMENSION -ErrorAction SilentlyContinue
```

`down` intentionally preserves named volumes for rerun and investigation. Volume deletion is a separate explicit cleanup decision scoped to project `dim0-task189`.

## 10. Failure handling

On the first failed command:

1. Record its non-zero exit code, timestamp, command, bounded stdout/stderr, `docker compose ... ps --all`, and service logs.
2. Stop the acceptance sequence; do not continue and create misleading downstream failures.
3. Classify the first failure:
   - **environment**: Docker unavailable, port collision, disk/resource shortage, or missing local toolchain;
   - **upstream baseline**: reproducible failure in unmodified Dim0 behavior at the pinned baseline;
   - **harness**: fake/tripwire/fixture cannot represent the existing boundary or changes product behavior.
4. If any provider invocation counter is non-zero, treat the run as invalid and failed even if the request itself failed before billing.
5. Preserve containers and volumes for inspection unless they create a local safety problem. Use `down --remove-orphans` only after evidence capture.
6. Rerun from Stage A after correction into a new timestamped subdirectory; never overwrite failed evidence.

Do not change runtime code to make the baseline pass. Record a reproducible upstream defect or open a separate harness correction before attempting the baseline again.

## 11. Acceptance criteria

Task #189's baseline is accepted only when all are true:

1. The pinned Git SHA, dirty-state report, Docker versions, exact commands, exit codes, logs, and SHA-256 manifest are present outside the repository.
2. Backend lint/unit tests and Web UI check/test/production build pass.
3. Compose expansion names only the five expected test-profile services and images build locally.
4. PostgreSQL, Qdrant, and Redis health checks pass; schema application is idempotent.
5. FastAPI lifespan and `/utils/ping` succeed; the Web UI returns HTTP 2xx.
6. Board, note, and link CRUD uses the canonical stores with a deterministic 512-dimensional fake embedder.
7. PostgreSQL metadata, Qdrant content/vector payloads, and the Redis sequence contract remain valid after restart.
8. Text mutation embeds; spatial/style-only mutation does not; fake embedding failure prevents the storage mutation without a zero-vector fallback.
9. Provider construction is limited to explicitly asserted harmless construction tests, and every network invocation counter is zero.
10. Browser network evidence contains no provider request and the agent/external-tool flows were not exercised.
11. The final worktree report contains no baseline credential, log, screenshot, database, Qdrant, Redis, or generated environment artifact.

The acceptance report lists each criterion as pass/fail with artifact paths. Optional real embedding and external-service checks are explicitly outside this baseline and cannot compensate for a failed mandatory criterion.
