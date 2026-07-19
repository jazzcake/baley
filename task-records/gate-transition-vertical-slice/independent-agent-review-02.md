---
baley_record: 1
record_id: "5322d396-4da0-4e12-a851-33c1b20df6b0"
task_id: null
task_key: "gate-transition-vertical-slice"
record_type: independent-agent-review
run_id: null
created_at: "2026-07-20T00:07:26+09:00"
created_by: "independent-review-agent"
registration_state: pending-bootstrap
supersedes: "19d370c7-03fd-4a6e-8ab6-f0772dead047"
---

# Gate Transition Vertical Slice 독립 Agent 리뷰 02

## Findings

### High

없음.

### Medium

없음.

### Low

1. **MCP stdio E2E는 새 execute 필드의 schema 노출만 확인하고 `baley_task_confirm_execute` 호출 자체는 수행하지 않는다.**
   - `server/integration/mcp_test.go:56-63`은 실제 stdio session에서 `acknowledgedWarningCodes`와 `proceedReason` schema를 확인한다.
   - `server/cmd/baley-mcp/main_test.go:11-47`은 handler를 직접 호출해 두 필드가 HTTP envelope로 전달되는지 확인한다.
   - 두 테스트와 tool 등록 코드 검토를 합치면 현재 구현은 정상임을 충분히 확인할 수 있으나, MCP SDK의 실제 argument decode부터 HTTP forwarding까지 하나의 E2E 호출로 묶은 회귀 테스트는 없다. 향후 SDK binding 또는 tool wiring 변경을 한 테스트가 끝까지 잡지는 못하는 비차단 test-depth 공백이다.

2. **warning 실패 원자성 테스트가 Task 상태와 approval row의 불변을 직접 assert하지 않는다.**
   - `server/integration/task_confirm_warning_test.go:46-59`는 실패 후 Workspace revision, command 수, Event 수가 그대로임을 확인한다.
   - 요구된 “no mutation”을 더 직접 고정하려면 실패 직후 Task #110이 계속 `implemented`인지와 `human_approval_attestations` 수가 0인지도 assert하는 편이 낫다.
   - 현재 소스에서는 warning mismatch가 repository mutation 전에 오류로 종료되고, 모든 write가 단일 transaction 안에 있으므로 실제 원자성 결함은 발견되지 않았다. 이 항목도 비차단 test-depth 개선 사항이다.

## 핵심 정합성 검토 결과

- **사람 승인 우회/약화 없음:** `task.confirm`은 여전히 `task:approve`와 human approval을 요구한다. execute는 승인 actor가 human actor인지, approved command hash가 현재 canonical hash와 일치하는지, Gate 계열은 decision snapshot hash까지 일치하는지 재검증한다. MCP의 모든 human-approved execute consumer(`task.confirm`, `gate.pass`, `gate.pass_task`, `gate.revoke_task_pass`)는 동일한 `executeEnvelope`와 `executeEnv`를 사용한다.
- **warning exact-match:** `sameDiagnosticCodes`는 실제 warning code와 acknowledgement를 정렬한 뒤 길이와 각 원소를 비교한다. 누락, 미지 code, 다른 code, 중복 수 차이는 모두 실패한다. `task.confirm`의 no-ack 및 mismatch 실패 후 같은 idempotency key에 exact `dangling_path` acknowledgement를 넣은 재시도가 성공하는 실제 PostgreSQL 테스트를 확인했다.
- **canonical hash 유지:** `hashCommand`와 `hashRequestFingerprint` 입력은 contract version, command name, typed arguments, expected revision 및 필요한 decision snapshot뿐이다. `acknowledgedWarningCodes`, `proceedReason`, actor, idempotency key, attestation은 canonical command hash에서 제외된다. 회귀 테스트도 acknowledgement/reason 추가 전후 hash 동일성을 확인한다.
- **Event evidence:** warning-bearing `task.confirmed` Event에는 평가/acknowledgement code와 trim된 `proceedReason`이 기록된다. 별도의 `human_approval_attestation.recorded` Event 및 row에는 action/entity/hash/revision/approver 결속이 유지된다.
- **rollback/revision/Event 원자성:** PostgreSQL repository는 Workspace row lock 이후 execute evaluation을 수행한다. warning acknowledgement 실패는 mutation switch 전에 반환된다. 성공 시 Task mutation, Workspace revision, command row, approval row, domain Event 및 approval Event가 같은 transaction에서 commit된다.
- **HTTP/application/MCP envelope 일치:** `contracts/v1/commands.json`, `application.CommandEnvelope`, HTTP JSON decode, MCP `executeEnvelope` 및 `executeEnv`가 `acknowledgedWarningCodes`와 `proceedReason`을 일관되게 운반한다.
- **dirty tree 보존:** raw status/diff를 검사했으며 reset, mass-format, commit, push는 수행하지 않았다. 이번 리뷰가 만든 파일 외 기존 변경을 수정하지 않았다. 작업 트리는 여러 선행 slice의 대규모 tracked/untracked 변경을 포함하므로, commit 기준만으로 개별 변경의 작성 주체를 역추적할 수 없다는 잔여 attribution 한계는 있다. `git diff --check`에서 whitespace error는 없고 CRLF 전환 예고만 확인됐다.

## 독립 검증 증거

- `go test -count=1 ./...`: 통과.
- `go vet ./...`: 통과.
- 실제 PostgreSQL integration: 기존 live Baley DB와 분리한 임시 PostgreSQL DB에 migration 00001~00006을 적용하고 `go test -v -count=1 ./integration` 실행. MCP 환경변수형 1건만 의도적으로 skip되고 나머지 PostgreSQL integration 전부 통과. 특히 `TestTaskConfirmWarningAcknowledgementIsAtomicAndRetryable` 통과.
- MCP stdio E2E: 위 임시 DB와 별도 loopback API에서 `BALEY_MCP_E2E=1 go test -v -count=1 ./integration -run '^TestMCPStdioListsAndCallsTools$'` 통과. 임시 API process, executable 및 database는 종료/삭제했다.
- Frontend: `npm test -- --reporter=dot` 13/13 통과, `npm run build` 통과. 기존 500 kB 초과 chunk warning만 남았다.
- Race: `GOOS=windows`, `GOARCH=amd64`, `CGO_ENABLED=0`이므로 `go test -race ./...` 실행 불가.

## 결론

차단 finding은 없다. MCP acknowledgement/proceed-reason adapter 수정은 기존 사람 승인 결속을 약화하지 않고 HTTP/application 계약과 일치하며, exact warning acknowledgement, same-idempotency retry, canonical hash 제외 및 transaction 원자성 요구를 충족한다. 위 Low 2건은 close-out을 막지 않는 회귀 테스트 깊이 개선 사항이다.
