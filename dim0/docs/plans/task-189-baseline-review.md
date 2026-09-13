# Task #189 baseline independent review

Date: 2026-09-14 (Asia/Seoul)

Reviewed commits:

- Planning: `bf6563fc249a8cc5eacafe0c0ce524f1b97a0706`
- Baseline result: `47268221157913cfa5c1b470771eb78787a94d9f`
- Normative specification: [`codex-runtime-and-persistence-spec.md`](./codex-runtime-and-persistence-spec.md)

Reviewer role: independent runtime-isolation, persistence, and Docker-evidence reviewer.

## Verdict

**Approve the reported safe-stop and blocked classification; do not accept the application baseline.** The result is appropriately limited to an infrastructure-only persistence probe. It does not satisfy the specification's upstream application baseline or Task #189's mandatory provider-free acceptance contract.

No Critical or High-severity defect was found in the safety decision, the scope of the persistence claim, or cleanup targeting. One Medium-severity evidence limitation remains: commit `4726822` contains only the narrative result and no immutable command output, exit-code bundle, logs, browser trace, tripwire counters, or manifest, so an independent reviewer cannot authenticate the historical Docker outputs from Git alone. That limitation is already consistent with the result's **not accepted** verdict and must be removed on rerun.

## Findings

| ID | Severity | Finding | Review conclusion |
| --- | --- | --- | --- |
| R1 | None / pass | External generative and embedding calls | No Dim0 application process capable of constructing an LLM or embedding client was started. The only started services were the Compose test-profile PostgreSQL, Qdrant, and Redis services; `backend-test` and `webui-test` were deliberately not built or started. Therefore no generative or embedding provider call was possible through an executed Dim0 path. The config string scan alone would not prove this, and the result correctly refuses to treat it as provider-counter evidence. |
| R2 | None / pass | Developer credentials and provider routes | Base Compose would mount `dim0/.env` into `backend-test`, while the required overlay and external `baseline.env` were absent. Stopping before application startup was the correct fail-closed boundary. The report states that the existing `.env` was neither read nor mounted into a running container. |
| R3 | None / pass | Restart persistence | The reported pre-restart seeds and post-restart reads support volume persistence for PostgreSQL (`1:persisted`), Qdrant (green collection, 4-dimensional vector configuration), and Redis (`persisted`, sequence `1`). The result correctly limits this to container-volume durability: it does not claim 512-dimensional embedding behavior, canonical `GraphStore -> ContentStore` writes, board/note/link recovery, schema idempotence, or application-level restart persistence. |
| R4 | Medium | Historical evidence provenance | The evidence commit adds only `task-189-baseline-result.md`. External raw evidence was not created because the prerequisite gate failed, and the attempt volumes were later deleted. Current read-only Docker inspection can confirm that no `dim0-task189` container, volume, or network remains, but cannot replay or authenticate the historical command outputs. This is acceptable only for a rejected diagnostic run, never for baseline acceptance. |
| R5 | None / pass | Cleanup scope | Cleanup used Compose project `dim0-task189`, first enumerated exact resources by `com.docker.compose.project=dim0-task189`, and removed the three attempt containers, three attempt volumes, and project network. Current label-filtered Docker inspection is empty while unrelated Baley, local-dev, and Daytripper containers remain running. The use of `down --volumes` differs from the execution plan's default evidence-preserving cleanup, but the report records the deviation and its coordinator authorization; the deletion target itself was narrowly scoped. |
| R6 | None / pass | Blocked acceptance assignment | The block is correctly assigned to the absent `docker-compose.baseline.yml`, backend provider tripwire, backend live-storage/fake-embedding test, and frontend provider-construction test. Without them, starting the application would violate the plan's explicit rule that absence of an observed call is not proof that no call was possible. |
| R7 | None / pass | Specification alignment | The planning commit preserves the specification's Qdrant/PostgreSQL/Redis topology and separates provider-free mandatory checks from opt-in real embedding checks. The stopped run leaves specification acceptance criteria 1 and 3 through 7 unproven; the result marks the corresponding Task #189 criteria blocked or partial rather than overstating success. |

## Evidence checked

- The commit range `bf6563f..4726822` changes only `dim0/docs/plans/task-189-baseline-result.md`.
- At planning commit `bf6563f`, all four mandatory harness paths are absent.
- Base Compose defines `postgres-test`, `qdrant-test`, and `redis-test` with named persistent volumes; `backend-test` depends on them and mounts `../${ENVFILE:-.env}` read-only.
- The checked `schema.sql` contains no provider endpoint/client, shell program, HTTP invocation, or external-data extension path.
- Installed image metadata matches the reported service families (`postgres:15`, `qdrant/qdrant:latest`, and `redis:7-alpine`).
- Read-only Docker label queries currently return no resources for project `dim0-task189`; unrelated running containers remain present.
- The untracked repository-root `debug.log` was not opened, changed, staged, or committed during this review.

## Exact prerequisites for rerun

Rerun from Stage A only after all of the following are true:

1. Add the four test-only harness assets required by the execution contract, without changing product runtime behavior:
   - `dim0/build/docker-compose.baseline.yml`
   - `dim0/backend/test/integration/baseline/provider_tripwire.py`
   - `dim0/backend/test/integration/baseline/test_provider_free_baseline.py`
   - `dim0/webui/src/features/agent/engine/__tests__/provider-free-baseline.test.ts`
2. Make the tripwire intercept LLM, embedding, search, fetch, OCR, image, and Daytona network boundaries, record construction separately from invocation, and fail on every outbound invocation. The fake embedder must deterministically return 512-dimensional vectors.
3. Prepare a fresh external, timestamped evidence directory and a non-secret external `baseline.env`; set `ENVFILE` so Compose mounts that file, never `dim0/.env`. Record the actual Baley commit SHA, upstream subtree provenance, dirty state, Docker versions, commands, exit codes, bounded logs, and SHA-256 manifest.
4. Ensure the run is still a pre-cutover baseline: harness-only changes may be present, but no Codex runtime or provider-policy implementation may be used to make the baseline pass.
5. Confirm that the `dim0-task189` project resources and required test ports/container names are free, then use only project name `dim0-task189`.
6. Run the complete Stage A-F sequence: backend checks, Web UI checks/tests/production build, two-file Compose expansion and local image builds, persistence health, schema application twice, canonical board/note/link storage checks, backend/Web UI smoke, browser console/network capture, restart, and identity-based retrieval of the pre-restart records.
7. Require zero invocation counters at every provider boundary. Also prove that text create/update invokes only the fake embedder, spatial/style-only update invokes neither embedding nor vector update, and forced fake-embedding failure prevents the storage mutation without a zero-vector fallback.
8. Prove post-restart PostgreSQL metadata, Qdrant 512-dimensional content/vector payloads, Redis sequence state, and the same board/note/link identities. Infrastructure probe rows or an arbitrary 4-dimensional Qdrant collection are not substitutes.
9. Preserve all raw artifacts outside Git and list each Task #189 acceptance criterion as pass/fail with artifact paths. Optional live embedding or external-service checks cannot compensate for a mandatory failure.
10. Follow the default cleanup in the execution contract: capture evidence first, run project-scoped `down --remove-orphans` without deleting named volumes, and treat any later `dim0-task189` volume deletion as a separate explicit cleanup decision.

Until these prerequisites and the full rerun succeed, Task #189 should remain blocked on the missing overlay/tripwire/tests, with the current infrastructure persistence observations retained only as diagnostic evidence.
