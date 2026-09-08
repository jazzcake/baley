---
baley_record: 1
record_id: "c6dff405-f4ad-41e4-a3a2-f5c11ab04804"
task_id: 179
run_id: "242bf463-27d7-45cf-8b13-1d53ead93d0a"
task_key: "bird-view-v2"
record_type: detailed-plan
created_at: "2026-09-06T11:51:02Z"
created_by: "codex"
registration_state: registered
---

# Task #179 detailed plan

## Objective

Replace phase-wide and dependency-closure matching with five explicit, evidence-derived outcome subgraphs. Keep the existing Bird View Nodes and same-canvas interaction while making the focus API and rendered React Flow show the same curated Task and Gate set.

Task #179 is an intentional independent improvement root. Task #178 is implemented but unconfirmed, so Baley V1 would block an implementation Run behind it; #178 currently also carries a `dangling_path` warning because the available MCP did not expose `task.set_terminal`.

## Evidence reviewed before implementation

The curation used all 60 copied Task titles, descriptions, current summaries, statuses, Phases, Lanes, all 52 dependency edges, all three Gates and 14 Gate conditions, plus Run and Task Record history. The source and operating database remained read-only; analysis used the isolated clone.

Current focus expansion is demonstrably over-broad:

| Node | Explicit bindings before | Focus API Tasks before | API Gates before | API dependencies before | Rendered Task rule before |
| --- | --- | --- | --- | ---: | --- |
| Intake and canonical foundations | Intake Phase, G#1 | #1-11, #13-60 except #12 and #22 (58 Tasks) | G#1-G#3 | 52 | no Task cards because none were explicitly Task-bound |
| Foundation Proof delivery | Foundation Proof Phase, G#2 | #2-60 (59 Tasks) | G#1-G#3 | 52 | no Task cards because none were explicitly Task-bound |
| PlaceMatch transition | #43-#52 | #43-#52 | G#1 inferred from an automatic entry | 9 | #43-#52 plus the unrelated inferred G#1 |
| Jeju Map v1 | Jeju Map v1 Phase, G#3 | 41 Tasks across all four Phases | G#1-G#3 | 38 | no Task cards because none were explicitly Task-bound |
| Curation and plan prototype | Curation Phase | 29 Tasks | G#1-G#3 | 29 | no Task cards because none were explicitly Task-bound |

The first divergence is server projection: Phase bindings add every Phase Task, Gate bindings add every condition and entry, and an undirected closure then adds every connected Task. A second divergence in the client infers Gates from selected condition or automatic-entry Tasks even when the Gate is not bound.

## Curated contract

| Node | Exact Tasks | Gates | Rationale |
| --- | --- | --- | --- |
| Intake and canonical foundations | #18 RAG raw resolver asset audit; #21 road/lot address API suitability audit; #16 MFDS/SBIZ RawPlaceAPI; #19 raw quality/freshness baseline; #17 independent POI identity-match contract; #13 canonical place identity Gate | G#1 | Two input audits converge on the RawPlace boundary, then quality, identity decision, and the canonical Gate form one readable foundation flow. |
| Foundation Proof delivery | #13 canonical place identity Gate; #2 data evidence baseline; #15 Intake Lens implementation; #3 web execution baseline; #7 AI-thumbnail quality contract; #8 missing-image thumbnail generation; #9 thumbnail quality verification; #39 OG feasibility proof; #40 OG intake core; #41 OG backfill; #42 new-enrichment OG integration; #23 FeedGrid F0 contract; #29 FeedGrid F1 lifecycle harness; #31 FeedGrid corrective proof | G#2 | Data, web, image evidence, and FeedGrid each retain their start-to-proof path; G#2 is the actual delivery boundary. |
| PlaceMatch transition | #43 scope/ownership; #44 rag-match analysis; #45 resolve-path analysis; #46 reuse/adapt/new/retire mapping; #47 contract/evaluation; #48 local infrastructure; #49 core/PostgreSQL adapter; #50 Qdrant adapter; #51 shadow/cutover; #52 long-term Qdrant decision | none | The ten Tasks already form the exact nine-edge transition chain from decision through implementation, integration, and final operating decision. |
| Jeju Map v1 | #2 data proof input; #3 web proof input; #4 data release input; #5 map experience; #9 thumbnail proof input; #10 real-photo precedence; #11 thumbnail display; #12 duplicate merge/prevention; #14 Lens menu | G#3 | The two release baselines feed the map result; the verified thumbnail chain, dedupe result, and Lens result are the concrete review conditions. |
| Curation and plan prototype | #5 Jeju map experience; #6 curation/plan prototype | G#3 | The validated map is the necessary connector into the prototype result; G#3 is the truthful transition boundary. |

Intentional overlaps are #13 (Intake result and Foundation data input), #2/#3/#9 (Foundation proofs reused by Jeju Map), #5 (Jeju Map result and prototype input), and G#3 (Jeju review and prototype transition). Every overlap represents a real cross-outcome handoff.

The 24 intentionally unbound Tasks are #1, #20, #22, #24-#28, #30, #32-#38, and #53-#60. They are administration, optional/downstream canonical extensions, superseded image experiments, branding, or independent YouTube/external-job/reference streams; including them would turn an outcome focus into a Phase or Workspace dump.

## Implementation and verification plan

1. Make explicit Task/Gate bindings authoritative in the server focus projection; use Phase bindings only to retain relevant Phase context and return only relevant Phase/Lane rows.
2. Make the React projection render only explicitly bound Gates and Tasks, retaining only dependency and Gate edges whose endpoints are rendered.
3. Encode the exact five-node contract in the repeatable seed and assert exact Tasks, Gates, dependencies, non-empty focus, overlap allowlist, and total coverage on every verification.
4. Add focused server/client regression tests, reseed only `baley_daytripper_birdview_test`, authenticate, verify all five APIs and Tailnet-rendered views, and run the full requested test/build/audit/isolation suite.
