import type { WorkspaceFixture } from "../domain/model";

export const pilotReadyFixture: WorkspaceFixture = {
	workspace: { id: "pilot", name: "Pilot delivery", revision: 1, activePhaseId: "build" },
  phases: [
    { id: "build", name: "Build", order: 0, state: "active" },
    { id: "validate", name: "Validate", order: 1, state: "planned" },
  ],
  lanes: [
    { id: "server", name: "Server", goal: "안정적인 Pilot API 제공", summary: "핵심 API 구현 중", lifecycle: "active" },
    { id: "client", name: "Client", goal: "Pilot 경험 완성", summary: "UI 구현과 접근성 개선 병행", lifecycle: "active" },
    { id: "art", name: "Art", goal: "Pilot용 시각 자산 완성", summary: "최종 asset 제작이 막힘", lifecycle: "active" },
    { id: "research", name: "Research", goal: "Pilot 가설 검증", summary: "조사 결과 정리 완료", lifecycle: "closed_out" },
  ],
  tasks: [
    { id: "api-design", publicId: 101, laneId: "server", phaseId: "build", title: "API 설계", description: "Pilot에 필요한 API 계약을 확정합니다.", status: "confirmed" },
    { id: "api-build", publicId: 102, laneId: "server", phaseId: "build", title: "API 구현", description: "설계된 계약에 맞춰 endpoint를 구현합니다.", status: "in_progress" },
    { id: "screen-design", publicId: 103, laneId: "client", phaseId: "build", title: "화면 설계", description: "Pilot 사용자 흐름과 핵심 화면을 정의합니다.", status: "confirmed" },
    { id: "pilot-ui", publicId: 104, laneId: "client", phaseId: "build", title: "Pilot UI", description: "확정된 흐름을 실제 화면으로 구현합니다.", status: "in_progress" },
    { id: "a11y", publicId: 105, laneId: "client", phaseId: "build", title: "접근성 개선", description: "Gate와 별개로 키보드 탐색과 대비를 개선합니다.", status: "pending" },
    { id: "assets", publicId: 106, laneId: "art", phaseId: "build", title: "Asset 제작", description: "Pilot에 사용할 핵심 시각 자산을 제작합니다.", status: "in_progress", blocker: "최종 art direction 승인 대기" },
    { id: "research", publicId: 107, laneId: "research", phaseId: "build", title: "사용자 조사", description: "Pilot 대상 사용자의 문제를 조사합니다.", status: "confirmed" },
    { id: "experiment", publicId: 108, laneId: "research", phaseId: "build", title: "가설 실험", description: "핵심 가설을 소규모로 검증합니다.", status: "confirmed" },
    { id: "findings", publicId: 109, laneId: "research", phaseId: "build", title: "결과 정리", description: "학습과 후속 제안을 기록합니다.", status: "confirmed" },
    { id: "user-test", publicId: 110, laneId: "client", phaseId: "validate", title: "사용자 테스트", description: "Pilot Ready 통과 후 실제 사용성을 검증합니다.", status: "pending" },
  ],
  dependencies: [
    { id: "d1", fromTaskId: "api-design", toTaskId: "api-build" },
    { id: "d2", fromTaskId: "screen-design", toTaskId: "pilot-ui" },
    { id: "d3", fromTaskId: "screen-design", toTaskId: "a11y" },
    { id: "d4", fromTaskId: "research", toTaskId: "experiment" },
    { id: "d5", fromTaskId: "experiment", toTaskId: "findings" },
  ],
  gates: [{ id: "pilot-ready", name: "Pilot Ready", fromPhaseId: "build", toPhaseId: "validate", status: "open" }],
  gateLinks: [
    { gateId: "pilot-ready", taskId: "api-build", kind: "required" },
    { gateId: "pilot-ready", taskId: "pilot-ui", kind: "required" },
    { gateId: "pilot-ready", taskId: "assets", kind: "required" },
    { gateId: "pilot-ready", taskId: "findings", kind: "reference" },
    { gateId: "pilot-ready", taskId: "user-test", kind: "unlocks" },
  ],
	decisions: [],
};
