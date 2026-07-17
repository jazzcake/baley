---
baley_record: 1
record_id: "ec559961-b842-4a52-bcdb-9e895edbb881"
task_id: null
task_key: "domain-core-batch-2"
record_type: independent-agent-review
run_id: null
created_at: "2026-07-17T22:30:00+09:00"
created_by: "codex"
registration_state: pending-bootstrap
supersedes: null
---

# Domain Core Batch 2 독립 Agent 리뷰

## 결론

**변경 요청.** Atomic dependency patch의 clone → 최종 그래프 검증 → 오류가 없을 때만 교체하는 기본 흐름은 올바르며, 실패한 patch가 원본 edge를 부분 변경하지 않는 테스트도 존재한다. 그러나 초기 그래프의 오류를 폐기하는 공개 호환 API, discarded Task에 blocker metadata가 영구 잔존하는 lifecycle 간극, 계약에 없는 Run kind를 허용하는 정책 간극이 있어 현재 상태를 승인할 수 없다.

## 검토 범위와 검증 상태

- 입력: `detailed-plan-01.md`, System Spec §8/§9/§20/§21, `states.json`, `diagnostics.json`, `commands.json`, command architecture, `server/internal/domain/*.go`
- `git diff -- server/internal/domain`: 출력 없음. 해당 디렉터리는 현재 Git 기준 전체가 untracked이므로 리뷰는 파일 원문을 기준으로 수행했다.
- 전달받은 검증 결과: `go test ./...` 통과, `go vet ./...` 통과
- `go test -race ./...`: CGO 비활성으로 미실행. 따라서 동시 호출 안전성은 검증되지 않았다.

## 발견사항

### [높음] 초기 그래프 오류가 공개 호환 생성자에서 조용히 폐기된다

- 근거: `server/internal/domain/dependency_patch.go:129-131`
- `NewDependencyGraph`는 `NewWorkspaceGraph`가 반환하는 `Evaluation`을 `_`로 버리고 항상 사용 가능한 `DependencyGraph`를 반환한다.
- `NewWorkspaceGraph`는 중복 edge, 누락 Task 참조, self-link, cross-Workspace edge, cycle, terminal conflict를 검출하지만(`workspace_graph.go:30-50`, `86-123`), 이 경로에서는 호출자가 오류를 관찰할 방법이 없다.
- 그 결과 잘못된 초기 graph가 정상 aggregate처럼 조회될 수 있고, 이후 patch는 기존 오류까지 재검증하므로 모든 정상 수정도 실패할 수 있다. 상세 계획의 “기존 edge는 Task reference를 검증하고, 이미 잘못된 graph를 조용히 받아들이지 않는다”는 요구와 맞지 않는다.
- 권고: 생성자가 `(*DependencyGraph, Evaluation)` 또는 error를 반환하도록 바꾸거나, 호환 API를 유지해야 한다면 invalid graph를 반환하지 않는 명시적 실패 경로를 둔다. 잘못된 초기 cycle/누락 참조/duplicate 각각에 대한 회귀 테스트가 필요하다.

### [중간] blocked Task를 discard하면 terminal Task에 blocker metadata가 영구 잔존한다

- 근거: `server/internal/domain/task.go:55-60`, `83-90`
- `Discard`는 pending/in_progress Task의 `BlockedAt`과 `BlockerReason`을 검사하거나 제거하지 않고 상태만 `discarded`로 바꾼다. 이후 `Unblock`은 pending/in_progress에서만 허용하므로 잔존 metadata를 정상 API로 제거할 수 없다.
- 이는 `states.json`에서 blocker를 현재 `blocked_at`/`blocker_reason` metadata로 정의하면서 terminal 상태와 분리한 모델, 그리고 confirmed/discarded에서는 block/unblock을 허용하지 않는 계획과 충돌한다. 조회·정책 코드가 `BlockedAt != nil`만으로 blocker를 판단하므로 discarded Task가 계속 blocked로 투영될 위험도 있다.
- 권고: discard 전환 시 blocker metadata를 원자적으로 제거하거나, blocked Task의 discard를 명시적으로 거부한다. 선택한 의미를 테스트로 고정하고, terminal Task에는 blocker metadata가 없다는 invariant를 aggregate 검증에 추가한다.

### [중간] 계약에 없는 Run kind가 active Phase에서 허용된다

- 근거: `server/internal/domain/run_policy.go:23-38`, `server/internal/domain/contracts_test.go:55-59`
- `EvaluateRunStart`는 `RunKind`가 `states.json`의 다섯 literal 중 하나인지 검증하지 않는다. 예를 들어 `RunKind("arbitrary")`는 active Phase에서 blocker와 dependency 검사를 모두 건너뛰고 오류 없이 허용된다.
- 현재 contract test는 코드에 선언된 `RunKinds`가 계약에 존재하는지만 확인한다. 런타임 입력이 계약 밖 literal인지, 또는 계약 literal이 코드에서 누락됐는지는 정책 함수에서 막지 못한다.
- 이는 literal authority인 `states.json`과 “정확한 상태·Run kind를 코드에서 새로 발명하지 않는다”는 계획을 위반한다.
- 권고: unknown Run kind를 hard error로 거부하는 검증을 추가한다. 기존 diagnostics 계약에 전용 code가 없으므로 현재 계약 범위에서 적절한 기존 error를 명시적으로 선택하거나 계약 변경을 선행해야 한다. 모든 계약 Run kind와 unknown kind에 대한 table test를 추가한다.

## 추가 관찰

- `WorkspaceGraph.ApplyPatch`는 candidate clone에 terminal update, remove, add를 적용한 뒤 전체 validate하고 error가 없을 때만 원본 map을 교체한다(`dependency_patch.go:40-107`). 이 경로의 patch atomicity는 적절하다.
- `dangling_path`는 warning으로만 반환되고 mutation을 막지 않으며, diff와 diagnostics는 정렬된다. 이는 계획과 일치한다.
- `GateReady`는 명시적으로 전달된 조건만 평가하고 0개 조건을 ready로 보지 않는다. cross-Phase dependency를 Gate readiness에 자동 포함하지 않는다.

## 잔여 위험

- race 검증이 실행되지 않아 같은 `WorkspaceGraph`에 대한 동시 `ApplyPatch` 안전성은 확인되지 않았다. 현재 구현에는 내부 동기화가 없으므로 command transaction/Workspace lock 계층에서 단일 writer를 보장해야 한다.
- `WorkspaceGraph`의 map 필드가 exported이므로 상위 패키지가 직접 변경하면 patch invariant를 우회할 수 있다. 후속 command service 구현에서 직접 map mutation을 금지하거나 API를 캡슐화하는 편이 안전하다.
