---
type: runbook
status: active
scope: local-day-tripper-onboarding-preflight
target_task: "#125"
---

# Day Tripper 온보딩 직전 Preflight

이 문서는 Day Tripper 데이터를 Baley에 등록하기 전에 필요한 최소 준비를 다룬다.

## 필수 조건

1. `scripts/local-pilot-runtime.ps1`가 현재 Git commit의 API와 Viewer를 실행한다.
2. PostgreSQL은 `127.0.0.1:54329`에만 공개되고 ready 상태다.
3. 최근 logical backup의 별도 DB 복원 검증이 성공했다.
4. Baley MCP는 전역 `baley` 등록 하나만 사용한다.
5. fresh read에서 #124는 `confirmed`, G#4는 `passed`, active Phase는
   `embedding-pilot`이다.
6. Owner가 로그인하고 Baley Pilot graph 표시를 확인한다.
7. 작업에 사용할 commit은 전체 Go/Viewer acceptance를 통과한다.

## 준비 명령

```powershell
cd D:\Project_AI\baley
.\scripts\local-pilot-runtime.ps1 start

$backup = .\scripts\local-pilot-db.ps1 backup
.\scripts\local-pilot-db.ps1 verify $backup

.\scripts\test-local-pilot-preflight.ps1
```

Workspace별 `.env` 생성, Agent token 복사, 별도 `codex mcp add`는 하지 않는다.
프로젝트 LLM에는 Viewer의 Workspace URL만 전달한다. 최초 typed MCP read가
`workspace_connection_required`를 반환하면 Owner가 그 approval URL에서
`Operator 연결 승인`을 한 번 누르고, LLM은 같은 read를 재시도한다.

승인 후 Workspace-scoped credential은 Git-ignored
`.tmp/baley-mcp/credentials.json`에 보관되며 다음 호출부터 자동 선택된다. 이 권한은
Operator 전용이고 Workspace 관리, Task 확인, Gate 통과 같은 사람 전용 행위는 할 수 없다.

## 확인 항목

- #124: `confirmed`
- G#4: `passed`
- active Phase: `embedding-pilot`
- Owner 계정: Workspace 관리와 MCP 연결 승인 가능
- MCP Agent: 대상 Workspace read/operate 가능, 다른 Workspace와 관리자 API 접근 불가
- runtime: API `127.0.0.1:8080`, Viewer `127.0.0.1:5174`

모든 항목을 통과한 뒤에만 #125 Run과 실제 Day Tripper 구조 등록을 시작한다.
