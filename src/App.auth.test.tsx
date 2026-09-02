// @vitest-environment jsdom

import React from "react";
import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  attachExistingAccount,
  archiveWorkspace,
  approveMCPConnection,
  createWorkspace,
  createWorkspaceMember,
  disableMemberAccount,
  fetchSession,
  fetchMCPConnection,
  fetchWorkspaceMembers,
  fetchWorkspaces,
	fetchOIDCProviders,
  logout,
  renameWorkspace,
  removeWorkspaceMember,
  resetMemberPassword,
  restoreWorkspace,
  transferWorkspaceOwnership,
  updateWorkspaceMember,
  executeCommand,
  issueApprovalGrant,
  previewCommand,
  revokeApprovalGrant,
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
	fetchOIDCProviders: vi.fn().mockResolvedValue([]),
  logout: vi.fn(),
  createWorkspace: vi.fn(),
  fetchWorkspaceMembers: vi.fn(),
  createWorkspaceMember: vi.fn(),
  updateWorkspaceMember: vi.fn(),
  removeWorkspaceMember: vi.fn(),
  transferWorkspaceOwnership: vi.fn(),
  attachExistingAccount: vi.fn(),
	archiveWorkspace: vi.fn(),
	renameWorkspace: vi.fn(),
	restoreWorkspace: vi.fn(),
	beginOIDCLink: vi.fn(),
  fetchMCPConnection: vi.fn(),
  approveMCPConnection: vi.fn(),
  disableMemberAccount: vi.fn(),
  resetMemberPassword: vi.fn(),
  previewCommand: vi.fn(),
  issueApprovalGrant: vi.fn(),
  executeCommand: vi.fn(),
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
    vi.mocked(renameWorkspace).mockResolvedValue({ id: "w1", name: "Renamed Workspace", state: "active", revision: 2, role: "owner", relationship: "owner", capabilities: ["workspace:read", "workspace:admin"] });
    vi.mocked(archiveWorkspace).mockResolvedValue({ id: "w1", name: "Workspace One", state: "archived", revision: 2, role: "owner", relationship: "owner", capabilities: ["workspace:read", "workspace:admin"] });
    vi.mocked(restoreWorkspace).mockResolvedValue({ id: "w1", name: "Workspace One", state: "active", revision: 3, role: "owner", relationship: "owner", capabilities: ["workspace:read", "workspace:admin"] });
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
    vi.mocked(previewCommand).mockResolvedValue({
      commandHash: "sha256:confirm",
      expectedWorkspaceRevision: 1,
      requiredCapability: "task:approve",
      projectedDiff: {},
      errors: [{ code: "human_approval_required", message: "Human confirmation required" }],
      warnings: [],
      advisories: [],
    });
    vi.mocked(issueApprovalGrant).mockResolvedValue({ id: "grant", expiresAt: "2026-09-02T00:01:00Z", commandHash: "sha256:confirm", workspaceRevision: 1 });
    vi.mocked(executeCommand).mockResolvedValue({ commandId: "command", workspaceRevision: 2, eventIds: [] });
    vi.mocked(revokeApprovalGrant).mockResolvedValue(undefined);
    vi.mocked(fetchMCPConnection).mockResolvedValue({
      id: "connection-1",
      workspaceId: "w1",
      agentActorId: "codex-operator",
      status: "pending",
      expiresAt: "2026-08-03T13:00:00Z",
    });
    vi.mocked(approveMCPConnection).mockResolvedValue(undefined);
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

  it("shows the login landing page at the public root", async () => {
    vi.mocked(fetchSession).mockRejectedValueOnce(new APIError("authentication required", 401, "unauthenticated"));
    window.history.replaceState({}, "", "/");
    render(<App />);
    expect(await screen.findByRole("heading", { name: "안전한 작업 흐름, 명확한 사람의 승인" })).toBeTruthy();
  });

  it("shows Google as the only login action without password inputs", async () => {
    vi.mocked(fetchSession).mockRejectedValueOnce(new APIError("authentication required", 401, "unauthenticated"));
    vi.mocked(fetchOIDCProviders).mockResolvedValueOnce([{ id: "google", label: "Google" }]);
    window.history.replaceState({}, "", "/login");
    render(<App />);

    expect(await screen.findByRole("button", { name: "Google로 계속" })).toBeTruthy();
    expect(screen.queryByLabelText("아이디")).toBeNull();
    expect(screen.queryByLabelText("암호")).toBeNull();
  });

  it("offers configured internal OIDC providers alongside Google", async () => {
    vi.mocked(fetchSession).mockRejectedValueOnce(new APIError("authentication required", 401, "unauthenticated"));
    vi.mocked(fetchOIDCProviders).mockResolvedValueOnce([{ id: "internal", label: "Keycloak" }, { id: "google", label: "Google" }]);
    window.history.replaceState({}, "", "/login");
    render(<App />);

    expect(await screen.findByRole("button", { name: /Google/ })).toBeTruthy();
    expect(screen.getByRole("button", { name: /Keycloak/ })).toBeTruthy();
  });

  it("lets a user log out from the Workspace chooser", async () => {
    window.history.replaceState({}, "", "/workspaces");
    render(<App />);
    const logoutButton = await screen.findByRole("button", { name: "Logout" });
    fireEvent.click(logoutButton);
    await waitFor(() => expect(logout).toHaveBeenCalledWith("csrf"));
    expect(await screen.findByRole("heading", { name: "로그인" })).toBeTruthy();
  });

  it("keeps an owner's final archived Workspace recoverable from the chooser", async () => {
    vi.mocked(fetchWorkspaces).mockResolvedValueOnce([{
      id: "w1", name: "Workspace One", state: "archived", revision: 2,
      role: "owner", relationship: "owner", capabilities: ["workspace:read", "workspace:admin"],
    }]);
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));
    window.history.replaceState({}, "", "/workspaces");
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Workspace One Workspace commands" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Restore" }));
    await waitFor(() => expect(restoreWorkspace).toHaveBeenCalledWith("w1", "csrf"));
  });

  it("automatically connects a signed-in Operator's local Codex gateway", async () => {
    window.history.replaceState({}, "", "/workspaces/w1/mcp-connect/connection-1");
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));

    render(<App />);

    expect(await screen.findByRole("heading", { name: "Codex Operator 연결" })).toBeTruthy();
    await waitFor(() => expect(approveMCPConnection).toHaveBeenCalledWith("w1", "connection-1", "csrf"));
    expect(await screen.findByText("연결되었습니다.")).toBeTruthy();
  });

  it("copies the Workspace UUID with the HTTP selection fallback and acknowledges it", async () => {
    vi.mocked(fetchGraph).mockResolvedValue(graph("w1", "Workspace One"));
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: undefined });
    const execCommand = vi.fn(() => true);
    Object.defineProperty(document, "execCommand", { configurable: true, value: execCommand });
    window.history.replaceState({}, "", "/workspaces/w1");
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Copy Workspace UUID" }));

    await waitFor(() => expect(execCommand).toHaveBeenCalledWith("copy"));
    expect(screen.getByRole("status").textContent).toBe("UUID copied");
  });

  it("offers the explicit confirmation flow only for an implemented Task", async () => {
    const implementedGraph = graph("w1", "Workspace One");
    implementedGraph.tasks = implementedGraph.tasks.map((item) => item.id === "pilot-ui" ? {
      ...item,
      status: "implemented",
      implementedAssessment: "Implementation and independent review passed.",
    } : item);
    vi.mocked(fetchGraph).mockResolvedValue(implementedGraph);
    window.history.replaceState({}, "", "/workspaces/w1?task=pilot-ui");
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Confirm task" }));
    fireEvent.click(await screen.findByRole("button", { name: "Confirm task once" }));

    await waitFor(() => expect(executeCommand).toHaveBeenCalled());
    expect(await screen.findByText("confirmed")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Confirm task" })).toBeNull();
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

  it("does not expose out-of-band approval controls to an Approver", async () => {
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
    expect(screen.queryByRole("menuitem", { name: "승인 Grant 발급" })).toBeNull();
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

  it("opens Owner Workspace commands from its card and submits rename", async () => {
    window.history.replaceState({}, "", "/workspaces");
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Workspace One Workspace commands" }));
    expect(screen.getByRole("menu", { name: "Workspace One Workspace commands" })).toBeTruthy();

    fireEvent.click(screen.getByRole("menuitem", { name: "Rename" }));
    const input = screen.getByLabelText("Workspace name");
    expect(screen.queryByRole("menu", { name: "Workspace One Workspace commands" })).toBeNull();
    await waitFor(() => expect(document.activeElement).toBe(input));
    fireEvent.change(input, { target: { value: "Renamed Workspace" } });
    fireEvent.submit(input.closest("form")!);

    await waitFor(() => expect(renameWorkspace).toHaveBeenCalledWith("w1", "Renamed Workspace", "csrf"));
  });

  it("clears the chooser immediately after archive revokes the current session", async () => {
    window.history.replaceState({}, "", "/workspaces");
    render(<App />);

    fireEvent.click(await screen.findByRole("button", { name: "Workspace One Workspace commands" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Archive" }));
    const dialog = screen.getByRole("dialog", { name: "Archive Workspace One" });
    fireEvent.click(within(dialog).getByRole("button", { name: "Archive" }));

    await waitFor(() => expect(archiveWorkspace).toHaveBeenCalledWith("w1", "csrf"));
    await waitFor(() => expect(screen.queryByRole("button", { name: "Workspace One Workspace commands" })).toBeNull());
  });

  it("returns focus to a card's command trigger when an archive prompt closes with Escape", async () => {
    window.history.replaceState({}, "", "/workspaces");
    render(<App />);
    const trigger = await screen.findByRole("button", { name: "Workspace One Workspace commands" });
    fireEvent.click(trigger);
    fireEvent.click(screen.getByRole("menuitem", { name: "Archive" }));
    expect(screen.getByRole("dialog", { name: "Archive Workspace One" })).toBeTruthy();

    fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Archive Workspace One" })).toBeNull());
    await waitFor(() => expect(document.activeElement).toBe(trigger));
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
