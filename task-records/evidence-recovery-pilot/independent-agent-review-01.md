---
baley_record: 1
record_id: "e29528cf-5e88-4dc7-b7c1-fc55c827fc5e"
task_id: 119
task_key: "evidence-recovery-pilot"
record_type: independent-agent-review
run_id: "46f31115-a579-4a88-b8ca-52c78f2ff9ed"
created_at: "2026-07-26T19:26:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #119 Independent Review

## Scope

`docs/evidence-recovery-pilot-runbook.md`를 Embedding Contract 및 Adoption manifest와 대조했다.

## Initial findings

1. Baley DB, repository Task Record, Git, worktree observation, Agent summary의 source-of-truth
   경계가 충분히 분리되지 않았다.
2. Pilot metric의 sample, deduplication, denominator, timing, evidence reference가 부족하여
   같은 결과를 재현하기 어려웠다.

## Resolution recorded

문서는 fact별 authority matrix, active/nonterminal Run 우선 recovery, terminal Task에는 follow-up만
제안하는 규칙, append-only `PilotMeasurement` schema 및 산술·dedup·timing 정의로 보완되었다.
이 문서는 initial findings의 감사 기록이며, 최종 판단은 re-review 결과와 review response record에
따른다.
