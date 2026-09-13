# Detailed implementation plan: Dim0 Codex runtime fork

Status: **reviewed and ready for implementation**  
Review amendments: [`codex-runtime-plan-review.md`](./codex-runtime-plan-review.md) (normative; overrides conflicting draft wording)  
Normative spec: [`codex-runtime-and-persistence-spec.md`](./codex-runtime-and-persistence-spec.md)  
Upstream baseline: `vcmf/dim0@c75cb32901aac30b1dd79f07d4a560d93b21750b`  
Fork location: `dim0/` squash subtree in the Baley repository

## 1. Outcome

Deliver an independently runnable Dim0 web service whose browser agent uses a persistent local `codex app-server` process for generation, while retaining upstream PostgreSQL, Qdrant, Redis, canvas, document, embedding, search/fetch/OCR, and Daytona behavior.

In the explicit Codex profile, provider API keys may serve embeddings and non-LLM tools but can never select a direct LLM completion route. Codex failure is visible and never falls back to LiteLLM, OpenAI Agents, BYOK LLM, or `codex exec`.

## 2. Constraints and non-goals

- No Baley/Dim0 data or API integration.
- No Qdrant removal, persistence rewrite, semantic/RAG redesign, or embedding replacement.
- No broad agent/canvas restructuring.
- No committed credential, `CODEX_HOME`, database data, or Qdrant data.
- Preserve the upstream provider runtime outside the explicit Codex build profile.
- Keep fork-specific changes localized and easy to reapply after subtree pulls.

## 3. Current prototype disposition

Commit `7c0136e` is a prototype, not the implementation baseline. Retain useful seams but revise before runtime validation:

| Prototype area | Keep | Required correction |
|---|---|---|
| `/ai/llm[/stream]` branch | Existing HTTP contract and injected runtime direction | Move environment checks into a policy/runtime selector; test fail-closed behavior |
| Persistent app-server subprocess | stdio transport and initialize/thread/turn primitives | Handle stderr, EOF, malformed frames, pending-future failure, timeout, cancellation, restart, shutdown, and bounded concurrency |
| `X-Run-Id -> threadId` | Existing browser run identity | Define retention/eviction; prevent unbounded memory; explicitly decide restart behavior |
| Structured tool choice | Browser remains tool executor | Validate schema and allowed tool names/arguments; preserve final-message contract |
| Streaming | Existing NDJSON event names | Do not claim token streaming when structured output is buffered; make behavior and UI expectation explicit |
| Docker overlay | Additive profile and external `CODEX_HOME` | Separate bundled/external PostgreSQL overlays, pin protocol-compatible CLI, add health/startup checks |

## 4. Work packages

### WP0 — Baseline and evidence harness

Purpose: distinguish upstream defects from fork regressions without sending real prompts to a provider.

Changes:

- Add a fork smoke-test checklist under `docs/plans/` with exact commands and expected service health.
- Add test doubles at the existing LLM and embedding boundaries; do not add a second application abstraction solely for tests.
- Validate Compose expansion, FastAPI import/startup wiring, Web UI production build, PostgreSQL schema application, Qdrant collection creation, Redis health, and basic board CRUD.
- For baseline board CRUD, use a deterministic fake embedding client of the configured 512 dimensions. A separate opt-in test validates the real retained embedding provider.

Evidence:

- Compose config output for all profiles.
- Backend unit/integration test reports.
- Web UI `check-all` and production build.
- Storage smoke: create board, add note/link, retrieve, restart services, retrieve again.

Exit criteria:

- Baseline failures are recorded before fork runtime changes.
- No real LLM completion was called.

### WP1 — Runtime policy and LLM/embedding separation

Purpose: make the direct-generation prohibition explicit and centrally testable without disabling embeddings.

Files:

- New `backend/topix/ai_runtime/policy.py`.
- `backend/topix/config/catalog.py`.
- `backend/topix/config/services.py` only if response shaping requires it.
- Focused unit tests under `backend/test/unit/ai_runtime/` and `backend/test/unit/config/`.

Design:

- Parse `DIM0_AI_RUNTIME` into a closed runtime enum (`provider` or `codex`); reject unknown values at startup.
- Expose `is_codex_runtime()` and `require_direct_llm_allowed(operation)`.
- Separate catalog resolution internally into `available_llm_providers()` and existing general/embedding provider availability.
- In Codex mode, `available_llms()` returns no executable provider LLM routes while `available_embedding()` continues resolving OpenAI/OpenRouter.
- Keep `public_llm_catalog()` as display metadata only or provide an explicit Codex model entry; it must not become executable provider evidence.
- `openai_compatible_client()` remains available for the retained embedding route. Its docstring and callers must no longer imply it is globally prohibited in Codex mode.

Tests:

- Matrix: provider mode/Codex mode × no keys/OpenAI key/OpenRouter key/all LLM keys.
- In Codex mode, every LLM resolver is empty/fails while embedding resolves identically to provider mode.
- Unknown runtime value fails startup rather than reverting to provider mode.

Exit criteria:

- Presence of `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, or `OPENROUTER_API_KEY` cannot produce an executable LLM call code in Codex mode.
- Embedding route and dimension remain unchanged.

### WP2 — Guard all direct LLM invocation boundaries

Purpose: ensure catalog bypasses and future accidental calls fail before network I/O.

Files and boundaries:

- `backend/topix/api/router/ai.py`: LiteLLM sync/stream branches.
- `backend/topix/agents/base.py`: `LitellmModel` construction.
- `backend/topix/agents/assistant/auto_model.py`: classifier completion.
- `backend/topix/api/router/chats.py`: legacy server-agent entry point.
- Frontend BYOK selection/construction in `webui/src/features/agent/engine/services/resolve.ts`, `context.ts`, `local-llm.ts`, and `webui/src/features/agent/engine/byok-client.ts` as confirmed by tests.

Design:

- Call `require_direct_llm_allowed()` immediately before each backend construction/invocation boundary.
- In Codex mode, legacy chat generation returns a stable “runtime unavailable for this path” response; transcript CRUD remains intact.
- Inject a build/runtime flag into the Web UI config. In Codex profile, LLM resolution always chooses managed when signed in and never constructs a BYOK LLM client. Non-LLM BYOK resolution is unchanged.
- Hide or disable only LLM provider controls in the Codex UI profile. Do not remove stored values automatically.

Tests:

- Patch all direct provider functions with counters that fail if invoked.
- Exercise `/ai/llm`, `/ai/llm/stream`, legacy chat generation, classifier, and frontend resolver matrices.
- Confirm search/fetch/OCR/Daytona resolution snapshots are unchanged.

Exit criteria:

- Zero direct LLM provider invocations in Codex mode, including when all keys and BYOK values are present.
- Provider mode retains upstream behavior.

### WP3 — Codex app-server protocol adapter

Purpose: turn the prototype into a bounded, observable application component.

Files:

- `backend/topix/ai_runtime/codex.py`, split into protocol/process/session modules only if tests demonstrate that one file is no longer maintainable.
- `backend/topix/api/app.py` lifecycle wiring.
- Runtime unit tests with a fake newline-delimited app-server process.

Process lifecycle:

1. Lazy-start or startup-start according to health semantics decided in WP5.
2. Spawn the pinned `codex app-server` executable with minimal inherited environment.
3. Consume stdout and stderr concurrently; redact and bound retained diagnostics.
4. Send exactly one `initialize`, wait for success with timeout, then send `initialized`.
5. On EOF/process exit, fail all pending requests, mark unhealthy, and allow one controlled restart for a subsequent request—not provider fallback.
6. On application shutdown, interrupt active turns where possible, close stdin, terminate with grace timeout, then kill only the owned child if required.

Protocol correlation:

- Allocate monotonically increasing request IDs.
- Route responses by request ID and notifications by thread/turn ID, not through one shared undifferentiated queue.
- Treat unknown/malformed messages as structured diagnostics; protocol-fatal messages fail the affected connection.
- Bound every request and turn with configurable timeouts.

Session policy:

- One thread per non-empty `X-Run-Id`.
- Requests without a run ID receive an ephemeral thread removed after completion.
- Store mappings in memory for the initial release; Codex persists its own thread data in external `CODEX_HOME`, but mappings are intentionally not resumed after backend restart.
- Evict completed run mappings after `DIM0_CODEX_SESSION_TTL_SECONDS`; cap total mappings with LRU behavior.
- Serialize turns per thread, not globally. Allow different runs concurrently up to `DIM0_CODEX_MAX_CONCURRENT_TURNS`.

Turn contract:

- Use restricted read-only access to the empty `DIM0_CODEX_WORKSPACE`, `approvalPolicy=never`, and an output schema.
- Translate the full existing message/tool contract without granting Codex filesystem or command responsibilities.
- Validate that returned tool names exist in the request and arguments are JSON objects.
- Return exactly one of: content, one or more valid tool calls, or a stable runtime error.
- Structured output is buffered until valid JSON is complete. The endpoint may emit a final content delta for compatibility but documentation must not label this as genuine token streaming.

Exit criteria:

- Fake-server tests cover handshake, text, tool call, concurrent runs, same-run serialization, timeout, cancellation, malformed JSON, stderr, child exit, restart, and shutdown.

### WP4 — HTTP adapter and error contract

Purpose: preserve Dim0 browser behavior while exposing runtime failures predictably.

Files:

- `backend/topix/api/router/ai.py`.
- Existing managed-client and stream assembly tests in `webui/src/features/agent/engine/`.

Design:

- Resolve runtime once through the application component; remove scattered raw environment comparisons.
- Non-stream failures return an appropriate 5xx response with a stable public error code and no child-process secrets.
- Streaming failures emit the event shape the existing client actually handles; if it lacks an error event contract, update both ends together and test it.
- Metering semantics remain one `X-Run-Id` unit; failed pre-turn startup/auth requests must be evaluated against the existing quota contract and documented.
- Model picker behavior in Codex mode is decoupled from provider catalog reachability; use a stable Codex label/model selection policy.

Exit criteria:

- Existing managed-client contract tests pass plus Codex success/error cases.
- Canvas agent loop accepts translated tool calls without changes to node/edge execution semantics.

### WP5 — Persistence and configuration

Purpose: preserve upstream stores while supporting bundled or existing PostgreSQL safely.

Backend configuration:

- Extend `PostgresConfig.model_post_init()` to support `POSTGRES_DB`, `POSTGRES_USER`, and `POSTGRES_PASSWORD` in addition to host/port.
- Never log password values.
- Keep `apply_schema()` idempotent startup behavior.
- Validate Qdrant collection dimension against the configured 512-dimensional embedder; never recreate a non-empty collection automatically.

Docker assets:

- Base additive Codex overlay: locally built backend/Web UI, dedicated external `CODEX_HOME`, bundled Qdrant and Redis, persistent volumes.
- Bundled PostgreSQL profile: dedicated named volume and healthcheck.
- External PostgreSQL overlay: remove dependency on the bundled service and require host, port, database, user, and password inputs.
- Prefer `host.docker.internal` for a PostgreSQL instance on the Windows Desktop host; document LAN/container-network alternatives.
- Use unique `dim0-*` container and volume names to avoid collisions with Baley or unrelated local services.
- Pin Qdrant/PostgreSQL/Redis/Codex image or package versions; do not use `latest` in the fork profile.

Secrets and state:

- Examples contain paths and variable names only.
- Actual `.env`, database password, embedding key, and Codex auth remain outside Git under the common data/security policy.
- `C:\ProgramData\Dim0\codex` is the proposed Windows `CODEX_HOME`; creation/login is a human-run bootstrap step.

Exit criteria:

- Both Compose variants pass `docker compose config`.
- Backend connects to each PostgreSQL variant and preserves Qdrant content across restart.
- Repository scan finds no runtime credentials or persistent data.

### WP6 — End-to-end validation

Purpose: prove the service is usable rather than merely assembled.

Test sequence:

1. Build all images with a recorded Codex CLI version.
2. Start bundled persistence and verify health endpoints/log readiness.
3. Sign in/bootstrap the single local user using the existing Dim0 flow.
4. Create a board, notes, and links manually; restart; verify persistence.
5. Make a text edit and observe one embedding request; make a position/style edit and observe none.
6. Run a Codex text-only prompt.
7. Run prompts producing note creation, note edit, and edge creation tool calls; verify rendered DOM, application store, persisted Qdrant payload, and reload behavior.
8. Stop app-server during a turn; verify visible error and zero provider fallback.
9. Run two distinct run IDs concurrently and one same-run sequence.
10. Repeat persistence startup with external PostgreSQL.
11. Regression-test search, fetch, OCR, and Daytona when their credentials are configured.

UI debugging rule:

If a canvas result diverges, add development-only structured traces at the user event, intended tool call, browser agent state, canvas store state, controller state, and rendered DOM boundary. Fix the first divergent layer and retain useful traces until verified.

Exit criteria:

- All normative acceptance criteria pass with evidence.
- Any opt-in external-service test not run is listed as an explicit unverified item, not silently treated as passing.

### WP7 — Documentation, upstream merge, and handoff

- Update ADR-CODEX-001 to cite the normative spec and final runtime/error decisions.
- Add operator instructions for bootstrap, startup, health checks, backup locations, upgrade, and rollback.
- Record the exact upstream commit and fork-only file list.
- Document subtree update procedure and a post-merge verification checklist.
- Keep the original provider profile available as rollback, but never as an automatic runtime fallback.

Exit criteria:

- A new operator can start the service from external secrets/state without writing into the worktree.
- Rollback is an explicit Compose/profile choice.

## 5. Commit sequence

Each commit is independently testable and uses the Dim0 conventional scope rule:

1. `test(dim0): add runtime baseline harness`
2. `refactor(runtime): separate llm and embedding policy`
3. `fix(runtime): block direct llm calls in codex mode`
4. `feat(runtime): harden codex app-server sessions`
5. `fix(agent): preserve codex http contracts`
6. `feat(build): add bundled and external postgres profiles`
7. `test(dim0): verify codex canvas and persistence flows`
8. `docs(dim0): document codex desktop operations`

The exact grouping may shrink when two steps cannot be meaningfully tested apart, but provider policy, runtime lifecycle, Docker configuration, and documentation must not be collapsed into one opaque commit.

## 6. Verification matrix

| Area | Provider profile | Codex + bundled PG | Codex + external PG |
|---|---:|---:|---:|
| Compose expansion | Required | Required | Required |
| Backend unit tests | Required | Required | Required |
| Frontend check/build | Required | Required | Required |
| Direct LLM calls | Upstream behavior | Must be zero | Must be zero |
| Embedding | Existing route | Existing route | Existing route |
| Qdrant CRUD/restart | Required | Required | Required |
| Canvas Codex tool E2E | N/A | Required | One representative smoke |
| Search/fetch/OCR/Daytona regression | Existing behavior | Unchanged | Unchanged |

## 7. Stop conditions and decisions requiring the operator

Pause implementation only if:

- the actual Codex app-server protocol cannot represent the browser tool-turn contract without executing tools itself;
- Qdrant collection compatibility would require recreation or data loss;
- existing PostgreSQL cannot provide a dedicated database/role;
- Codex authentication or an external embedding credential is unavailable for opt-in E2E;
- a required fix would materially restructure upstream canvas, agent, or storage domains.

Do not treat a slow Docker build, ordinary test failure, or missing optional non-LLM credential as a design blocker.

## 8. Independent review contract

After this plan is drafted, an independent agent must review it without editing implementation code. The reviewer must check:

- every normative spec statement maps to at least one work item and acceptance check;
- direct LLM prohibition does not accidentally block retained embeddings;
- non-LLM tools remain unchanged;
- process/session concurrency and failure modes are testable;
- Docker external-PostgreSQL claims match actual Compose and config behavior;
- upstream merge surface is genuinely narrow;
- security boundaries cover credentials, filesystem access, error redaction, and fallback behavior;
- the plan does not silently claim genuine token streaming from buffered structured output.

Review findings are classified as blocking, important, or optional. Blocking and important findings are incorporated before the plan is considered ready.
