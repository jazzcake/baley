---
baley_record: 1
record_id: "abdeff99-dafe-481b-85f5-8e9d0bd9b4ba"
task_id: null
task_key: "run-record-vertical-slice"
record_type: handoff
run_id: null
created_at: "2026-07-18T16:30:35+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Phase 2~4 데스크탑 Handoff

Wave 1~7의 환경 독립 구현, 자체 검증, 독립 리뷰와 findings 반영을 완료했다. 데스크탑에서는 [`docs/baley-phase2-4-desktop-handoff.md`](../../docs/baley-phase2-4-desktop-handoff.md)의 전용 `baley_test` 절차로 PostgreSQL migration/integration과 MCP E2E의 skip을 제거한 뒤 Run/Record persistence 연결을 시작한다.

최종 로컬 baseline에서 전체 Go test/vet/race, 웹 9개 test, TypeScript typecheck와 Vite build가 통과했다. Vite의 500 kB 초과 chunk warning은 Viewer 통합 시 최적화 대상으로 남긴다.

복사용 재개 프롬프트는 [`docs/baley-phase2-4-desktop-handoff-prompt.md`](../../docs/baley-phase2-4-desktop-handoff-prompt.md)에 있다.

통합 전까지 유지할 경계:

- Task Records는 `pending-bootstrap`이며 live Baley에 등록됐다고 주장하지 않는다.
- 사람 승인 mutation은 Agent가 대신 실행하지 않는다.
- adapter는 이미 검증된 domain rule을 복제하지 않는다.
- PostgreSQL integration test는 개발 DB가 아닌 폐기 가능한 `baley_test`에만 실행한다.
