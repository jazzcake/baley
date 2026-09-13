# Independent review and reconciliation record: Dim0 Codex runtime plan

Review verdict: **approved; all blocking and important amendments incorporated**

Reviewed plan: [`codex-runtime-implementation-plan.md`](./codex-runtime-implementation-plan.md)

Normative spec: [`codex-runtime-and-persistence-spec.md`](./codex-runtime-and-persistence-spec.md)

Reconciliation status: this file preserves the review findings and rationale. The reviewed implementation plan now contains every required amendment and is the executable plan; this record no longer overrides it. Task #189's pre-change Docker baseline is defined separately in [`task-189-baseline-execution.md`](./task-189-baseline-execution.md).

An independent agent reviewed the plan against the repository's backend, frontend, provider catalog, app lifecycle, persistence configuration, and Compose files. It initially returned **changes required** with four blocking and eight important findings. This document records how those findings were resolved; the reconciled implementation plan carries the executable requirements.

## 1. Backend startup must not instantiate provider agents in Codex mode

Finding: application startup eagerly constructs `SubscriptionStore -> NewsfeedPipeline -> BaseAgent`, which resolves a direct provider model. Simply making catalog LLM resolution fail closed would prevent the Codex backend from starting.

Required implementation:

- Inventory every OpenAI Agents SDK entry point, including legacy chat generation, newsfeed agents, board/chat auto-labeling, image description, and `/tools/*` routes.
- Make provider-agent pipelines lazy, or install explicit inert/disabled components in Codex mode. Persistence stores and transcript operations must initialize without constructing a provider model.
- Enforce the direct-LLM prohibition at execution boundaries (`AgentRunner`/`Runner`, LiteLLM completion, and classifier invocation), not by making harmless object construction crash unpredictably.
- Add a full FastAPI lifespan test in Codex mode with an embedding key: PostgreSQL, Qdrant, Redis, subscriptions, and transcripts initialize while provider-agent invocation count remains zero.

This extends WP1/WP2 and is a blocking exit criterion.

## 2. Session identity and validation

Finding: `X-Run-Id` alone allows two authenticated users choosing the same value to share a Codex thread.

Required implementation:

- The session key is `(authenticated_user_id, run_id)`.
- Pass `user_id` from both LLM routes into `CodexRuntime`.
- Require a canonical UUID run ID, with a bounded textual length before parsing. Requests without one use an ephemeral non-reusable session.
- Never place raw user IDs or run IDs in child logs without structured redaction.
- Test two users with the same run ID and prove distinct thread IDs, locks, transcript state, and eviction.

This finding is incorporated in WP3 and WP4; the former run-ID-only wording has been removed.

## 3. Persistent-thread transcript reconciliation and retry

Finding: the browser sends the complete growing message array on every tool-loop turn, while Codex threads already retain prior turns. Replaying the whole history duplicates context.

Required contract:

1. Normalize each inbound OpenAI-shaped message/tool-result into a canonical JSON representation and hash it.
2. On the first request for a session, send the full history and retain the canonical sequence.
3. On subsequent requests, require the retained sequence to be an exact prefix and send only the new suffix/tool outputs to the existing Codex thread.
4. Cache the completed Dim0 response against an inbound request hash for the session. An exact retry returns that cached response and does not start another Codex turn.
5. A prefix mismatch returns a stable conflict error; it must not append ambiguous context or fall back to a provider.
6. After TTL eviction or backend/child restart, create a new thread and accept the next request's full history as a new baseline.
7. Bound retained canonical history and request/response cache sizes; eviction removes the entire session atomically.

Required tests:

- `prompt -> tool call -> tool result -> final` sends prior messages only once.
- Exact retry starts no second turn.
- Prefix mismatch fails without mutating the session.
- TTL eviction creates a new thread using full history.

This is a blocking addition to WP3.

## 4. App-server connection generations

Finding: restarting only the child process leaves mappings that refer to dead thread IDs.

Required implementation:

- Assign every child connection a monotonically increasing generation.
- Associate sessions, per-thread locks, pending requests, and notification routes with that generation.
- On EOF, fatal parse error, or child exit: atomically mark the generation dead; fail all pending operations with one stable sanitized error; interrupt/cancel readers; and clear every session/route/lock owned by that generation.
- A later request may start exactly one replacement child under the start lock. It always creates a new thread.
- Add the scenario: several active runs -> child exit -> all fail -> concurrent later requests -> one restart -> new isolated threads.

This is a blocking addition to WP3.

## 5. Cancellation, bounds, authentication, and restart policy

Required decisions:

- Runtime startup is **lazy**. Manual canvas and persistence remain available if Codex is not authenticated or temporarily unavailable.
- Add a distinct runtime status/readiness response; ordinary service liveness is not Codex readiness. The UI disables agent submission while runtime status is degraded but does not disable the board.
- Initialization, thread creation, turn start, turn completion, and concurrency-queue waits each have explicit configurable timeouts.
- HTTP disconnect, asyncio cancellation, and turn timeout send `turn/interrupt` best-effort, then release all locks/semaphores/waiters in `finally`.
- Restart uses bounded exponential backoff and a circuit-open interval. One failed request cannot create a restart storm.
- Bound stdout line length, stderr line length/retention, structured output bytes, pending requests, notification routes, sessions, canonical transcript bytes, and concurrent turns.
- Classify executable-not-found, initialization/protocol, unauthenticated account, timeout, interrupted, child-exit, schema-invalid, and circuit-open errors into stable internal codes. HTTP/log output is sanitized.
- Existing metering is retained deliberately: a managed request reaching the handler consumes its current run unit even when Codex startup/authentication fails. Tests pin this behavior; changing quota rollback is a separate task.

## 6. Structured output validity

Required implementation:

- Output must contain exactly one non-empty mode: `content` or `tool_calls`.
- Validate each tool name against the request's allowed tools.
- Validate arguments against that tool's supplied JSON Schema using an explicit maintained validator dependency.
- Reject unknown tools, wrong/missing required values, duplicate call IDs, simultaneous content and tool calls, empty results, and oversized output.
- Structured output remains buffered. The stream endpoint emits no claimed token deltas before validation; after completion it may emit one compatibility content delta followed by `final`.
- Update runtime-specific route documentation so provider streaming remains token streaming while Codex structured output is described as buffered.

## 7. Embedding verification remains separate from LLM policy

Required tests and evidence:

- In Codex mode with OpenAI/OpenRouter keys, LLM resolution is empty but the existing 512-dimensional embedding route resolves and constructs its client.
- A fake embedding failure during add/update prevents the subsequent Qdrant upsert/vector update; no zero-vector fallback is introduced.
- Payload-only spatial/style updates invoke neither the embedding client nor vector update.
- “Embedding reported separately” means test and operator-health evidence, not a new public service API in this phase.
- Document the observed user/storage behavior of an embedding failure without redesigning it.

## 8. External PostgreSQL and Compose structure

Required implementation:

- Do not derive the external variant from `docker-compose.images.yml`, whose backend hard-codes `depends_on: postgres`.
- Create a fork-specific common Compose file containing backend, Web UI, Qdrant, and Redis.
- Create a bundled-PostgreSQL variant that adds the PostgreSQL service/volume/dependency.
- Create an external-PostgreSQL variant with no PostgreSQL service and no PostgreSQL `depends_on`; avoid Compose `!reset` and its version portability issue.
- Add `POSTGRES_DB`, `POSTGRES_USER`, and `POSTGRES_PASSWORD` environment overrides to `PostgresConfig`; never log the password.
- External bootstrap documentation grants the dedicated Dim0 role ownership or sufficient schema DDL rights for `apply_schema()`.

Required verification:

- `docker compose config --services` for the external variant lists no PostgreSQL service.
- Actual `up`/`ps` creates no bundled PostgreSQL container.
- `SELECT current_database(), current_user` proves the dedicated database and role.
- Apply the schema twice and verify idempotency.

## 9. Credential, environment, and filesystem boundary

Required implementation:

- The fork Compose files require an absolute external env-file path and external `CODEX_HOME`; bootstrap rejects resolved paths inside the worktree.
- Do not mount the upstream repository `.env` into the Codex backend profile.
- Construct the child environment from an allowlist rather than inheriting the backend environment. Allow only `CODEX_HOME`, `PATH`, the minimum home/temp/certificate variables required by the executable, and explicitly documented proxy variables when configured.
- Never pass LLM/embedding API keys, database/Redis credentials, signing secrets, or unrelated backend variables to the Codex child.
- Redact sentinel secrets and sensitive paths from child stderr, protocol errors, application logs, and HTTP responses.
- Prefer a Codex-specific Docker build target or Dockerfile so the upstream provider image does not unconditionally contain the Codex CLI.

Required tests:

- A fake child records its environment and proves forbidden variables are absent.
- Sentinel secrets in stderr/protocol errors never appear in logs or HTTP responses.
- An actual sandbox smoke cannot read `/app`, cannot write the workspace, and sees an empty dedicated workspace.
- Repository scans find no credential or persistent state.

## 10. Web UI runtime configuration

Required files to evaluate/change:

- `webui/config.template.js`;
- `webui/docker-entrypoint.sh`;
- `webui/src/config/api.ts` or a narrowly scoped new runtime-config module;
- `webui/src/features/agent/engine/services/resolve.ts`;
- `webui/src/features/agent/engine/services/context.ts`;
- `webui/src/features/agent/engine/services/local-llm.ts`;
- `webui/src/features/agent/engine/byok-client.ts`;
- Codex Compose environment.

Required behavior:

- Codex + signed-in + stored LLM BYOK resolves managed Codex.
- Codex + signed-out + stored LLM BYOK resolves LLM off; no OpenAI-compatible browser/Tauri client is constructed.
- Only LLM provider controls are hidden/disabled; stored keys are not deleted.
- Search/fetch/OCR/Daytona resolver output and confirmation behavior are byte-for-byte/structurally equivalent across runtime modes.

## 11. Non-LLM regression tests are mandatory

Fake clients make these tests credential-free and mandatory:

- resolver selection for search, fetch, OCR, and Daytona;
- `X-Provider-Key` relay behavior;
- existing metering-bypass contract;
- UI confirmation gate.

Only live calls to the external services are optional. Missing live credentials must not downgrade the required contract tests.

## 12. Traceability additions

| Spec requirement | Plan implementation | Required evidence |
|---|---|---|
| §3.1 sole Codex generation/no fallback | WP1–WP4; amendments §1–§6 | provider-call counters zero; Codex failure tests |
| §3.2 fail-closed LLM selection | WP1–WP2; amendments §1 | key/runtime matrix; full lifespan startup |
| §4.1 Qdrant mandatory | WP0, WP5–WP6 | CRUD + restart persistence |
| §4.2 current embedding coupling | WP0, WP5; amendments §7 | add/update/payload-only call observations |
| §4.3 embedding retained | WP1, WP5; amendments §7 | embedding resolves while LLM routes do not |
| §5 non-LLM unchanged | WP2, WP6; amendments §10–§11 | mandatory fake-client contract suite |
| §6 persistence/Docker | WP5; amendments §8–§9 | both variants config/up/ps; external DB identity |
| §7 session lifecycle | WP3–WP4; amendments §2–§6 | protocol, isolation, retry, restart, cancel tests |
| §8 acceptance 1–12 | WP0–WP7 | consolidated evidence checklist before completion |
| §11 upstream merge constraint | WP7; amendments §8–§10 | fork-only file inventory and subtree replay check |

## 13. Reconciliation audit

| Review finding | Incorporated location | Closure evidence expected during implementation |
|---|---|---|
| Provider-agent construction during startup | WP1–WP2 | Codex-mode lifespan and zero-call counters |
| Session identity and transcript replay | WP3–WP4 | Two-user isolation, suffix, retry, conflict, and eviction tests |
| Connection generation and restart safety | WP3 | Multi-session child-exit and single-restart test |
| Cancellation, bounds, readiness, and metering | WP3–WP4 | Timeout/cancel cleanup, circuit, readiness, and quota tests |
| Structured output | WP3–WP4 | Schema, exclusive-mode, call-ID, size, and buffered-stream tests |
| Embedding separation | WP0–WP1, WP6 | 512-dimension resolution and mutation/failure observations |
| External PostgreSQL structure | WP5–WP6 | Compose service list, `up`/`ps`, identity, and idempotent schema evidence |
| Credential/filesystem boundary | WP3, WP5 | Child env capture, redaction, sandbox, and repository scans |
| Web UI runtime selection | WP2, WP4 | Signed-in/out BYOK matrix and no-client-construction assertions |
| Mandatory non-LLM regressions | WP0, WP2, WP6 | Credential-free fake-client suite |

## 14. Independent reviewer conclusion

The reviewer found the architectural direction sound: LLM/embedding separation, Qdrant and Redis retention, provider fallback prohibition, non-LLM preservation, and honest buffered-stream semantics. The reconciled implementation plan incorporates the amendments above without requiring a storage, canvas, or agent-domain redesign.

Implementation remains incomplete until every blocking exit criterion and normative acceptance criterion has corresponding passing evidence.
