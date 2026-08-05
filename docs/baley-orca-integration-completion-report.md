# Baley–Orca Integration 완료보고

작성일: 2026-08-05

## 완료 범위

- Lane 전체가 아닌 `Baley Task × provider` 단위 ExternalExecution 잠금
- `creating → active → review → settled` 및 `lost → active` 복구 상태 전이
- `lost` 상태의 잠금 유지와 명시적 `settle` 시 잠금 해제
- ExternalExecution PostgreSQL 스키마와 Run 연결 필드
- Application, HTTP, CLI command model, MCP 조회·변경 도구
- Web Viewer Task Inspector의 기존 Orca terminal 이동 액션
- loopback 전용 `baley-orca-bridge`
  - Baley 실행과 Task 일치 여부 검증
  - 기존 Orca worktree의 연결된 terminal 조회
  - 가장 최근 terminal 전환
  - worktree 또는 terminal 자동 생성 금지

## 주요 파일

- 설계 제안: `docs/baley-orca-integration-plan.md`
- 상세 구현 계획: `docs/baley-orca-integration-implementation-plan.md`
- 도메인: `server/internal/domain/external_execution.go`
- 마이그레이션: `server/migrations/00018_external_executions.sql`
- 로컬 브리지: `server/cmd/baley-orca-bridge/main.go`
- Viewer 액션: `src/App.tsx`

## 검증 결과

- `go test ./...`: 통과
- `go vet ./...`: 통과
- `npm run typecheck`: 통과
- `npm test -- --run`: 15개 파일, 64개 테스트 통과
- `npm run build`: 통과
- `git diff --check`: 통과

Viewer production build의 JavaScript chunk 크기 경고는 기존 구성상의 비차단 경고이며 빌드는 성공했다.

## 다른 머신에서 확인할 사항

이 구현 환경에는 접속 가능한 live Baley 서비스가 없었다. 따라서 다음 항목은 코드는 작성했지만 실제 서비스 E2E로 검증하지 못했다.

1. PostgreSQL에 `00018_external_executions.sql` 적용
2. Baley 인증 쿠키 또는 Authorization 전달
3. MCP를 통한 reserve/attach/review/settle 전체 흐름
4. Viewer에서 로컬 브리지를 거쳐 실제 Orca terminal로 전환

브리지는 기본적으로 `127.0.0.1:47831`에만 바인딩하며, Viewer와 같은 hostname을 사용해 인증 쿠키가 전달되도록 구성한다. 기존 실행을 찾지 못했을 때 새 Orca task/worktree/terminal을 생성하지 않는다.

## 운영 불변식

- 하나의 Task에는 provider별로 잠금을 보유한 ExternalExecution이 최대 하나다.
- `creating`, `active`, `review`, `lost`는 잠금을 보유한다.
- `settled`만 잠금을 해제한다.
- Viewer의 Orca 이동은 관찰 및 화면 전환 액션이며 Baley 상태를 변경하지 않는다.
- `lost` 실행은 자동 대체하지 않고 복구 또는 명시적 정산을 요구한다.
