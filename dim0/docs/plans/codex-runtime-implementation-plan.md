# Detailed implementation plan: Dim0 Codex runtime fork

Status: **reviewed, reconciled, and ready for implementation**

Review record: [`codex-runtime-plan-review.md`](./codex-runtime-plan-review.md) (all blocking and important amendments are incorporated here)

Normative spec: [`codex-runtime-and-persistence-spec.md`](./codex-runtime-and-persistence-spec.md)

Baseline execution contract: [`task-189-baseline-execution.md`](./task-189-baseline-execution.md)

Upstream baseline: `vcmf/dim0@c75cb32901aac30b1dd79f07d4a560d93b21750b`

Fork location: `dim0/` squash subtree in the Baley repository

## 1. Outcome

Deliver an independently runnable Dim0 web service whose browser agent uses one persistent local `codex app-server` process per backend worker for generation, while retaining upstream PostgreSQL, Qdrant, Redis, canvas, document, embedding, search/fetch/OCR, and Daytona behavior.

In the explicit Codex profile, provider keys may serve embeddings and non-LLM tools but can never select a direct LLM completion route. Codex failure is visible and never falls back to LiteLLM, OpenAI Agents, browser BYOK LLM, or `codex exec`.

## 2. Constraints and non-goals

- No Baley/Dim0 data or API integration.
- No Qdrant removal, persistence rewrite, semantic/RAG redesign, or embedding replacement.
- No broad agent/canvas restructuring.
- No committed credential, `CODEX_HOME`, database data, or Qdrant data.
- Preserve the upstream provider runtime outside the explicit Codex profile.
- Keep fork-specific changes localized and easy to replay after subtree pulls.
- Live provider calls are opt-in only. All mandatory contract tests use fakes and prove which boundaries were not called.

## 3. Prototype disposition

Commit `7c0136e` is a prototype, not the implementation baseline.

| Prototype area | Keep | Required correction |
|---|---|---|
| `/ai/llm[/stream]` branch | Existing HTTP contract and injected runtime direction | Centralize policy selection and test fail-closed behavior |
| Persistent app-server subprocess | Stdio transport and initialize/thread/turn primitives | Add generations, bounds, correlation, cancellation, restart backoff, and shutdown |
| `X-Run-Id -> threadId` | Browser run identity | Key by authenticated user plus canonical run UUID; reconcile growing histories and retries |
| Structured tool choice | Browser remains the tool executor | Validate exclusive output mode, tool name, JSON Schema, call IDs, and size |
| Streaming | Existing NDJSON event names | Describe Codex output as buffered and emit no token delta before validation |
| Docker overlay | Local image build and external `CODEX_HOME` | Split common, bundled-PG, and external-PG files and enforce external state paths |

## 4. Work packages

### WP0 — Baseline and evidence harness

Purpose: distinguish upstream defects from fork regressions without contacting any provider.

Changes and execution:

- Use [`task-189-baseline-execution.md`](./task-189-baseline-execution.md) as the command sequence, evidence manifest, failure ledger, and acceptance contract.
- Add test doubles at the existing LLM, embedding, search, fetch, OCR, and Daytona boundaries; do not create a second application abstraction solely for tests.
- Validate Compose expansion, image builds, FastAPI import/lifespan, Web UI checks/build, PostgreSQL schema application, Qdrant collection creation, Redis health, and basic board/note/link CRUD.
- Use a deterministic 512-dimensional fake embedder for mandatory storage checks. Keep real embedding validation separate and opt-in.

Exit criteria:

- Every baseline command has a recorded exit code and bounded output artifact before runtime changes begin.
- Failures are classified as upstream, environment, or harness failures and are not silently normalized.
- Provider-boundary counters prove zero real LLM, embedding, search, fetch, OCR, and Daytona calls.
- Board, note, and link data survive the specified service restart through PostgreSQL and Qdrant.

### WP1 — Runtime policy, startup, and LLM/embedding separation

Purpose: make direct-generation prohibition explicit without disabling embeddings or persistence startup.

Files:

- New `backend/topix/ai_runtime/policy.py`.
- `backend/topix/config/catalog.py` and, only if response shaping requires it, `backend/topix/config/services.py`.
- Application construction paths for subscriptions, newsfeed, transcripts, and provider agents.
- Focused tests under `backend/test/unit/ai_runtime/`, `backend/test/unit/config/`, and lifespan tests.

Design:

- Parse `DIM0_AI_RUNTIME` as a closed enum (`provider` or `codex`) and reject unknown values at startup.
- Expose `is_codex_runtime()` and `require_direct_llm_allowed(operation)`.
- Separate executable LLM availability from embedding availability. In Codex mode, every LLM resolver is empty or fails closed while OpenAI/OpenRouter embedding resolution remains unchanged.
- Keep public catalog display metadata from becoming evidence of an executable provider route.
- Make provider-agent pipelines lazy or inject explicit disabled components in Codex mode. Persistence stores and transcript operations must initialize without constructing an executable provider model.

Required evidence:

- Provider/Codex × no-key/OpenAI/OpenRouter/all-key matrix.
- Full FastAPI lifespan in Codex mode with PostgreSQL, Qdrant, Redis, subscriptions, and transcripts initialized and provider-agent invocation count zero.
- Existing 512-dimensional embedding client remains constructible and reported separately from LLM availability.

### WP2 — Guard every direct LLM boundary and preserve non-LLM tools

Purpose: make catalog bypasses and future accidental provider calls fail before network I/O.

Inventory and guard:

- `backend/topix/api/router/ai.py` LiteLLM sync/stream branches.
- `backend/topix/agents/base.py` `LitellmModel` construction and every OpenAI Agents SDK runner.
- Classifier, legacy chat generation, newsfeed agents, board/chat auto-labeling, image description, and `/tools/*` generation routes.
- Frontend resolution/construction in `resolve.ts`, `context.ts`, `local-llm.ts`, and `byok-client.ts`.
- Web runtime config through `webui/config.template.js`, `webui/docker-entrypoint.sh`, and `webui/src/config/api.ts` or one narrow replacement module.

Behavior:

- Guard immediately before provider construction or invocation.
- Codex + signed-in + stored LLM BYOK resolves managed Codex. Codex + signed-out resolves LLM off and constructs no browser/Tauri provider client.
- Hide or disable only LLM controls; never delete stored keys.
- Search, fetch, OCR, and Daytona resolution and confirmation behavior remain structurally equivalent in both modes.

Required evidence:

- Provider counters remain zero for `/ai/llm`, `/ai/llm/stream`, legacy chat, classifier, and all inventoried agent entry points in Codex mode.
- Credential-free fake-client tests cover search/fetch/OCR/Daytona resolution, `X-Provider-Key` relay, metering bypass, and UI confirmation gates.
- Provider mode retains upstream behavior.

### WP3 — Bounded Codex app-server protocol adapter

Purpose: turn the prototype into an observable application component with isolated sessions.

Process and generation lifecycle:

1. Start lazily so board and persistence features remain available while Codex is unauthenticated or degraded.
2. Spawn the pinned executable with an allowlisted environment containing only `CODEX_HOME`, `PATH`, required home/temp/certificate variables, and explicitly configured proxy variables.
3. Consume stdout and stderr concurrently; redact and bound retained diagnostics.
4. Send exactly one `initialize`, await it with a dedicated timeout, then send `initialized`.
5. Give every child connection a monotonically increasing generation. Associate pending requests, notification routes, sessions, and locks with that generation.
6. On EOF, fatal parse error, or child exit, atomically mark the generation dead, fail its pending work with one sanitized error, interrupt readers, and clear all generation-owned state.
7. Permit one replacement child under a start lock for a later request, controlled by bounded exponential backoff and a circuit-open interval. Never fall back to a provider.
8. On shutdown, interrupt active turns, close stdin, wait a grace period, then kill only the owned child if required.

Protocol bounds and correlation:

- Allocate monotonic request IDs and route responses by request ID and notifications by thread/turn ID.
- Configure separate timeouts for initialization, thread creation, turn start, turn completion, and concurrency-queue waits.
- Bound stdout/stderr line size and retention, structured output bytes, pending requests, notification routes, sessions, canonical transcript bytes, response-cache bytes, and concurrent turns.
- Classify executable-not-found, initialization/protocol, unauthenticated, timeout, interrupted, child-exit, schema-invalid, and circuit-open failures into stable internal codes.

Session contract:

- Key reusable sessions by `(authenticated_user_id, canonical_run_uuid)`. Bound textual length before UUID parsing and never log either value raw.
- Requests without a run ID receive an ephemeral session removed after completion.
- On the first request, canonicalize and hash the full OpenAI-shaped message/tool-result sequence. Later requests must contain that retained sequence as an exact prefix; send only the new suffix to the existing Codex thread.
- Cache a completed Dim0 response by inbound request hash. An exact retry returns it without starting a second turn. A prefix mismatch returns a stable conflict and does not mutate the session.
- Evict the complete session atomically by TTL/LRU. After eviction, backend restart, or child-generation change, the next request creates a new thread from its full history.
- Serialize turns per session, while allowing distinct sessions concurrently up to `DIM0_CODEX_MAX_CONCURRENT_TURNS`.

Turn and output contract:

- Use an empty dedicated `DIM0_CODEX_WORKSPACE`, restricted read-only access, `approvalPolicy=never`, and an output schema. Codex never executes canvas tools.
- Require exactly one non-empty output mode: `content` or `tool_calls`.
- Validate tool names against the request, arguments against each supplied JSON Schema using an explicit maintained validator, call IDs for uniqueness, and output against size limits.
- Reject unknown tools, invalid/missing values, duplicate call IDs, mixed modes, and empty results.
- Buffer structured output through complete validation. The stream route may then emit one compatibility content delta and `final`, but never claims token streaming.
- On HTTP disconnect, asyncio cancellation, or timeout, send `turn/interrupt` best-effort and release all locks, semaphores, and waiters in `finally`.

Required fake-server evidence:

- Handshake, text, schema-valid tool calls, invalid/oversized output, stderr redaction, and shutdown.
- Two-user isolation for the same run UUID, transcript suffixing, exact retry, prefix conflict, TTL reset, same-session serialization, and concurrent sessions.
- Several active sessions failing on child exit, followed by concurrent later requests producing one controlled restart and new isolated threads.

### WP4 — HTTP adapter, readiness, and error contract

- Resolve the runtime once through the application component; remove scattered raw environment comparisons.
- Pass authenticated `user_id` and validated run UUID from both LLM routes into `CodexRuntime`.
- Expose Codex readiness separately from ordinary service liveness. Degraded readiness disables agent submission without disabling the board.
- Return stable sanitized public errors for the internal classes defined in WP3.
- Emit the error event shape the current stream client handles and no token delta before Codex output validates.
- Preserve current metering deliberately: a managed request reaching the handler consumes its run unit even when Codex startup or authentication fails.
- Decouple Codex model-picker behavior from provider catalog reachability.

Exit criteria:

- Existing managed-client contract tests pass with Codex success/error additions.
- Canvas accepts translated tool calls without changes to node/edge execution semantics.
- No child-process secret, raw user/run identity, or sensitive path reaches logs or HTTP output.

### WP5 — Persistence, Compose, credentials, and filesystem boundaries

Backend configuration:

- Extend `PostgresConfig.model_post_init()` for `POSTGRES_DB`, `POSTGRES_USER`, and `POSTGRES_PASSWORD`; never log the password.
- Keep `apply_schema()` idempotent.
- Validate Qdrant collection dimension against the 512-dimensional embedder and never recreate a non-empty collection automatically.

Docker structure:

- Create a fork-specific common Compose file containing locally built backend/Web UI, Qdrant, Redis, persistent volumes, and a dedicated external `CODEX_HOME`. Do not derive it from `docker-compose.images.yml`.
- Add a bundled-PostgreSQL variant with its own service, named volume, healthcheck, and backend dependency.
- Add an external-PostgreSQL variant with no PostgreSQL service or PostgreSQL `depends_on`; do not rely on Compose `!reset`.
- Require host, port, database, user, and password for external PostgreSQL. Grant the dedicated Dim0 role ownership or sufficient schema DDL rights.
- Pin Qdrant, PostgreSQL, Redis, and Codex versions and use unique `dim0-*` resource names.
- Prefer a Codex-specific backend build target so the provider image does not unconditionally contain the Codex CLI.

Secrets and state:

- Require absolute external env-file and `CODEX_HOME` paths and reject paths resolved inside the worktree. Never mount the upstream repository `.env` into the Codex profile.
- `C:\ProgramData\Dim0\codex` is the proposed Windows `CODEX_HOME`; creation and login are human bootstrap steps.
- Never pass provider keys, database/Redis credentials, signing secrets, or unrelated backend variables to the Codex child.

Required evidence:

- Both variants pass `docker compose config`; external `config --services` and actual `up`/`ps` show no PostgreSQL container.
- `SELECT current_database(), current_user` proves the external database/role, and applying schema twice proves idempotency.
- Fake-child environment capture excludes forbidden variables. Sentinel secrets in stderr/protocol errors never appear in logs or HTTP responses.
- An actual sandbox smoke cannot read `/app`, cannot write its workspace, and sees an empty dedicated workspace.
- Repository scans find no credentials, `CODEX_HOME`, or persistent data.

### WP6 — End-to-end validation

1. Build all images and record the exact Codex CLI version.
2. Start bundled persistence and verify PostgreSQL, Qdrant, Redis, backend liveness, separate Codex readiness, and Web UI.
3. Bootstrap the local user through the existing flow.
4. Create a board, notes, and links; restart; verify persistence.
5. Observe one embedding request for text creation/update and none for spatial/style-only updates. Verify a fake embedding failure prevents Qdrant upsert/vector update and introduces no zero-vector fallback.
6. Run Codex text-only and valid note-create/note-edit/edge-create tool turns.
7. Verify browser event, intended tool call, application store, canvas controller, rendered DOM, Qdrant payload, and reload state.
8. Stop app-server during active turns and verify sanitized visible failure, generation cleanup, bounded restart, and zero provider fallback.
9. Exercise distinct users/runs concurrently, same-session ordering, full-history suffixing, retry, and conflict handling.
10. Repeat persistence with external PostgreSQL and prove no bundled PostgreSQL starts.
11. Run mandatory fake-client non-LLM regressions; list optional live external-service checks separately.

If canvas state diverges, add development-only structured traces at the user event, calculated target, React/application store, library/controller, and rendered DOM boundaries. Fix the first layer where expected and actual state diverge and retain useful traces until verification.

Exit criteria:

- Every normative acceptance criterion has passing evidence.
- Missing credentials affect only explicitly optional live-service checks and are listed as unverified, never passed.

### WP7 — Documentation, upstream merge, and handoff

- Update ADR-CODEX-001 with final runtime, session, readiness, metering, and error decisions.
- Document bootstrap, startup, health, backup locations, upgrade, explicit rollback, and both PostgreSQL modes.
- Record the upstream commit, fork-only file list, subtree update procedure, and post-merge verification checklist.
- Keep provider mode available as an explicit profile choice, never an automatic fallback.

## 5. Commit sequence

1. `test(dim0): add runtime baseline harness`
2. `refactor(runtime): separate llm and embedding policy`
3. `fix(runtime): block direct llm calls in codex mode`
4. `feat(runtime): harden codex app-server sessions`
5. `fix(agent): preserve codex http contracts`
6. `feat(build): add bundled and external postgres profiles`
7. `test(dim0): verify codex canvas and persistence flows`
8. `docs(dim0): document codex desktop operations`

Provider policy, process/session lifecycle, Docker configuration, and operator documentation must remain independently reviewable even if adjacent commits are combined for testability.

## 6. Verification matrix

| Area | Provider profile | Codex + bundled PG | Codex + external PG |
|---|---:|---:|---:|
| Baseline command/evidence ledger | Required | Reused as comparison | Reused as comparison |
| Compose expansion and service list | Required | Required | Required; no PG service |
| Backend unit/lifespan tests | Required | Required | Required |
| Frontend check/build | Required | Required | Required |
| Direct LLM calls | Upstream behavior | Must be zero outside Codex | Must be zero outside Codex |
| Embedding | Existing route | Existing route, separately evidenced | Existing route, separately evidenced |
| PostgreSQL/Qdrant CRUD and restart | Required | Required | Required |
| Canvas Codex tool E2E | N/A | Required | Representative smoke |
| Search/fetch/OCR/Daytona fake contracts | Required | Structurally unchanged | Structurally unchanged |

## 7. Stop conditions

Pause implementation only if:

- the actual app-server protocol cannot represent the browser tool-turn contract without executing tools itself;
- Qdrant compatibility would require recreation or data loss;
- external PostgreSQL cannot provide a dedicated database/role with schema DDL rights;
- Codex authentication or an embedding credential is unavailable for an explicitly opt-in live check; or
- a required fix would materially restructure upstream canvas, agent, or storage domains.

A slow build, ordinary test failure, or missing optional non-LLM credential is evidence to classify, not a design blocker.

## 8. Review reconciliation

The independent review is complete. Its blocking and important findings are integrated as follows:

| Review area | Integrated work package |
|---|---|
| Startup provider construction | WP1–WP2 |
| User-scoped session identity and history reconciliation | WP3–WP4 |
| Connection generations, cancellation, bounds, and restart | WP3 |
| Structured-output validity and buffered streaming | WP3–WP4 |
| Embedding separation and failure behavior | WP0–WP1, WP6 |
| External PostgreSQL Compose structure | WP5–WP6 |
| Credentials, child environment, and filesystem isolation | WP3, WP5 |
| Web UI runtime configuration | WP2, WP4 |
| Mandatory non-LLM regressions | WP0, WP2, WP6 |

The review document remains a durable rationale and traceability record. It no longer overrides this plan; any future conflict is a new review finding that must be resolved in this implementation plan before work proceeds.
