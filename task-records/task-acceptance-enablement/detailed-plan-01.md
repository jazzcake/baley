---
baley_record: 1
record_id: "0044a812-3dc0-4d40-b665-39dfe4f98b7a"
task_id: 130
task_key: "task-acceptance-enablement"
record_type: detailed-plan
run_id: "f58162f3-26d6-4323-8c63-0d05b13a161b"
created_at: "2026-07-26T23:39:00+09:00"
created_by: "codex"
registration_state: registered
supersedes: null
---

# Task #130 상세 계획

Workspace future-task policy, immutable Task assignment, versioned EvidenceProfile과 typed
TaskAcceptanceEvidence를 분리한다. 기존 Task는 migration 시 `human_required`로 고정한다.
delegated auto-confirm은 implemented Task의 결속된 typed evidence가 profile을 충족할 때
동일 transaction에서만 수행한다. escalation은 human 승인 delegated→human_required만
허용한다.
