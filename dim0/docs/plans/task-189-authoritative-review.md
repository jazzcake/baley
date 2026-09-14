# Task #189 authoritative provider-free acceptance review

Review date: **2026-09-15 KST**

Result commit: **`7c9a675f5727579f045dfbf5e6d214a267286875`**

Implementation baseline: **`86f8233b81cbd9e7136656a6d26e338fb3052cb2`**

Evidence: **`C:\ProgramData\Dim0\validation\task-189\run-20260915-053337-790-ad76dea1`**

## Verdict: REJECT

The evidence package is internally consistent and demonstrates a substantial,
successful network-isolated Docker exercise. It does **not**, however, satisfy
the execution plan's mandatory fail-closed provider-tripwire contract. Several
current HTTP provider paths retain pre-patch function aliases, OCR has a direct
constructor path that bypasses the patched factory, and the positive self-test
only exercises LLM and embedding seams. In addition, restart replaces rather
than accumulates the counter files. The seven-key all-zero JSON artifacts
therefore cannot prove that every supported provider boundary was guarded or
that counters stayed zero for the whole run.

This is an acceptance-level failure, not evidence that a provider call
succeeded. The internal Compose network had no default route, and the browser
HAR contains no provider origin. The baseline must nevertheless be rerun after
the tripwire and its positive coverage are corrected; the execution plan
explicitly says that absence of an observed call is not proof that no call was
possible.

## Findings

### High — F1: the mandatory seven-boundary provider tripwire is incomplete

The declared contract says the harness replaces LLM, embedding, search, fetch,
OCR, image, and Daytona clients; records constructions and invocations
separately; fails every outbound invocation; and positively proves supported
boundaries increment counters before I/O
(`dim0/docs/plans/task-189-baseline-execution.md:15`, `:34`, `:114`, `:290`,
and `:308`). The implementation and recorded self-test do not meet that
contract:

- `install_provider_tripwire()` imports `topix.api.router.ai` at
  `dim0/backend/test/integration/baseline/provider_tripwire.py:131`, then only
  patches the source modules at `:241-247`. The router had already copied
  search/fetch/Daytona callables into module globals at
  `dim0/backend/topix/api/router/ai.py:29-36`, including the `_SEARCH_FNS`
  dictionary at `:395-400`. Its live endpoints call those retained aliases at
  `:427-432`, `:472`, and `:503`, so patching `web_tools` and `daytona_code`
  afterward does not replace the router's current call sites.
- OCR has no `tripwire.invocation("ocr")` or OCR construction counter in the
  tripwire. The only OCR substitution patches `MistralParser.from_config` at
  `provider_tripwire.py:236`; its fake parser raises without incrementing the
  OCR counter at `:89-94`. The BYOK route directly constructs
  `MistralParser(api_key=...)` at `ai.py:546`, bypassing that factory and
  constructing the real Mistral client at
  `dim0/backend/topix/nlp/parser.py:23`.
- Construction accounting is similarly limited: only LLM and embedding
  constructions invoke `tripwire.construction()` (`provider_tripwire.py:151-183`).
  The seven-key construction schema is therefore broader than its actual
  instrumentation.
- The positive test is named
  `test_every_current_llm_and_embedding_seam_fails_closed` and exercises only
  LLM and embedding behavior
  (`dim0/backend/test/integration/baseline/test_provider_tripwire.py:12-126`).
  The authoritative artifact `15-provider-tripwire-self-test.txt` reports one
  passing test; it contains no search, fetch, OCR, image, or Daytona endpoint
  exercise.

Consequently, the zero values in `provider-constructions.json` and
`provider-invocations.json` are valid JSON with the expected seven numeric keys,
but some zeros are unobservable-by-design and others do not cover the current
router path. Network isolation reduced actual egress risk; it cannot substitute
for the required counter/fail-closed assertion.

Required correction: patch the call sites actually used by the application
(including imported router aliases and both configured/BYOK OCR construction),
instrument construction and invocation for every declared key, and add a
positive network-disabled test that drives all seven boundaries and proves each
increments and fails before I/O. Then perform a fresh full acceptance run.

### Medium — F2: restart overwrites, rather than preserves, run-wide counters

Every backend process creates a new in-memory `ProviderTripwire` and writes a
fresh zero snapshot before serving
(`dim0/backend/test/integration/baseline/provider_tripwire.py:254-266`). It
writes the same snapshot again on shutdown (`:267-273`) to the fixed paths in
`dim0/build/docker-compose.baseline.yml:46-47`. The command ledger stops and
later starts the backend (`commands.jsonl` rows 33 and 36; artifacts
`56-backend-stop-before-restart.*` and
`59-backend-start-after-persistence.*`). A new process therefore replaces the
first process's files, and finalization after teardown (ledger row 46) validates
only the most recent snapshot.

No recorded pre-restart command intentionally exercises a provider route, so
this review found no affirmative evidence of a hidden call. The evidence design
still cannot prove the claimed run-wide invariant. Counters should be written
per process and aggregated monotonically, or merged atomically into a durable
run-level record that a restart cannot reset. The corrected design needs its
own restart/aggregation self-test and a fresh acceptance run.

### Low — F3: “no external requests” is only true for the browser and successful egress

The sanitized HAR has 44 entries and only the allowed `http://localhost` and
`http://backend-test:8082` origins; it contains no provider or other external
request. Runtime isolation also records no default routes, host mappings, or
published ports. However, the bounded Compose log shows Qdrant attempting to
report telemetry twice and failing on `https://telemetry.qdrant.io/`
(`70-compose-logs.txt:203` and `:279`). This is blocked external-request intent,
not successful egress and not a Dim0 provider invocation.

The final result section generally uses the accurate narrower phrases “zero
external/provider browser requests” and “no ... external route”
(`dim0/docs/plans/task-189-baseline-result.md:722`, `:737-738`), and the same
document previously discloses the blocked telemetry attempt at `:592-593`.
Future summaries should preserve that distinction and must not generalize the
browser counter to “the stack made no external request attempts.”

## Independently verified evidence

### Integrity, finalization, and ledger

- Recomputed `manifest.sha256` SHA-256:
  **`dae5d1346ec3f28c1588d19426b6efd1d9e8da6ee9a083c1acf004e2bc3d390a`**.
  It matches `finalized.json`.
- The manifest has 107 sorted, unique entries. All paths are well formed; every
  listed file exists and matches its recorded SHA-256; no pre-seal artifact is
  unlisted. The directory contains exactly 109 files: those 107 plus
  `manifest.sha256` and `finalized.json`.
- `finalized.json` records the correct manifest hash, 107 files, and
  `filesystemImmutable: false`. The finalization marker and manifest are valid,
  but not cryptographically signed or protected by an immutable filesystem.
- `commands.jsonl` contains exactly 48 unique, timestamp-ordered records.
  Every record has a command, working directory, output artifact, matching exit
  artifact, and line count; all 48 exit values are exactly numeric text `0`.
  The ledger covers provenance, static/build checks, Compose expansion/build,
  persistence, application/restart/browser/isolation checks, logs/status, and
  cleanup. Forty-six commands ran from the repository root and two from
  `dim0`.
- `secret-screening.json` is `clear` and covers 106 artifacts: every manifest
  entry present at screening time except the screening report itself. The
  manifest and marker are produced afterward. The helper accepts only its
  text/JSON allowlist, so no unsupported binary evidence escaped that scope.
  This is regex-based credential screening, not a proof against arbitrary
  undisclosed secret formats.

### Docker, application, browser, and storage observations

- `20-compose-config.txt` and `21-compose-services.txt` resolve exactly
  `postgres-test`, `qdrant-test`, `redis-test`, `backend-test`, and
  `webui-test`; there is one `internal: true` default network, no published
  ports, and no provider or Codex service/profile. The external baseline
  environment file is mounted read-only.
- `55-app-ps-before-restart.txt`, `65-five-service-final-state.txt`, and
  `71-compose-ps-final.txt` each show exactly those five services running.
  PostgreSQL, Qdrant, and Redis are healthy; backend and Web UI are running.
  `64-runtime-isolation.txt` reports an empty default-route set, empty effective
  host mappings, empty host-port bindings, and one internal network per service.
- Backend probes returned ping `204`, models `200` with 20 entries, and Web UI
  HTTP `200` with 4,014 bytes before and after restart. The canonical live
  storage checks passed before and after restart.
- `40-storage-contract.txt` and `61-persistence-after-restart.txt` support the
  documented graph/note/link persistence assertions: 512-dimensional vectors,
  text-change re-embedding, spatial-only update without vector work, embedding
  failure atomicity, and increasing Redis sequence values. The post-restart
  check reads the same fixed records and updated content.
- `36-persistence-image-identities.txt` records configured tags plus immutable
  local image IDs and repo digests for PostgreSQL, Qdrant, and Redis. Qdrant's
  configured tag is mutable `latest`, but the evidence binds the actual run to
  a concrete image identity.
- `63-browser-observation.txt` records a visible 1280x720 canvas, real
  Control+wheel input, zoom `100%` to `110%`, successful backend probes, and
  zero browser provider/external counters. `browser-console.json` contains no
  error (only the expected service-worker warning). `browser-network.har` is
  sanitized: all request/response headers, cookies, query strings, redirects,
  and bodies are absent.
- Cleanup artifacts show all five services and the project network removed, the
  run-specific Stage A volume removed, and the browser image removed. A
  read-only post-run Docker inspection found no current project container or
  network and no browser image. Four intended data volumes remain, as disclosed.
  Eight older Stage A volumes from non-authoritative attempts also remain; they
  are outside this run's scoped cleanup claim.

### Git and documentation scope

- `01-git-head.txt` equals the implementation baseline. Both scoped status
  artifacts (`02-git-status-before.txt` and `72-git-status-after.txt`) are empty.
  The current worktree also had no scoped `dim0` changes before this review was
  added. Root-level `debug.log` was neither read nor used in this review; scoped
  Git evidence cannot itself prove historical non-access.
- The result commit changes only
  `dim0/docs/plans/task-189-baseline-result.md` relative to the implementation
  baseline; `git diff --check` passes. No acceptance output was committed.
- The final result accurately limits the exercise to provider-free,
  container-internal acceptance and says it does not validate ingress,
  user-accessible host operation, provider quality/availability, or retained
  volume deletion (`task-189-baseline-result.md:743-755`). The baseline contains
  dormant Codex-runtime implementation, but the acceptance environment does
  not select `DIM0_AI_RUNTIME=codex` and no Codex service/profile is present.
  Therefore this run does **not** validate the later user-accessible ingress or
  Codex runtime work.
- The local Dim0 import is a squashed subtree (`43b4a670...`, upstream split
  `c75cb329...`), followed by Task #189 harness changes. Provider seams rely on
  private module paths and captured function objects; a future upstream merge
  can add or move provider paths without making the seven JSON keys nonzero.
  Acceptance must be rerun after such a merge, backed by positive endpoint/seam
  coverage rather than a fixed schema alone.

## Focused checks rerun by this review

All checks were read-only or used mocked/temp state; no Docker service was
started, stopped, or restarted.

- `powershell -NoProfile -File build/capture-task-189-evidence.tests.ps1` — pass.
- `powershell -NoProfile -File build/capture-task-189-preflight.tests.ps1` — pass.
- `powershell -NoProfile -File build/task-189-runtime-check.tests.ps1` — pass.
- `node --test scripts/task189-localize-dotlottie.test.mjs scripts/task189-browser-observation.test.mjs` — 7/7 pass.
- Independent manifest/hash/path/count, ledger/exit/artifact, secret-screening,
  JSON-schema, HAR-origin/sanitization, five-service, runtime-isolation, and
  cleanup reconciliation checks — pass, subject to F1-F2.
- Baseline-to-result changed-path assertion and `git diff --check` — pass.

## Disposition and residual risk

F1 and F2 require harness corrections and a fresh unique, fully finalized run;
the existing package cannot be repaired by editing its counters or result
document. The manifest is helper-sealed but remains mutable and self-authenticated
(`filesystemImmutable: false`), the credential scan is pattern-bounded, the
Qdrant tag is mutable despite the captured immutable identity, retained volumes
intentionally preserve state, and Git cleanliness is scoped to `dim0`. These are
properly disclosed or bounded residuals, but they reinforce that this evidence
is a reproducible internal acceptance artifact—not a signed provenance record,
an ingress test, a live-provider test, or a Codex-runtime acceptance.
