# Task #189 hardened baseline harness re-review

Date: 2026-09-14 (Asia/Seoul)

Reviewed implementation: `9bd90d2215b80fca494f69c65758a517acb61c91`

Compared with:

- [`task-189-harness-review.md`](./task-189-harness-review.md)
- [`task-189-baseline-execution.md`](./task-189-baseline-execution.md)
- [`codex-runtime-and-persistence-spec.md`](./codex-runtime-and-persistence-spec.md)

Reviewer role: independent test-harness, provider-isolation, persistence, Compose-safety, UI-boundary, and evidence-provenance reviewer. This review changed no implementation.

## Verdict

**Reject commit `9bd90d2` as an acceptance-ready Task #189 harness.** H1, H2, H3, and M2 are materially closed in the implementation, and the evidence helper is substantially safer than the reviewed predecessor. Acceptance remains blocked because there is still no fresh baseline run (H5), the UI test still does not render the actual board/canvas application boundary (M1), the canonical execution plan never runs the new positive backend tripwire self-test, and secret screening cannot establish the plan's stronger no-credential claim.

This is not a rejection of the repaired Compose overlay, provider interception, or storage instrumentation. It is a rejection of the claim that the documented sequence is ready to produce complete acceptance evidence without further harness/plan correction and a fresh Stage A-F run.

No real external AI API was called. No container was started. `debug.log` was not read or modified.

## Severity-ranked residual findings

### H5 — High, open: no hardened-harness execution result exists

Commit `9bd90d2` contains harness and plan changes but no new external evidence manifest, command ledger, provider counter export, storage/restart output, screenshot, HAR/console export, or criterion-by-criterion result. The historical result remains a blocked infrastructure probe against the predecessor harness. It cannot establish normative acceptance criterion 1 or Task #189 acceptance criteria 1-11 for this commit.

A fresh run is necessarily separate from this code review, but until that run exists the baseline itself remains rejected.

### M1 — Medium, partially open: React home rendering is real, but the board/canvas interaction is still synthetic

The first frontend case now genuinely mounts `BoardsHome`, waits for its empty state, clicks the rendered New Board control, observes navigation, and proves the monitored BYOK factory was not called. The focused Vitest invocation passed both tests.

The second case does not mount `LocalBoardScreen`, `BoardView`, `HarnessCanvas`, or the canvas library's `<Canvas>`. `ProviderLifecycleFixture` manually installs persistence and global refs, directly constructs `StoreMutator`, and exposes a test-only button whose handler calls `mutator.createNote()`. This verifies a React-dispatched direct store mutation, not the actual rendered board lifecycle or a canvas gesture. It also bypasses the application path that mounts the board alongside agent/service-resolution UI, so the zero `ByokLlmClient.fromConfig` assertion still cannot detect construction introduced by that real boundary.

Stage E's prose asks an operator to load a board and save a screenshot/network export, but it gives no recorded browser command or assertion that turns the actual canvas interaction and provider-construction observation into repeatable evidence.

### M3 — Medium, new: the execution plan never runs the positive provider-tripwire self-test

`test_provider_tripwire.py` deliberately exercises the current SDK `Runner` object, the two direct imported SDK aliases, the project `AgentRunner`, synchronous and asynchronous LiteLLM entry points, native OpenAI dispatch, LiteLLM-model construction, retained SDK `AsyncOpenAI` aliases, both OpenAI-compatible embedding constructors, and the embedding invocation seam. Static inventory found no additional current `Runner.run`/`run_streamed`, `litellm.completion`/`acompletion`, or embedding-client call boundary outside those patched objects.

However, Stage A's `make test-backend` runs only `test/unit`, and Stage D runs only `test_provider_free_baseline.py`. No canonical command invokes `test_provider_tripwire.py`. A completed execution could therefore claim H2 coverage without ever proving that the tripwires count and stop positive calls. The prerequisite list also still names the deleted `.ts` frontend path rather than the committed `.tsx` file.

### M4 — Medium, residual H4: screening is best-effort rather than a fail-closed no-secret proof

`Finalize` screens recognized bearer, `sk-*`, and key/token/password/secret assignment patterns before hashing, and it covers common text, JSON, YAML, env, HAR, log, and Markdown files. It does not reject unrecognized credential shapes, cookies/session identifiers, credentials embedded in URLs without a recognized field name, or secrets visible in the required screenshot. Such content can produce `secret-screening.json` with `result: clear` and enter the manifest unchanged. The implementation supports the plan's narrowly worded "recognized credential patterns" claim, but not acceptance criterion 11's categorical assertion that the evidence contains no credential.

The generated run name and no-overwrite command files prevent accidental reuse, and every helper action refuses a run after `finalized.json` exists. This is helper-enforced non-overwrite behavior, not filesystem immutability: direct writers can still alter a finalized directory. In addition, `finalized.json` is deliberately created after and excluded from `manifest.sha256`, so its finalization timestamp and policy statement are not integrity-covered.

## Disposition of every prior finding

### H1 — Closed

The overlay supplies `/.env` using Compose long syntax with `${BASELINE_ENV_FILE}` and replaces the base volume by container target. Read-only expansion from the D: repository with C:/ProgramData-style `BASELINE_ENV_FILE` and `BASELINE_RUN_DIR` succeeded. It produced exactly the five expected services and showed the external env and evidence bind sources without the predecessor's `../C:/...` invalid-volume error.

### H2 — Implementation closed; execution-plan gap recorded as M3

The tripwire now patches both LiteLLM entry points, the project runner, the SDK runner and current direct aliases, `LitellmModel` construction, and retained OpenAI SDK client aliases. The positive test covers the actual current LLM and embedding seams and asserts counted failures. Static inventory agrees with that boundary set. The test could not be executed in this terminal because `uv` is unavailable and the fallback Python environment lacks `asyncpg`; no external call was attempted. The remaining defect is that the normative execution sequence does not run this test.

### H3 — Closed for unrelated-stack isolation

The overlay overrides all five global test container names with `dim0-task189-*`, overrides every published port with localhost-only Task #189 defaults, and `Preflight` checks exact-name Compose ownership plus port listeners before startup. A read-only local probe found all expected names absent and ports 15434, 16335, 16381, 18082, 15175, and 15182 clear. Fixed project volumes intentionally remain reusable between Task #189 attempts; that persistence must be treated as prior-run state, not evidence-directory isolation.

### H4 — Substantially closed; residual security/integrity limits recorded as M4

`Initialize` uses a millisecond timestamp plus random suffix and creates without `-Force`; command outputs reject duplicate names; each command record contains exact text, resolved working directory, start/end timestamps, duration, exit code, bounded log metadata, and adjacent output/exit files. The backend writes invocation and construction counters directly into the external evidence bind; `ValidateTripwires` requires both files, the exact fixed schema, and zero values. `Finalize` screens/redacts before generating stable relative-path SHA-256 entries and then seals further helper actions. The PowerShell parser accepted the helper. M4 limits the stronger secret-free/immutable interpretation.

### H5 — Open

No new run exists, as described in the high finding above.

### M1 — Partially open

The actual Boards home lifecycle is now rendered and clicked. The actual board/canvas lifecycle and canvas interaction are not, as described above.

### M2 — Closed

The live-store test wraps `update_vectors`, `upsert`, and `batch_update_points` on the actual `ContentStore` Qdrant client during the spatial patch. It records direct vector/upsert calls and vector-bearing batch operation types, then requires no such operation, no embedder call, and an unchanged retrieved vector. Payload-only `SetPayloadOperation` remains permitted, matching `GraphStore.patch_note` and the normative spatial/style-only rule.

## Normative-spec and execution-plan assessment

The hardened harness preserves the normative PostgreSQL/Qdrant/Redis topology, canonical `GraphStore -> ContentStore` storage path, deterministic 512-dimensional embedding substitute, spatial-update rule, and separation between generative LLM isolation and embedding behavior. It does not alter runtime/product code or claim to satisfy later Codex-mode criteria 2-10; that scope is appropriate for the upstream baseline.

The changed execution plan is **not fully accurate or complete**. Its Compose paths, service names, ports, evidence helper commands, counter export, restart probe, and finalization order match the implementation. Its frontend prerequisite path is stale, its Stage A/D commands omit the new positive backend tripwire test, and its manual UI paragraph is not a reproducible recorded actual-canvas check. The plan should be corrected before the acceptance run; an acceptance report must then cite artifacts for every criterion.

## Verification performed

- `git diff 9bd90d2^ 9bd90d2 --check` passed.
- PowerShell parser validation of `capture-task-189-evidence.ps1` passed.
- Read-only Compose expansion with D: repository files and C:/ProgramData-style env/evidence sources passed; exactly five expected services, overridden names/ports, external mounts, and exported counter paths were observed.
- Read-only exact-name and host-port preflight probe found no Task #189 containers and all six configured host ports clear.
- Static backend inventory covered all current direct SDK runner, project runner, LiteLLM, OpenAI-compatible client, and embedding seams.
- `npm run test:run -- src/features/agent/engine/__tests__/provider-free-baseline.test.tsx` passed: one file, two tests. jsdom also emitted its expected unimplemented canvas-context notice, consistent with the test not rendering the canvas surface.
- The focused Python self-test was attempted without provider credentials or calls. It could not collect because `uv` is not installed in this terminal and the available Python environment lacks `asyncpg`; this is an environment limitation, not the basis for the findings.

## Acceptance disposition

Do not accept Task #189 or begin the baseline run from this plan unchanged. First add the positive tripwire test to the recorded sequence, make the UI check mount and interact with the actual board/canvas application boundary (or provide an equivalently deterministic browser test), correct the `.tsx` path, and define fail-closed handling for evidence that secret screening cannot classify. Then execute Stages A-F in a new external run directory and independently review its checksummed evidence.
