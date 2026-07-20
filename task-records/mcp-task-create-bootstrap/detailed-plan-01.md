---
baley_record: 1
record_id: "73bbf5d5-69b8-4d91-9805-6205272dca85"
task_id: 111
task_key: "mcp-task-create-bootstrap"
record_type: detailed-plan
run_id: "e589d695-ac21-40d5-9f9a-e9183977e872"
created_at: "2026-07-20T02:19:43+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# MCP task.create Bootstrap 상세계획

## 목표

Baley의 정본 `task.create` command를 Codex가 typed MCP preview/execute 도구로 사용할 수 있게 연결한다. 이후 로드맵 작업을 실제 Baley Task로 생성하고, 계획·구현·독립 리뷰·완료보고를 Run/Record lifecycle에 연결할 수 있어야 한다.

## 현재 제약

- 라이브 Workspace의 기존 Task는 모두 `confirmed`이고 새 작업을 위한 합법적인 Run target이 없다.
- application/HTTP는 `task.create`를 지원하지만 현재 MCP manifest에는 도구가 없다.
- 이 bootstrap 자체는 Baley Task/Run에 연결할 수 없으므로 lifecycle limitation을 명시한다. HTTP·DB 직접 mutation으로 우회하지 않는다.
- 이 thread의 tool manifest는 시작 시 고정되므로 구현 후 새 Codex host/thread에서 schema reload가 필요할 수 있다.

## 구현 범위

1. `baley_task_create_preview`와 `baley_task_create_execute` typed input을 추가한다.
2. arguments는 Workspace/Lane/Phase, caller-generated Task UUID, parent, predecessor/successor 목록, title/description과 terminal reason을 전달한다.
3. execute envelope는 expected revision, idempotency, executing Actor, exact warning acknowledgement와 proceed reason을 전달한다.
4. MCP adapter는 domain rule을 복제하지 않고 HTTP command envelope만 조립한다.
5. stdio E2E에서 schema, write-free preview, execute, 발급된 public Task ID, revision 증가와 query projection을 검증한다.
6. 기존 승인 도구와 Run/Record 흐름을 회귀 검증한다.

## 검증

- `gofmt`, `go test -count=1 ./...`, `go vet ./...`
- 실제 PostgreSQL integration
- 별도 loopback API를 사용한 MCP stdio E2E
- frontend test/build 및 Skill validation
- 독립 Agent raw-diff 리뷰

## 완료 경계

코드·테스트·독립 리뷰·리뷰 반영·완료보고까지 자율 수행한다. 현재 thread에서 새 tool schema를 사용할 수 없으면 handoff checkpoint를 남기고, 새 thread에서 로드맵 Task 생성부터 재개한다. 사람 전용 confirm은 실행하지 않고 최종 Task를 `implemented` 상태에 둔다.
