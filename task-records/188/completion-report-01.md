---
baley_record: 1
record_id: "b1bca40d-7b6a-47c1-9697-a6b7f591d5f8"
task_id: 188
record_type: completion-report
run_id: "3ccd35d7-df8f-49bf-a2aa-dc0790d5ad13"
created_at: "2026-09-10T21:12:17+09:00"
created_by: "codex-worker-term_842b524a"
status: complete
---

# Task #188 schema-28 복구 백업과 배포 증거 정정 보고서

## 결론

#187 완료 Record의 `AA6AA947...` schema-28 dump는 `D:\Project_AI\baley-backups` 아래의 어떤 보존 dump와도 일치하지 않았다. 대신 현재 운영 schema-29 DB를 revision 1480에서 읽기 전용으로 스냅샷하고, 새 clone만 schema 28로 내린 뒤 실제 복원·API readiness까지 통과한 restorable dump를 새로 보존했다.

- schema-29 source snapshot: `D:\Project_AI\baley-backups\task-188\20260910T120800Z\baley-schema29-source.dump`, SHA-256 `B2DF4F85A173225866DF504AE499F42E06F679BE99E4C64A282BBBD44AC7EB09`, 3,538,878 bytes.
- schema-28 recovery dump: `D:\Project_AI\baley-backups\task-188\20260910T120800Z\baley-schema28.dump`, SHA-256 `E45F71BB50A81D525ADAB61F6C36D1975FEEBB1ABF7F07045164D0C34BBF8C68`, 3,542,514 bytes.
- metadata: `D:\Project_AI\baley-backups\task-188\20260910T120800Z\backup.json`.

## 복원 및 데이터 증명

원본 스냅샷 clone과 schema-28 dump를 복원한 두 번째 disposable DB는 36개 application table의 exact row count가 모두 같았다. Workspace `00000000-0000-4000-8000-000000000001`은 revision 1480, Task 총수 217이며 #184 `62786a24...`, #185 `c515c874...`, #186 `cf80bfb3...`, #187 `7980fc93...`, #188 `90edeb48...`의 public/internal ID와 상태가 보존됐다. 주요 수치는 Events 3,353, Commands 3,421, Runs 727, Task Records 511, Journal entries 1,210이다.

운영 schema 29에는 append-only Event trigger 2개가 있고 schema-28 restore에는 0개가 있어 migration boundary도 정확하다. schema-28 호환 commit `e64c2fbe38561ce68755f162e3e37351d6c0d31b`에서 재현 빌드한 image `sha256:73be51cc3573b581eb547706be203df551fd1ff8594f98eb263235cab530ea78`은 schema label 28을 가졌고, 두 번째 restore DB에서 `/healthz=ok`, `/readyz=ready/schemaVersion 28`, `/versionz=task-186/e64c2fbe.../schemaVersion 28`을 반환했다.

## 배포 증거 정정

현재 배포 API image ID는 `sha256:9ac4f8be3a60929b67e259faaa27007405a32542c5cea2188fb75c31e7716fbd`이고 OCI labels는 revision `5a814f3c444e709dfdecab63c13fc0e614a6f9bb`, schema `29`이다. 실행 중인 container ID는 `f2c6796c588c8128640bc78b98b575af38dcdfe4c963aa15f128f0d3c147a445`이다.

배포 컨테이너의 `/app/baley-server` 실제 SHA-256은 `9B5ACD4E96B6D7D0AFFFAB1F87E55060CE0EC3749342E2A25F5123CF8453C50E`이다. 따라서 #187 Record의 `55AC3FD7...` 주장을 이 값으로 supersede한다. MCP executable `C:\dev-bin\baley\releases\5a814f3c444e\baley-mcp.exe` SHA-256은 `089AC705F6E6AF96E8AA8B331BFFB3AA9EDA734F30CCB543058E22F2FC0DA141`이다.

## 운영 확인과 잔여 위험

로컬 API health/ready/version, Viewer root와 same-origin ready proxy, tailnet root/ready/version이 HTTP 200 및 schema 29/revision `5a814f3...`을 반환했다. 포트 8080, 5174, 8090은 계속 loopback-only였고 Tailscale Serve mapping은 변경하지 않았다. 운영 DB downgrade, 직접 live DB mutation, 방화벽 변경, merge, 기존 immutable Record 수정은 없었다.

백업은 current snapshot의 revision 1480 시점 복구본이다. 그 이후의 쓰기는 포함하지 않으므로 실제 재해복구 시에는 이 시점 경계와 이후 audit/event 손실을 명시적으로 승인해야 한다. #184~#187은 `implemented`이지만 human confirmation 없이 그대로 유지한다.
