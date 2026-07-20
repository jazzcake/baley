---
baley_record: 1
record_id: "506c5c62-d230-4a46-af89-ff3029cb63e7"
task_id: 111
task_key: "mcp-task-create-bootstrap"
record_type: review-response
run_id: "e589d695-ac21-40d5-9f9a-e9183977e872"
created_at: "2026-07-20T23:49:27+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# MCP task.create Bootstrap 리뷰 반영

## 결론

독립 Agent가 보고한 비차단 Low 2건을 모두 테스트로 보강했다. 재리뷰 결과 High/Medium/Low finding은 모두 0건이다.

## 반영 내용

1. MCP stdio E2E의 preview write-free 검증을 Workspace revision뿐 아니라 `tasks`, `commands`, `events` row count 불변까지 확장했다.
2. `task.created` Event를 execute 결과의 Command ID, 새 Task UUID와 Workspace revision으로 선택한 뒤 동일 Event payload에서 `acknowledgedWarningCodes`와 `proceedReason`을 구조적으로 검증한다.

## 재검증

- Go 전체 test/vet 통과.
- 분리된 실제 PostgreSQL integration 통과.
- 실제 MCP stdio preview/execute/idempotent retry/query/Event E2E 통과.
- frontend 13/13 및 production build 통과.
- Skill validation 통과.

## 잔여 운영 경계

- bootstrap 시작 시에는 live Baley에 합법적인 새 Task/Run target이 없었다. 이후 공식 MCP stdio preview/execute로 Task #111을 생성하고 이 Record를 등록했다.
- root thread의 manifest는 stale 상태지만 공식 repository MCP stdio 도구와 current-source 18080 runtime으로 계약을 검증했다.
- 권한이 다른 legacy 8080 API는 유지한다. 새 MCP process는 user-level `BALEY_SERVER_URL`을 통해 18080 current-source runtime을 사용한다.
- Windows `CGO_ENABLED=0`으로 race detector는 실행하지 못했다.
