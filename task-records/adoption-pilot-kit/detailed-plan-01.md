---
baley_record: 1
record_id: "dd841b3f-d53d-4bfe-9bc8-03fe3cb2d5fa"
task_id: 123
task_key: "adoption-pilot-kit"
record_type: detailed-plan
run_id: "58d6d9a8-7317-4695-8659-9bd90d0fab32"
created_at: "2026-07-30T23:41:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #123 상세 구현 계획

## 목표

기존 Baley 기능을 새 단일-repository Pilot에 반복 적용할 수 있도록 프로젝트
bootstrap, 중단 복구, 증거 등록, acceptance, Gate 승인 경계를 하나의 실행 가능한
운영 키트로 묶는다.

## 구현 범위

1. repository-local `baley-adopt-project` Skill을 만들고 `baley-manage-work`의 명령·승인
   경계를 재사용한다.
2. Skill reference에 사전 점검, project bootstrap, 파일 manifest 적용, Account/Workspace
   activation, Run 복구, lane brief, Record/Git mismatch, Backlog 승격, delegated 및
   human-required acceptance, Gate 통과 절차를 기록한다.
3. 기존 `projectinit.Build`가 생성하는 manifest에 `pilot-measurement` Task Record
   template을 추가한다.
4. `pilot-measurement`를 정식 append-only Task Record type으로 추가하고 migration,
   literal contract, domain validation을 정렬한다.
5. 생성된 kit의 `baley.yaml`, retry identity, `.rgignore`, Record template, secret 부재를
   검증하는 deterministic validator script를 제공한다.
6. clean temporary project에서 manifest를 적용하고 재실행·부분 실패 복구·tamper
   rejection·measurement template 검증을 수행한다.
7. 전체 테스트와 독립 Agent 리뷰 후 completion evidence를 등록한다.

## 안전 경계

- password, Session, Agent token, approval grant, lease secret은 Git, `baley.yaml`,
  Task Record, 명령 인자에 기록하지 않는다.
- bootstrap은 고정된 `clientProjectId`와 recovery state로 재시도하며 기존 파일을
  덮어쓰지 않는다.
- lane brief 복원은 read-only다.
- Task confirmation, Gate Task pass/revoke, Gate/Lane/Workspace 결정은
  `baley-manage-work`의 사람 승인 경계를 유지한다.
- live Pilot DB를 destructive fixture로 사용하지 않는다.

## 검증

- focused domain/project-init/validator tests
- isolated PostgreSQL migration up/down and Record registration
- full `go test ./...` and `go vet ./...`
- frontend tests and production build
- Skill validators and forward test
- `git diff --check`
- independent Agent review with zero blocking findings
