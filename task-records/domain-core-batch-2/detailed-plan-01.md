---
baley_record: 1
record_id: "1003142f-22fe-4b26-930d-71728f3140bc"
task_id: null
task_key: "domain-core-batch-2"
record_type: detailed-plan
run_id: null
created_at: "2026-07-17T21:53:20.5632496+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Domain Core Batch 2 상세계획

## 목표

Baley V1의 다음 순수 Go 도메인 슬라이스로 다음 세 규칙을 완성한다.

1. Task lifecycle과 blocker metadata를 분리한다.
2. 모든 dependency 변경 경로를 atomic patch 하나로 수렴시킨다.
3. Run 전체 모델을 만들기 전에 Task와 dependency에 따른 실행 가능성을 순수 정책으로 계산한다.

이번 배치는 PostgreSQL, HTTP API, MCP와 Viewer를 연결하지 않는다. 영속화 전에 규칙과 실패 원자성을 테스트로 고정하는 것이 목적이다.

## 정본

구현 중 충돌이 있으면 다음 순서로 판단한다.

1. `docs/baley-system-spec-v1.md`의 규범적 의미와 불변식
2. `contracts/v1/states.json`
3. `contracts/v1/diagnostics.json`
4. `contracts/v1/commands.json`
5. `docs/baley-command-architecture.md`

정확한 상태와 diagnostic literal을 Go 코드에서 새로 발명하지 않는다.

## 구현 전 명세 보정

다음 해석을 System Spec과 계약에 먼저 명시한다.

- `task.block`은 `pending`, `in_progress`에서만 허용한다.
- `implemented` Task를 다시 막아야 하면 `task.rework` 후 block한다.
- `confirmed`, `discarded`는 block/unblock할 수 없다.
- block과 unblock에는 비어 있지 않은 사유가 필요하다.
- blocker는 기존 Run을 중단하지 않는다.
- blocker는 새 `implementation`, `review_response` Run과 `task.report_implemented`를 차단한다.
- `dangling_path` 변화는 dependency patch preview에 표시하지만 patch 자체의 실패 조건은 아니다.
- warning acknowledgement는 domain core가 아니라 이후 command service가 처리한다.

## 목표 코드 구조

```text
server/internal/domain/
├─ diagnostic.go
├─ task.go
├─ task_test.go
├─ workspace_graph.go
├─ dependency_patch.go
├─ dependency_patch_test.go
├─ path_projection.go
├─ run_policy.go
├─ run_policy_test.go
├─ gate.go
└─ contracts_test.go
```

파일 개수 자체가 목표는 아니다. 공통 diagnostic, Task lifecycle, graph aggregate, patch 평가와 Run 시작 정책의 책임이 섞이지 않도록 분리한다.

## 1. Diagnostic 기반

현재 `dependency.go`의 단일 `Violation`과 `Diagnostic`을 공통 파일로 이동하고 preview가 여러 진단을 반환할 수 있게 한다.

```go
type Diagnostic struct {
    Code     string
    EntityID string
    Details  map[string]any
}

type Evaluation struct {
    Errors     []Diagnostic
    Warnings   []Diagnostic
    Advisories []Diagnostic
}
```

- error가 하나라도 있으면 mutation을 적용하지 않는다.
- warning과 advisory는 상태를 막지 않고 호출자에게 반환한다.
- 진단과 diff의 순서를 안정적으로 정렬한다.
- Go에서 사용하는 모든 code가 `contracts/v1/diagnostics.json`에 존재하는지 contract test로 검사한다.

## 2. Task 상태 머신

Task status를 typed string으로 만들고 다음 transition을 순수 함수 또는 복사본 반환 method로 구현한다.

```text
pending → in_progress → implemented → confirmed
pending/in_progress/implemented → discarded
implemented → in_progress
```

필수 동작:

- 첫 Run이 사용할 `Start`
- assessment가 필요한 `ReportImplemented`
- 상태 규칙만 담당하는 `Confirm`
- 사유가 필요한 `Discard`
- 사유가 필요한 `Rework`
- blocker metadata를 설정하는 `Block`
- blocker metadata를 해제하는 `Unblock`

사람 승인 진술 검증은 이번 배치에 넣지 않는다. `Confirm`과 `Discard`의 상태 전이만 제공하고, 이후 command service가 호출 전에 HumanApprovalAttestation을 검증한다.

시간이 필요한 transition은 내부에서 `time.Now()`를 호출하지 않고 `now time.Time`을 입력받아 테스트를 결정적으로 유지한다.

## 3. WorkspaceGraph aggregate

현재 `DependencyGraph`를 다음 정보가 함께 보이는 Workspace 범위 aggregate로 확장한다.

```go
type WorkspaceGraph struct {
    Tasks                map[TaskID]Task
    Dependencies         map[DependencyKey]Dependency
    GateConditionTaskIDs map[TaskID]struct{}
}
```

Gate 전체 상태 머신은 아직 구현하지 않는다. 다만 다음 검사를 위해 명시적 Gate 조건 Task index가 필요하다.

- terminal reason과 outgoing dependency의 배타성
- terminal reason과 Gate 조건의 배타성
- dependency가 Gate readiness 조건으로 자동 승격되지 않음

초기 graph 생성 시에도 기존 edge와 Task reference를 검증한다. 이미 잘못된 graph를 조용히 받아들이지 않는다.

## 4. Atomic dependency patch

```go
type DependencyPatch struct {
    Remove          []DependencyRef
    Add             []DependencyRef
    TerminalUpdates []TerminalUpdate
}
```

평가 순서:

1. 현재 WorkspaceGraph 복제
2. terminal update 적용
3. remove 적용
4. add 적용
5. 최종 graph 전체 검증
6. preview diff와 diagnostic 계산
7. error가 없을 때만 원본 교체

검사 대상:

- 존재하지 않는 Task와 edge
- self-link와 duplicate
- cross-Workspace 연결
- 최종 graph cycle
- 공백 terminal reason
- terminal reason과 outgoing dependency/Gate 조건 동시 존재
- 뒤 Phase에서 앞 Phase로 향하는 dependency의 `phase_order_inversion` warning

`dependency.connect`, `dependency.disconnect`, `task.set_terminal`, `task.clear_terminal`은 모두 한 항목짜리 patch를 만드는 convenience operation으로 구현한다. 별도 invariant 경로를 만들지 않는다.

## 5. Patch preview와 path projection

Preview diff는 최소한 다음을 포함한다.

```text
addedDependencies
removedDependencies
terminalReasonChanges
newRootTaskIds
newLeafTaskIds
becameDanglingTaskIds
resolvedDanglingTaskIds
```

`dangling_path` 조건:

```text
outgoing dependency 없음
AND 명시적 Gate 조건 연결 없음
AND terminal_reason 없음
```

Task가 새로 생성되거나 patch에서 leaf가 되었다는 이유만으로 mutation을 실패시키지 않는다.

## 6. Run 시작 정책

Run lease나 heartbeat를 만들기 전에 다음 순수 정책만 구현한다.

```go
EvaluateRunStart(task, runKind, phaseState, predecessors)
```

- `implementation`, `review_response`: blocker 또는 미해소 predecessor가 있으면 거부
- `detailed_planning`: 미래 Phase에서도 허용
- `independent_agent_review`, `completion_reporting`: blocker와 dependency로 차단하지 않음
- predecessor가 `implemented`, `confirmed`이면 해소
- predecessor가 `discarded`이면 해소되지 않음

진행 중 implementation Run에 dependency가 추가되는 상황은 Task status만으로 정확히 판단할 수 없다. `running_task_dependency_added`는 실제 Run 모델이 추가되는 후속 배치로 미룬다.

## 필수 테스트

### Task

- 허용된 transition 전부 성공
- 허용되지 않은 상태 조합 전수 거부
- terminal 상태 불변
- assessment, discard와 rework 사유 누락 거부
- 실패한 transition이 원본 Task를 변경하지 않음

### Blocker

- pending/in_progress block 성공
- implemented/confirmed/discarded block 거부
- block/unblock 사유 검증
- implementation/review-response 차단
- planning/review/reporting 허용
- unblock 후 실행 가능성 복구
- blocked Task의 구현완료 보고 거부

### Dependency patch

- 여러 remove/add 동시 성공
- 방향 전환 성공
- cycle이면 전체 rollback
- patch 일부만 적용되지 않음
- cross-lane과 cross-Phase 성공
- 역방향 Phase 성공과 warning
- cross-Workspace 거부
- 복수 predecessor/successor와 disconnected component 허용
- 같은 patch에서 terminal reason 해제 후 edge 추가 성공
- terminal reason을 유지한 채 edge 추가 거부
- Gate 조건 Task에 terminal reason 설정 거부
- preview의 root/leaf/dangling 변화 계산

### 계약

- Task status와 Run kind가 `states.json`에 존재
- 모든 diagnostic code가 `diagnostics.json`에 존재
- 새 literal을 코드에 임의로 추가하지 않음

## 비범위

- PostgreSQL과 migration
- Workspace revision과 idempotency 저장
- HTTP preview/execute endpoint
- Event 저장
- HumanApprovalAttestation 검증
- Gate pass와 Phase 전이
- Run lease/heartbeat/terminal CAS
- Task Record 등록 API와 Git metadata
- Viewer 변경

## 검증과 리뷰 순서

1. `gofmt`
2. `go test ./...`
3. `go test -race ./...`
4. `go vet ./...`
5. 계약 JSON parse와 literal contract test
6. 기존 `npm test`
7. 기존 `npm run build`
8. raw diff와 정본 문서를 입력으로 독립 Agent 리뷰
9. 리뷰 반영
10. 전체 검증 재실행
11. 완료보고 Record 작성

## 완료 조건

- 모든 dependency 변경 경로가 atomic patch 하나로 수렴한다.
- 실패한 patch와 transition이 원본 상태를 바꾸지 않는다.
- Task status와 blocker가 분리된다.
- cross-Phase dependency와 Gate 조건이 섞이지 않는다.
- 코드의 상태와 diagnostic literal이 계약 정본과 일치한다.
- 다음 command service가 domain core를 우회할 이유가 없다.

## 후속 배치

이 배치 통과 후 Gate readiness, Phase 전이와 HumanApprovalAttestation command service를 구현한다.
