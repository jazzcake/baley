---
baley_record: 1
record_id: "976557a6-0f6a-4868-84af-0bf28eea226e"
task_id: 121
task_key: "mutation-attempt-audit"
record_type: review-response
run_id: "eaaba35e-6c08-4e3a-bc80-82ea67eaba41"
created_at: "2026-07-26T23:31:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #121 리뷰 반영

- 기존 DB upgrade를 위한 00011과 현재 00009 baseline의 rollback 소유권을 정합화했다.
- 두 idempotent replay 경로에서 저장된 `command_hash`를 복원한다.
- 1 MiB 초과 요청도 bounded raw payload의 digest만 사용해 rejected attempt로 기록한다.
- TRUNCATE, tuple cursor와 transport rejection을 격리 PostgreSQL 통합 테스트로 검증했다.
