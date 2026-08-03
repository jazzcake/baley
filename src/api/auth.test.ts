import { afterEach, describe, expect, it, vi } from "vitest";
import {
  attachExistingAccount,
  createWorkspace,
  disableMemberAccount,
  login,
  logout,
  resetMemberPassword,
  updateWorkspaceMember,
} from "./auth";

function jsonResponse(value: unknown, status = 200) {
  return {
    ok: status >= 200 && status < 300,
    status,
    headers: new Headers(),
    json: async () => value,
  };
}

describe("credentialed account API", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("uses a credentialed login without adding a CSRF header", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      account: { id: "a", actorId: "actor", loginId: "owner", displayName: "Owner" },
      csrfToken: "csrf",
      expiresAt: "2026-07-28T12:00:00Z",
    }));
    vi.stubGlobal("fetch", fetchMock);

    await login("owner", "a sufficiently long password");

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.credentials).toBe("include");
    expect((init.headers as Headers).get("X-Baley-CSRF")).toBeNull();
    expect(JSON.parse(String(init.body))).toEqual({
      loginId: "owner",
      password: "a sufficiently long password",
    });
  });

  it("binds cookie-authenticated mutations to the CSRF value", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(undefined, 204))
      .mockResolvedValueOnce(jsonResponse({
        actorId: "participant",
        displayName: "Participant",
        role: "viewer",
        relationship: "participant",
        active: true,
      }));
    vi.stubGlobal("fetch", fetchMock);

    await logout("csrf-value");
    await updateWorkspaceMember("workspace", "participant", { role: "viewer" }, "csrf-value");

    for (const [, init] of fetchMock.mock.calls as Array<[string, RequestInit]>) {
      expect(init.credentials).toBe("include");
      expect((init.headers as Headers).get("X-Baley-CSRF")).toBe("csrf-value");
    }
  });

  it("creates a Workspace with a client UUID and CSRF-bound human session", async () => {
    const workspace = {
      id: "6279cb62-d52f-4642-942c-15e7bd72c912",
      name: "Adoption",
      state: "active",
      revision: 1,
      role: "owner",
      relationship: "owner",
      capabilities: ["workspace:admin"],
    };
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(workspace, 201));
    vi.stubGlobal("fetch", fetchMock);

    await createWorkspace({ workspaceId: workspace.id, name: workspace.name }, "csrf-value");

    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toContain("/v1/workspaces");
    expect((init.headers as Headers).get("X-Baley-CSRF")).toBe("csrf-value");
    expect(JSON.parse(String(init.body))).toEqual({
      workspaceId: workspace.id,
      name: workspace.name,
    });
  });

  it("uses distinct admin endpoints for attach, account disable, and password reset", async () => {
    const member = {
      actorId: "participant",
      accountId: "account",
      displayName: "Participant",
      role: "operator",
      relationship: "participant",
      active: true,
    };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(member))
      .mockResolvedValueOnce(jsonResponse(undefined, 204))
      .mockResolvedValueOnce(jsonResponse(undefined, 204));
    vi.stubGlobal("fetch", fetchMock);

    await attachExistingAccount("workspace", { loginId: "existing", role: "operator" }, "csrf");
    await disableMemberAccount("workspace", "participant", "csrf");
    await resetMemberPassword("workspace", "participant", "new long password value", "csrf");
    expect(fetchMock.mock.calls.map((call) => call[0])).toEqual([
      expect.stringContaining("/v1/workspaces/workspace/memberships"),
      expect.stringContaining("/v1/workspaces/workspace/members/participant/disable-account"),
      expect.stringContaining("/v1/workspaces/workspace/members/participant/reset-password"),
    ]);
    for (const [, init] of fetchMock.mock.calls as Array<[string, RequestInit]>) {
      expect((init.headers as Headers).get("X-Baley-CSRF")).toBe("csrf");
    }
  });
});
