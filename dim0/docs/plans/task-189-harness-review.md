# Task #189 provider-free baseline harness review

Date: 2026-09-14 (Asia/Seoul)

Reviewed implementation: `24299320b6f5c5f8f75f1a3cff06361f02098ec0`

Compared with:

- [`task-189-baseline-execution.md`](./task-189-baseline-execution.md)
- [`task-189-baseline-result.md`](./task-189-baseline-result.md)
- [`task-189-baseline-review.md`](./task-189-baseline-review.md)
- [`codex-runtime-and-persistence-spec.md`](./codex-runtime-and-persistence-spec.md)

Reviewer role: independent runtime-isolation, persistence, Compose-safety, and evidence reviewer. This review changed no implementation.

## Verdict

**Reject commit `2429932` as the provider-free baseline acceptance harness.** The deterministic embedding and canonical live-store test are directionally sound, but the documented Compose run cannot expand with the repository and evidence roots used by Task #189, the provider tripwire leaves current OpenAI Agents SDK invocation paths outside its counters, the frontend test does not render the claimed page path, and the evidence helper neither makes artifacts immutable nor exports the runtime counters into the checksummed evidence set. The historical result and review therefore remain truthful as a safe-stop record; they do not become an accepted application baseline merely because the four missing files now exist.

No real external AI API was called during this review. No container was started. `debug.log` was not read or modified.

## Severity-ranked findings

### H1 — High: the documented default Compose invocation is not runnable across the actual D: worktree and C: evidence root

The execution plan computes `ENVFILE` from `D:\Project_AI\baley\dim0` to the default `C:\ProgramData\Dim0\validation\task-189\baseline.env`, while Base Compose unconditionally prefixes the result with `../` (`docker-compose.yml:315`). On this checkout, .NET returns the cross-volume absolute path `C:/ProgramData/...`; Compose consequently receives `../C:/ProgramData/...:/.env:ro` and exits before service enumeration with:

```text
invalid spec: ../C:/ProgramData/Dim0/validation/task-189/baseline.env:/.env:ro: too many colons
```

The overlay does not replace that volume entry. This blocks Stages B through F using the canonical commands and default evidence location, so the harness cannot satisfy the execution plan as written.

### H2 — High: the tripwire does not intercept every current LLM invocation path and can report zero counters without proving provider isolation

`provider_tripwire.py:180-183` patches `litellm.acompletion`, `LitellmModel.__init__`, and the project wrapper `AgentRunner`. It does not patch the OpenAI Agents SDK `agents.Runner` itself or the aliases already imported by `topix.agents.tool_handler` and `topix.agents.websearch.handler`; those modules call `Runner.run` / `Runner.run_streamed` directly (`tool_handler.py:200,208`; `websearch/handler.py:156,167`). The native OpenAI branch in `BaseAgent.__post_init__` also deliberately leaves an `openai/...` model as a string (`base.py:66-68`), so it bypasses the patched `LitellmModel` constructor and delegates client construction/invocation to the SDK.

Patching `openai.AsyncOpenAI` on the public module is insufficient proof because the Agents SDK may hold its own imported client/factory reference. Empty API keys may make a missed path fail before billing, but that failure would not increment either tripwire file. There is also no tripwire self-test that deliberately traverses every OpenAI, Anthropic, OpenRouter, and embedding construction/invocation seam and asserts the appropriate counter and exception. A zero JSON result therefore gives false confidence rather than the plan's required fail-closed coverage.

### H3 — High: the Compose project name does not isolate the harness from other test-profile stacks

The overlay inherits fixed global `container_name` values `backend-test`, `webui-test`, `postgres-test`, `qdrant-test`, and `redis-test` (`docker-compose.yml:96,105,120,312,339`) plus fixed host ports. `-p dim0-task189` scopes labels, networks, and default volume names, but it cannot namespace those container names or occupied host ports. An unrelated Dim0 test stack can therefore prevent this run from starting, and a pre-existing same-name container must be inspected rather than assumed to belong to the Task #189 project. The overlay is additive, but it is not independently isolated or safe to launch solely on the strength of the project name.

### H4 — High: evidence capture is overwriteable, incomplete, and not secret-safe

`capture-task-189-evidence.ps1` uses one fixed directory and `New-Item -Force` (`:7,23`), then `Tee-Object` / `Set-Content` overwrite predictable filenames (`:31-34,41-56`). A later run can replace an earlier run and regenerate a matching manifest; checksums provide integrity at one moment, not immutability or run provenance. The helper captures only four initialization commands and final Git status. It does not centrally capture the Stage A-F commands, command text, timestamps, exit codes, bounded logs, screenshots, or browser export.

More importantly, the overlay writes provider invocation and construction JSON only to `/app/data` (`docker-compose.baseline.yml:8-9`), backed by `backend_data_test`; neither the plan's Stage F commands nor the helper copy those files to the external evidence root before manifest generation. Thus the two artifacts carrying the core provider claim are absent from the checksummed bundle. The helper also has no secret detection or redaction before hashing/preserving outputs; `docker compose config` can include interpolated environment values such as `DOPPLER_TOKEN`, so a mistaken environment can be durably captured despite the intended non-secret env file.

### H5 — High: no rerun result exists, so the historical blocked result cannot support acceptance of this harness

The baseline result and its independent review describe the earlier state where the four prerequisites were absent and explicitly classify application validation as blocked. Commit `2429932` adds harness code only; it supplies no new external manifest, counter files, live-store output, browser/network capture, restart evidence, or updated criterion-by-criterion execution result. Even if the implementation defects above were absent, Task #189 acceptance would still require a fresh Stage A-F run in a new evidence directory. The old infrastructure-only persistence probe is not evidence for the new harness.

### M1 — Medium: the frontend test does not load or interact with the claimed application boundary

The test dynamically imports `boards-home` and checks that `BoardsHome` is a function (`provider-free-baseline.test.ts:26-28`); it never renders the component, runs its effects/providers/router, or performs a page-level board load. Its explicit `llmClientFromResolution(... mode: "off")` calls (`:29-30`) only verify a hand-picked disabled branch. The canvas case instantiates `freshStore` and `StoreMutator` directly (`:35-39`), bypassing the mounted board application and its service-resolution lifecycle.

The mock is placed at the real `ByokLlmClient.fromConfig` seam used by `clients.ts:57`, which is useful, but none of the asserted UI flows actually reaches the code that decides whether to call that seam. Consequently the test cannot support the plan's claim that page loading and basic non-agent canvas interactions construct no provider client, nor can it detect a future construction added in a React effect or provider during real rendering.

### M2 — Medium: the spatial-update assertion proves vector equality, not absence of a Qdrant vector operation

The live-store test correctly proves that a spatial-only patch does not call the fake embedder and that the retrieved vector is unchanged (`test_provider_free_baseline.py:93-101`). It does not instrument Qdrant's vector-update/upsert boundary, so a redundant write of the existing vector would still pass. The execution plan requires that the spatial/style path perform neither embedding nor vector update. This is a narrower gap than H2 because current `GraphStore.patch_note` visibly routes unchanged embeddable content to `update_payload_only`, but the harness does not independently lock that behavior down.

### L1 — Low / pass: deterministic embedding dimension and failure behavior match the current Qdrant contract

`DeterministicFakeEmbedder` deterministically derives 512 floats from SHA-256 (`provider_tripwire.py:65-84`), and `ContentStore.create_collection()` takes its default vector size from `embedder.dimensions`. The test reads Qdrant collection configuration and requires size 512, exercises text embedding, and verifies that a forced embed failure leaves both content and vector unchanged (`test_provider_free_baseline.py:71,86-110`). The restart test retrieves the same board, notes, link, updated content, multivector dimensions, and Redis sequence (`:121-143`). This genuinely exercises PostgreSQL graph metadata plus canonical Qdrant content and Redis sequencing against live configured services when those services are available. The environment variable named `DIM0_BASELINE_FAKE_EMBEDDING_DIMENSION` is currently decorative—the fake is hard-coded to 512—but that matches the fixed Task #189 contract and does not by itself invalidate the semantics.

### L2 — Low / pass: non-LLM baseline substitutions do not invalidate the storage test

Search, fetch, OCR, image, and Daytona are outside the normative LLM product-policy boundary. Their substitutions are relevant only to keeping app startup isolated. The storage test does not use those services, and replacing newsfeed/OCR startup construction does not replace PostgreSQL, Qdrant, Redis, `GraphStore`, or `ContentStore`, so no baseline-validity defect was found on that limited point.

## Verification performed

- `git diff 24299320^ 24299320 --check` passed.
- Static symbol inventory found direct SDK `Runner` call sites outside the patched project wrapper, as described in H2.
- Read-only Compose expansion with the documented C: evidence path failed with the invalid volume specification in H1; no service was started.
- Read-only Compose expansion with a same-volume substitute path succeeded and confirmed the inherited fixed container names, ports, provider-key blanking, external read-only env mount, and live PostgreSQL/Qdrant/Redis topology.
- The focused frontend Vitest command reached test collection but failed before running tests because this checkout's installed `fake-indexeddb` package is incomplete (`fake-indexeddb/auto` could not be resolved). This is an environment/dependency-state limitation, not the basis for M1; M1 follows directly from the test code.

## Acceptance disposition

Do not run the full application baseline or accept Task #189 on commit `2429932`. A later harness correction must close every provider construction/invocation seam with positive tripwire tests, make the actual React page/canvas flow traverse the monitored construction boundary, remove global Compose naming/port collisions or add explicit preflight enforcement, make cross-volume external env mounting work with the documented default, and produce a fresh non-overwriteable external evidence bundle that includes exported counter files and secret screening before checksumming.
