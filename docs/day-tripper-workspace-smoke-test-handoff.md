---
type: handoff
status: ready
scope: day-tripper-workspace-smoke-test
---

# Day Tripper Workspace smoke-test handoff

아래 Prompt를 Day Tripper 작업 세션에 전달한다. Workspace URL 외의 토큰, env 파일,
MCP 등록 명령은 전달하지 않는다.

## Session prompt

```text
Baley를 사용해 Day Tripper Pilot Workspace의 격리 smoke test를 수행하세요.

Workspace URL:
http://127.0.0.1:5174/workspaces/410f335e-ddb2-443f-be3c-7d1d18ccd534

baley-manage-work와 baley-adopt-project Skill을 사용하고 각 SKILL.md를 먼저 끝까지
읽으세요. Baley typed MCP만 사용하며 직접 HTTP, DB, fixture 수정으로 우회하지 마세요.
Day Tripper repository 파일과 기존 dirty worktree는 변경하지 마세요.

1. URL에서 Workspace UUID를 추출하고 Baley fresh read를 시도하세요.
2. MCP가 `workspace_login_required`와 loopback `loginUrl`을 반환하면 그 링크만
   사용자에게 보여주세요. 활성 Workspace 멤버가 로그인하고 `Connect local Gateway`를
   누르면, 같은 PC의 pending device secret과 브라우저 일회용 코드가 모두 맞을 때만 연결됩니다.
3. 로그인 뒤 같은 read를 다시 실행하세요. 별도 env 파일 생성, 토큰 복사, MCP 재등록,
   새 thread 요청은 하지 마세요.
4. Workspace 이름, revision, active Phase, Lane, Task, Backlog, Gate를 보고하세요.
5. 같은 credential로 Baley Pilot Workspace
   00000000-0000-4000-8000-000000000001 읽기가 거부되는지 확인하세요. 데이터가
   반환되면 tenant isolation 실패이므로 mutation 없이 중단하세요.
6. Adoption Lane에 phase를 지정하지 않은 Backlog 항목을 typed preview/execute로
   생성하세요.
   - title: [Smoke] Workspace-scoped MCP lifecycle
   - description: Day Tripper Pilot create/read/update/discard and isolation verification
7. fresh read로 B#와 phase-free 상태를 확인하고, description 끝에
   ` / update verified`를 추가한 뒤 다시 확인하세요.
8. `workspace onboarding smoke test completed` 사유로 해당 항목을 discard하고,
   active Backlog에서 사라졌으며 audit Event가 남았는지 확인하세요.
9. 최종 revision, B#, Event ID, isolation 결과를 요약하세요.

이 smoke test에는 Task/Phase/Gate 생성, repository 등록, Task confirmation, Gate passage가
포함되지 않습니다. 사람 전용 승인이 필요한 mutation은 실행하지 마세요.
```

## Expected operator experience

등록되지 않은 로컬 Gateway에서만 loopback 로그인 링크가 한 번 나타난다. 로그인한
멤버가 `Connect local Gateway`를 누르면 같은 LLM 세션이 요청을 재시도해 즉시 작업을
계속한다. 다음 Workspace도 동일한 MCP 등록과 device credential을 사용하며, 활성
membership과 역할이 있으면 Workspace별 추가 연결 절차 없이 접근한다.
