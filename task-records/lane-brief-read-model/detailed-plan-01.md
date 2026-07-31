---
baley_record: 1
record_id: "02877c7d-c2e6-4b02-82a0-3a979321b854"
task_id: 122
task_key: "lane-brief-read-model"
record_type: detailed-plan
run_id: "13b72722-3723-44ed-8a0e-79983f6ed7db"
created_at: "2026-07-26T23:39:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #122 상세 계획

Lane Brief는 read-only snapshot query로 구현한다. 모든 active Run을 먼저 보존하고 Task별
마지막 terminal Run만 최근 history로 남긴다. Task Record는 실제 repository path,
SHA-256 working-tree hash, commit/blob와 indexed remote identity를 대조한다. Phase, Lane,
dependency, Gate와 condition은 각각의 observation timestamp로 staleness를 판정한다.
