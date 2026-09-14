# Task #189: upstream Docker baseline execution

Status: **execution plan; no runtime implementation in this task**

Parent plan: [`codex-runtime-implementation-plan.md`](./codex-runtime-implementation-plan.md)

Normative spec: [`codex-runtime-and-persistence-spec.md`](./codex-runtime-and-persistence-spec.md)

Reviewed upstream baseline: `vcmf/dim0@c75cb32901aac30b1dd79f07d4a560d93b21750b`

## 1. Purpose and boundary

Establish a reproducible provider-free baseline for the existing Dim0 Docker stack before further Codex integration. The baseline proves what already builds, starts, persists, and renders; it does not prove provider generation, real embedding quality, or external search/OCR/code services.

Task #189 produces evidence only. Runtime, Compose, application, and test-harness code belongs to the later WP0 implementation commit. The operator must stop if a required provider tripwire cannot be installed: absence of an observed call is not proof that no call was possible.

The remaining H5 item is the next fresh Stage A-F execution of this corrected harness. It is not an implementation defect and cannot be closed by reusing historical evidence.

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
- `backend/test/integration/baseline/provider_free_pytest.py`: prevents the backend unit suite from consulting Doppler by returning an empty configuration before application modules are collected; it is used only in the network-isolated Stage A container.
- `webui/src/features/agent/engine/__tests__/provider-free-baseline.test.tsx`: mounts the real `HarnessCanvas` application boundary and asserts that page loading plus a basic non-agent viewport interaction construct no BYOK/provider LLM client instances.
- `build/docker-compose.baseline.yml`: a test-only overlay that adds provider-tripwire configuration without adding a provider service or weakening the upstream PostgreSQL/Qdrant/Redis topology.
- `build/Dockerfile.task189-browser` and `webui/scripts/task189-browser-observation.mjs`: a disposable Docker-only Chromium observer that creates and opens a local board, records a non-agent canvas interaction, removes credential-bearing HAR fields, and fails on any provider host.

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

Use a unique external run directory and one manifest. Commands below are PowerShell and start from the Baley repository root. The helper creates the run exclusively (timestamp plus random suffix), records provenance, and refuses further writes after finalization.

```powershell
$Harness = Resolve-Path 'dim0/build/capture-task-189-evidence.ps1'
$Init = & $Harness -Action Initialize | ConvertFrom-Json
$EvidenceRoot = $Init.runDirectory
$BaselineEnv = $Init.baselineEnv
$Manifest = Join-Path $EvidenceRoot 'manifest.sha256'
$Compose = "docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file `"$BaselineEnv`" --profile test"
& $Harness -Action Preflight -RunDirectory $EvidenceRoot
```

`Initialize` creates `baseline.env` with non-secret settings only. It includes absolute forward-slash `BASELINE_ENV_FILE` and `BASELINE_RUN_DIR` values, allowing the `D:` repository to bind the `C:` evidence/environment roots through Compose long syntax.

```powershell
Get-Content $BaselineEnv
```

The overlay replaces the base `/.env` mount by container target and binds the evidence directory at `/baseline-evidence`, where the backend exports both counter files directly.

For every command, use the helper's `Run` action with a unique `NN-lowercase-kebab-case` name. It records exact command text, working directory, timestamps, duration, exit code, and the bounded final 1,000 log lines in `commands.jsonl`, `NAME.txt`, and `NAME.exit.txt`; duplicate names are rejected rather than overwritten.

## 4. Stage A — static and build baseline

Run repository checks before starting services. The Windows host does not need
GNU Make or project package installation: the exact commands behind the Make
targets run inside the locked backend and Web UI images. The first two commands
build those images from the current checkout, then install the backend's locked
development dependencies into a run-unique, project-labelled Docker volume.
That dependency fetch is recorded separately and is the only Stage A command
with container network access. Every acceptance check runs with Docker network
mode `none`, blank provider credentials, and the provider tripwire enabled.

```powershell
$StageAVenv = "dim0-task189_stage_a_$((Split-Path $EvidenceRoot -Leaf) -replace '[^a-zA-Z0-9_.-]', '-')"
$BlankProviderEnv = '-e DOPPLER_TOKEN= -e OPENAI_API_KEY= -e ANTHROPIC_API_KEY= -e OPENROUTER_API_KEY= -e MISTRAL_API_KEY= -e PERPLEXITY_API_KEY= -e TAVILY_API_KEY= -e LINKUP_API_KEY= -e EXA_API_KEY= -e DAYTONA_API_KEY= -e OPENAI_AGENTS_DISABLE_TRACING=1 -e OPENAI_AGENTS_DONT_LOG_MODEL_DATA=1 -e OPENAI_AGENTS_DONT_LOG_TOOL_DATA=1 -e LITELLM_LOCAL_MODEL_COST_MAP=True -e DIM0_BASELINE_PROVIDER_TRIPWIRE=1'
$BackendStageA = "docker run --rm --label com.docker.compose.project=dim0-task189 --network none --mount `"type=volume,source=$StageAVenv,target=/app/.venv`" $BlankProviderEnv dim0-task189-backend-test:latest sh -lc"
$WebUiStageA = 'docker run --rm --label com.docker.compose.project=dim0-task189 --network none -e DIM0_BASELINE_PROVIDER_TRIPWIRE=1 -e "NODE_OPTIONS=--max_old_space_size=4096 --localstorage-file=/tmp/task189-localstorage" --entrypoint sh dim0-task189-webui-test:latest -lc'
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '08-stage-a-images' -CommandText "$Compose build backend-test webui-test"
$StageADeps = "docker volume create --label com.docker.compose.project=dim0-task189 $StageAVenv; if (`$LASTEXITCODE -ne 0) { throw 'Stage A venv volume creation failed.' }; docker run --rm --label com.docker.compose.project=dim0-task189 --network bridge --mount `"type=volume,source=$StageAVenv,target=/app/.venv`" $BlankProviderEnv dim0-task189-backend-test:latest sh -lc 'uv sync --frozen'"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '09-stage-a-backend-deps' -CommandText $StageADeps
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '10-lint-backend' -CommandText "$BackendStageA 'uv run --offline --frozen ruff check topix test/unit'"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '11-test-backend' -CommandText "$BackendStageA 'uv run --offline --frozen pytest -o log_cli=false -p test.integration.baseline.provider_free_pytest test/unit'"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '12-lint-ui' -CommandText "$WebUiStageA 'npm run check-all'"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '13-test-ui' -CommandText "$WebUiStageA 'npm run test:run'"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '14-webui-build' -CommandText "$WebUiStageA 'npm run build'"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '15-provider-tripwire-self-test' -CommandText "$BackendStageA 'uv run --offline --frozen pytest -q test/integration/baseline/test_provider_tripwire.py'"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '16-evidence-finalization-self-test' -CommandText 'powershell -NoProfile -File build/capture-task-189-evidence.tests.ps1' -WorkingDirectory dim0
```

Expected result: each exit file contains `0`; backend unit tests, the positive provider-tripwire self-test, evidence finalization self-tests, frontend type/lint/tests, and the production Web UI build pass. The focused tripwire self-test must prove that supported provider boundaries increment their counters and fail before outbound I/O; it does not make a real provider call. A dependency-install network fetch is environment setup, not a provider call, but it must be recorded separately from the acceptance run.

The Stage A pytest bootstrap replaces only the imported Doppler configuration
loader with a deterministic empty configuration before test collection. Docker
network mode `none` remains the enforcement boundary for all test commands.
Pytest live logging is disabled for the passing unit suite so test-owned fake
token values are not exported into the credential-screened evidence; failure
tracebacks and the complete test result remain recorded.
The Web UI image currently uses Node 25 although the package supports Node 20
and 22; the run-local `--localstorage-file` option restores Node's complete
Web Storage implementation for the jsdom suite without changing application
code or persisting browser state outside the disposable container.

## 5. Stage B — Compose expansion and images

```powershell
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '20-compose-config' -CommandText "$Compose config"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '21-compose-services' -CommandText "$Compose config --services"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '22-image-build' -CommandText "$Compose build backend-test webui-test"
```

Expected services are exactly `postgres-test`, `qdrant-test`, `redis-test`, `backend-test`, and `webui-test`. Expansion must show persistent PostgreSQL, Qdrant, and Redis volumes, no Codex profile, no real provider endpoint, and the external read-only `baseline.env` mount. Image build success is recorded independently from service startup.

## 6. Stage C — persistence services

```powershell
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '30-persistence-up' -CommandText "$Compose up -d postgres-test qdrant-test redis-test"
$PersistenceReady = "`$deadline=(Get-Date).AddMinutes(3); `$ready=`$false; do { docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file `"$BaselineEnv`" --profile test exec -T postgres-test pg_isready -U topix -d topix; `$pg=`$LASTEXITCODE; docker compose -p dim0-task189 -f dim0/build/docker-compose.yml -f dim0/build/docker-compose.baseline.yml --env-file `"$BaselineEnv`" --profile test exec -T redis-test redis-cli ping; `$redis=`$LASTEXITCODE; `$qdrant=`$false; try { `$null=Invoke-RestMethod http://localhost:16335/readyz; `$qdrant=`$true } catch {}; if (`$pg -eq 0 -and `$redis -eq 0 -and `$qdrant) { `$ready=`$true; break }; Start-Sleep -Seconds 2 } while ((Get-Date) -lt `$deadline); if (-not `$ready) { throw 'Persistence readiness deadline exceeded.' }; Write-Output 'PostgreSQL, Qdrant, and Redis are ready.'"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '31-persistence-ready-wait' -CommandText $PersistenceReady
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '32-persistence-ps' -CommandText "$Compose ps"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '33-postgres-health' -CommandText "$Compose exec -T postgres-test pg_isready -U topix -d topix"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '34-qdrant-health' -CommandText 'Invoke-RestMethod http://localhost:16335/readyz | ConvertTo-Json -Compress'
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '35-redis-health' -CommandText "$Compose exec -T redis-test redis-cli ping"
```

Expected results: PostgreSQL reports accepting connections for database/user `topix`; Qdrant `/readyz` succeeds; Redis prints `PONG`; all three containers are running and PostgreSQL/Redis are healthy. Capture `docker compose ... logs --no-color --tail 200` if any check fails.

## 7. Stage D — provider-free backend and storage contract

Run the live-storage test against the named Compose services. It must apply the PostgreSQL schema twice, create a Qdrant collection through the existing `GraphStore -> ContentStore` path, and use the deterministic fake embedder.

```powershell
$StageDContainer = "docker run --rm --network dim0-task189_default --env-file `"$BaselineEnv`" --mount `"type=bind,source=$EvidenceRoot,target=/baseline-evidence`" --mount `"type=volume,source=$StageAVenv,target=/app/.venv`" -e POSTGRES_HOST=postgres-test -e POSTGRES_PORT=5432 -e QDRANT_HOST=qdrant-test -e QDRANT_PORT=6333 -e REDIS_HOST=redis-test -e REDIS_PORT=6379 -e DIM0_BASELINE_PROVIDER_TRIPWIRE=1 -e DIM0_BASELINE_FAKE_EMBEDDING_DIMENSION=512 -e DIM0_BASELINE_TRIPWIRE_OUTPUT=/baseline-evidence/provider-invocations.json -e DIM0_BASELINE_CONSTRUCTION_OUTPUT=/baseline-evidence/provider-constructions.json -e LITELLM_LOCAL_MODEL_COST_MAP=True dim0-task189-backend-test:latest"
$StageDTest = "$StageDContainer sh -lc 'uv run --offline --frozen pytest -q test/integration/baseline/test_provider_free_baseline.py -k provider_free_board_content_crud_and_persistence 2>&1'"
try {
    & $Harness -Action Run -RunDirectory $EvidenceRoot -Name '40-storage-contract' -CommandText $StageDTest
}
catch {
    $StageDFailure = $_
    $StageDLog = Join-Path $EvidenceRoot '40-storage-contract.txt'
    $ProtocolRecurrence =
        (Select-String -LiteralPath $StageDLog -SimpleMatch 'protocol.pyx' -Quiet) -and
        (Select-String -LiteralPath $StageDLog -SimpleMatch "'NoneType' object has no attribute 'decode'" -Quiet)
    if ($ProtocolRecurrence) {
        $StageDRuntime = "$StageDContainer sh -lc 'python -VV; python -c `"import asyncpg,asyncpg.protocol.protocol as p,hashlib,pathlib; q=pathlib.Path(p.__file__); print(`"asyncpg=`"+asyncpg.__version__); print(`"protocol_extension=`"+q.name); print(`"protocol_sha256=`"+hashlib.sha256(q.read_bytes()).hexdigest()); print(`"dsn=postgresql://topix@postgres-test:5432/topix`")`"'"
        $StageDDiagnostics = @(
            @{ Name = '41-storage-runtime-on-recurrence'; Command = $StageDRuntime },
            @{ Name = '42-postgres-runtime-on-recurrence'; Command = "docker exec dim0-task189-postgres sh -lc 'psql -U topix -d topix -At -v ON_ERROR_STOP=1 -c `"SELECT version(); SHOW server_encoding; SHOW client_encoding;`" 2>&1'" },
            @{ Name = '43-postgres-transport-on-recurrence'; Command = 'docker logs --tail 200 dim0-task189-postgres' }
        )
        foreach ($Diagnostic in $StageDDiagnostics) {
            try {
                & $Harness -Action Run -RunDirectory $EvidenceRoot -Name $Diagnostic.Name -CommandText $Diagnostic.Command
            }
            catch {
                Write-Warning "Bounded Stage D diagnostic failed: $($Diagnostic.Name)"
            }
        }
    }
    throw $StageDFailure
}
```

`docker run` uses the exact `dim0-task189-backend-test:latest` image already
built in Stage B and joins the Compose project's isolated
`dim0-task189_default` network. It mounts the unique evidence directory, loads
the generated non-secret `baseline.env`, reuses the locked development
environment prepared in Stage A, and repeats the overlay's
provider-tripwire settings and output paths. The
explicit service-name endpoints preserve the container DSN
`postgresql://topix@postgres-test:5432/topix` and prevent host environment
variables from redirecting the test to published loopback ports. The command
requires the three Stage C services to remain the only persistence services;
`--rm` removes only the one-off Stage D container. The container shell merges
pytest's stderr into its stdout so PowerShell records the complete bounded
stream instead of treating native status output as a terminating error. The
shell still returns pytest's exit code unchanged. Stage D selects only the
pre-restart seed/contract case; Stage E selects the read-only persistence case
after the required container restart. Before seeding, the fixture deletes only
its own fixed Task 189 board if a preserved project volume contains an earlier
attempt, leaving all unrelated data untouched.

The catch inspects the bounded test log and runs diagnostics only if the known
asyncpg `protocol.pyx` missing-status decode signature recurs. Those non-secret
diagnostics record the locked Python/asyncpg protocol extension identity and
hash, the sanitized DSN shape, PostgreSQL runtime and encoding, and the final
200 PostgreSQL log lines. The catch then rethrows the original failure,
preserving the first-failure stop and non-zero result; it must not be used to
continue to Stage E.

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
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '50-app-up' -CommandText "$Compose up -d backend-test webui-test"
$AppReady = "`$deadline=(Get-Date).AddMinutes(3); `$ready=`$false; do { try { `$api=(Invoke-WebRequest http://localhost:18082/utils/ping -UseBasicParsing).StatusCode } catch { `$api=0 }; try { `$ui=(Invoke-WebRequest http://localhost:15175 -UseBasicParsing).StatusCode } catch { `$ui=0 }; if (`$api -ge 200 -and `$api -lt 300 -and `$ui -ge 200 -and `$ui -lt 300) { `$ready=`$true; break }; Start-Sleep -Seconds 2 } while ((Get-Date) -lt `$deadline); if (-not `$ready) { throw 'Application readiness deadline exceeded.' }; Write-Output `"backend=`$api webui=`$ui`""
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '51-app-ready-wait' -CommandText $AppReady
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '52-backend-ping' -CommandText 'Invoke-WebRequest http://localhost:18082/utils/ping -UseBasicParsing | Select-Object StatusCode'
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '53-webui-http' -CommandText 'Invoke-WebRequest http://localhost:15175 -UseBasicParsing | Select-Object StatusCode'
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '54-app-ps-before-restart' -CommandText "$Compose ps"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '55-restart' -CommandText "$Compose restart postgres-test qdrant-test redis-test backend-test"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '56-persistence-ready-after-restart' -CommandText $PersistenceReady
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '57-persistence-after-restart' -CommandText "$StageDContainer sh -lc 'uv run --offline --frozen pytest -q test/integration/baseline/test_provider_free_baseline.py -k persisted_after_restart 2>&1'"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '58-browser-image-build' -CommandText 'docker build -f dim0/build/Dockerfile.task189-browser -t dim0-task189-browser-observer:latest dim0'
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '59-browser-observation' -CommandText "docker run --rm --label com.docker.compose.project=dim0-task189 --network container:dim0-task189-webui --mount `"type=bind,source=$EvidenceRoot,target=/baseline-evidence`" -e TASK189_WEBUI_URL=http://localhost -e TASK189_EVIDENCE_DIR=/baseline-evidence dim0-task189-browser-observer:latest"
```

Expected results: backend and Web UI return HTTP 2xx; after container restart, the seeded board, note, link, Qdrant payload/vector, and Redis-backed sequence contract remain readable. The persistence assertion must identify the records created before restart rather than creating replacements.

The bounded persistence readiness command is repeated after restart so the
read-only assertion never races PostgreSQL recovery, Qdrant shard readiness,
or Redis startup.

The recorded Stage A frontend test mounts the real `HarnessCanvas`, observes its canvas host, and uses its viewport control without opening agent or external-tool controls. For the Stage E browser observation, load the existing board without opening those controls and export sanitized `browser-console.json` and `browser-network.har`; the network export must contain no provider host and both files must contain no authorization, cookie, session, or credential-bearing URL data. Screenshots and other binary artifacts are deliberately unsupported because this helper cannot credential-screen their pixels.

## 9. Stage F — final evidence and cleanup

Before cleanup, record bounded logs, container/volume names, tripwire counters, and worktree state:

```powershell
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '60-compose-logs' -CommandText "$Compose logs --no-color --tail 300"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '61-compose-ps-final' -CommandText "$Compose ps --all"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '62-git-status-after' -CommandText 'git status --short -- dim0'
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '63-compose-down' -CommandText "$Compose down --remove-orphans"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '64-stage-a-volume-cleanup' -CommandText "docker volume rm $StageAVenv"
& $Harness -Action Run -RunDirectory $EvidenceRoot -Name '65-browser-image-cleanup' -CommandText 'docker image rm dim0-task189-browser-observer:latest'
& $Harness -Action ValidateTripwires -RunDirectory $EvidenceRoot
& $Harness -Action Finalize -RunDirectory $EvidenceRoot
Remove-Item Env:POSTGRES_HOST,Env:POSTGRES_PORT,Env:QDRANT_HOST,Env:QDRANT_PORT,Env:REDIS_HOST,Env:REDIS_PORT,Env:DIM0_BASELINE_PROVIDER_TRIPWIRE,Env:DIM0_BASELINE_FAKE_EMBEDDING_DIMENSION -ErrorAction SilentlyContinue
```

`Finalize` accepts only the explicit safe artifact allowlist recorded in `finalization-policy.json`; unclassified files, subdirectories, and unsupported binary artifacts fail closed. It rejects recognized credentials, credential-bearing URLs, authorization/cookie headers, and non-empty session fields, writes `secret-screening.json`, and then hashes the allowlisted evidence plus `finalization-policy.json` and `integrity-metadata.json`. The final marker binds the manifest hash and makes later helper actions refuse the run; it explicitly does not claim filesystem immutability against direct external writers.

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
