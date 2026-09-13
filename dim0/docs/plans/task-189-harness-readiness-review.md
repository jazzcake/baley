# Task #189 baseline harness final readiness review

Date: 2026-09-14 (Asia/Seoul)

Reviewed implementation: `44711b9e80f92a6f61efcb3ae55faca199d00778`, together with harness commits `24299320b6f5c5f8f75f1a3cff06361f02098ec0` and `9bd90d2215b80fca494f69c65758a517acb61c91`, and the two prior harness review reports.

Reviewer role: independent provider-isolation, React boundary, Compose-safety, and evidence-integrity reviewer. This review changed no implementation.

## Verdict

**Reject commit `44711b9` as ready for the provider-free Stage A-F baseline execution.** Residual M1 and M3 are closed, and the Compose overlay remains isolated, but M4 is not closed: the fail-closed credential scanner rejects the normal multi-line `baseline.env` produced by its own `Initialize` action. The execution can safely begin and will not call a provider through the reviewed tests, but it cannot complete Stage F or produce a finalized integrity manifest without correcting this defect, so it is not an executable acceptance baseline as committed.

H5 is deliberately not a reason for this rejection. A fresh Stage A-F evidence run is expected to be the next task after harness readiness; historical evidence was not reused.

No real external AI API was called. No baseline service was started. `debug.log` was not read or modified.

## Residual disposition

### M1 — closed

The frontend test now mounts the real `HarnessCanvas` component under its required Query and theme providers, waits for both the rendered `[data-canvas-host]` and registered canvas store, changes the real store camera, clicks the rendered `Reset zoom to 100%` viewport control, and verifies the camera returns to zoom `1`. This is an actual React/canvas boundary interaction rather than the predecessor's test-only store mutation. The monitored `ByokLlmClient.fromConfig` construction seam remains at zero across both the Boards home lifecycle and canvas interaction.

The focused command passed: one Vitest file and two tests succeeded. jsdom printed its expected unsupported `HTMLCanvasElement.getContext()` notices, but the canvas host, store registration, control click, and state transition all completed.

### M3 — closed

Stage A now explicitly records `uv run pytest -q test/integration/baseline/test_provider_tripwire.py`, and the prerequisite path correctly names the `.tsx` frontend test. The positive provider test body passed against the current worktree source inside a `--network none` backend runtime: all supported LLM and embedding construction/invocation seams raised their tripwire assertion before outbound I/O.

The exact `uv run` wrapper could not be launched directly in this terminal because `uv` is not installed. This is an execution-environment prerequisite, not a plan omission; the positive test itself was run without network access and passed.

### M4 — open and execution-blocking

`Assert-NoCredentials` applies its `credential-field` expression to each entire file. The expression uses `\s*` around the assignment delimiter, so it can consume a newline after an intentionally empty credential and then treat the next environment assignment as that credential's value. A direct in-memory probe of the committed expression against:

```text
DOPPLER_TOKEN=
OPENAI_API_KEY=
ANTHROPIC_API_KEY=
API_ORIGIN=http://localhost:18082
```

matched `DOPPLER_TOKEN=\nOPENAI_API_KEY=` as a non-empty credential. `Initialize` writes this same adjacent-empty-key shape in `baseline.env` (`capture-task-189-evidence.ps1:289,298-300`), and `Finalize` scans that file (`:251-255`). Consequently the normal evidence bundle fails with `Credential screening failed closed for baseline.env: credential-field` before the manifest and finalized marker are written.

The new self-test passes because its nominal fixture contains only `OPENAI_API_KEY=` followed by end-of-file (`capture-task-189-evidence.tests.ps1:17`); it does not test the actual initialized multi-line environment file. The allowlist, strict UTF-8/JSON checks, rejection cases, integrity metadata, manifest coverage, and manifest-hash binding are otherwise materially improved and their focused self-test passed. However, because finalization policy and integrity metadata are written before credential screening, this false rejection also leaves pre-existing finalization artifacts that make an immediate retry refuse the same run directory.

## Compose isolation

Read-only two-file Compose expansion succeeded using cross-drive `C:/ProgramData/...` env and evidence paths. The service list is exactly `postgres-test`, `qdrant-test`, `redis-test`, `backend-test`, and `webui-test`; all five fixed container names use the `dim0-task189-*` namespace, all six published ports are localhost-only Task #189 ports, the external `baseline.env` mount replaces `/.env` read-only, the evidence bind targets `/baseline-evidence`, provider keys expand to empty strings, and no Codex service/profile appears. No container was started by this check.

## Verification summary

- `git diff 44711b9^ 44711b9 --check` passed, as did the cumulative harness diff check from the predecessor base.
- Focused HarnessCanvas Vitest: passed, 2/2 tests.
- Evidence finalization self-tests: passed, while the additional realistic multi-line empty-key probe exposed the M4 false positive.
- Positive provider-seam test body: passed in a network-disabled container against current source; no provider call was possible.
- Compose `config --services` and selected expansion inspection: passed with the expected five-service topology and isolation properties.

## Required correction and execution caveats

Before Stage A, constrain blank-value handling so credential-field matching cannot cross line boundaries (or parse `.env` assignments line-by-line), and add a success self-test that finalizes the actual `Initialize`-shaped `baseline.env` with all adjacent blank provider keys. Retain the current fail-closed rejection tests, allowlist, strict structured-file validation, and manifest binding.

After that correction, run the complete Stage A-F sequence in a fresh external run directory. Ensure `uv` and locked backend development dependencies are available before Stage A, preserve `--network none` or equivalent provider egress denial for focused seam verification where practical, treat retained named volumes as prior-run state, and continue to require zero invocation counters and sanitized JSON/HAR artifacts before finalization.
