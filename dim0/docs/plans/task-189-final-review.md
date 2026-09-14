# Task #189 final independent review

Date: 2026-09-15 (Asia/Seoul)

Reviewed acceptance commit: `36442268a4fa6f323d3987c1097579b4d22eefc2`

Reviewed run commit: `da2ee2cf4302ffa89cd0ee73063b92f68d74d5f7`

External evidence: `C:\ProgramData\Dim0\validation\task-189\run-20260915-025900-783-70e2c872`

Manifest SHA-256: `b95e3d4d83b1828eac458276386fc4e34f5045be0d2fcc9dd223d6c11762a085`

## Verdict

**Rejected.** The bundle is internally hash-consistent and much of the
provider-free storage baseline passed, but the accepted result overlooks an
authoritative final-state failure: the backend exited `1` after the recorded
restart. The same run also had unrestricted Docker egress and made actual
external Doppler and jsDelivr requests, so the stronger claim that only the
dependency synchronization had network access is false and the evidence cannot
establish that external LLM/embedding access was impossible at the network
boundary. Commit `3644226` changes only the result document, but its
`accepted; all eleven mandatory criteria passed` conclusion is not supported by
its own finalized evidence.

## Severity-ranked findings

| ID | Severity | Finding | Exact evidence |
| --- | --- | --- | --- |
| F1 | **High** | The backend did not survive the Stage E restart. The restart command itself exited `0`, and the persistence-only wait/test passed, but neither checked backend readiness. Final Compose state records `dim0-task189-backend` as `Exited (1)`. Logs show the restarted process failed in `setup()` while Qdrant was unavailable. This invalidates the claimed healthy five-service baseline and requires a fresh Stage A-F run with post-restart backend HTTP readiness. | `commands.jsonl` records `55-restart`, `56-persistence-ready-after-restart`, and `57-persistence-after-restart` as exit `0`; `60-compose-logs.txt:286-406` contains `httpx.ConnectError` and `qdrant_client...ResponseHandlingException`; `61-compose-ps-final.txt:2` records `Exited (1)`. The contrary acceptance language is `task-189-baseline-result.md:333,367-369,391`. |
| F2 | **High** | External network access was possible and used outside the dependency synchronization. Backend startup made two unauthorized Doppler requests, and the browser fetched a jsDelivr WASM asset. Known LLM and embedding seams were guarded and no LLM/embedding request is evidenced, but egress was not disabled for the live application/browser paths; therefore the requested absolute statement that such APIs were *not possible* is not established. | `60-compose-logs.txt:9-17` and `:260-264` record Doppler HTTP `401` responses; `browser-network.har:1457` records a successful `https://cdn.jsdelivr.net/.../dotlottie-player.wasm` request. `commands.jsonl` shows Stage D on `dim0-task189_default`, Compose applications on their default network, and the browser on `container:dim0-task189-webui`. This contradicts `task-189-baseline-result.md:354-356`. |
| F3 | **Medium** | The recorded browser canvas observation is real but not a healthy end-to-end application observation. It created a local board, found a `1280x720` canvas host, and dispatched a wheel event, while both browser requests to the backend failed and the console recorded two connection-refused errors. | `59-browser-observation.txt:1`; `browser-console.json:15-22`; `browser-network.har:1531-1598` records failed `http://localhost:18082/ai/models` and `/utils/ping`; `61-compose-ps-final.txt:2` explains the unavailable backend. |
| F4 | **Medium** | Persistence-service provenance is not immutable in the evidence. Application builds record resolved base digests and produced image manifests, but PostgreSQL, Qdrant, and Redis are identified only by mutable tags, including `qdrant/qdrant:latest`; no container image ID or repo digest was captured. The run is auditable as observed, but not exactly reproducible from those three references. | `08-stage-a-images.txt:48-67,110-166` and `22-image-build.txt:39-170` record application base/output digests; `20-compose-config.txt:88,110,140` and `32-persistence-ps.txt:2-4` identify `postgres:15`, `qdrant/qdrant:latest`, and `redis:7-alpine` without immutable digests. |
| F5 | **Low** | The evidence supports a bounded no-host-install conclusion, not a system-wide forensic one. The complete 39-command ledger contains no host package manager, executable/archive placement, `PATH` update, `setx`, registry mutation, or persistent environment write; the only dependency installation was `uv sync --frozen` inside a Docker volume, and the only host-side test was the PowerShell helper self-test. There is no before/after inventory of host packages, executables, `PATH`, or persistent environment values, so an absolute machine-wide negative cannot be independently reconstructed. | `commands.jsonl` entries `09-stage-a-backend-deps` and `16-evidence-finalization-self-test`; `09-stage-a-backend-deps.txt`; `02-git-status-before.txt` and `62-git-status-after.txt` are empty for `dim0`. |

## Independently confirmed evidence

- Manifest integrity passed. All 89 manifest entries exist and independently
  match their SHA-256 values; only `manifest.sha256` and `finalized.json` are
  intentionally outside the manifest. The independently calculated manifest
  hash matches `finalized.json`, whose `fileCount` is 89.
- The command ledger contains exactly 39 JSON records, all with exit code `0`,
  and every paired `.exit.txt` contains `0`. F1 demonstrates why those command
  exits do not by themselves prove asynchronous service health.
- Source and worktree evidence is coherent: `01-git-head.txt` pins `da2ee2c`,
  and `02-git-status-before.txt` plus `62-git-status-after.txt` are empty for
  `dim0`. The acceptance commit range `da2ee2c..3644226` changes only
  `dim0/docs/plans/task-189-baseline-result.md`. The bundle does not establish a
  before/after hash for the root `debug.log`; this review did not read or modify
  that file.
- Compose expansion lists exactly `redis-test`, `postgres-test`, `qdrant-test`,
  `backend-test`, and `webui-test`. Fixed container names and all six published
  ports are Task-189-scoped and loopback-only; the external `baseline.env` is
  mounted read-only at `/.env`; provider credentials expand to empty strings;
  and no provider or Codex service is present.
- Backend lint and 708 unit tests passed. Web UI type/lint, 1,356 tests in 142
  files, and production build passed. The positive provider-tripwire and
  evidence-finalization self-tests passed in network-disabled containers.
- Stage D entered FastAPI lifespan, applied the schema idempotently, and passed
  canonical board/note/link CRUD against PostgreSQL/Qdrant/Redis. The test uses
  a SHA-256-derived deterministic 512-dimensional fake embedder, verifies text
  mutation embeds, a position-only payload mutation preserves the vector and
  performs no vector operation, and a forced embedding failure preserves the
  prior content/vector. The second test reads the exact graph, note, vector,
  and Redis sequence after persistence-service restart.
- `provider-invocations.json` and `provider-constructions.json` have the exact
  seven-key schema and all values are zero. Blank provider credentials, the
  positive tripwire test, Stage D assertions, and the HAR support the narrower
  conclusion that no external LLM or embedding API was invoked through the
  covered application seams. They do not prove network-level impossibility.
- Secret screening is internally sound for the captured text/JSON evidence:
  `secret-screening.json` reports `clear`, lists all 88 pre-screen artifacts,
  and is itself manifest-covered. The HAR strips headers, cookies, query
  strings, and response bodies. Independent inspection found no provider-host
  request in its 43 entries.
- Cleanup was narrow for the accepted run. `63-compose-down.txt` removes only
  its five containers and `dim0-task189_default`; `64` removes the accepted
  run's unique Stage A volume; `65` deletes the disposable browser image.
  Current label-filtered inspection finds no Task-189 container or network, and
  the accepted run's Stage A volume/browser image are absent. Four named
  persistence volumes and several Stage A volumes from older runs remain;
  these are outside the accepted run's claimed cleanup and were not modified by
  this review.

## Required disposition

Do not accept Task #189 on commit `3644226`. Correct the restart orchestration
or readiness behavior, prevent unintended external configuration/CDN access (or
explicitly narrow and accurately state the network contract), add a backend
HTTP check after restart, record immutable persistence image identities, and
rerun the entire Stage A-F sequence in a new external evidence directory. The
replacement result must report any service exit or external request rather than
inferring health from a successful orchestration command.
