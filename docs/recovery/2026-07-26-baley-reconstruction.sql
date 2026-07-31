\set ON_ERROR_STOP on

BEGIN;

INSERT INTO actors(id, display_name, actor_type) VALUES
  ('00000000-0000-4000-8000-000000000002', 'Workspace Owner', 'human'),
  ('00000000-0000-4000-8000-000000000003', 'Codex Operator', 'agent');

INSERT INTO workspaces(id, name, revision, state)
VALUES ('00000000-0000-4000-8000-000000000001', 'Baley Pilot', 194, 'active');

INSERT INTO repositories(
  workspace_id, id, name, remote_url, default_branch,
  is_record_repository, task_records_root
) VALUES (
  '00000000-0000-4000-8000-000000000001',
  'bca59f7a-5f27-4880-a2ab-cb4cdbf13949',
  'Baley',
  'https://github.com/jazzcake/baley',
  'main',
  true,
  'task-records'
);

INSERT INTO phases(workspace_id, id, name, position, state) VALUES
  ('00000000-0000-4000-8000-000000000001', 'build', 'Build', 0, 'completed'),
  ('00000000-0000-4000-8000-000000000001', 'validate', 'Validate', 1, 'active'),
  ('00000000-0000-4000-8000-000000000001', 'embedding-contract', 'Embedding Contract', 2, 'planned'),
  ('00000000-0000-4000-8000-000000000001', 'embedding-enablement', 'Embedding Enablement', 3, 'planned'),
  ('00000000-0000-4000-8000-000000000001', 'embedding-pilot', 'Embedding Pilot', 4, 'planned');

INSERT INTO lanes(workspace_id, id, name, state, goal, summary) VALUES
  ('00000000-0000-4000-8000-000000000001', 'server', 'Server', 'active', '', ''),
  ('00000000-0000-4000-8000-000000000001', 'client', 'Client', 'active', '', ''),
  ('00000000-0000-4000-8000-000000000001', 'art', 'Art', 'active', '', ''),
  ('00000000-0000-4000-8000-000000000001', 'adoption', 'Adoption', 'active',
   'Baley를 실제 작업 운영에 embedding한다.',
   'Contract, Enablement, Pilot 순서로 adoption outcome을 검증한다.');

INSERT INTO tasks(
  workspace_id, id, public_id, lane_id, phase_id, title, description,
  status, current_summary, next_action, terminal_reason, implemented_assessment
) VALUES
  ('00000000-0000-4000-8000-000000000001', 'api', 101, 'server', 'build',
   'API 구현', 'Pilot API 구현', 'confirmed', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', 'ui', 104, 'client', 'build',
   'Pilot UI', 'Pilot UI 구현', 'confirmed', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', 'assets', 106, 'art', 'build',
   'Asset 제작', 'Pilot asset 제작', 'confirmed', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', 'user-test', 110, 'client', 'validate',
   'Gate transition vertical slice', 'Gate transition의 PostgreSQL/API/MCP/Viewer vertical slice.', 'confirmed', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000111', 111, 'server', 'validate',
   'API runtime contract alignment', 'Current-source API와 typed task.create MCP runtime contract를 정렬한다.',
   'implemented', '구현·독립 리뷰·완료 보고 완료.', '사람 confirmation 대기.', NULL,
   'Repository completion report에서 구현과 독립 리뷰가 확인됨.'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000112', 112, 'client', 'validate',
   'Home navigation entry points', 'Viewer의 Home navigation entry points를 완성한다.',
   'confirmed', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000113', 113, 'client', 'validate',
   'Resizable Task inspector', 'Task inspector resize와 scroll 동작을 완성한다.',
   'confirmed', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000114', 114, 'client', 'validate',
   'React Flow canvas navigation', 'Canvas navigation과 viewport interaction을 안정화한다.',
   'implemented', '기능 구현과 검증 완료.', '사람 confirmation 대기.', NULL,
   'Roadmap confirmation recommendation과 Git history에서 구현 완료가 확인됨.'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000115', 115, 'client', 'validate',
   'React Flow viewport synchronization', 'Zoom in/out/Fit의 renderer/store/DOM viewport를 동기화한다.',
   'implemented', '사용자 시각 검증과 자동 테스트, 독립 리뷰 완료.', '사람 confirmation 대기.', NULL,
   'Completion report 529ec028-23fb-491e-9f77-e1bcf3b825be 근거.'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000116', 116, 'server', 'validate',
   '구조적 Baley 객체 typed MCP 도구', 'Phase/Lane/Gate와 Gate Task를 typed MCP로 생성·연결한다.',
   'implemented', 'typed structural MCP와 Adoption 구조 생성 완료.', 'Gate 조건 연결 후 사람 confirmation 대기.', NULL,
   'Structural MCP completion reports와 Adoption manifest 근거.'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000117', 117, 'adoption', 'embedding-contract',
   'Embedding 범위 및 성공 기준 계약',
   'Roadmap Phase 3~5를 Embedding 세 Phase에 대응시키고 제품 경계, 비목표, 승인 경계, 각 Gate의 검증 가능한 exit criteria를 하나의 계약으로 고정한다.',
   'pending', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000118', 118, 'adoption', 'embedding-contract',
   'Operator 승인 및 Task intake 계약',
   'Outcome-first 단일·공동 승인, 순차 fresh-preview 실행, 특정 Phase의 Task 생성, lane별 BacklogItem과 정식 Task 승격 계약 및 capability 경계를 정의한다.',
   'pending', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000119', 119, 'adoption', 'embedding-contract',
   '증거·복원 및 Pilot 측정 계약',
   'Task Record와 Git evidence, lane brief 복원 규칙 및 Pilot 측정 지표를 정의한다.',
   'pending', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000120', 120, 'adoption', 'embedding-contract',
   'Embedding Contract 기준선 리뷰',
   '승인/intake와 증거/복원 계약을 통합해 Enablement acceptance checklist를 확정한다.',
   'pending', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '47c2d962-9008-49cb-9e41-2b063d0213e4', 121, 'adoption', 'embedding-enablement',
   'Operator 승인 및 Task intake 경로 구현',
   'Outcome-first 공동 승인, phase-targeted Task 생성, lane Backlog promotion vertical slice를 구현한다.',
   'in_progress',
   '구현·전체 검증·독립 리뷰 완료. DB 사고 후 repository evidence로 운영 상태 재구성 중.',
   'completion report/독립 리뷰 Record 등록 후 implemented 보고.',
   NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000122', 122, 'adoption', 'embedding-enablement',
   'Lane brief 및 증거 복원 경로 구현',
   'Task Record 인덱스와 repository 불일치 탐지, lane brief 복원을 연결한다.',
   'pending', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000123', 123, 'adoption', 'embedding-enablement',
   'Adoption Pilot 운영 키트',
   'Pilot workspace bootstrap Skill/runbook/template과 중단 복구·Gate 승인 경계를 재현 가능하게 만든다.',
   'pending', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000124', 124, 'adoption', 'embedding-enablement',
   'Embedding Enablement 수용 검증',
   '격리 시나리오 E2E, 독립 리뷰, 잔여 위험을 완료 증거로 남긴다.',
   'pending', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000125', 125, 'adoption', 'embedding-pilot',
   'Day Tripper Pilot 온보딩',
   'Day Tripper 대표 lane, Backlog, Phase Task와 공유 Gate의 pilot baseline을 만든다.',
   'pending', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000126', 126, 'adoption', 'embedding-pilot',
   '실사용 중단·복원 및 Gate 주기 실행',
   '실제 작업에서 Run 중단·복원, evidence, Backlog 승격과 공유 Gate 조율을 수행한다.',
   'pending', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000127', 127, 'adoption', 'embedding-pilot',
   'Adoption 효과 측정 및 불일치 분석',
   '복원 시간, 다음 Task 정확도, Git/Task 불일치와 Gate 조율 비용을 측정한다.',
   'pending', '', '', NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000128', 128, 'adoption', 'embedding-pilot',
   'Baley 지속·일반화 결정',
   'Pilot evidence로 지속 여부와 일반화 요구를 결정한다.',
   'pending', '', '',
   'Adoption pilot의 최종 제품 결정 Task이며 후속 실행은 결정 결과로 새 manifest에서 시작한다.',
   NULL),
  ('00000000-0000-4000-8000-000000000001', '31900000-0000-4000-8000-000000000129', 129, 'adoption', 'embedding-enablement',
   'Gate entry and unlock bindings',
   'Gate condition과 별도로 to-Phase entry/unlock Task binding을 모델링한다.',
   'in_progress', '상세 계획과 migration/UI 구현이 working tree에 존재한다.',
   'Task #121 완료 후 구현·리뷰·보고를 계속한다.', NULL, NULL);

INSERT INTO task_dependencies(workspace_id, from_task_id, to_task_id) VALUES
  ('00000000-0000-4000-8000-000000000001', 'ui', 'user-test'),
  ('00000000-0000-4000-8000-000000000001', 'user-test', '00000000-0000-4000-8000-000000000111'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000111', '00000000-0000-4000-8000-000000000116'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000116', '00000000-0000-4000-8000-000000000117'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000117', '00000000-0000-4000-8000-000000000118'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000117', '00000000-0000-4000-8000-000000000119'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000118', '00000000-0000-4000-8000-000000000120'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000119', '00000000-0000-4000-8000-000000000120'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000120', '47c2d962-9008-49cb-9e41-2b063d0213e4'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000120', '00000000-0000-4000-8000-000000000122'),
  ('00000000-0000-4000-8000-000000000001', '47c2d962-9008-49cb-9e41-2b063d0213e4', '00000000-0000-4000-8000-000000000123'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000122', '00000000-0000-4000-8000-000000000123'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000123', '00000000-0000-4000-8000-000000000124'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000124', '00000000-0000-4000-8000-000000000125'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000125', '00000000-0000-4000-8000-000000000126'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000126', '00000000-0000-4000-8000-000000000127'),
  ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000127', '00000000-0000-4000-8000-000000000128'),
  ('00000000-0000-4000-8000-000000000001', '47c2d962-9008-49cb-9e41-2b063d0213e4', '31900000-0000-4000-8000-000000000129');

INSERT INTO gates(
  workspace_id, id, name, from_phase_id, to_phase_id,
  criteria_revision, passed_at, passed_by_actor_id
) VALUES
  ('00000000-0000-4000-8000-000000000001', 'pilot-ready', 'Pilot Ready',
   'build', 'validate', 1, '2026-07-20T00:00:00+09:00', '00000000-0000-4000-8000-000000000002'),
  ('00000000-0000-4000-8000-000000000001', 'embedding-contract-entry', 'Validate → Embedding Contract',
   'validate', 'embedding-contract', 2, NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', 'embedding-enablement-entry', 'Embedding Contract → Embedding Enablement',
   'embedding-contract', 'embedding-enablement', 2, NULL, NULL),
  ('00000000-0000-4000-8000-000000000001', 'embedding-pilot-entry', 'Embedding Enablement → Embedding Pilot',
   'embedding-enablement', 'embedding-pilot', 2, NULL, NULL);

INSERT INTO gate_tasks(workspace_id, id, gate_id, task_id) VALUES
  ('00000000-0000-4000-8000-000000000001', 'gt-api', 'pilot-ready', 'api'),
  ('00000000-0000-4000-8000-000000000001', 'gt-ui', 'pilot-ready', 'ui'),
  ('00000000-0000-4000-8000-000000000001', 'gt-assets', 'pilot-ready', 'assets'),
  ('00000000-0000-4000-8000-000000000001', 'gt-embedding-contract-116', 'embedding-contract-entry', '00000000-0000-4000-8000-000000000116'),
  ('00000000-0000-4000-8000-000000000001', 'gt-embedding-enablement-120', 'embedding-enablement-entry', '00000000-0000-4000-8000-000000000120'),
  ('00000000-0000-4000-8000-000000000001', 'gt-embedding-pilot-124', 'embedding-pilot-entry', '00000000-0000-4000-8000-000000000124');

INSERT INTO workspace_counters(workspace_id, next_task_public_id, next_backlog_public_id)
VALUES ('00000000-0000-4000-8000-000000000001', 130, 1);

COMMIT;
