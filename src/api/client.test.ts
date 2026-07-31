import { afterEach, describe, expect, it, vi } from "vitest";
import { fetchGraph } from "./client";

describe("live Backlog graph mapping", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("keeps active Backlog items and orders them by lane position", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        workspace: { id: "w", name: "W", revision: 1 },
        phases: [], lanes: [{ id: "client", name: "Client", state: "active" }],
        tasks: [], dependencies: [], gates: [], decisions: [], runs: [], records: [],
        backlogItems: [
          { id: "b2", publicId: 2, laneId: "client", title: "Second", description: "", status: "active", position: 2, promotedTaskId: null, promotedTaskPublicId: null },
          { id: "done", publicId: 3, laneId: "client", title: "Done", description: "", status: "promoted", position: null, promotedTaskId: "t", promotedTaskPublicId: 9 },
          { id: "b1", publicId: 1, laneId: "client", title: "First", description: "", status: "active", position: 1, promotedTaskId: null, promotedTaskPublicId: null },
        ],
      }),
    }));
    const graph = await fetchGraph("w");
    expect(graph.backlogItems.map((item) => item.publicId)).toEqual([1, 2]);
    expect(vi.mocked(fetch).mock.calls[0]?.[0]).toContain("/v1/workspaces/w/graph");
    expect(vi.mocked(fetch).mock.calls[0]?.[1]).toMatchObject({ credentials: "include" });
  });

  it("maps Gate conditions and resolved entries to opposite edge directions", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        workspace: { id: "w", name: "W", revision: 2 },
        phases: [], lanes: [], tasks: [], dependencies: [], decisions: [], runs: [], records: [], backlogItems: [],
        gates: [{
          id: "ready", publicId: 4, alias: "release-ready", name: "Ready", fromPhaseId: "from", toPhaseId: "to", status: "ready",
          conditions: [{ id: "condition", taskId: "before", satisfied: true, satisfactionReason: "confirmed" }],
          entryTasks: [{ taskId: "after", selectionSource: "automatic" }],
        }],
      }),
    }));
    const graph = await fetchGraph("w");
    expect(graph.gateLinks).toEqual([
      { gateId: "ready", taskId: "before", kind: "required", satisfied: true, satisfactionReason: "confirmed" },
      { gateId: "ready", taskId: "after", kind: "unlocks", satisfactionReason: "automatic" },
    ]);
    expect(graph.gates[0]).toMatchObject({ id: "ready", publicId: 4, alias: "release-ready" });
  });

  it("derives stable phase-order Gate numbers during an additive migration rollout", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({
        workspace: { id: "w", name: "W", revision: 3 },
        phases: [
          { id: "build", name: "Build", position: 0, state: "completed" },
          { id: "validate", name: "Validate", position: 1, state: "active" },
          { id: "pilot", name: "Pilot", position: 2, state: "planned" },
        ],
        lanes: [], tasks: [], dependencies: [], decisions: [], runs: [], records: [], backlogItems: [],
        gates: [
          { id: "later", name: "Later", fromPhaseId: "validate", toPhaseId: "pilot", status: "open", conditions: [], entryTasks: [] },
          { id: "earlier", name: "Earlier", fromPhaseId: "build", toPhaseId: "validate", status: "passed", conditions: [], entryTasks: [] },
        ],
      }),
    }));
    const graph = await fetchGraph("w");
    expect(graph.gates.map((gate) => [gate.id, gate.publicId])).toEqual([["later", 2], ["earlier", 1]]);
  });
});
