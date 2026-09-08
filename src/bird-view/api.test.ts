import { afterEach, describe, expect, it, vi } from "vitest";
import { executeBirdViewCommand, fetchBirdViewGraph, fetchBirdViewNodeFocus, listBirdViews } from "./api";

describe("Bird View read-only API", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("uses account-scoped read endpoints with browser credentials", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ items: [] }) })
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ birdView: {}, nodes: [], edges: [] }) })
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ birdView: {}, node: {}, workspaces: [] }) })
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => ({ commandId: "c", birdViewRevision: 2, eventIds: [] }) });
    vi.stubGlobal("fetch", fetchMock);
    await listBirdViews();
    await fetchBirdViewGraph("view id");
    await fetchBirdViewNodeFocus("view id", "node id");
    await executeBirdViewCommand({ name: "bird_view.node.update", arguments: { birdViewId: "view id", nodeId: "node id", positionX: 1, positionY: 2 }, envelope: { idempotencyKey: "k", expectedBirdViewRevision: 1 } }, "csrf");
    expect(fetchMock.mock.calls.map((call) => call[0])).toEqual([
      expect.stringContaining("/v1/bird-views"),
      expect.stringContaining("/v1/bird-views/view%20id/graph"),
      expect.stringContaining("/v1/bird-views/view%20id/nodes/node%20id/focus"),
      expect.stringContaining("/v1/commands/execute"),
    ]);
    expect(fetchMock.mock.calls.every((call) => call[1].credentials === "include")).toBe(true);
    expect(new Headers(fetchMock.mock.calls[3]![1].headers).get("X-Baley-CSRF")).toBe("csrf");
  });
});
