import { afterEach, describe, expect, it, vi } from "vitest";
import {
  attachExistingAccount,
  archiveWorkspace,
  createWorkspace,
  disableMemberAccount,
  executeCommand,
  fetchMCPLoginLink,
  fetchOIDCProviders,
  issueApprovalGrant,
  completeMCPGatewayLogin,
  login,
  logout,
  renameWorkspace,
  resetMemberPassword,
  revokeApprovalGrant,
  restoreWorkspace,
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

  it("discovers only server-configured OIDC providers", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ items: [{ id: "google", label: "Google" }] }));
    vi.stubGlobal("fetch", fetchMock);
    await expect(fetchOIDCProviders()).resolves.toEqual([{ id: "google", label: "Google" }]);
    expect(fetchMock.mock.calls[0]?.[0]).toContain("/v1/auth/oidc/providers");
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

  it("uses distinct CSRF-bound endpoints for rename, archive, and restore", async () => {
    const workspace = { id: "workspace", name: "Renamed", state: "active", revision: 2, role: "owner", relationship: "owner", capabilities: [] };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(workspace))
      .mockResolvedValueOnce(jsonResponse({ ...workspace, state: "archived" }))
      .mockResolvedValueOnce(jsonResponse(workspace));
    vi.stubGlobal("fetch", fetchMock);

    await renameWorkspace("workspace", "Renamed", "csrf");
    await archiveWorkspace("workspace", "csrf");
    await restoreWorkspace("workspace", "csrf");

    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      expect.stringContaining("/v1/workspaces/workspace"),
      expect.stringContaining("/v1/workspaces/workspace/archive"),
      expect.stringContaining("/v1/workspaces/workspace/restore"),
    ]);
    expect(fetchMock.mock.calls.map(([, init]) => (init as RequestInit).method)).toEqual(["PATCH", "POST", "POST"]);
    for (const [, init] of fetchMock.mock.calls as Array<[string, RequestInit]>) {
      expect((init.headers as Headers).get("X-Baley-CSRF")).toBe("csrf");
    }
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

  it("uses membership-bound MCP login-link endpoints", async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse({
        id: "link", workspaceId: "workspace", agentActorId: "agent",
        status: "pending", expiresAt: "2026-09-02T12:00:00Z",
      }))
      .mockResolvedValueOnce(jsonResponse({ callbackUrl: "http://127.0.0.1:8090/mcp-login/callback?connectionId=link&code=one-time" }))
      .mockResolvedValueOnce(jsonResponse({ status: "linked" }));
    vi.stubGlobal("fetch", fetchMock);

    await fetchMCPLoginLink("workspace", "link");
    await completeMCPGatewayLogin("workspace", "link", "csrf");

    expect(fetchMock.mock.calls.map(([url]) => String(url))).toEqual([
      expect.stringContaining("/v1/workspaces/workspace/mcp-login-links/link"),
      expect.stringContaining("/v1/workspaces/workspace/mcp-login-links/link/complete"),
      "http://127.0.0.1:8090/mcp-login/callback?connectionId=link&code=one-time",
    ]);
    expect((fetchMock.mock.calls[1]?.[1] as RequestInit).method).toBe("POST");
    expect(((fetchMock.mock.calls[1]?.[1] as RequestInit).headers as Headers).get("X-Baley-CSRF")).toBe("csrf");
		expect((fetchMock.mock.calls[2]?.[1] as RequestInit).credentials).toBe("omit");
  });

	it("rejects non-loopback MCP completion callbacks", async () => {
		const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse({ callbackUrl: "https://attacker.example/mcp-login/callback?code=stolen" }));
		vi.stubGlobal("fetch", fetchMock);
		await expect(completeMCPGatewayLogin("workspace", "link", "csrf")).rejects.toThrow("unsafe local Gateway callback URL");
		expect(fetchMock).toHaveBeenCalledTimes(1);
	});

  it("issues, references, and revokes a non-secret browser approval grant with CSRF", async () => {
    const command = {
      name: "task.confirm",
      arguments: { workspaceId: "workspace", taskId: 158 },
      envelope: { idempotencyKey: "approve-158", expectedWorkspaceRevision: 7, executedByActorId: "human" },
    };
    const grant = { id: "11111111-1111-4111-8111-111111111111", expiresAt: "2026-08-31T06:00:00Z" };
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(jsonResponse(grant, 201))
      .mockResolvedValueOnce(jsonResponse({ commandId: "command", workspaceRevision: 8, eventIds: [] }))
      .mockResolvedValueOnce(jsonResponse(undefined, 204));
    vi.stubGlobal("fetch", fetchMock);

    const issued = await issueApprovalGrant("workspace", { command, acknowledgedWarningCodes: [], proceedReason: "" }, "csrf");
    await executeCommand({ ...command, envelope: { ...command.envelope, approvalGrantId: issued.id } }, "csrf");
    await revokeApprovalGrant("workspace", issued.id, "csrf");

    for (const [, init] of fetchMock.mock.calls as Array<[string, RequestInit]>) {
      expect(init.credentials).toBe("include");
      expect((init.headers as Headers).get("X-Baley-CSRF")).toBe("csrf");
      expect(String(init.body ?? "")).not.toContain("approvalGrantToken");
      expect(String(init.body ?? "")).not.toContain("humanApprovalAttestation");
    }
    expect(fetchMock.mock.calls.map(([url]) => url)).toEqual([
      expect.stringContaining("/v1/workspaces/workspace/approval-grants"),
      expect.stringContaining("/v1/commands/execute"),
      expect.stringContaining(`/v1/workspaces/workspace/approval-grants/${grant.id}`),
    ]);
  });
});
