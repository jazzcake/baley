---
baley_record: 1
record_id: "414b95bb-576c-46be-ab1e-87f2c80bf093"
task_id: 111
task_key: "mcp-task-create-bootstrap"
record_type: independent-agent-review
run_id: "e589d695-ac21-40d5-9f9a-e9183977e872"
created_at: "2026-07-20T02:31:13+09:00"
created_by: "independent-review-agent"
registration_state: registered
supersedes: null
---

# MCP Task Create Bootstrap 독립 Agent 리뷰

## Findings

### Low — preview write-free E2E가 Workspace revision만 확인한다

`server/integration/mcp_test.go:249-259`는 preview 전후 Workspace revision이 같다는 사실만 확인한다. 따라서 preview가 잘못 Task, Event 또는 Command row를 기록하면서 revision을 올리지 않는 회귀가 생겨도 이 검증은 통과할 수 있다. 계획에서 요구한 write-free 보장을 직접 고정하려면 preview 직후 새 public ID/UUID Task가 조회되지 않는지와 Event 또는 Command 수가 증가하지 않는지를 함께 확인하는 편이 안전하다.

영향은 현재 구현 결함이 아니라 회귀 검출 범위의 공백이다. MCP handler는 `/v1/commands/preview`만 호출하고 현재 command service의 preview 경로는 쓰지 않으므로 즉시 차단할 문제는 아니다.

### Low — Event evidence E2E가 동일 Event와의 상관관계를 검증하지 않는다

`server/integration/mcp_test.go:302-306`는 전체 Event 목록 JSON에 `task.created`, acknowledgement code, proceed reason 문자열이 각각 존재하는지만 확인한다. 세 값이 동일한 `task.created` Event, 동일한 새 Task UUID와 실행 Command에 결속됐는지는 확인하지 않으므로 서로 다른 Event가 문자열 조건을 만족하는 거짓 양성이 가능하다.

Event 목록을 구조체로 decode한 뒤 `eventType == task.created`, 새 Task UUID/entity ID 및 execute 결과의 Command ID를 기준으로 한 Event 하나에서 `acknowledgedWarningCodes`와 `proceedReason`을 함께 검증하는 보강을 권장한다.

## 판정

- High: 없음
- Medium: 없음
- Low: 2건
- 결론: 구현을 막는 finding은 없다. 위 두 건은 테스트 증거의 정밀도를 높이는 보강 사항이다.

## 계약 및 구현 검토

### MCP schema

- `workspaceId`, `taskUuid`, `laneId`, `phaseId`, `title`, `expectedWorkspaceRevision`, `idempotencyKey`, `executedByActorId`가 preview/execute schema에서 required로 고정된다.
- `parentTaskId`, `description`, `predecessorTaskIds`, `successorTaskIds`, `terminalReason`은 optional이다.
- 관계 배열의 `items.type`은 `integer`로 검증된다.
- `acknowledgedWarningCodes`와 `proceedReason`은 execute schema에만 있고 preview schema에는 없다.
- `taskUuid`는 client-generated 내부 UUID이고 관계 및 조회에 쓰는 `taskId`는 서버 발급 public integer ID라는 경계가 구현과 E2E에서 구분된다.

### preview/execute 전달 경계

- 두 handler는 동일한 `taskCreateArguments` helper를 사용하므로 command name과 arguments shape가 일치한다.
- preview는 `/v1/commands/preview`, execute는 `/v1/commands/execute`만 호출한다.
- warning acknowledgement와 proceed reason은 execute envelope에만 추가되며 `task.create` arguments로 유입되지 않는다.
- MCP adapter에 별도 graph 규칙이나 승인 판단을 복제하지 않고 기존 HTTP command service에 위임한다.

### hash, idempotency 및 증거

- PostgreSQL integration test는 acknowledgement/proceed reason 유무가 `task.create` canonical command hash를 바꾸지 않음을 확인한다.
- stdio E2E는 같은 idempotency key 재호출이 같은 Command ID와 `idempotent=true`를 반환함을 확인한다.
- warning-bearing stdio E2E는 `phase_order_inversion`의 exact acknowledgement와 proceed reason을 execute envelope로 보내고, Task 생성, public ID 조회, 양방향 관계 결과와 Event evidence를 확인한다.
- `task.create`는 literal contract상 `workspace:operate`, `humanApproval: none`이다. execute schema에 human approval attestation 필드를 노출하지 않으며, 관계 생성도 Gate 조건 추가나 Task confirmation을 우회하지 않는다.

### 기존 도구 회귀

- tool 목록 기대치를 29개에서 31개로 갱신하고 기존 도구 전체 집합을 계속 확인한다.
- 기존 `task.confirm` warning acknowledgement E2E와 execute 결과 검증을 유지한다.
- 새 handler는 기존 envelope/helper 또는 기존 tool 이름을 변경하지 않는다.

## 검증 증거

작업자가 제시한 증거:

- Go 전체 test 및 vet: PASS
- 실제 PostgreSQL integration: PASS
- 실제 MCP stdio E2E: PASS
- frontend test 13/13 및 production build: PASS
- Skill validation: valid
- race detector: Windows `CGO_ENABLED=0` 환경 제한으로 실행 불가

독립 리뷰 중 재확인:

- `go test ./cmd/baley-mcp`: PASS
- `git diff --check`: 오류 없음. Git의 LF→CRLF 안내만 존재

## Lifecycle 및 운영 제한

- 이 bootstrap은 아직 live Baley Task가 없고 MCP에 `task.create`가 없어서 정식 Run/Record 등록을 선행할 수 없다. 따라서 Record의 `task_id`, `run_id`가 null이고 `registration_state: pending-bootstrap`인 처리는 계획과 일치한다.
- 현재 thread가 이미 로드한 MCP manifest에는 새 tool/schema가 자동 반영되지 않는다. source 반영 뒤 MCP schema reload가 되는 새 thread 또는 host 재시작이 필요하다.
- 기본 포트 8080에는 2026-07-19 11:55:50에 시작된 `baley-server` 프로세스가 listening 중이다. 이 raw diff보다 먼저 시작된 live binary가 현재 source/runtime 계약과 일치한다고 간주할 수 없으므로, 실제 roadmap Task 운용 전에 현재 source로 재시작하고 계약을 재검증해야 한다.
- race detector 미실행과 live 8080/source 불일치는 잔여 운영 위험이며, 이번 MCP adapter raw diff의 승인/권한 결함으로 판정하지 않는다.

## 재리뷰

2026-07-20 최신 `server/integration/mcp_test.go` diff와 분리 PostgreSQL 기반 stdio E2E 재실행 증거를 검토했다.

- 첫 번째 Low finding은 해결됐다. preview 전후 Workspace revision에 더해 동일 테스트 PostgreSQL의 `tasks`, `commands`, `events` Workspace별 row count를 직접 비교하므로, revision을 올리지 않는 비정상 기록도 검출한다.
- 두 번째 Low finding은 해결됐다. execute 결과의 `CommandID`, 새 `taskUUID`, 결과 `WorkspaceRevision`과 모두 일치하는 단일 `task.created` Event를 선택한 뒤, 그 Event payload의 `acknowledgedWarningCodes`와 `proceedReason`을 구조적으로 검증한다.
- 보강된 실제 분리 DB MCP stdio E2E는 PASS했다.
- 독립 재검토에서 `go test ./integration -run TestMCPStdioListsAndCallsTools -count=1`로 integration package compile/skip 경로가 PASS했고 `git diff --check -- server/integration/mcp_test.go`에 오류가 없음을 확인했다.

재리뷰 결론: 기존 Low 2건은 모두 해결됐고 새 finding은 없다. 최종 finding 현황은 High 0, Medium 0, Low 0이다. schema reload와 live 8080 source/runtime 일치 검증은 기존 운영 제한으로 남으며 코드 finding은 아니다.
