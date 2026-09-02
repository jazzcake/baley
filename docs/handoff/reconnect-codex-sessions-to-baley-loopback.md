# Codex session reconnect prompt

아래 프롬프트를 재부팅 전에 열려 있는 각 Codex/Orca 작업 세션에 전달한다. 각 세션은
자신의 작업 내용을 보존한 뒤 종료하고, 재부팅 후 같은 작업공간에서 새 세션으로 다시
연결한다.

```text
Baley MCP가 구형 세션별 stdio 실행 방식에서 단일 loopback Gateway로 전환됐습니다.

지금 이 세션에서 다음을 수행하세요.

1. 현재 작업트리와 진행 중인 작업을 읽고, 사용자 소유 변경을 삭제하거나 덮어쓰지 마세요.
2. 진행 중인 Baley Task와 Run이 있으면 현재 상태, 마지막 성공 단계, 남은 작업, 관련 파일,
   테스트 결과, branch/HEAD를 짧은 handoff로 남기세요. 실행 중 Run은 중단 또는 정상 종료로
   정리하되 Task를 임의로 confirmed 처리하지 마세요.
3. 필요한 변경은 기존 작업공간의 정상 절차에 따라 저장·커밋·푸시하세요. 미완성 변경을
   억지로 커밋하지 말고 handoff에 정확히 기록하세요.
4. 더 이상 `C:\Users\jazzc\AppData\Local\Baley\mcp\baley-mcp.exe`를 직접 실행하거나
   stdio `command` 등록, BALEY_MCP_GATEWAY_TOKEN, Authorization header를 사용하지 마세요.
5. handoff를 남긴 뒤 이 Codex 세션을 완전히 종료하세요.

시스템 재부팅 후 같은 작업공간에서 새 Codex 세션을 열고 다음을 확인하세요.

- `baley` MCP transport는 `streamable_http`이며 URL은
  `http://127.0.0.1:8090/mcp`입니다.
- 기존 작업트리와 `git status`를 먼저 확인하고, 이전 handoff와 Baley Task/Run을 fresh-read한
  뒤 남은 작업부터 계속하세요.
- 로그인한 Account의 활성 Workspace membership과 역할이 MCP 권한을 결정합니다.
  Workspace별 토큰·환경변수·별도 연결 승인을 요청하지 마세요.
- MCP가 실패하면 구형 바이너리를 복구하지 말고 단일 Gateway와 scheduled task 상태를
  진단해 보고하세요.
```
