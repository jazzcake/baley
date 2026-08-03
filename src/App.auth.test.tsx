// @vitest-environment jsdom

import React from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  attachExistingAccount,
  approveMCPConnection,
  createWorkspace,
  createWorkspaceMember,
  disableMemberAccount,
  fetchSession,
  fetchMCPConnection,
  fetchWorkspaceMembers,
  fetchWorkspaces,
  issueApprovalGrant,
  login,
  logout,
  previewCommand,
  removeWorkspaceMember,
  resetMemberPassword,
  revokeApprovalGrant,
  transferWorkspaceOwnership,
  updateWorkspaceMember,
} from "./api/auth";
import { APIError } from "./api/http";
import { fetchGraph } from "./api/client";
import App from "./App";
import { pilotReadyFixture } from "./fixtures/pilot-ready";
import { layoutGraph } from "./graph/layout";
import type { WorkspaceFixture } from "./domain/model";

vi.mock("./api/auth", () => ({
  fetchSession: vi.fn(),
  fetchWorkspaces: vi.fn(),
  login: vi.fn(),
  logout: vi.fn(),
  createWorkspace: vi.fn(),
  fetchWorkspaceMembers: vi.fn(),
  createWorkspaceMember: vi.fn(),
  updateWorkspaceMember: vi.fn(),
  removeWorkspaceMember: vi.fn(),
  transferWorkspaceOwnership: vi.fn(),
  attachExistingAccount: vi.fn(),
  fetchMCPConnection: vi.fn(),
  approveMCPConnection: vi.fn(),
  disableMemberAccount: vi.fn(),
  resetMemberPassword: vi.fn(),
  previewCommand: vi.fn(),
  issueApprovalGrant: vi.fn(),
  revokeApprovalGrant: vi.fn(),
}));
vi.mock("./api/client", () => ({ fetchGraph: vi.fn() }));
vi.mock("./graph/layout", () => ({
  NODE_WIDTH: 190,
  NODE_HEIGHT: 110,
  laneBandRect: vi.fn(),
  laneLabelTop: vi.fn(),
  layoutGraph: vi.fn(async () => ({
    taskPositions: new Map(),
    gatePositions: new Map(),
    phaseRects: [],
    lanePositions: new Map(),
    laneHeights: new Map(),
    width: 1200,
    height: 740,
  })),
}));
vi.mock("@xyflow/react", () => ({
  Background: () => null,
  Panel: ({ children, ...props }: { children: React.ReactNode }) => React.createElement("div", props, children),
  ReactFlow: ({ children }: { children: React.ReactNode }) => React.createElement("div", { "data-testid": "graph" }, children),
  ViewportPortal: ({ children }: { children: React.ReactNode }) => React.createElement(React.Fragment, null, children),
  useReactFlow: () => ({ setCenter: vi.fn(() => Promise.resolve()) }),
  useStore: (selector: (state: unknown) => unknown) => selector({ transform: [0, 0, 1], minZoom: 0.55, maxZoom: 1.55, width: 1200, height: 700, panZoom: { setViewport: vi.fn() } }),
  useStoreApi: () => ({ getState: () => ({ transform: [0, 0, 1], minZoom: 0.55, maxZoom: 1.55, width: 1200, height: 700, nodeLookup: new Map(), panZoom: { setViewport: vi.fn() } }), setState: vi.fn() }),
}));

const account = { id: "account", actorId: "owner-actor", loginId: "owner", displayName: "Pilot Owner" };
const session = { account, csrfToken: "csrf", expiresAt: "2026-07-28T12:00:00Z" };
const memberships = [
  { id: "w1", name: "Workspace One", state: "active", revision: 1, role: "owner" as const, relationship: "owner" as const, capabilities: ["workspace:read", "workspace:admin"] },
  { id: "w2", name: "Workspace Two", state: "active", revision: 1, role: "operator" as const, relationship: "participant" as const, capabilities: ["workspace:read", "workspace:operate"] },
];

function graph(id: string, name: string): WorkspaceFixture {
  return { ...pilotReadyFixture, workspace: { ...pilotReadyFixture.workspace, id, name } };
}

describe("authenticated Workspace routing", () => {
  beforeEach(() => {
    vi.stubEnv("VITE_BALEY_AUTH_MODE", "enforced");
    vi.mocked(fetchSession).mockResolvedValue(session);
    vi.mocked(fetchWorkspaces).mockResolvedValue(memberships);
    vi.mocked(logout).mockResolvedValue(undefined);
    vi.mocked(createWorkspace).mockResolvedValue({
      id: "w3",
      name: "Day Tripper Pilot",
      state: "active",
      revision: 1,
      role: "owner",
      relationship: "owner",
      capabilities: ["workspace:read", "workspace:admin"],
    });
    vi.mocked(fetchWorkspaceMembers).mockResolvedValue([]);
    vi.mocked(createWorkspaceMember).mockResolvedValue({
      actorId: "new", displayName: "New", role: "operator", relationship: "participant", active: true,
    });
    vi.mocked(updateWorkspaceMember).mockResolvedValue({
      actorId: "participant", displayName: "Participant", role: "operator", relationship: "participant", active: true,
    });
    vi.mocked(removeWorkspaceMember).mockResolvedValue(undefined);
    vi.mocked(transferWorkspaceOwnership).mockResolvedValue(undefined);
    vi.mocked(attachExistingAccount).mockResolvedValue({
      actorId: "existing", accountId: "existing-account", displayName: "Existing", role: "operator", relationship: "participant", active: true,
    });
    vi.mocked(disableMemberAccount).mockResolvedValue(undefined);
    vi.mocked(resetMemberPassword).mockResolvedValue(undefined);
    vi.mocked(revokeApprovalGrant).mockResolvedValue(undefined);
    vi.mocked(fetchMCPConnection).mockResolvedValue({
      id: "connection-1",
      workspaceId: "w1",
      agentActorId: "codex-operator",
      status: "pending",
      expiresAt: "2026-08-03T13:00:00Z",
    });
    vi.mocked(approveMCPConnection).mockResolvedValue(undefined);
    vi.mocked(previewCommand).mockResolvedValue({
      commandHash: "sha256:command",
      expectedWorkspaceRevision: 7,
      requiredCapability: "task:approve",
      projectedDiff: { status: ["implemented", "confirmed"] },
      errors: [{ code: "human_approval_required", message: "human approval required" }],
      warnings: [],
      advisories: [],
      decisionSnapshotHash: "sha256:snapshot",
      entityType: "task",
      entityId: "123",
    });
    vi.mocked(issueApprovalGrant).mockResolvedValue({
      id: "grant-id",
      grantToken: "one-time-secret-token",
      expiresAt: "2099-07-28T12:00:00Z",
      commandHash: "sha256:command",
      workspaceRevision: 7,
    });
    vi.mocked(layoutGraph).mockResolvedValue({
      taskPositions: new Map(),
      gatePositions: new Map(),
      phaseRects: [],
      lanePositions: new Map(),
      laneHeights: new Map(),
      width: 1200,
      height: 740,
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
    vi.unstubAllEnvs();
  });

  it("clears the password input after submit and enters the account Workspace list", async () => {
    vi.mocked(fetchSession).mockRejectedValueOnce(new APIError("authentication required", 401, "unauthenticated"));
    vi.mocked(login).mockResolvedValue(session);
    window.history.replaceState({}, "", "/login");
    render(<App />);

    const loginId = await screen.findByLabelText("아이디");
    const password = screen.getByLabelText("암호") as HTMLInputElement;
    fireEvent.change(loginId, { target: { value: "owner" } });
    fireEvent.change(password, { target: { value: "a sufficiently long password" } });
    fireEvent.submit(password.closest("form")!);

    expect(password.value).toBe("");
    await waitFor(() => expect(login).toHaveBeenCalledWith("owner", "a sufficiently long password"));
    expect(await screen.findByRole("heading", { name: "Pilot Owner님의 Workspace" })).toBeTruthy();
  });

  it("lets the Owner approve a one-time Codex Operator connection", async () => {
    window.history.replaceState({}, "", "/workspaces/w1/mcp-connect/connection-1");
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));

    render(<App />);

    expect(await screen.findByRole("heading", { name: "Codex Operator 연결" })).toBeTruthy();
    expect(screen.getByText(/사람 전용 승인은 할 수 없습니다/)).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Operator 연결 승인" }));

    await waitFor(() => expect(approveMCPConnection).toHaveBeenCalledWith("w1", "connection-1", "csrf"));
    expect(await screen.findByText("연결되었습니다.")).toBeTruthy();
  });

  it("offers Workspace creation from the account Workspace list", async () => {
    const createdMembership = {
      id: "w3",
      name: "Day Tripper Pilot",
      state: "active",
      revision: 1,
      role: "owner" as const,
      relationship: "owner" as const,
      capabilities: ["workspace:read", "workspace:admin"],
    };
    vi.mocked(fetchWorkspaces).mockResolvedValue([...memberships, createdMembership]);
    vi.mocked(createWorkspace).mockResolvedValue(createdMembership);
    vi.mocked(fetchGraph).mockResolvedValue(graph("w3", "Day Tripper Pilot"));
    window.history.replaceState({}, "", "/workspaces");
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "새 Workspace" }));
    const name = screen.getByLabelText("Workspace 이름");
    fireEvent.change(name, { target: { value: "Day Tripper Pilot" } });
    fireEvent.submit(name.closest("form")!);

    await waitFor(() => expect(createWorkspace).toHaveBeenCalledWith(
      { workspaceId: expect.any(String), name: "Day Tripper Pilot" },
      "csrf",
    ));
    expect(await screen.findByRole("heading", { name: "Day Tripper Pilot" })).toBeTruthy();
  });

  it("closes the Workspace menu before opening the creation form", async () => {
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));
    window.history.replaceState({}, "", "/workspaces/w1");
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Workspace One Workspace 전환" }));
    expect(screen.getByRole("menu", { name: "Workspace 전환" })).toBeTruthy();
    fireEvent.click(screen.getByRole("menuitem", { name: /새 Workspace/ }));

    expect(screen.queryByRole("menu", { name: "Workspace 전환" })).toBeNull();
    expect(screen.getByLabelText("Workspace 이름")).toBeTruthy();
  });

  it("aborts an old poll and prevents its late graph response from replacing the selected Workspace", async () => {
    let resolveOldPoll: ((value: WorkspaceFixture) => void) | undefined;
    let oldPollSignal: AbortSignal | undefined;
    let w1Calls = 0;
    vi.mocked(fetchGraph).mockImplementation((workspaceId, signal) => {
      if (workspaceId === "w1" && ++w1Calls === 1) return Promise.resolve(graph("w1", "Workspace One"));
      if (workspaceId === "w1") {
        oldPollSignal = signal;
        return new Promise((resolve) => { resolveOldPoll = resolve; });
      }
      return Promise.resolve(graph("w2", "Workspace Two"));
    });
    window.history.replaceState({}, "", "/workspaces/w1");
    render(<App />);

    expect(await screen.findByRole("heading", { name: "Workspace One" })).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Pilot Owner 계정 메뉴" }));
    expect(screen.getByRole("menuitem", { name: "멤버 관리" })).toBeTruthy();
    window.dispatchEvent(new Event("focus"));
    await waitFor(() => expect(w1Calls).toBe(2));

    fireEvent.click(screen.getByRole("button", { name: "Workspace One Workspace 전환" }));
    fireEvent.click(screen.getByRole("menuitemradio", { name: /Workspace Two/ }));
    expect(await screen.findByRole("heading", { name: "Workspace Two" })).toBeTruthy();
    expect(oldPollSignal?.aborted).toBe(true);
    fireEvent.click(screen.getByRole("button", { name: "Pilot Owner 계정 메뉴" }));
    expect(screen.queryByRole("menuitem", { name: "멤버 관리" })).toBeNull();

    resolveOldPoll?.(graph("w1", "Stale Workspace One"));
    await Promise.resolve();
    expect(screen.getByRole("heading", { name: "Workspace Two" })).toBeTruthy();
    expect(document.querySelector("[data-workspace-id='w2']")).toBeTruthy();
    expect(document.querySelector("[data-workspace-id='w1']")).toBeNull();
  });

  it("loads member administration only for an Owner and sends the CSRF-bound role update", async () => {
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));
    vi.mocked(fetchWorkspaceMembers).mockResolvedValue([
      { actorId: "owner-actor", accountId: "account", displayName: "Pilot Owner", role: "owner", relationship: "owner", active: true },
      { actorId: "participant", accountId: "p", displayName: "Participant", role: "viewer", relationship: "participant", active: true },
    ]);
    window.history.replaceState({}, "", "/workspaces/w1");
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Pilot Owner 계정 메뉴" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "멤버 관리" }));
    expect(await screen.findByRole("dialog", { name: "Workspace One 멤버" })).toBeTruthy();
    fireEvent.change(screen.getByLabelText("Participant 역할"), { target: { value: "operator" } });

    await waitFor(() => expect(updateWorkspaceMember).toHaveBeenCalledWith(
      "w1",
      "participant",
      { role: "operator" },
      "csrf",
    ));
  });

  it("lets a participant with an approval capability issue grants without exposing member administration", async () => {
    vi.mocked(fetchWorkspaces).mockResolvedValue([{
      id: "w3",
      name: "Approver Workspace",
      state: "active",
      revision: 1,
      role: "approver",
      relationship: "participant",
      capabilities: ["workspace:read", "task:approve"],
    }]);
    vi.mocked(fetchGraph).mockResolvedValue(graph("w3", "Approver Workspace"));
    window.history.replaceState({}, "", "/workspaces/w3");
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Pilot Owner 계정 메뉴" }));
    expect(screen.getByRole("menuitem", { name: "승인 Grant 발급" })).toBeTruthy();
    expect(screen.queryByRole("menuitem", { name: "멤버 관리" })).toBeNull();
  });

  it("separates existing-account attach from account creation and clears an admin reset password immediately", async () => {
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));
    vi.mocked(fetchWorkspaceMembers).mockResolvedValue([
      { actorId: "owner-actor", accountId: "account", displayName: "Pilot Owner", role: "owner", relationship: "owner", active: true },
      { actorId: "participant", accountId: "p", displayName: "Participant", role: "viewer", relationship: "participant", active: true },
    ]);
    window.history.replaceState({}, "", "/workspaces/w1");
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Pilot Owner 계정 메뉴" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "멤버 관리" }));
    const existingLogin = await screen.findByLabelText("기존 로그인 아이디");
    fireEvent.change(existingLogin, { target: { value: "existing-user" } });
    fireEvent.submit(existingLogin.closest("form")!);
    await waitFor(() => expect(attachExistingAccount).toHaveBeenCalledWith(
      "w1",
      { loginId: "existing-user", role: "operator" },
      "csrf",
    ));
    expect(createWorkspaceMember).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "암호 재설정" }));
    const newPassword = screen.getByLabelText("새 암호") as HTMLInputElement;
    fireEvent.change(newPassword, { target: { value: "a new sufficiently long password" } });
    fireEvent.submit(newPassword.closest("form")!);
    expect(newPassword.value).toBe("");
    await waitFor(() => expect(resetMemberPassword).toHaveBeenCalledWith(
      "w1",
      "participant",
      "a new sufficiently long password",
      "csrf",
    ));

    fireEvent.click(screen.getByRole("button", { name: "계정 비활성화" }));
    await waitFor(() => expect(disableMemberAccount).toHaveBeenCalledWith("w1", "participant", "csrf"));
  });

  it("issues a preview-bound approval grant and removes the token from the DOM after one copy", async () => {
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));
    const localStorageSetItem = vi.fn();
    Object.defineProperty(window, "localStorage", {
      configurable: true,
      value: {
        getItem: vi.fn(() => null),
        setItem: localStorageSetItem,
        removeItem: vi.fn(),
        clear: vi.fn(),
        key: vi.fn(() => null),
        length: 0,
      },
    });
    vi.mocked(previewCommand).mockResolvedValue({
      commandHash: "sha256:command",
      expectedWorkspaceRevision: 7,
      requiredCapability: "task:approve",
      projectedDiff: { status: ["implemented", "confirmed"] },
      errors: [{ code: "human_approval_required", message: "human approval required" }],
      warnings: [{ code: "dangling_path", message: "terminal topology warning" }],
      advisories: [],
      decisionSnapshotHash: "sha256:snapshot",
      entityType: "task",
      entityId: "123",
    });
    const clipboardWrite = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText: clipboardWrite },
    });
    window.history.replaceState({}, "", "/workspaces/w1");
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Pilot Owner 계정 메뉴" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "승인 Grant 발급" }));
    const commandInput = screen.getByLabelText("Typed command JSON");
    const command = {
      name: "task.confirm",
      arguments: { workspaceId: "w1", taskId: 123 },
      envelope: { idempotencyKey: "command-key", expectedWorkspaceRevision: 7 },
    };
    fireEvent.change(commandInput, { target: { value: JSON.stringify(command) } });
    fireEvent.click(screen.getByRole("button", { name: "Fresh preview 확인" }));

    expect(await screen.findByText("sha256:command")).toBeTruthy();
    expect(screen.getByText("sha256:snapshot")).toBeTruthy();
    const issueButton = screen.getByRole("button", { name: "이 명령의 Grant 발급" });
    expect(issueButton.hasAttribute("disabled")).toBe(true);
    fireEvent.click(screen.getByRole("checkbox", { name: /dangling_path/ }));
    fireEvent.change(screen.getByLabelText("진행 사유"), { target: { value: "의도된 topology 경고를 확인함" } });
    expect(issueButton.hasAttribute("disabled")).toBe(false);
    fireEvent.click(issueButton);

    const executeInput = await screen.findByLabelText("발급된 approval grant MCP execute 입력");
    await waitFor(() => expect(executeInput.textContent).toContain("one-time-secret-token"));
    expect(issueApprovalGrant).toHaveBeenCalledWith("w1", {
      command,
      acknowledgedWarningCodes: ["dangling_path"],
      proceedReason: "의도된 topology 경고를 확인함",
    }, "csrf");
    expect(localStorageSetItem).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "MCP execute 입력을 복사하고 화면에서 폐기" }));

    await waitFor(() => expect(clipboardWrite).toHaveBeenCalledWith(JSON.stringify({
      workspaceId: "w1",
      taskId: 123,
      idempotencyKey: "command-key",
      expectedWorkspaceRevision: 7,
      acknowledgedWarningCodes: ["dangling_path"],
      proceedReason: "의도된 topology 경고를 확인함",
      approvalGrantToken: "one-time-secret-token",
    }, null, 2)));
    expect(executeInput.textContent).toBe("");
    expect(screen.getByText("token은 화면에서 폐기되었습니다.")).toBeTruthy();
  });

  it("keeps logout inside the account menu and ends the authenticated session", async () => {
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));
    window.history.replaceState({}, "", "/workspaces/w1");
    render(<App />);

    const accountMenu = await screen.findByRole("button", { name: "Pilot Owner 계정 메뉴" });
    expect(screen.queryByRole("menuitem", { name: "로그아웃" })).toBeNull();

    fireEvent.click(accountMenu);
    fireEvent.click(screen.getByRole("menuitem", { name: "로그아웃" }));

    await waitFor(() => expect(logout).toHaveBeenCalledWith("csrf"));
    expect(await screen.findByRole("heading", { name: "로그인" })).toBeTruthy();
  });

  it("supports keyboard navigation and Escape in the Workspace menu", async () => {
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));
    window.history.replaceState({}, "", "/workspaces/w1");
    render(<App />);

    const trigger = await screen.findByRole("button", { name: "Workspace One Workspace 전환" });
    fireEvent.click(trigger);
    const menu = screen.getByRole("menu", { name: "Workspace 전환" });
    const items = screen.getAllByRole("menuitemradio");
    await waitFor(() => expect(document.activeElement).toBe(items[0]));

    fireEvent.keyDown(menu, { key: "ArrowDown" });
    expect(document.activeElement).toBe(items[1]);
    fireEvent.keyDown(window, { key: "Escape" });

    expect(screen.queryByRole("menu", { name: "Workspace 전환" })).toBeNull();
    expect(document.activeElement).toBe(trigger);
  });

  it("returns focus after selecting the current Workspace", async () => {
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));
    window.history.replaceState({}, "", "/workspaces/w1");
    render(<App />);

    const trigger = await screen.findByRole("button", { name: "Workspace One Workspace 전환" });
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole("menuitemradio", { name: /Workspace One/ }));

    await waitFor(() => expect(screen.queryByRole("menu", { name: "Workspace 전환" })).toBeNull());
    expect(document.activeElement).toBe(trigger);
  });

  it("supports account menu keyboard navigation and outside-click dismissal", async () => {
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));
    window.history.replaceState({}, "", "/workspaces/w1");
    render(<App />);

    const trigger = await screen.findByRole("button", { name: "Pilot Owner 계정 메뉴" });
    fireEvent.click(trigger);
    const menu = screen.getByRole("menu", { name: "계정 메뉴" });
    const items = screen.getAllByRole("menuitem");
    await waitFor(() => expect(document.activeElement).toBe(items[0]));

    fireEvent.keyDown(menu, { key: "End" });
    expect(document.activeElement).toBe(items.at(-1));
    fireEvent.keyDown(menu, { key: "Home" });
    expect(document.activeElement).toBe(items[0]);

    fireEvent.pointerDown(document.body);
    expect(screen.queryByRole("menu", { name: "계정 메뉴" })).toBeNull();
  });

  it("keeps the account menu open with an error when logout fails", async () => {
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));
    vi.mocked(logout).mockRejectedValueOnce(new Error("Logout service unavailable"));
    window.history.replaceState({}, "", "/workspaces/w1");
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Pilot Owner 계정 메뉴" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "로그아웃" }));

    expect((await screen.findByRole("alert")).textContent).toContain("Logout service unavailable");
    expect(screen.getByRole("menu", { name: "계정 메뉴" })).toBeTruthy();
    expect(screen.getByRole("heading", { name: "Workspace One" })).toBeTruthy();
  });
});
