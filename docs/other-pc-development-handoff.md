# 다른 PC에서 Baley 개발 이어가기

이 문서는 다른 PC에서 저장소를 받은 뒤 Baley 개발 환경을 복원하고, 새 Codex
세션이 안전하게 작업을 이어가도록 하기 위한 실행 안내와 복사·붙여넣기용
handoff prompt다.

## 가장 중요한 구분

`git pull`로 옮겨지는 것은 코드와 Git에 기록된 Task Record뿐이다. 다음 항목은
Git에 포함되지 않는다.

- PostgreSQL의 Workspace, Account, membership, Task, Run, Event 데이터
- `BALEY_LEASE_TOKEN_SECRET`
- 로그인 암호와 Agent token
- 실행 중인 프로세스와 로컬 환경 변수

따라서 새 PC에서 다음 두 방식 중 하나를 선택해야 한다.

1. **기존 Pilot 계속 사용**: 기존 PostgreSQL을 안전하게 공유하거나 백업을
   복원하고, 기존 서버와 동일한 `BALEY_LEASE_TOKEN_SECRET`을 별도 보안
   채널로 새 PC에 설정한다. 현재 Workspace와 계정을 이어 쓰려면 이 방식이다.
2. **새 로컬 환경 시작**: 비어 있는 로컬 PostgreSQL에 migration과 demo seed를
   적용하고 새 Owner 계정을 만든다. 기존 Workspace 진행 상태는 따라오지 않는다.

일반적으로 다른 PC에서도 현재 Baley Pilot을 계속 개발하려는 경우에는 1번이
맞다. DB 백업과 secret은 저장소에 추가하거나 채팅에 붙여 넣지 않는다.

## 사람이 먼저 할 일

PowerShell에서 저장소를 받은 뒤 해당 폴더로 이동한다.

```powershell
git clone <repository-url> D:\Project_AI\baley
Set-Location D:\Project_AI\baley
git pull --ff-only origin main
```

이미 clone한 저장소라면 마지막 두 명령만 실행한다. 그다음 아래 prompt 전체를
새 Codex 세션에 붙여 넣는다. 저장소 경로가 다르면 prompt 첫 줄의 경로만 바꾼다.

## 새 구현 세션용 handoff prompt

```text
D:\Project_AI\baley 저장소의 다른 PC 개발 환경을 복원하고 작업 재개 준비를 끝까지 진행하세요.

운영 원칙:
- 먼저 git status --short --branch와 remote를 확인하세요. 로컬 변경이 있으면 덮어쓰거나 삭제하지 말고 보고하세요.
- main을 origin/main과 git pull --ff-only로 동기화하세요. 강제 reset, checkout으로 변경 폐기, clean은 하지 마세요.
- AGENTS.md를 읽고, Baley 작업에는 .agents/skills/baley-manage-work/SKILL.md를 적용하세요. 기존 프로젝트 채택 작업이 실제로 필요한 경우에만 .agents/skills/baley-adopt-project/SKILL.md도 읽으세요.
- docs/other-pc-development-handoff.md와 docs/account-workspace-access-operations.md를 기준으로 진행하세요.
- 비밀값, 로그인 암호, DB dump, Agent token을 Git, Task Record, 로그, 명령 인자, 채팅에 노출하지 마세요.
- 기존 데이터가 있는 DB에 demo seed나 restore를 임의로 실행하지 마세요.
- destructive DB restore, 기존 DB 교체, 새로운 secret으로 기존 Pilot을 여는 일은 명시적 사용자 확인 없이는 하지 마세요.

1. 환경 점검
- git, Docker Desktop/docker compose, Go 1.26 이상, Node.js/npm, Python이 실행 가능한지 확인하세요.
- package-lock.json과 server/go.mod를 기준으로 npm ci 및 server에서 go mod download를 실행하세요.
- docker compose config로 로컬 PostgreSQL 구성을 검증하세요.

2. 데이터 모드 판별
- 이 PC가 기존 Baley Pilot의 PostgreSQL에 연결 가능한지, 또는 전달받은 안전한 DB backup이 있는지 읽기 전용으로 확인하세요.
- 기존 Pilot을 이어 쓸 자료가 보이지 않으면, 아래 둘 중 어떤 목표인지 사용자에게 한 번만 쉽게 질문하세요.
  A) 기존 Workspace와 계정까지 그대로 이어 쓰기
  B) 새 로컬 demo Workspace로 시작하기
- A라면 기존 DB 연결 또는 backup 복원 경로와 기존 BALEY_LEASE_TOKEN_SECRET의 로컬 주입이 필요하다고 설명하세요. 비밀값 자체를 채팅으로 요청하지 말고 사용자가 이 PC의 User 환경 변수나 별도 secret manager에 설정하게 하세요.
- A의 DB restore는 대상이 비어 있거나 교체 승인되었을 때만 실행하세요. 복원 후 demo seed와 account-bootstrap을 실행하지 마세요.
- B라면 docker compose up -d postgres, migration, demo seed 순서로 초기화하세요. 새 BALEY_LEASE_TOKEN_SECRET은 충분히 긴 임의값을 로컬 User 환경 변수나 secret manager에만 저장하세요. 그 후 기존 human Actor 00000000-0000-4000-8000-000000000002로 첫 Owner 계정을 account-bootstrap 하되, 로그인 ID와 표시 이름은 사용자에게 받고 암호는 hidden stdin으로만 입력하게 하세요. 암호는 15~64 Unicode code point 정책을 따릅니다.

3. 공통 서버 설정과 실행
- 실제 DB URL을 BALEY_DATABASE_URL에 설정하세요. 로컬 compose 기본값은 postgres://baley:baley@127.0.0.1:54329/baley?sslmode=disable 입니다.
- 현재 프로세스에 BALEY_LEASE_TOKEN_SECRET이 비어 있지 않은지 값 자체를 출력하지 않고 확인하세요.
- 로컬 HTTP 개발 설정은 다음과 같습니다.
  BALEY_ENV=development
  BALEY_AUTH_MODE=enforced
  BALEY_COOKIE_SECURE=false
  BALEY_VIEWER_ORIGINS=http://127.0.0.1:5173
- server 디렉터리에서 go run ./cmd/baley-server migrate up을 실행하세요.
- server 디렉터리에서 go run ./cmd/baley-server serve를 시작하세요.
- 저장소 루트에서 다음 Viewer 환경을 설정하고 npm run dev -- --host 127.0.0.1 --port 5173 --strictPort로 시작하세요.
  VITE_BALEY_AUTH_MODE=enforced
  VITE_BALEY_API_URL=http://127.0.0.1:8080
- 장시간 실행 프로세스는 별도로 시작하고, 창을 불필요하게 노출하지 마세요. 로그에는 비밀값이 없어야 합니다.

4. 검증
- GET http://127.0.0.1:8080/healthz가 {"status":"ok"}를 반환하는지 확인하세요.
- http://127.0.0.1:5173에서 로그인, 우상단 계정 메뉴, 로그아웃, Workspace 드롭다운과 Workspace 전환을 브라우저로 smoke test 하세요.
- 기존 Pilot 모드라면 Workspace ID 00000000-0000-4000-8000-000000000001을 fresh read 하세요. 문서의 과거 revision을 신뢰하지 말고 DB의 현재 revision, active phase, pending Task, Gate 상태를 보고하세요.
- npm test, npm run typecheck, npm run build, server에서 go test ./...와 go vet ./...를 실행하세요.
- PostgreSQL aggregate acceptance가 필요하면 운영 DB가 아니라 이름에 test 또는 acceptance가 포함된 별도 loopback DB를 BALEY_TEST_DATABASE_URL로 지정하고 scripts/run-embedding-enablement-acceptance.ps1을 실행하세요. 운영/Pilot DB를 테스트 DB로 사용하지 마세요.

5. MCP와 작업 재개
- Agent MCP는 tokenless stdio registration만 사용하세요. `BALEY_SERVER_URL`과 `BALEY_MCP_CREDENTIAL_STORE`만 Codex에 등록하고, Workspace 연결 후 OS keychain이 device credential을 보관합니다; Agent or gateway token은 채팅, 저장소, 환경 파일, 또는 Codex configuration에 기록하지 마세요.
- 환경 변수 또는 MCP schema가 바뀌었다면 새 thread/프로세스로 다시 연결하세요.
- fresh Workspace read 후 다음 실제 Task를 제안하되, 사람 전용 승인, Task confirm, Gate pass는 baley-manage-work 승인 경계를 그대로 지키세요.
- 현재 진행 상태를 임의로 seed 데이터나 문서의 숫자로 덮어쓰지 마세요.

완료 보고에는 다음만 간결하게 포함하세요.
- checked-out commit과 origin/main 동기화 여부
- 선택한 데이터 모드(A 기존 Pilot / B 새 로컬)
- DB migration과 server/viewer 기동 결과
- 로그인 및 Workspace 전환 smoke test 결과
- 테스트/빌드 결과
- fresh Baley Workspace 상태와 다음 작업
- 사용자가 별도로 해야 할 일이 남았다면 비밀값을 노출하지 않는 정확한 한 단계
```

## 수동 실행 명령 요약

새 로컬 DB를 만드는 경우에만 다음 순서를 사용한다. 기존 Pilot DB를 복원하거나
공유하는 경우에는 `baley-dev-seed`와 `account-bootstrap`을 생략한다.

```powershell
docker compose up -d postgres

$env:BALEY_DATABASE_URL = "postgres://baley:baley@127.0.0.1:54329/baley?sslmode=disable"
$env:BALEY_LEASE_TOKEN_SECRET =
  [Environment]::GetEnvironmentVariable("BALEY_LEASE_TOKEN_SECRET", "User")

Set-Location .\server
go run ./cmd/baley-server migrate up
go run ./cmd/baley-dev-seed
go run ./cmd/baley-server account-bootstrap `
  00000000-0000-4000-8000-000000000001 `
  00000000-0000-4000-8000-000000000002 `
  <LOGIN_ID> "<DISPLAY_NAME>"
```

서버 터미널:

```powershell
$env:BALEY_DATABASE_URL = "postgres://baley:baley@127.0.0.1:54329/baley?sslmode=disable"
$env:BALEY_LEASE_TOKEN_SECRET =
  [Environment]::GetEnvironmentVariable("BALEY_LEASE_TOKEN_SECRET", "User")
if ([string]::IsNullOrWhiteSpace($env:BALEY_LEASE_TOKEN_SECRET)) {
  throw "BALEY_LEASE_TOKEN_SECRET is not configured for the current Windows user"
}
$env:BALEY_ENV = "development"
$env:BALEY_AUTH_MODE = "enforced"
$env:BALEY_COOKIE_SECURE = "false"
$env:BALEY_VIEWER_ORIGINS = "http://127.0.0.1:5173"
Set-Location .\server
go run ./cmd/baley-server serve
```

Viewer 터미널:

```powershell
$env:VITE_BALEY_AUTH_MODE = "enforced"
$env:VITE_BALEY_API_URL = "http://127.0.0.1:8080"
npm run dev -- --host 127.0.0.1 --port 5173 --strictPort
```

`BALEY_LEASE_TOKEN_SECRET is required`가 나오면 secret을 새로 즉흥 생성하기 전에
선택한 데이터 모드를 다시 확인한다. 기존 Pilot에서는 기존 secret을 같은 값으로
주입해야 하며, 새 로컬 환경에서만 새 secret을 만든다.
