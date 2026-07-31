---
baley_record: 1
record_id: "90cd51e5-ffc7-4e8b-ade0-1a8239a02868"
task_id: 136
task_key: "pilot-auth-activation"
record_type: completion-report
run_id: "84259c16-6b04-4d04-a9b8-cd2848bdc7be"
created_at: "2026-07-29T00:43:41+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #136 후속 UI 완료 보고

로그인된 사용자가 우상단 계정 영역에서 계정 정보와 권한별 작업 메뉴를 열고
로그아웃할 수 있도록 구현했다. 화면의 Workspace 이름은 접근 가능한 Workspace 목록을
보여주는 드롭다운이 되었으며, 선택 시 해당 Workspace의 정규 URL로 안전하게 전환한다.

키보드 이동, 포커스 복원, 바깥 클릭 닫힘, 로그아웃 실패 유지, 권한별 메뉴 노출,
Workspace 전환 중 응답 경합 방지를 포함해 검증했다.

- 프런트엔드 테스트: 13개 파일, 54개 테스트 통과
- 프로덕션 빌드: 통과
- `git diff --check`: 통과
- Viewer HTTP 및 인증 서버 health check: 통과
- 독립 프런트엔드·보안 재검토: PASS, 잔여 발견 사항 없음

현재 계정에는 Baley Pilot 한 곳만 연결되어 있으므로 드롭다운에도 한 항목만 표시된다.
다른 Workspace 멤버십이 생기면 같은 메뉴에 자동으로 추가된다. 자동 브라우저 조작
스크립트가 설치되어 있지 않아 최종 실제 화면 확인은 사람 확인 단계로 남긴다.
