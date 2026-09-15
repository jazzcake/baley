# Baley Dim0 downstream rebase history

This is the durable ledger for changes Baley carries on top of upstream Dim0.
Use it when importing a newer upstream tree, rebuilding the downstream patch
stack, or deciding that an upstream change has made one of our patches obsolete.
It is intentionally separate from upstream's `CHANGELOG.md`, which describes
product releases rather than Baley-specific divergence.

## Anchors

- Upstream Dim0 commit imported into Baley: `43b4a670c78c189b90cf1bab392f110b6905e4fa`
- Baley import merge: `e3d83541daa8c3bc63f7e3fb0ebee07f56763b4b`
- Ledger initialized against Baley HEAD: `5c9764f`

Commit hashes are useful historical evidence but are not stable across a
rebase. The `BD-*` change ID, intent, affected seams, and verification contract
are the durable identity of each downstream change.

## Recording rules

Add one entry for every accepted logical change under `dim0/` that Baley may
need to carry across a future upstream refresh. Keep previous entries intact;
when circumstances change, append a dated disposition rather than rewriting
the original evidence.

Each entry must record:

- the user-visible symptom or operational requirement;
- the first layer where expected and actual behavior diverged;
- the minimal downstream change and important affected paths;
- whether to carry, re-evaluate, or drop it during an upstream refresh;
- focused tests plus any broader acceptance evidence;
- configuration, data, or manual steps that Git cannot replay.

Use these dispositions:

- **Carry** — still required unless upstream contains an equivalent fix.
- **Re-evaluate** — compare with upstream first; replay only the missing part.
- **Drop** — retained for history but no longer belongs in the patch stack.
- **Operational only** — runtime state, not a source patch; recreate explicitly.

Never put credentials, tokens, unredacted logs, or generated evidence bundles
in this file. Link their documented location and integrity hash instead.

## Upstream refresh checklist

1. Record the old and new upstream commits and preserve this ledger before
   replacing or merging the subtree.
2. Search the new upstream tree for each entry's intent, tests, and affected
   seams. Mark equivalent upstream implementations **Drop** with a dated note.
3. Replay remaining entries in `BD-*` order, adapting behavior rather than
   blindly resolving textual conflicts.
4. Run each entry's focused verification immediately after replay.
5. Run the current complete provider-free Docker baseline after the stack is
   rebuilt, then append the new run path and manifest hash to `BD-002`.
6. Confirm host/runtime-only settings separately; Git does not restore them.

## Downstream changes

### BD-001 — Codex runtime adapter and persistence contract

- **Introduced:** 2026-09-13
- **Disposition:** **Carry**, unless upstream gains the same local Codex
  runtime API and persistence semantics.
- **Historical commits:** `7c0136e`, `4a21fbc`, `cc7ed15`
- **Requirement:** Let the Baley-hosted Dim0 fork invoke its Codex runtime
  adapter through the backend while keeping the integration isolated from
  Baley task synchronization and UI embedding.
- **Divergence:** Upstream had no `topix.ai_runtime.codex` adapter, matching API
  route, Compose overlay, or documented persistence contract.
- **Implementation seams:**
  - `.env.codex.example`
  - `backend/topix/ai_runtime/`
  - `backend/topix/api/app.py`
  - `backend/topix/api/router/ai.py`
  - `backend/Dockerfile`
  - `build/docker-compose.codex.yml`
  - `docs/adr/ADR-CODEX-001.md`
  - `docs/plans/codex-runtime-*.md`
- **Rebase notes:** Compare backend application wiring and `/ai` router changes
  carefully. Preserve the ADR boundary: no Baley Task Record coupling, schema
  migration, or Baley UI embedding is implied by this adapter.
- **Verification:** Run the Codex runtime unit tests and then the provider-free
  Docker baseline in `BD-002`. Provider credentials must remain blank for the
  baseline.

### BD-002 — Provider-free five-service Docker baseline

- **Introduced:** 2026-09-14; authoritative acceptance completed 2026-09-15
- **Disposition:** **Re-evaluate**, then **Carry** every assertion not provided
  equivalently by upstream.
- **Historical commits:** `bf6563f..5c9764f` (plans, harness iterations,
  corrections, acceptance, and independent re-review)
- **Requirement:** Prove that backend, Web UI, PostgreSQL, Qdrant, and Redis can
  build and operate without constructing or invoking any external LLM,
  embedding, search, fetch, OCR, image, or Daytona provider.
- **Divergence:** The original tree did not provide a locked, provider-free,
  no-egress acceptance sequence with canonical store CRUD, restart persistence,
  browser canvas observation, secret screening, and sealed evidence.
- **Implementation seams:**
  - `build/capture-task-189-evidence.ps1` and its tests
  - `build/task-189-runtime-check.ps1` and its tests
  - `build/docker-compose.baseline.yml`
  - `build/Dockerfile.task189-browser`
  - `backend/test/integration/baseline/`
  - `webui/scripts/task189-*`
  - `webui/src/features/agent/engine/__tests__/provider-free-baseline.test.tsx`
  - `Makefile` and `backend/Dockerfile` baseline-specific build/test seams
  - `docs/plans/task-189-*.md`
- **Rebase notes:** Provider tripwires must follow callables captured in aliases,
  dispatch maps, routers, and prebuilt tool objects, not only their definition
  sites. Preserve monotonic cross-process counters across backend restarts,
  network isolation, credential-safe raw diagnostics, and the corrected locked
  image Stage D storage execution. Rebuild browser localization if upstream
  changes the dotLottie asset or Vite bundle behavior.
- **Focused verification:** Follow
  `docs/plans/task-189-baseline-execution.md` from Initialize through Finalize;
  all backend/Web UI checks, five-service health, PostgreSQL/Qdrant/Redis CRUD,
  HTTP smoke, restart persistence, recorded non-agent canvas interaction,
  provider zero counters, secret screening, and scoped cleanup are mandatory.
- **Latest authoritative evidence:**
  `C:\ProgramData\Dim0\validation\task-189\run-20260915-080609-761-78800b12`
  with manifest SHA-256
  `cfe2512384eb65de9aba55b8b741916fb450bfad659e752d3df1dcf62bdde00b`.
  The independent verdict is **APPROVE** in
  `docs/plans/task-189-authoritative-rereview.md`.
- **Boundary:** This establishes provider-free operation, not real-provider
  behavior or a permanently published host endpoint.

### BD-003 — Baley.Dim0 product title

- **Introduced:** 2026-09-15
- **Disposition:** **Carry**; this is the fork's intentional user-facing
  identity and is not expected from upstream.
- **Task/commit:** `feat(webui): brand product as baley.dim0`
- **Symptom/requirement:** The imported application presented itself as `Dim0`,
  while this private MIT-licensed fork is operated as `Baley.Dim0`.
- **First divergence:** Static Web/PWA/Tauri title and wordmark values still
  contained the upstream product name; no React state or controller divergence
  was involved.
- **Change:** Replace formal user-facing app titles and wordmarks with
  `Baley.Dim0`. Keep internal package names, APIs, storage identifiers, Docker
  resources, executable identifiers, and upstream attribution unchanged.
- **Affected paths:** `webui/index.html`, `webui/vite.config.ts`, the sidebar
  and desktop chrome components, the PWA install screen, Tauri configuration,
  and generated desktop OAuth result pages plus their generator.
- **Rebase notes:** Upstream commonly edits PWA and Tauri metadata during
  releases. After resolving those files, search formal title/wordmark surfaces
  for an exact standalone `Dim0`; do not mechanically rename domain types or
  historical/upstream documentation.
- **Focused verification:** Run Web UI type-check and production build; inspect
  the built `index.html` and generated manifest for `Baley.Dim0`, then confirm
  the browser tab and sidebar wordmark in the live Web UI.
- **Broad verification:** Run the provider-free Docker baseline when this is
  replayed as part of a full upstream refresh.
- **Non-Git steps/evidence:** Rebuild and recreate the Web UI container before
  checking the existing Tailscale Serve URL; the Serve mapping itself does not
  change.

### BD-004 — Sheet body selection before inline editing

- **Introduced:** 2026-09-15
- **Disposition:** **Carry**, unless upstream implements the same explicit
  select-then-edit interaction contract for custom Sheet nodes.
- **Task/commit:** `fix(webui): select sheet body before editing`
- **Symptom/requirement:** Clicking a Sheet's text body did not select the node;
  users had to click its title area. The desired contract is one body click to
  select, another single click to do nothing, and a double-click on a selected
  body to enter inline editing.
- **First divergence:** `useStopCanvasGesture(bodyRef)` correctly stopped the
  native `pointerdown` before canvas-harness could capture an interactive TipTap
  surface, but `SheetView`'s subsequent React `click` handler only stopped
  propagation and never copied the intended selection into the canvas store.
  The library selection state therefore remained unchanged while the rendered
  Sheet body consumed the event.
- **Change:** Resolve Sheet body input through an explicit `select | edit | none`
  state decision. An unselected body selects its node, a selected body's single
  click is inert, and only a selected editable body double-click enters TipTap.
  Double-clicks during editing remain inert at the wrapper so native text word
  selection is preserved.
- **Instrumentation:** Development builds log `[baley.dim0:sheet-interaction]`
  with the user event, resolved action, edit permission/state, canvas selection,
  library interaction mode, active element, and rendered `data-sheet-*` state.
- **Affected paths:**
  `webui/src/features/board/harness/node-types/sheet/{view,interaction}.ts`
  and `interaction.test.ts`.
- **Rebase notes:** Preserve the native pointerdown stop; removing it lets the
  canvas capture gestures before TipTap receives them. Reconcile only the React
  click action after checking whether upstream's custom-node event boundary now
  performs selection itself.
- **Focused verification:** Six state-decision tests must pass. A real Chromium
  observation must create a local Note and show rendered state transitions
  `false/false -> true/false -> true/false -> true/true` for initial state,
  first click, selected single click, and selected double-click respectively;
  the final state must contain an editable TipTap surface and no page error.
- **Broad verification:** Run Web UI type-check, ESLint, and production build.
- **Non-Git steps/evidence:** Rebuild and recreate only the Web UI container to
  update the live Tailscale endpoint; no persistence service or mapping changes.

## Operational-only history

### OPS-001 — Tailnet-only live evaluation endpoint

- **Introduced:** 2026-09-15
- **Disposition:** **Operational only**; do not cherry-pick or treat as part of
  the accepted Task #189 topology.
- **Purpose:** Allow the operator to evaluate the already-built Web UI and API
  from devices on the same Tailscale network.
- **Runtime state:** Tailscale Serve maps Web UI port `8451` to host loopback
  port `15175` and API port `8452` to host loopback port `18082`. The Docker
  project is `dim0-task189`; only its two application services are loopback
  published, while persistence services remain internal and unpublished.
- **Source impact:** None. The Compose overlay and Vite allowed-host adjustment
  were runtime-only; no repository file, host PATH, persistent environment, or
  firewall rule was changed.
- **Recreation check:** Reconfirm the tailnet DNS name and occupied Serve ports,
  start the scoped five-service project, then verify Web UI `200`, API ping
  `204`, and models `200`. Do not overwrite unrelated Tailscale Serve mappings
  or Docker resources.
- **Removal:** Remove only the two scoped Serve mappings and the
  `dim0-task189` runtime resources after confirming their exact targets.

## Entry template

### BD-NNN — Short change name

- **Introduced:** YYYY-MM-DD
- **Disposition:** **Carry** | **Re-evaluate** | **Drop** | **Operational only**
- **Task/commit:** stable task reference and commit subject or historical SHA
- **Symptom/requirement:** what the operator or user observed or needs
- **First divergence:** event, calculated state, application/store state,
  library/controller state, and rendered/runtime state as applicable
- **Change:** minimal behavioral correction
- **Affected paths:** paths and interfaces likely to conflict upstream
- **Rebase notes:** upstream-equivalence test and manual conflict guidance
- **Focused verification:** exact tests and observable result
- **Broad verification:** baseline or integration suite
- **Non-Git steps/evidence:** runtime configuration, migration, evidence path,
  and integrity hash; never secrets
- **Later disposition:** append dated updates here
