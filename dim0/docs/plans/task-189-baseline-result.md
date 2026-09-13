# Task #189: provider-free Docker baseline result

Date: 2026-09-14 (Asia/Seoul)

Outcome: **not accepted; provider-protected application validation blocked, safe infrastructure subset passed**

Execution source: [`task-189-baseline-execution.md`](./task-189-baseline-execution.md)

## Executive result

The required provider-free harness is not present at the current Baley repository HEAD. In particular, the baseline Compose overlay, provider tripwire, backend live-storage test, and frontend provider-construction test are absent. The execution plan says to stop application acceptance when the tripwire cannot be installed, so no backend or Web UI container was built or started and no board/content API or UI action was attempted.

With coordinator approval, the run was limited to read-only Base Compose diagnostics and isolated PostgreSQL, Qdrant, and Redis checks under Compose project `dim0-task189`. All three persistence services became reachable, retained dedicated probe data across a restart, and were then removed together with only the network and volumes carrying the `com.docker.compose.project=dim0-task189` label. No OpenAI, Anthropic, OpenRouter, generative, embedding, search, fetch, OCR, image, or Daytona credential or endpoint was configured or invoked.

## Source and environment

| Check | Observed result |
| --- | --- |
| `git -C dim0 rev-parse HEAD` | `bf6563fc249a8cc5eacafe0c0ce524f1b97a0706` |
| Plan's reviewed upstream baseline | `vcmf/dim0@c75cb32901aac30b1dd79f07d4a560d93b21750b` |
| SHA deviation | Expected: `c75cb329` is the imported upstream subtree base; `bf6563fc` is the current Baley repository HEAD containing planning commits. |
| `git status --short -- dim0` before execution | Clean |
| Repository status outside `dim0` | Pre-existing untracked `debug.log`; not opened, modified, staged, or committed |
| Docker client/server | `29.2.1` / `29.2.1`; Docker Desktop `4.61.0 (219004)`; context `desktop-linux` |
| Docker Compose | `v5.0.2` |
| Existing containers | `baley-api-1`, `baley-viewer-1`, `local-dev-qdrant`, `daytripper-media`, `local-dev-postgres`, and `daytripper-pipeline`; none was changed |
| Conflicting `postgres-test`, `qdrant-test`, or `redis-test` container before start | None |

## Missing prerequisites and safety decision

All four prerequisite probes returned `False`:

```text
dim0/build/docker-compose.baseline.yml=False
dim0/backend/test/integration/baseline/test_provider_free_baseline.py=False
dim0/backend/test/integration/baseline/provider_tripwire.py=False
dim0/webui/src/features/agent/engine/__tests__/provider-free-baseline.test.ts=False
```

Base Compose expansion succeeded and listed `postgres-test`, `qdrant-test`, `redis-test`, `backend-test`, and `webui-test`, plus volumes `qdrant_data_test`, `pg_data_test`, `redis_data_test`, and `backend_data_test`. A case-insensitive scan of the expansion found zero `openai`, `anthropic`, `openrouter`, or `codex` strings, but also zero baseline tripwire/fake-embedding settings. Critically, Base Compose resolves the application mount to the developer path below rather than the required external `baseline.env`:

```text
source: D:\Project_AI\baley\dim0\.env
target: /.env
read_only: true
```

The source `.env` was not read. Because starting `backend-test` would mount it and could instantiate an unprotected provider client, application execution stopped at this boundary. The coordinator explicitly authorized only Base Compose static/config diagnostics and DB-only health/persistence checks.

## Commands and outcomes

Commands ran from `D:\Project_AI\baley` unless noted. `$env:DOPPLER_TOKEN=''` was used only as a non-secret Compose interpolation value; Compose reported that an empty value was treated as unset and defaulted it to blank.

### Inspection and config

```powershell
git status --short
git -C dim0 rev-parse HEAD
git status --short -- dim0
Get-Content -Raw 'dim0/build/docker-compose.yml'
Get-Content -Raw 'dim0/Makefile'
docker version
docker compose version
docker ps --format '{{.Names}}|{{.Image}}|{{.Ports}}'
docker ps -a --filter name='^postgres-test$' --filter name='^qdrant-test$' --filter name='^redis-test$' --format '{{.Names}}|{{.Status}}|{{.Labels}}'
$env:DOPPLER_TOKEN=''; docker compose -p dim0-task189 -f dim0/build/docker-compose.yml --profile test config --services
$env:DOPPLER_TOKEN=''; docker compose -p dim0-task189 -f dim0/build/docker-compose.yml --profile test config --volumes
$env:DOPPLER_TOKEN=''; $cfg = docker compose -p dim0-task189 -f dim0/build/docker-compose.yml --profile test config 2>&1
```

Outcome: all inspection/config commands exited `0`. Full config expansion exited `0`; provider/Codex string hits were `0`, baseline tripwire setting hits were `0`, and the unsafe-for-this-run `.env` mount shown above was present.

### Persistence service startup and health

```powershell
$env:DOPPLER_TOKEN=''; docker compose -p dim0-task189 -f dim0/build/docker-compose.yml --profile test up -d postgres-test qdrant-test redis-test
$env:DOPPLER_TOKEN=''; docker compose -p dim0-task189 -f dim0/build/docker-compose.yml --profile test ps
$env:DOPPLER_TOKEN=''; docker compose -p dim0-task189 -f dim0/build/docker-compose.yml exec -T postgres-test pg_isready -U topix -d topix
(Invoke-WebRequest 'http://localhost:6335/readyz' -UseBasicParsing).StatusCode
$env:DOPPLER_TOKEN=''; docker compose -p dim0-task189 -f dim0/build/docker-compose.yml exec -T redis-test redis-cli ping
docker logs --tail 120 postgres-test
Start-Sleep -Seconds 8
$env:DOPPLER_TOKEN=''; docker compose -p dim0-task189 -f dim0/build/docker-compose.yml exec -T postgres-test pg_isready -U topix -d topix
```

Outcome: `up` exited `0`. Qdrant returned HTTP `200`; Redis returned `PONG`. PostgreSQL's first probes during first-time schema initialization returned `no response` (exit `2`) and then `rejecting connections`; bounded logs showed normal entrypoint initialization, schema DDL, shutdown of the temporary bootstrap server, and startup of the final server. The subsequent probe returned `accepting connections`; final pre-cleanup status was PostgreSQL healthy, Redis healthy, and Qdrant running.

### Seed and restart-persistence checks

```powershell
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml exec -T postgres-test psql -U topix -d topix -v ON_ERROR_STOP=1 -c "CREATE TABLE IF NOT EXISTS task189_probe (id integer PRIMARY KEY, value text NOT NULL); INSERT INTO task189_probe (id, value) VALUES (1, 'persisted') ON CONFLICT (id) DO UPDATE SET value = EXCLUDED.value; SELECT id, value FROM task189_probe;"
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml exec -T redis-test redis-cli SET task189:probe persisted
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml exec -T redis-test redis-cli INCR task189:sequence
Invoke-RestMethod -Method Put -Uri 'http://localhost:6335/collections/task189_probe' -ContentType 'application/json' -Body '{"vectors":{"size":4,"distance":"Cosine"}}'
$env:DOPPLER_TOKEN=''; docker compose -p dim0-task189 -f dim0/build/docker-compose.yml restart postgres-test qdrant-test redis-test
Start-Sleep -Seconds 8
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml exec -T postgres-test psql -U topix -d topix -At -c "SELECT id || ':' || value FROM task189_probe WHERE id=1;"
Invoke-RestMethod -Uri 'http://localhost:6335/collections/task189_probe'
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml exec -T redis-test redis-cli GET task189:probe
docker compose -p dim0-task189 -f dim0/build/docker-compose.yml exec -T redis-test redis-cli GET task189:sequence
```

Outcome: all seed and restart commands exited `0`. After restart PostgreSQL returned `1:persisted`; Qdrant returned collection status `green` with vector size `4` and distance `Cosine`; Redis returned `persisted` and sequence `1`. This validates container-volume persistence only, not the Dim0 `GraphStore -> ContentStore` or board/note/link contract.

### Cleanup

The exact label-scoped resources found before cleanup were containers `postgres-test`, `qdrant-test`, `redis-test`; volumes `dim0-task189_pg_data_test`, `dim0-task189_qdrant_data_test`, `dim0-task189_redis_data_test`; and network `dim0-task189_default`.

```powershell
docker ps -a --filter label=com.docker.compose.project=dim0-task189 --format '{{.Names}}|{{.Status}}'
docker volume ls --filter label=com.docker.compose.project=dim0-task189 --format '{{.Name}}'
docker network ls --filter label=com.docker.compose.project=dim0-task189 --format '{{.Name}}'
$env:DOPPLER_TOKEN=''; docker compose -p dim0-task189 -f dim0/build/docker-compose.yml --profile test down --volumes --remove-orphans
docker ps -a --filter label=com.docker.compose.project=dim0-task189 --format '{{.Names}}'
docker volume ls --filter label=com.docker.compose.project=dim0-task189 --format '{{.Name}}'
docker network ls --filter label=com.docker.compose.project=dim0-task189 --format '{{.Name}}'
```

Outcome: `down` exited `0`. All three attempt-created containers and volumes and the attempt-created network were removed. The three final label-filtered listings were empty.

## Acceptance criteria

| # | Criterion | Result | Evidence / blocker |
| --- | --- | --- | --- |
| 1 | Pinned SHA, dirty state, Docker versions, logs, manifest | **Blocked** | Current HEAD is expected Baley HEAD rather than upstream subtree base. Facts are recorded here, but the external acceptance artifact/manifest sequence was not started after the prerequisite hard stop. |
| 2 | Backend lint/unit and Web UI check/test/build | **Blocked** | Not run in the coordinator-authorized safe subset; provider-free harness absent. |
| 3 | Baseline Compose expansion and app image build | **Blocked** | Base Compose expansion passed, but required overlay is absent and images were not built. |
| 4 | PostgreSQL/Qdrant/Redis and idempotent schema | **Partial pass** | Health and restart persistence passed. The harness assertion applying the schema twice was unavailable. |
| 5 | FastAPI lifespan, `/utils/ping`, Web UI HTTP 2xx | **Blocked** | App startup forbidden without verified tripwire. |
| 6 | Board/note/link CRUD with fake 512-d embedder | **Blocked** | Backend baseline test and fake embedder absent. |
| 7 | PostgreSQL/Qdrant/Redis application contract after restart | **Partial pass** | Infrastructure probes persisted; canonical app records could not be created safely. |
| 8 | Mutation/fake-embedding failure semantics | **Blocked** | Harness absent. |
| 9 | Provider construction bounded and invocation counters zero | **Blocked** | Tripwire/counter implementation absent. No app path was run; absence of observed calls is not accepted as proof. |
| 10 | Browser network and frontend construction evidence | **Blocked** | Web UI not started; frontend assertion absent. |
| 11 | Clean worktree/no generated acceptance artifacts | **Pass** | Only this result document was created under `dim0`; `debug.log` remained untouched; attempt Docker resources were removed. |

## Deviations and remaining work

- The run used Base Compose only for diagnostics and DB-only services because `build/docker-compose.baseline.yml` is missing. It did not claim the Base Compose stack as the provider-free acceptance stack.
- No external `baseline.env`, artifact directory, screenshots, browser trace, log bundle, or manifest was created because the prerequisite check blocked acceptance before Stage A. No existing developer environment file was read or mounted into a running container.
- Docker application images, backend health, Web UI reachability, browser observation, and board/content persistence remain unvalidated.
- WP0 must supply the four prerequisite assets from the execution plan. The full baseline must then be rerun from Stage A in a fresh evidence directory, with every provider invocation counter equal to zero, before Task #189 can be accepted.
