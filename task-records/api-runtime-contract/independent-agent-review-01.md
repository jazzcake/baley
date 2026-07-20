---
baley_record: 1
record_id: "0e312f38-f289-4f30-929f-bb38ad041d2b"
task_id: 111
task_key: "api-runtime-contract"
record_type: independent-agent-review
run_id: "939f12f7-a963-4287-a823-13cf203503c3"
created_at: "2026-07-21T00:27:59+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# API Runtime Contract 독립 Agent 리뷰

## 최종 판정

- High: 0
- Medium: 0
- Low: 0
- 결론: Task #111 구현과 운영 cutover를 막는 finding이 없다. 사람 `task.confirm`은 수행하지 않았다.

## 코드 및 계약 검토

`baley_task_create_preview`와 `baley_task_create_execute`는 동일한 typed `taskCreateFields`를 사용하고 각각 `/v1/commands/preview`와 `/v1/commands/execute`로 `task.create`를 전달한다. Workspace, caller-generated Task UUID, Lane/Phase, parent, predecessor/successor, title/description과 terminal reason 경계가 application command 계약과 일치한다.

Schema 검증은 필수 필드, optional 필드, integer 관계 배열과 execute 전용 warning evidence 경계를 고정한다. Preview에는 `acknowledgedWarningCodes`와 `proceedReason`이 노출되지 않는다. Execute는 값이 있을 때만 이 두 필드를 envelope에 추가하므로 warning이 없는 명령에서 빈 optional evidence가 직렬화되지 않는다. Warning evidence는 arguments에 유입되지 않고 canonical command hash도 바꾸지 않는다.

stdio E2E는 preview 전후 Workspace revision과 `tasks`, `commands`, `events` row count 불변을 확인한다. Execute는 exact `phase_order_inversion` acknowledgement, idempotent retry, 서버 발급 public ID, parent/dependency 관계를 검증하고, Command ID·Task UUID·Workspace revision이 모두 일치하는 단일 `task.created` Event payload에서 warning code와 proceed reason을 확인한다.

## Runtime cutover 및 운영 안전성

18080 정렬은 Task #111의 effective MCP runtime으로 수용 가능하다.

- `127.0.0.1:18080`은 `C:\tmp\baley-runtime\baley-server.exe` PID 21580이 loopback으로 수신하며, binary는 현재 HEAD `057d6f70e560dc5a9c44964adaa673ae661ab42b`의 modified current-source build로 확인됐다.
- user-level `BALEY_SERVER_URL`은 `http://127.0.0.1:18080`이고 새 MCP stdio 호출이 이 runtime에서 live Workspace를 정상 조회했다.
- user-level lease secret은 존재하며 값은 출력하거나 repository에 기록하지 않았다. Runtime launcher도 사용자 환경 값을 읽고 loopback 및 명시적 live database URL만 설정한다.
- legacy 8080 PID 17232는 권한이 다른 기존 process라 종료하지 않았고, 정확한 listener 확인 뒤 보존했다. 8080과 18080 모두 같은 live Workspace revision을 반환한다.

8080이 남아 있으므로 user-level 환경을 상속하지 않거나 8080을 명시하는 별도 client는 legacy runtime과 다른 lease-secret context로 갈 수 있다. 이는 문서화된 split-routing 잔여 운영 위험이다. 새 MCP process의 18080 선택, loopback 한정, owning launch context에서의 후속 8080 정리 항목이 유지되므로 이번 Task의 구현 완료를 막지는 않는다.

## Task Record 및 문서 검토

Repository와 기존/bootstrap Record index는 Baley command로 등록됐고 front matter가 실제 Task #110/#111 및 registration 상태와 맞는다. Bootstrap handoff와 review response에 남아 있던 과거 `pending-bootstrap`/schema-reload 문구는 현재 #111·registered·18080 상태로 갱신됐다. Roadmap은 effective 18080 cutover와 legacy 8080 cleanup 미완료를 구분한다.

검토 중 발견한 untracked `server/.tmp` helper 파일 2개는 제거됐고 빈 디렉터리만 남아 Git 대상이 아니다. Secret 또는 runtime binary가 working tree에 포함되지 않는다.

## 독립 검증

- `go test -count=1 ./cmd/baley-mcp`: PASS
- `go test -count=1 ./integration`: PASS
- `git diff --check`: PASS. LF→CRLF 안내만 존재한다.
- live HTTP read: 8080과 18080 모두 동일 Workspace를 반환했다.
- current-source MCP stdio read: 18080에서 PASS.

작업자가 보고한 전체 Go test/vet, 실제 PostgreSQL MCP E2E, frontend 13/13 및 production build 결과와 현재 diff 사이에 모순을 찾지 못했다. Windows `CGO_ENABLED=0`에 따른 race detector 미실행은 알려진 환경 제한이다.

## 재리뷰 이력

초기 검토에서 stale bootstrap 문구와 untracked runtime helper를 각각 Low로 알렸다. 구현 Agent가 문구를 현재 상태로 갱신하고 helper 파일을 제거한 뒤 재검토했다. 두 항목 모두 해결됐으며 최종 finding은 High 0, Medium 0, Low 0이다.
