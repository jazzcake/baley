---
baley_record: 1
record_id: "24fca44b-8a9d-4d75-99f2-00a6c78dc079"
task_id: 188
record_type: detailed-plan
run_id: "3ccd35d7-df8f-49bf-a2aa-dc0790d5ad13"
created_at: "2026-09-10T21:04:00+09:00"
created_by: "codex-worker-term_842b524a"
status: complete
---

# Task #188 schema-28 복구 증거 및 배포 실측 정정 계획

운영 `baley` DB는 읽기 전용 `pg_dump`로만 스냅샷한다. 스냅샷을 새 격리 DB에 복원하고 그 clone만 지원 `migrate down`으로 schema 29에서 28로 내린 뒤, Workspace revision·전체 테이블 수·Task #184~#188의 public/internal ID와 상태를 원본 스냅샷과 비교한다.

검증된 clone에서 custom-format schema-28 dump를 `D:\Project_AI\baley-backups\task-188` 아래에 생성한다. 이 덤프를 두 번째 disposable DB에 복원해 모든 테이블 수와 Task 정체성이 같은지 확인하고, schema-28 호환 commit `e64c2fbe...`에서 재현 빌드한 API로 `/healthz`, `/readyz`, `/versionz`를 검사한다.

배포 중인 schema-29 API 컨테이너·이미지의 immutable ID, OCI revision/schema labels, `/app/baley-server` SHA-256과 현재 MCP 실행 파일 SHA-256을 다시 측정한다. immutable #187 Record는 수정하지 않고 이 Task의 완료 Record로 누락 덤프 주장과 잘못된 실행 파일 해시를 supersede한다.

검증 후 schema-28 API 컨테이너와 두 disposable DB 및 임시 빌드 소스를 제거한다. 운영 DB, 방화벽, Tailscale Serve, 기존 Task confirmation은 변경하지 않는다.
