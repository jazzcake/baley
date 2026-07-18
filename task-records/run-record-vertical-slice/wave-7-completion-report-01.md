---
baley_record: 1
record_id: "f0da1dfe-2b9b-4898-97c9-9ffe3a8eb820"
task_id: null
task_key: "run-record-vertical-slice"
record_type: completion-report
run_id: null
created_at: "2026-07-18T15:30:07+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Wave 7 완료보고

## P4-05 — 협업 알림 후보 탐지

단위: stale Lane, 장기 blocker, ready Gate, implemented Task 결정 대기, 만료 Run lease 후보 탐지

구현 파일: `server/internal/collab/notifications.go`

테스트 파일: `server/internal/collab/notifications_test.go`

정본 근거: `docs/baley-phase2-4-prebuild-plan.md` P4-05

이 환경에서 실행한 검증: exact threshold, injected clock, stable fingerprint, 상태 해소, canonical Task/condition/run 결속, readiness 시각, Run lifecycle 불변식, deterministic ordering, package/full test·vet·race

실행하지 못한 검증: 실제 PostgreSQL query projection과 delivery channel 연결

데스크탑 명령: `cd server && BALEY_TEST_DATABASE_URL=... go test -count=1 ./integration`

통합 시 예상 연결점: Workspace snapshot query → `DetectNotifications` → read-only notification projection

잔여 위험: 조건 만족 시각과 Run/Task snapshot을 persistence query가 같은 revision에서 조립해야 한다.

## P4-06 — Audit visibility projection

단위: 사람 요청·Agent 실행·사람 승인 분리, important mutation, Workspace/Lane/Task timeline

구현 파일: `server/internal/collab/audit_visibility.go`

테스트 파일: `server/internal/collab/audit_visibility_test.go`

정본 근거: `docs/baley-phase2-4-prebuild-plan.md` P4-06와 domain mutation/Event/approval 정책

이 환경에서 실행한 검증: actor 누락 표시, command/Event/entity 결속, 다중 Task–Lane scope, approval attestation·owner·Gate snapshot, command group provenance, Event importance totality, interleaved ordering, package/full test·vet·race

실행하지 못한 검증: 실제 Event query, HTTP/API와 Viewer projection 연결

데스크탑 명령: `cd server && BALEY_TEST_DATABASE_URL=... go test -count=1 ./integration` 후 Viewer 인수 시나리오 실행

통합 시 예상 연결점: commands/events/approval attestation query → `BuildAuditTimeline` → Viewer audit panel

잔여 위험: persistence adapter가 executed command 단위의 동일 provenance와 Task–Lane scope를 누락 없이 제공해야 한다.

## Wave 판정

P4-05와 P4-06은 환경 독립 `단위 검증 완료`다. PostgreSQL·MCP·HTTP·Viewer를 포함한 `통합 검증 완료` 판정은 데스크탑 큐 이후로 유지한다.
