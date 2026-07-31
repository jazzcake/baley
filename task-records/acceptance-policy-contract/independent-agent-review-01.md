---
baley_record: 1
record_id: "492e6e14-7f46-4d96-a62f-4ee688ed0340"
task_id: 118
task_key: "acceptance-policy-contract"
record_type: independent-agent-review
run_id: "cb0891fe-0c55-46b7-b3e8-764ca42b1fe4"
created_at: "2026-07-26T19:26:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #118 Independent Review

## Scope

`docs/task-acceptance-policy-contract.md`를 Embedding Contract 및 Adoption manifest와 대조했다.

## Initial findings

1. effective acceptance mode의 불변성 설명과 정책 변경 경로가 충돌했다.
2. `backlog.promote`가 mode/profile을 어떤 시점에 해석하고 고정하는지 부족했다.
3. evidence profile의 참조 형식·cardinality와 재평가 lifecycle이 충분히 명시되지 않았다.

## Resolution recorded

문서는 append-only Assignment, human-only policy change, promotion 시 atomic binding resolve,
versioned EvidenceProfile, `task.evidence.report` 후 재평가와 첫 valid evaluation의 auto-confirm
규칙으로 보완되었다. 이 문서는 initial findings의 감사 기록이며, 최종 판단은 re-review 결과와
review response record에 따른다.
