# 데스크탑 재개 프롬프트

아래 블록을 데스크탑의 새 Codex thread에 그대로 전달한다.

```text
Baley Phase 2~4 환경 독립 선행 구현의 데스크탑 통합 검증과 후속 Run/Record vertical slice를 재개하세요.

먼저 다음 문서를 순서대로 읽으세요.

1. docs/baley-phase2-4-desktop-handoff.md
2. docs/baley-system-spec-v1.md
3. docs/baley-command-architecture.md
4. docs/baley-phase2-4-prebuild-plan.md
5. docs/baley-roadmap.md
6. task-records/run-record-vertical-slice의 현재 작업에 필요한 정확한 파일

baley-manage-work skill을 적용하세요. Web Viewer는 read-only이고, Agent는 사람 승인 권한을 대신하지 않습니다. live Baley command tool이 없으면 fixture나 DB 직접 수정으로 command 실행을 흉내 내지 마세요.

현재 상태:

- Wave 1~7, P2-01~P4-06 순수 Go 구현과 단위 테스트 완료
- 각 Wave 자체 test/vet/race 및 독립 리뷰 findings 반영 완료
- 최종 Wave 7 독립 리뷰 CLOSE
- PostgreSQL과 MCP integration은 이전 환경에서 env 미설정으로 skip
- 실제 Git/HTTP/CLI/project-init/Viewer adapter 연결은 미완료
- Task Records는 pending-bootstrap 상태

첫 목표는 코드를 더 작성하는 것이 아니라 handoff 문서 5장의 절차대로 깨끗한 baseline을 재현하는 것입니다.

1. git status와 branch/remote 확인
2. 전용 baley_test DB 생성
3. migration up/down/up
4. go test, vet, race
5. PostgreSQL integration skip 0건
6. server를 띄운 뒤 MCP stdio E2E skip 0건
7. npm test, typecheck, build
8. Viewer baseline 사람 인수 확인

주의: integration test는 Baley table을 TRUNCATE하므로 개발용 baley DB에 절대 연결하지 말고 disposable baley_test만 사용하세요.

baseline에서 실패하면 해당 원인을 진단·수정·재검증하고 다음 단계로 넘어가지 마세요. baseline이 모두 통과하면 docs/baley-phase2-4-desktop-handoff.md의 ‘통합 구현 권장 순서’에 따라 Run schema부터 한 단위씩 진행하세요. 한 번에 여러 Wave를 대형 migration이나 service 변경으로 합치지 마세요.

각 단위마다:

- domain 정본을 adapter에서 재구현하지 말 것
- Workspace row lock 안에서 현재 snapshot 기반 plan/authz를 다시 평가할 것
- migration/repository/application/HTTP/MCP/Viewer 테스트를 분리할 것
- 독립 리뷰 → findings 반영 → 재검증 → Task Record를 남길 것
- 실행하지 못한 검증과 skip을 성공으로 보고하지 말 것
- 사람 전용 Task/Lane/Gate/Workspace 결정을 만나면 preview와 snapshot을 보여주고 멈출 것

먼저 baseline 검증 결과와 실패/skip 0건 여부를 보고한 다음, Run persistence 첫 단위의 구현 범위와 예상 연결점을 제시하고 계속 진행하세요.
```
