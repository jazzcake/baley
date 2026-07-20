---
baley_record: 1
record_id: "0c8b059b-e065-4316-ae09-5f4aac61aa17"
task_id: 111
task_key: "mcp-task-create-bootstrap"
record_type: completion-report
run_id: "e589d695-ac21-40d5-9f9a-e9183977e872"
created_at: "2026-07-20T23:49:27+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# MCP task.create Bootstrap 완료보고

## Outcome

`task.create`의 relationship-aware preview/execute가 typed Baley MCP 도구로 구현되고 실제 PostgreSQL·stdio E2E와 독립 Agent 리뷰를 통과했다. 사람 승인 capability를 추가하거나 우회하지 않았으며, 다음 정식 Phase 3 Task를 Baley 자체 command로 만들 수 있는 source/runtime 준비가 끝났다.

## 구현 범위

- `baley_task_create_preview`
- `baley_task_create_execute`
- Workspace, caller-generated Task UUID, Lane/Phase, parent, predecessor/successor, title/description, terminal reason 전달
- execute envelope의 exact warning acknowledgement와 proceed reason 전달
- 기존 HTTP/application command service에 domain 판단 위임
- tool count와 schema required/optional/array item type 회귀 검증

## 검증 결과

- `go test -count=1 ./...`: 통과.
- `go vet ./...`: 통과.
- 실제 `baley_test` PostgreSQL integration: 통과.
- 실제 loopback API와 MCP stdio E2E: 통과.
  - preview가 revision, Task, Command와 Event를 쓰지 않음.
  - `phase_order_inversion` warning의 exact acknowledgement 후 execute 성공.
  - 같은 idempotency key 재시도는 같은 Command ID의 idempotent 결과.
  - 발급 public ID, parent/dependency 관계와 단일 `task.created` Event 결속 확인.
- frontend test: 13/13 통과.
- production build: 통과, 기존 large-chunk warning 유지.
- Skill validation: 통과.
- race detector: Windows `CGO_ENABLED=0`으로 실행 불가.

## 독립 리뷰

`independent-agent-review-01.md` Record `414b95bb-576c-46be-ab1e-87f2c80bf093`가 초기 Low 2건을 보고했다. `review-response-01.md`에서 preview write-free row count와 Event 상관관계 검증을 보강했다. 독립 재리뷰의 최종 finding은 High 0, Medium 0, Low 0이다.

## Lifecycle limitation

bootstrap 시작 시에는 live Workspace에 새 작업용 pending Task가 없고 기존 MCP manifest에도 `task.create`가 없었다. 이후 공식 Baley MCP stdio 도구로 preview/execute를 수행해 Task #111을 생성했고, 이 Record를 #111 구현 Run에 등록했다.

## 다음 checkpoint

1. Task #111의 current-source runtime 계약과 독립 리뷰를 완료한다.
2. 현재 source보다 오래된 elevated 8080 process는 owning launch context에서 정리한다. 새 MCP process는 user-level 18080 current-source runtime을 사용한다.
3. `task.report_implemented`까지 자동 수행한 뒤 사람 `task.confirm`만 남긴다.

## Git/운영 상태

이번 bootstrap 변경은 아직 commit/push하지 않았다. Task #111과 실제 Repository/Record index가 생성됐고, 새 Run은 current-source 18080 runtime에서 stable user-level lease secret으로 관리된다. 2026-07-19 시작 legacy 8080 binary는 종료 권한 거부로 유지됐다.
