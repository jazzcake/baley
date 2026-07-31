---
baley_record: 1
record_id: "76af92eb-3c5f-4c9f-9b63-63749af8b972"
task_id: 123
task_key: "adoption-pilot-kit"
record_type: completion-report
run_id: "3b78d53f-2766-4a63-a4d8-f8300e6611fd"
created_at: "2026-07-30T15:26:30.7086226Z"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #123 completion report

Adoption Pilot 운영 키트가 구현되었습니다.

- 새 `baley-adopt-project` Skill은 기존 Git 프로젝트를 안전하게 Baley에
  연결하고, 작업 디렉터리 파일 생성·병합 계획, 충돌 복구, 증거 등록,
  Gate 판단, Pilot 측정 순서를 안내합니다.
- 실행 가능한 `baley-project-init` CLI는 preview와 guarded apply를
  지원하며 같은 입력의 rerun이 모두 `keep`으로 수렴합니다.
- 로그인한 human 사용자는 UI의 Workspace 메뉴에서 새 Workspace를 만들 수
  있고 즉시 Owner가 됩니다. 서버는 Intake Phase, Adoption Lane, 공개 번호
  counters, human-required acceptance baseline을 하나의 트랜잭션으로 만듭니다.
- `pilot-measurement`는 정식 Task Record type, migration, template, strict
  validator를 갖습니다. validator는 Record identity, UTC 시간, 후보와 수용
  결과, 후보별 거절 사유, 증거 참조, secret 패턴을 검증합니다.
- 수용 스크립트는 disposable loopback database만 허용하고 effective pgx
  connection을 재검증한 뒤 migration, project-init, PostgreSQL integration,
  전체 Go test/vet, validator, frontend test/build를 실행합니다.

Validation:

- aggregate acceptance PASS on isolated `baley_test`;
- `go test ./...` and `go vet ./...`;
- PostgreSQL migration 16 and integration tests;
- project-init temporary-project apply/rerun;
- PilotMeasurement validator: 6 tests;
- Skill quick validation;
- frontend: 14 files, 57 tests;
- production TypeScript/Vite build;
- `git diff --check`;
- independent Agent re-review PASS, blocker 0, major 0.

Residual risk: an exact retry of a successfully created Workspace after HTTP
response loss returns conflict. It cannot create duplicate contents, but
idempotent response replay is deferred as hardening.
