import { useEffect, useRef, useState } from "react";
import { Archive, Check, ChevronDown, Ellipsis, KeyRound, LayoutGrid, LogOut, Pencil, Plus, RotateCcw, Settings, ShieldCheck, X } from "lucide-react";
import { useLocation, useNavigate } from "react-router-dom";
import {
	attachExistingAccount,
	archiveWorkspace,
	beginOIDCLink,
  linkMCPGateway,
  createWorkspace,
	createWorkspaceMember,
  disableMemberAccount,
	executeCommand,
	fetchOIDCProviders,
	fetchWorkspaceMembers,
  fetchMCPLoginLink,
	issueApprovalGrant,
	previewCommand,
  removeWorkspaceMember,
	resetMemberPassword,
	renameWorkspace,
	restoreWorkspace,
	revokeApprovalGrant,
  transferWorkspaceOwnership,
	updateWorkspaceMember,
	oidcLoginURL,
} from "../api/auth";
import type { CommandExecution, CommandPreview, CommandRequest, MCPLoginLink, OIDCProvider } from "../api/auth";
import { APIError } from "../api/http";
import { traceViewer } from "../debug/viewer-trace";
import type {
  Account,
  WorkspaceMember,
  WorkspaceMembership,
  WorkspaceRole,
} from "../auth/model";

export function MCPLoginLink({
  workspace,
  connectionId,
  csrfToken,
  onClose,
}: {
  workspace: WorkspaceMembership;
  connectionId: string;
  csrfToken: string;
  onClose: () => void;
}) {
  const [connection, setConnection] = useState<MCPLoginLink>();
  const [busy, setBusy] = useState(false);
  const [linked, setLinked] = useState(false);
  const [error, setError] = useState<string>();
  const activationStarted = useRef(false);
  const closeRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const controller = new AbortController();
    traceViewer("mcp-login:request", {
      event: "load",
      targetWorkspaceId: workspace.id,
      connectionId,
      authState: "authenticated",
    });
    void fetchMCPLoginLink(workspace.id, connectionId, controller.signal)
      .then((next) => {
        setConnection(next);
        setLinked(next.status === "linked");
        traceViewer("mcp-login:state", {
          targetWorkspaceId: workspace.id,
          connectionId,
          status: next.status,
        });
        window.requestAnimationFrame(() => traceViewer("mcp-login:dom", {
          targetWorkspaceId: workspace.id,
          connectionId,
          dialogPresent: Boolean(document.querySelector("[data-mcp-login-link-id]")),
        }));
        if (next.status === "pending" && !activationStarted.current) {
          activationStarted.current = true;
          void link(next.status);
        }
      })
      .catch((reason: unknown) => {
        if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : "연결 요청을 불러올 수 없습니다.");
      });
    window.setTimeout(() => closeRef.current?.focus(), 0);
    return () => controller.abort();
  }, [workspace.id, connectionId]);

  const link = async (currentStatus = connection?.status) => {
    setBusy(true);
    setError(undefined);
    traceViewer("mcp-login:link", {
      event: "authenticated-page-load",
      targetWorkspaceId: workspace.id,
      connectionId,
      calculatedTargetState: "linked",
      currentState: currentStatus,
    });
    try {
      await linkMCPGateway(workspace.id, connectionId, csrfToken);
      setLinked(true);
      setConnection((current) => current ? { ...current, status: "linked" } : current);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "로그인한 계정에 로컬 Gateway를 연결하지 못했습니다.");
    } finally {
      setBusy(false);
    }
  };

  return <div className="admin-overlay">
    <section className="mcp-login-dialog" role="dialog" aria-modal="true" aria-labelledby="mcp-login-title" data-mcp-login-link-id={connectionId}>
      <header>
        <div><span>MCP LOGIN</span><h2 id="mcp-login-title">로그인 계정으로 Codex 연결</h2></div>
        <button ref={closeRef} className="icon-button" type="button" aria-label="연결 창 닫기" onClick={onClose}><X size={18} /></button>
      </header>
      {error && <div className="form-error" role="alert">{error}</div>}
      {!error && !connection && <p>연결 요청을 확인하는 중입니다.</p>}
      {connection && !linked && <>
		<p><strong>{workspace.name}</strong>의 로그인 계정에 로컬 Codex를 연결합니다.</p>
		<p className="mcp-login-scope">MCP 접근 범위는 이 Workspace의 사용자 역할에서 결정되며, 사람 전용 확인 권한은 Agent에 전달되지 않습니다.</p>
		<dl><div><dt>사용자 역할</dt><dd>{workspace.role}</dd></div><div><dt>Agent</dt><dd><code>{connection.agentActorId}</code></dd></div></dl>
        <p role="status">{busy ? "Connecting local gateway…" : "Checking gateway connection…"}</p>
      </>}
      {linked && <div className="mcp-login-success" role="status">
		<Check size={22} /><div><strong>로그인 계정에 연결되었습니다.</strong><p>LLM 세션으로 돌아가 같은 요청을 다시 실행하면 됩니다.</p></div>
      </div>}
    </section>
  </div>;
}

function moveMenuFocus(event: React.KeyboardEvent<HTMLElement>) {
  if (!["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) return;
  const items = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>(
    "[role='menuitem']:not(:disabled),[role='menuitemradio']:not(:disabled)",
  ));
  if (items.length === 0) return;
  const currentIndex = items.indexOf(document.activeElement as HTMLButtonElement);
  const nextIndex = event.key === "Home"
    ? 0
    : event.key === "End"
      ? items.length - 1
      : event.key === "ArrowUp"
        ? (currentIndex <= 0 ? items.length - 1 : currentIndex - 1)
        : (currentIndex + 1) % items.length;
  event.preventDefault();
  items.forEach((item, index) => {
    item.tabIndex = index === nextIndex ? 0 : -1;
  });
  items[nextIndex]?.focus();
}

export function LoginScreen() {
  const location = useLocation();
  const [oidcProviders, setOIDCProviders] = useState<OIDCProvider[]>();

  useEffect(() => {
    const controller = new AbortController();
    void fetchOIDCProviders().then((items) => {
      if (!controller.signal.aborted) setOIDCProviders(items);
    }).catch(() => {
      if (!controller.signal.aborted) setOIDCProviders([]);
    });
    return () => controller.abort();
  }, []);

  const providers = oidcProviders ? [...oidcProviders].sort((left, right) => {
    if (left.id === "google") return -1;
    if (right.id === "google") return 1;
    return left.label.localeCompare(right.label);
  }) : [];
  const startOIDC = (provider: OIDCProvider) => {
    const returnTo = new URLSearchParams(location.search).get("returnTo");
    if (isMCPLoginPath(returnTo)) {
      window.sessionStorage.setItem("baley:mcp-post-login-path", returnTo);
    }
    traceViewer("oidc-login:event", { providerId: provider.id, authState: "anonymous", calculatedTarget: "authorization-redirect" });
    window.location.assign(oidcLoginURL(provider.id));
  };

  return <main className="auth-shell login-shell">
    <section className="auth-card login-card" aria-labelledby="login-title">
      <div className="brand-mark auth-brand">B</div>
      <span className="auth-kicker">BALEY ACCOUNT</span>
      <h1 id="login-title">로그인</h1>
      <p>Google 또는 조직의 OIDC 계정으로 안전하게 로그인하고, 권한이 있는 Workspace로 돌아가세요.</p>
      {!oidcProviders && <p className="login-provider-status" role="status">로그인 제공자를 확인하는 중…</p>}
      {providers.map((provider) => <button className="google-login-button" type="button" key={provider.id} onClick={() => startOIDC(provider)}>
        <span className="google-login-mark" aria-hidden="true">{provider.id === "google" ? "G" : provider.label.slice(0, 1)}</span>
        {provider.id === "google" ? "Google로 계속" : `${provider.label}로 계속`}
      </button>)}
      {oidcProviders?.length === 0 && <div className="form-error" role="alert">현재 사용할 수 있는 로그인 제공자가 없습니다.</div>}
      <p className="login-security-note"><ShieldCheck size={15} aria-hidden="true" /> OIDC 인증은 Workspace 권한이나 사람 전용 승인 권한을 변경하지 않습니다.</p>
    </section>
  </main>;
}

export function isMCPLoginPath(value: string | null | undefined): value is string {
  return Boolean(value && /^\/workspaces\/[0-9a-f-]{36}\/mcp-login\/[^/?#]+$/i.test(value));
}

export function LoginLanding() {
  const navigate = useNavigate();
  return <main className="auth-shell landing-shell">
    <section className="auth-card landing-card" aria-labelledby="landing-title">
      <div className="brand-mark auth-brand">B</div>
      <span className="auth-kicker">BALEY WORKSPACE</span>
      <h1 id="landing-title">안전한 작업 흐름, 명확한 사람의 승인</h1>
      <p>Baley Account로 로그인하면 권한이 있는 Workspace와 MCP 작업 환경으로 안전하게 돌아갑니다.</p>
      <button className="primary-button" type="button" onClick={() => {
        traceViewer("login-landing:event", { event: "start-click", calculatedTarget: "/login", renderedState: "landing" });
        navigate("/login");
      }}>로그인 시작</button>
    </section>
  </main>;
}

export function WorkspaceChooser({
	account,
	memberships,
	csrfToken,
	onLogout,
	onMembershipsChanged,
	onSessionExpired,
}: {
	account: Account;
	memberships: WorkspaceMembership[];
	csrfToken: string;
	onLogout: () => Promise<void>;
	onMembershipsChanged: () => Promise<void>;
	onSessionExpired: () => void;
}) {
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);
  const [createBusy, setCreateBusy] = useState(false);
  const [createError, setCreateError] = useState<string>();
	const [logoutBusy, setLogoutBusy] = useState(false);
	const [logoutError, setLogoutError] = useState<string>();
  const createTriggerRef = useRef<HTMLButtonElement>(null);
  const createNameRef = useRef<HTMLInputElement>(null);
  const activeMemberships = memberships.filter((item) => item.state === "active");
  const archivedOwnerMemberships = memberships.filter((item) => item.state === "archived" && item.role === "owner");

  useEffect(() => {
    if (!creating) return;
    const frame = window.requestAnimationFrame(() => {
      traceViewer("workspace-create:dom-rendered", {
        source: "workspace-chooser",
        renderedCreateOpenState: createNameRef.current ? "open" : "closed",
      });
    });
    return () => window.cancelAnimationFrame(frame);
  }, [creating]);


  const submitWorkspace = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = createNameRef.current?.value.trim() ?? "";
    if (!name || createBusy) return;
    setCreateBusy(true);
    setCreateError(undefined);
    try {
      const created = await createWorkspace({ workspaceId: crypto.randomUUID(), name }, csrfToken);
      traceViewer("workspace-create:completed", {
        source: "workspace-chooser",
        createdWorkspaceId: created.id,
        calculatedTargetPath: `/workspaces/${encodeURIComponent(created.id)}`,
      });
      await onMembershipsChanged();
      navigate(`/workspaces/${encodeURIComponent(created.id)}`);
    } catch (reason) {
      setCreateError(errorMessage(reason));
    } finally {
      setCreateBusy(false);
    }
  };

  return <main className="workspace-chooser-shell">
    <header>
      <div className="brand-mark">B</div>
      <div><span>BALEY WORKSPACES</span><h1>{account.displayName}님의 Workspace</h1></div>
      <div className="workspace-chooser-actions">
        <div className="workspace-chooser-create">
          <button
            ref={createTriggerRef}
            type="button"
            className="workspace-chooser-create-trigger"
            aria-expanded={creating}
            onClick={() => {
              traceViewer("workspace-create:event", {
                source: "workspace-chooser",
                event: "trigger-click",
                currentOpenState: creating,
                calculatedOpenState: !creating,
              });
              setCreating((current) => !current);
              setCreateError(undefined);
              window.setTimeout(() => createNameRef.current?.focus(), 0);
            }}
          ><Plus size={16} aria-hidden="true" /> 새 Workspace</button>
          {creating && <form className="workspace-create-popover workspace-chooser-create-popover" onSubmit={submitWorkspace}>
            <label htmlFor="workspace-chooser-create-name">Workspace 이름</label>
            <input ref={createNameRef} id="workspace-chooser-create-name" maxLength={120} required />
            {createError && <span className="form-error" role="alert">{createError}</span>}
            <span className="workspace-create-actions">
              <button type="button" onClick={() => {
                setCreating(false);
                setCreateError(undefined);
                createTriggerRef.current?.focus();
              }}>취소</button>
              <button type="submit" disabled={createBusy}>{createBusy ? "생성 중…" : "생성"}</button>
            </span>
          </form>}
        </div>
        <button type="button" className="workspace-chooser-create-trigger workspace-chooser-logout" disabled={logoutBusy} onClick={() => {
          if (logoutBusy) return;
          setLogoutBusy(true);
          setLogoutError(undefined);
          traceViewer("workspace-chooser-logout:event", { accountId: account.id, currentState: "authenticated", calculatedTargetState: "anonymous" });
          void onLogout().catch((reason: unknown) => {
            setLogoutError(reason instanceof Error ? reason.message : "로그아웃하지 못했습니다.");
          }).finally(() => setLogoutBusy(false));
        }}><LogOut size={16} />{logoutBusy ? "Logging out…" : "Logout"}</button>
      </div>
    </header>
    {logoutError && <div className="form-error workspace-chooser-logout-error" role="alert">{logoutError}</div>}
    {activeMemberships.length === 0
      ? <section className="chooser-empty"><h2>소속된 Workspace가 없습니다</h2><p>Owner에게 참여 요청을 보내주세요.</p></section>
      : <ul className="workspace-card-grid">
        {activeMemberships.map((membership) => <WorkspaceCard key={membership.id} membership={membership} csrfToken={csrfToken} onMembershipsChanged={onMembershipsChanged} onSessionExpired={onSessionExpired} />)}
      </ul>}
    {archivedOwnerMemberships.length > 0 && <section className="workspace-archived-restore" aria-label="Archived Workspaces">
      <h2>ARCHIVED WORKSPACES</h2>
      <ul className="workspace-card-grid">
        {archivedOwnerMemberships.map((workspace) => <WorkspaceCard key={workspace.id} membership={workspace} csrfToken={csrfToken} onMembershipsChanged={onMembershipsChanged} onSessionExpired={onSessionExpired} />)}
      </ul>
    </section>}
  </main>;
}

function WorkspaceCard({
  membership,
  csrfToken,
  onMembershipsChanged,
  onSessionExpired,
}: {
  membership: WorkspaceMembership;
  csrfToken: string;
  onMembershipsChanged: () => Promise<void>;
  onSessionExpired: () => void;
}) {
  const navigate = useNavigate();
  const [menuOpen, setMenuOpen] = useState(false);
  const [mode, setMode] = useState<"actions" | "rename" | "archive">("actions");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  const rootRef = useRef<HTMLLIElement>(null);
  const renameRef = useRef<HTMLInputElement>(null);
  const menuTriggerRef = useRef<HTMLButtonElement>(null);
  const archiveCancelRef = useRef<HTMLButtonElement>(null);
  const canManage = membership.role === "owner";

  useEffect(() => {
    if (!menuOpen) return;
    const frame = window.requestAnimationFrame(() => traceViewer("workspace-card-menu:dom-rendered", {
      targetWorkspaceId: membership.id,
      calculatedMode: mode,
      renderedMenu: rootRef.current?.querySelector("[role='menu']") ? "open" : "closed",
    }));
    const onPointerDown = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setMenuOpen(false);
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setMenuOpen(false);
      window.setTimeout(() => menuTriggerRef.current?.focus(), 0);
    };
    document.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.cancelAnimationFrame(frame);
      document.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [menuOpen, membership.id, mode]);

  useEffect(() => {
    if (!menuOpen) return;
    const focusTarget = mode === "rename" ? renameRef.current : mode === "archive" ? archiveCancelRef.current : undefined;
    if (focusTarget) window.setTimeout(() => focusTarget?.focus(), 0);
  }, [menuOpen, mode]);

  const openMenu = (event: React.MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
    const next = !menuOpen;
    traceViewer("workspace-card-menu:event", {
      event: "overflow-click",
      targetWorkspaceId: membership.id,
      calculatedOpenState: next,
      applicationState: { menuOpen, mode },
    });
    setMode("actions");
    setError(undefined);
    setMenuOpen(next);
  };

  const selectWorkspace = () => {
    if (membership.state !== "active") return;
    traceViewer("workspace-select:event", {
      event: "chooser-card-click",
      targetWorkspaceId: membership.id,
      authState: "authenticated",
      route: location.pathname,
    });
    navigate(`/workspaces/${encodeURIComponent(membership.id)}`);
  };

  const rename = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = renameRef.current?.value.trim() ?? "";
    if (!name || busy) return;
    setBusy(true);
    setError(undefined);
    try {
      traceViewer("workspace-card-menu:event", { event: "rename-submit", targetWorkspaceId: membership.id, calculatedName: name });
      await renameWorkspace(membership.id, name, csrfToken);
      await onMembershipsChanged();
      setMenuOpen(false);
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusy(false);
    }
  };

  const archive = async () => {
    if (busy) return;
    setBusy(true);
    setError(undefined);
    try {
      traceViewer("workspace-card-menu:event", { event: "archive-confirm", targetWorkspaceId: membership.id });
      await archiveWorkspace(membership.id, csrfToken);
      // The archive endpoint revokes this Account session. Clear local auth
      // immediately so the chooser cannot keep showing stale active cards.
      onSessionExpired();
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusy(false);
    }
  };

  const restore = async () => {
    if (busy) return;
    setBusy(true);
    setError(undefined);
    try {
      traceViewer("workspace-card-menu:event", { event: "restore", targetWorkspaceId: membership.id });
      const restored = await restoreWorkspace(membership.id, csrfToken);
      await onMembershipsChanged();
      navigate(`/workspaces/${encodeURIComponent(restored.id)}`);
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusy(false);
    }
  };

  return <li className="workspace-card" ref={rootRef}>
    <button type="button" className="workspace-card-select" disabled={membership.state !== "active"} onClick={selectWorkspace}>
      <span>{membership.state === "archived" ? "ARCHIVED · OWNER" : membership.relationship === "owner" ? "OWNER" : "PARTICIPANT"}</span>
      <strong>{membership.name}</strong>
      <small>{membership.state === "archived" ? "Archived Workspace" : roleLabel(membership)}</small>
    </button>
    {canManage && <button ref={menuTriggerRef} type="button" className="workspace-card-more" aria-label={`${membership.name} Workspace commands`} aria-haspopup="menu" aria-expanded={menuOpen && mode === "actions"} onClick={openMenu}><Ellipsis size={18} aria-hidden="true" /></button>}
    {menuOpen && mode === "actions" && <div className="workspace-card-action-menu" role="menu" aria-label={`${membership.name} Workspace commands`}>
      {membership.state === "active"
        ? <><button type="button" role="menuitem" onClick={() => setMode("rename")}><Pencil size={14} />Rename</button><button type="button" role="menuitem" className="workspace-context-archive" onClick={() => setMode("archive")}><Archive size={14} />Archive</button></>
        : <button type="button" role="menuitem" disabled={busy} onClick={() => void restore()}><RotateCcw size={14} />{busy ? "Restoring…" : "Restore"}</button>}
    </div>}
    {menuOpen && mode === "rename" && <form className="workspace-card-action-popover" onSubmit={rename}>
      <label htmlFor={`workspace-rename-${membership.id}`}>Workspace name</label>
      <input ref={renameRef} id={`workspace-rename-${membership.id}`} defaultValue={membership.name} maxLength={120} required />
      {error && <span className="form-error" role="alert">{error}</span>}
      <span className="workspace-create-actions"><button type="button" onClick={() => { setMode("actions"); setError(undefined); menuTriggerRef.current?.focus(); }}>Cancel</button><button type="submit" disabled={busy}>{busy ? "Saving…" : "Save"}</button></span>
    </form>}
    {menuOpen && mode === "archive" && <section className="workspace-card-action-popover workspace-card-archive-confirm" role="dialog" aria-label={`Archive ${membership.name}`}>
      <strong>Archive this Workspace?</strong><p>Tasks and records stay preserved. Members must log in and reconnect MCP before restoration.</p>
      {error && <span className="form-error" role="alert">{error}</span>}
      <span className="workspace-create-actions"><button ref={archiveCancelRef} type="button" onClick={() => { setMode("actions"); setError(undefined); menuTriggerRef.current?.focus(); }}>Cancel</button><button type="button" className="workspace-archive-confirm-button" disabled={busy} onClick={() => void archive()}>{busy ? "Archiving…" : "Archive"}</button></span>
    </section>}
  </li>;
}

export function WorkspaceAccessControls({
  account,
  membership,
  csrfToken,
  onLogout,
  onMembershipsChanged,
}: {
  account: Account;
  membership: WorkspaceMembership;
  csrfToken: string;
  onLogout: () => Promise<void>;
  onMembershipsChanged: () => Promise<void>;
}) {
  const navigate = useNavigate();
  const [memberAdminOpen, setMemberAdminOpen] = useState(false);
  const [approvalOpen, setApprovalOpen] = useState(false);
  const [accountMenuOpen, setAccountMenuOpen] = useState(false);
  const [accountMenuError, setAccountMenuError] = useState<string>();
	const [logoutBusy, setLogoutBusy] = useState(false);
	const [oidcProviders, setOIDCProviders] = useState<OIDCProvider[]>([]);
	const [oidcLinkBusy, setOIDCLinkBusy] = useState(false);
  const accountMenuRootRef = useRef<HTMLDivElement>(null);
  const accountMenuTriggerRef = useRef<HTMLButtonElement>(null);
  const accountMenuRef = useRef<HTMLDivElement>(null);
	const canAdmin = membership.capabilities.includes("workspace:admin") || membership.role === "owner";
	const canApprove = membership.role === "owner" || membership.capabilities.some((capability) =>
		capability === "task:approve" || capability === "lane:approve" || capability === "gate:approve");

	useEffect(() => {
		if (!accountMenuOpen) return;
		void fetchOIDCProviders().then(setOIDCProviders).catch(() => setOIDCProviders([]));
	}, [accountMenuOpen]);

  useEffect(() => {
    if (!accountMenuOpen) return;
    traceViewer("account-menu:state-committed", {
      calculatedOpenState: true,
      accountId: account.id,
      workspaceId: membership.id,
    });
    const frame = window.requestAnimationFrame(() => {
      traceViewer("account-menu:dom-rendered", {
        renderedOpenState: accountMenuRef.current?.dataset.state,
        accountId: account.id,
        workspaceId: membership.id,
      });
    });
    const focusTimer = window.setTimeout(() => {
      accountMenuRef.current?.querySelector<HTMLButtonElement>("[role='menuitem']")?.focus();
    }, 0);
    const onPointerDown = (event: PointerEvent) => {
      if (!accountMenuRootRef.current?.contains(event.target as Node)) {
        setAccountMenuOpen(false);
      }
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setAccountMenuOpen(false);
      accountMenuTriggerRef.current?.focus();
    };
    document.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.cancelAnimationFrame(frame);
      window.clearTimeout(focusTimer);
      document.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [account.id, accountMenuOpen, membership.id]);

  const closeAccountMenu = () => setAccountMenuOpen(false);

  return <div className="access-controls">
    <div className="account-menu-root" ref={accountMenuRootRef} onBlur={(event) => {
      if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setAccountMenuOpen(false);
    }}>
      <button
        ref={accountMenuTriggerRef}
        type="button"
        className="account-menu-trigger"
        aria-label={`${account.displayName} 계정 메뉴`}
        aria-haspopup="menu"
        aria-expanded={accountMenuOpen}
        onClick={() => {
          setAccountMenuOpen((current) => {
            const next = !current;
            traceViewer("account-menu:event", {
              event: "trigger-click",
              currentOpenState: current,
              calculatedOpenState: next,
              accountId: account.id,
              workspaceId: membership.id,
            });
            return next;
          });
        }}
      >
        <span className="account-avatar" aria-hidden="true">{account.displayName.trim().charAt(0).toUpperCase() || "A"}</span>
        <span className="account-summary">
          <strong>{account.displayName}</strong>
          <span>{roleLabel(membership)}</span>
        </span>
        <ChevronDown size={14} aria-hidden="true" />
      </button>
      {accountMenuOpen && <div
        ref={accountMenuRef}
        className="account-menu"
        role="menu"
        aria-label="계정 메뉴"
        data-state="open"
        onKeyDown={moveMenuFocus}
      >
        <div className="account-menu-identity" role="presentation">
          <strong>{account.displayName}</strong>
          <span>{account.loginId}</span>
          <small>{membership.name} · {roleLabel(membership)}</small>
        </div>
        {accountMenuError && <div role="presentation"><div className="account-menu-error" role="alert">{accountMenuError}</div></div>}
        <button type="button" role="menuitem" tabIndex={0} onClick={() => {
          closeAccountMenu();
          navigate("/workspaces");
        }}><LayoutGrid size={16} />내 Workspace</button>
        {canApprove && <button type="button" role="menuitem" tabIndex={-1} onClick={() => {
          closeAccountMenu();
          setApprovalOpen(true);
        }}><KeyRound size={16} />Human approval</button>}
        {canAdmin && <button type="button" role="menuitem" tabIndex={-1} onClick={() => {
          closeAccountMenu();
          setMemberAdminOpen(true);
        }}><Settings size={16} />멤버 관리</button>}
        {oidcProviders.map((provider) => <button key={provider.id} type="button" role="menuitem" tabIndex={-1} disabled={oidcLinkBusy} onClick={() => {
          setOIDCLinkBusy(true);
          setAccountMenuError(undefined);
          traceViewer("oidc-link:event", { providerId: provider.id, accountId: account.id, workspaceId: membership.id, calculatedTarget: "authorization-redirect" });
          void beginOIDCLink(provider.id, csrfToken).then(({ authorizationUrl }) => window.location.assign(authorizationUrl)).catch((reason: unknown) => {
            setOIDCLinkBusy(false);
            setAccountMenuError(reason instanceof Error ? reason.message : "Identity provider 연결을 시작하지 못했습니다.");
          });
        }}>{provider.id === "google" ? "Google 계정 연결" : `${provider.label} 연결`}</button>)}
        <div className="account-menu-separator" role="separator" />
        <button type="button" role="menuitem" tabIndex={-1} className="account-menu-logout" aria-disabled={logoutBusy} onClick={() => {
          if (logoutBusy) return;
          setLogoutBusy(true);
          setAccountMenuError(undefined);
          traceViewer("logout:event", {
            event: "account-menu-click",
            accountId: account.id,
            workspaceId: membership.id,
          });
          void onLogout()
            .catch((reason: unknown) => {
              setAccountMenuError(reason instanceof Error ? reason.message : "로그아웃하지 못했습니다.");
            })
            .finally(() => setLogoutBusy(false));
        }}><LogOut size={16} />{logoutBusy ? "로그아웃 중…" : "로그아웃"}</button>
      </div>}
    </div>
    {memberAdminOpen && <MemberAdministration
      account={account}
      workspace={membership}
      csrfToken={csrfToken}
      onClose={() => setMemberAdminOpen(false)}
      onMembershipsChanged={onMembershipsChanged}
    />}
    {approvalOpen && <HumanApprovalPanel
      workspace={membership}
      csrfToken={csrfToken}
      onClose={() => setApprovalOpen(false)}
    />}
  </div>;
}

function HumanApprovalPanel({
  workspace,
  csrfToken,
  onClose,
}: {
  workspace: WorkspaceMembership;
  csrfToken: string;
  onClose: () => void;
}) {
  const closeRef = useRef<HTMLButtonElement>(null);
  const commandRef = useRef<HTMLTextAreaElement>(null);
  const [preview, setPreview] = useState<{ command: CommandRequest; result: CommandPreview }>();
  const [acknowledgedWarnings, setAcknowledgedWarnings] = useState<string[]>([]);
  const [proceedReason, setProceedReason] = useState("");
  const [execution, setExecution] = useState<CommandExecution>();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();

  useEffect(() => {
    window.setTimeout(() => closeRef.current?.focus(), 0);
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  useEffect(() => {
    if (!preview && !execution) return;
    window.requestAnimationFrame(() => traceViewer("human-approval:dom-rendered", {
      workspaceId: workspace.id,
      previewVisible: Boolean(document.querySelector("[data-human-approval-preview]")),
      executionVisible: Boolean(document.querySelector("[data-human-approval-execution]")),
      applicationState: { hasPreview: Boolean(preview), hasExecution: Boolean(execution), busy },
    }));
  }, [busy, execution, preview, workspace.id]);

  const loadPreview = async () => {
    traceViewer("human-approval:event", { event: "preview-click", workspaceId: workspace.id });
    setError(undefined);
    setExecution(undefined);
    let command: CommandRequest;
    try {
      command = JSON.parse(commandRef.current?.value ?? "") as CommandRequest;
    } catch {
      setError("Command JSON is invalid.");
      return;
    }
    if (!command || typeof command.name !== "string" || !command.arguments ||
      !command.envelope || typeof command.envelope.idempotencyKey !== "string" ||
      typeof command.envelope.expectedWorkspaceRevision !== "number") {
      setError("A typed command with idempotencyKey and expectedWorkspaceRevision is required.");
      return;
    }
    if (command.arguments.workspaceId !== workspace.id) {
      setError("The command belongs to a different Workspace.");
      return;
    }
    if (command.envelope.approvalGrantId || command.envelope.humanApprovalAttestation) {
      setError("Use the original command without any approval fields.");
      return;
    }
    setBusy(true);
    try {
      const result = await previewCommand(command, csrfToken);
      traceViewer("human-approval:preview-state", {
        workspaceId: workspace.id,
        calculatedTarget: { action: command.name, entityType: result.entityType, entityId: result.entityId },
        commandHash: result.commandHash,
        workspaceRevision: result.expectedWorkspaceRevision,
        controllerState: { warningCodes: result.warnings.map((item) => item.code), errorCodes: result.errors.map((item) => item.code) },
      });
      setPreview({ command, result });
      setAcknowledgedWarnings([]);
      setProceedReason("");
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusy(false);
    }
  };

  const approveAndExecute = async () => {
    if (!preview) return;
    traceViewer("human-approval:event", {
      event: "approve-and-execute-click",
      workspaceId: workspace.id,
      calculatedTarget: { action: preview.command.name, entityType: preview.result.entityType, entityId: preview.result.entityId },
      applicationState: { acknowledgedWarnings, proceedReasonPresent: Boolean(proceedReason.trim()) },
    });
    setBusy(true);
    setError(undefined);
    let grantId = "";
    try {
      const grant = await issueApprovalGrant(workspace.id, {
        command: preview.command,
        acknowledgedWarningCodes: acknowledgedWarnings,
        proceedReason,
      }, csrfToken);
      grantId = grant.id;
      traceViewer("human-approval:grant-state", {
        workspaceId: workspace.id, grantId: grant.id, expiresAt: grant.expiresAt,
        commandHash: grant.commandHash, workspaceRevision: grant.workspaceRevision,
      });
      const command: CommandRequest = {
        ...preview.command,
        envelope: {
          ...preview.command.envelope,
          expectedWorkspaceRevision: grant.workspaceRevision,
          acknowledgedWarningCodes: acknowledgedWarnings,
          ...(proceedReason.trim() ? { proceedReason: proceedReason.trim() } : {}),
          approvalGrantId: grant.id,
        },
      };
      const result = await executeCommand(command, csrfToken);
      setExecution(result);
      traceViewer("human-approval:execution-state", {
        workspaceId: workspace.id, commandId: result.commandId,
        workspaceRevision: result.workspaceRevision, approvalProtocol: result.approvalProtocol,
      });
    } catch (reason) {
      if (grantId) void revokeApprovalGrant(workspace.id, grantId, csrfToken).catch(() => undefined);
      setError(errorMessage(reason));
    } finally {
      setBusy(false);
    }
  };

  const blockingErrors = preview?.result.errors.filter((item) => item.code !== "human_approval_required") ?? [];
  const allWarningsAcknowledged = preview?.result.warnings.every((warning) => acknowledgedWarnings.includes(warning.code)) ?? false;
  const canApprove = Boolean(preview && blockingErrors.length === 0 && allWarningsAcknowledged &&
    (preview.result.warnings.length === 0 || proceedReason.trim()));

  return <div className="admin-overlay">
    <section className="member-admin human-approval-panel" role="dialog" aria-modal="true" aria-labelledby="human-approval-title">
      <header>
        <div><span>HUMAN ONLY</span><h2 id="human-approval-title">Review and approve command</h2></div>
        <button ref={closeRef} className="icon-button" type="button" aria-label="Close human approval" onClick={onClose}><X size={18} /></button>
      </header>
      <p className="admin-intro">Paste the non-secret typed command, review the fresh server preview, then approve and execute it in this authenticated browser session. No token, header, or environment secret is created.</p>
      {error && <div className="form-error" role="alert">{error}</div>}
      <label className="command-input-label" htmlFor="human-approval-command">Typed command JSON</label>
      <textarea ref={commandRef} id="human-approval-command" spellCheck={false} onChange={() => { setPreview(undefined); setExecution(undefined); }} />
      <button className="primary-button preview-button" type="button" disabled={busy} onClick={() => void loadPreview()}>Load fresh preview</button>
      {preview && <section className="approval-preview" data-human-approval-preview>
        <h3>Exact command decision</h3>
        <dl>
          <div><dt>Action</dt><dd>{preview.command.name}</dd></div>
          <div><dt>Target</dt><dd>{preview.result.entityType || "workspace"} · {preview.result.entityId || workspace.id}</dd></div>
          <div><dt>Revision</dt><dd>{preview.result.expectedWorkspaceRevision}</dd></div>
          <div><dt>Capability</dt><dd>{preview.result.requiredCapability}</dd></div>
          <div><dt>Command hash</dt><dd><code>{preview.result.commandHash}</code></dd></div>
          <div><dt>Decision snapshot</dt><dd><code>{preview.result.decisionSnapshotHash || "none"}</code></dd></div>
        </dl>
        <details><summary>Projected diff</summary><pre>{JSON.stringify(preview.result.projectedDiff, null, 2)}</pre></details>
        {preview.result.errors.map((item) => <div className={item.code === "human_approval_required" ? "approval-required" : "form-error"} key={item.code}><strong>{item.code}</strong><span>{item.message}</span></div>)}
        {preview.result.warnings.length > 0 && <fieldset className="warning-list"><legend>Warning acknowledgement</legend>
          {preview.result.warnings.map((warning) => <label key={warning.code}><input type="checkbox" checked={acknowledgedWarnings.includes(warning.code)} onChange={(event) => {
            const checked = event.currentTarget.checked;
            setAcknowledgedWarnings((current) => checked ? [...new Set([...current, warning.code])] : current.filter((code) => code !== warning.code));
          }} /><span><strong>{warning.code}</strong>{warning.message}</span></label>)}
        </fieldset>}
        {preview.result.warnings.length > 0 && <label className="command-input-label">Proceed reason<textarea className="proceed-reason" value={proceedReason} onChange={(event) => setProceedReason(event.currentTarget.value)} /></label>}
        <button className="primary-button" type="button" disabled={busy || !canApprove} onClick={() => void approveAndExecute()}>Approve and execute once</button>
      </section>}
      {execution && <section className="approval-execution" data-human-approval-execution><h3>Executed</h3><p>Command <code>{execution.commandId}</code> committed at Workspace revision {execution.workspaceRevision}.</p></section>}
    </section>
  </div>;
}

export function WorkspaceContextSwitcher({
  membership,
  memberships,
  currentWorkspaceName,
  csrfToken,
  onMembershipsChanged,
}: {
  membership: WorkspaceMembership;
  memberships: WorkspaceMembership[];
  currentWorkspaceName: string;
  csrfToken: string;
  onMembershipsChanged: () => Promise<void>;
}) {
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createBusy, setCreateBusy] = useState(false);
  const [createError, setCreateError] = useState<string>();
  const rootRef = useRef<HTMLSpanElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLSpanElement>(null);
  const createNameRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!creating) return;
    const frame = window.requestAnimationFrame(() => {
      traceViewer("workspace-create:dom-rendered", {
        source: "workspace-context-menu",
        renderedMenuOpenState: menuRef.current?.dataset.state ?? "closed",
        renderedCreateOpenState: createNameRef.current ? "open" : "closed",
        currentWorkspaceId: membership.id,
      });
    });
    return () => window.cancelAnimationFrame(frame);
  }, [creating, membership.id]);

  useEffect(() => {
    if (!open) return;
    traceViewer("workspace-menu:state-committed", {
      calculatedOpenState: true,
      currentWorkspaceId: membership.id,
      membershipCount: memberships.length,
    });
    const frame = window.requestAnimationFrame(() => {
      traceViewer("workspace-menu:dom-rendered", {
        renderedOpenState: menuRef.current?.dataset.state,
        renderedCurrentWorkspaceId: menuRef.current?.dataset.workspaceId,
      });
    });
    const focusTimer = window.setTimeout(() => {
      menuRef.current?.querySelector<HTMLButtonElement>("[role='menuitemradio']")?.focus();
    }, 0);
    const onPointerDown = (event: PointerEvent) => {
    if (!rootRef.current?.contains(event.target as Node)) {
      setOpen(false);
    }
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      setOpen(false);
      triggerRef.current?.focus();
    };
    document.addEventListener("pointerdown", onPointerDown);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.cancelAnimationFrame(frame);
      window.clearTimeout(focusTimer);
      document.removeEventListener("pointerdown", onPointerDown);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [membership.id, memberships.length, open]);

  const selectWorkspace = (targetWorkspaceId: string) => {
    const targetPath = `/workspaces/${encodeURIComponent(targetWorkspaceId)}`;
    traceViewer("workspace-select:event", {
      event: "context-menu-click",
      targetWorkspaceId,
      currentWorkspaceId: membership.id,
      calculatedTargetPath: targetPath,
      authState: "authenticated",
      route: location.pathname,
    });
    setOpen(false);
    if (targetWorkspaceId === membership.id) {
      window.setTimeout(() => triggerRef.current?.focus(), 0);
      return;
    }
    navigate(targetPath);
  };
  const submitWorkspace = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const name = createNameRef.current?.value.trim() ?? "";
    if (!name || createBusy) return;
    setCreateBusy(true);
    setCreateError(undefined);
    try {
      const created = await createWorkspace({ workspaceId: crypto.randomUUID(), name }, csrfToken);
      if (createNameRef.current) createNameRef.current.value = "";
      await onMembershipsChanged();
      setCreating(false);
      setOpen(false);
      navigate(`/workspaces/${encodeURIComponent(created.id)}`);
    } catch (reason) {
      setCreateError(errorMessage(reason));
    } finally {
      setCreateBusy(false);
    }
  };

  return <span className="workspace-context-switcher" ref={rootRef} onBlur={(event) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setOpen(false);
  }}>
    <button
      ref={triggerRef}
      type="button"
      className="workspace-context-trigger"
      aria-label={`${currentWorkspaceName} Workspace 전환`}
      aria-haspopup="menu"
      aria-expanded={open}
      onClick={() => {
        setOpen((current) => {
          const next = !current;
          traceViewer("workspace-menu:event", {
            event: "trigger-click",
            currentOpenState: current,
            calculatedOpenState: next,
            currentWorkspaceId: membership.id,
          });
          return next;
        });
      }}
    >
      <span>{currentWorkspaceName}</span>
      <ChevronDown size={17} aria-hidden="true" />
    </button>
    {open && <span
      ref={menuRef}
      className="workspace-context-menu"
      role="menu"
      aria-label="Workspace 전환"
      data-state="open"
      data-workspace-id={membership.id}
      onKeyDown={moveMenuFocus}
    >
      <span className="workspace-context-menu-label" role="presentation">WORKSPACES</span>
      {memberships.filter((item) => item.state === "active").map((item, index) => <span className="workspace-context-row" key={item.id}>
        <button
          type="button"
          role="menuitemradio"
          aria-checked={item.id === membership.id}
          tabIndex={index === 0 ? 0 : -1}
          onClick={() => selectWorkspace(item.id)}
        >
          <span>
            <strong>{item.name}</strong>
            <small>{roleLabel(item)}</small>
          </span>
          {item.id === membership.id && <Check size={16} aria-label="현재 Workspace" />}
        </button>
      </span>)}
      <span className="workspace-context-menu-separator" role="separator" />
      <button
        type="button"
        role="menuitem"
        tabIndex={-1}
        onClick={() => {
          traceViewer("workspace-create:event", {
            source: "workspace-context-menu",
            event: "new-workspace-click",
            currentMenuOpenState: open,
            calculatedMenuOpenState: false,
            calculatedCreateOpenState: true,
            currentWorkspaceId: membership.id,
          });
          setOpen(false);
          setCreating(true);
          setCreateError(undefined);
          window.setTimeout(() => createNameRef.current?.focus(), 0);
        }}
      >
        <span><strong><Plus size={14} aria-hidden="true" /> 새 Workspace</strong><small>Owner로 생성</small></span>
      </button>
    </span>}
    {creating && <form className="workspace-create-popover" onSubmit={submitWorkspace}>
      <label htmlFor="workspace-create-name">Workspace 이름</label>
      <input ref={createNameRef} id="workspace-create-name" maxLength={120} required />
      {createError && <span className="form-error" role="alert">{createError}</span>}
      <span className="workspace-create-actions">
        <button type="button" onClick={() => {
          setCreating(false);
          setCreateError(undefined);
          triggerRef.current?.focus();
        }}>취소</button>
        <button type="submit" disabled={createBusy}>{createBusy ? "생성 중…" : "생성"}</button>
      </span>
    </form>}
  </span>;
}

function MemberAdministration({
  account,
  workspace,
  csrfToken,
  onClose,
  onMembershipsChanged,
}: {
  account: Account;
  workspace: WorkspaceMembership;
  csrfToken: string;
  onClose: () => void;
  onMembershipsChanged: () => Promise<void>;
}) {
  const closeRef = useRef<HTMLButtonElement>(null);
  const passwordRef = useRef<HTMLInputElement>(null);
  const [members, setMembers] = useState<WorkspaceMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [busyActorId, setBusyActorId] = useState<string>();
  const [resetPasswordActorId, setResetPasswordActorId] = useState<string>();
  const [error, setError] = useState<string>();

  const refresh = async (signal?: AbortSignal) => {
    const items = await fetchWorkspaceMembers(workspace.id, signal);
    setMembers(items);
    setError(undefined);
  };

  useEffect(() => {
    const controller = new AbortController();
    void refresh(controller.signal)
      .catch((reason: unknown) => {
        if (!controller.signal.aborted) setError(errorMessage(reason));
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    window.setTimeout(() => closeRef.current?.focus(), 0);
    return () => controller.abort();
  }, [workspace.id]);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const mutate = async (actorId: string, action: () => Promise<unknown>) => {
    setBusyActorId(actorId);
    setError(undefined);
    traceViewer("member-mutation:request", {
      targetWorkspaceId: workspace.id,
      targetActorId: actorId,
      authState: "authenticated",
    });
    try {
      await action();
      await Promise.all([refresh(), onMembershipsChanged()]);
      traceViewer("member-mutation:committed", {
        targetWorkspaceId: workspace.id,
        targetActorId: actorId,
      });
    } catch (reason) {
      setError(errorMessage(reason));
      if (reason instanceof APIError && reason.status === 409) {
        await refresh().catch(() => undefined);
      }
    } finally {
      setBusyActorId(undefined);
    }
  };

  const create = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const initialPassword = String(data.get("initialPassword") ?? "");
    if (passwordRef.current) passwordRef.current.value = "";
    setBusyActorId("new");
    setError(undefined);
    try {
      await createWorkspaceMember(workspace.id, {
        loginId: String(data.get("loginId") ?? ""),
        displayName: String(data.get("displayName") ?? ""),
        initialPassword,
        role: String(data.get("role") ?? "operator") as Exclude<WorkspaceRole, "owner">,
      }, csrfToken);
      form.reset();
      await refresh();
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusyActorId(undefined);
    }
  };

  const attach = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    setBusyActorId("existing");
    setError(undefined);
    try {
      await attachExistingAccount(workspace.id, {
        loginId: String(data.get("loginId") ?? ""),
        role: String(data.get("role") ?? "operator") as WorkspaceRole,
      }, csrfToken);
      form.reset();
      await refresh();
    } catch (reason) {
      setError(errorMessage(reason));
    } finally {
      setBusyActorId(undefined);
    }
  };

  const resetPassword = async (event: React.FormEvent<HTMLFormElement>, member: WorkspaceMember) => {
    event.preventDefault();
    const form = event.currentTarget;
    const passwordInput = form.elements.namedItem("newPassword") as HTMLInputElement | null;
    const newPassword = passwordInput?.value ?? "";
    if (passwordInput) passwordInput.value = "";
    await mutate(member.actorId, () => resetMemberPassword(workspace.id, member.actorId, newPassword, csrfToken));
    setResetPasswordActorId(undefined);
  };

  return <div className="admin-overlay">
    <section
      className="member-admin"
      role="dialog"
      aria-modal="true"
      aria-labelledby="member-admin-title"
      data-workspace-id={workspace.id}
    >
      <header>
        <div><span>WORKSPACE ADMINISTRATION</span><h2 id="member-admin-title">{workspace.name} 멤버</h2></div>
        <button ref={closeRef} className="icon-button" type="button" aria-label="멤버 관리 닫기" onClick={onClose}><X size={18} /></button>
      </header>
      {error && <div className="form-error" role="alert">{error}</div>}
      <form className="member-create-form" onSubmit={create}>
        <h3>새 계정 생성 및 추가</h3>
        <label>아이디<input name="loginId" autoComplete="off" required /></label>
        <label>표시 이름<input name="displayName" autoComplete="off" required /></label>
        <label>초기 암호<input ref={passwordRef} name="initialPassword" type="password" autoComplete="new-password" minLength={15} required /></label>
        <label>역할<select name="role" defaultValue="operator"><ParticipantRoleOptions /></select></label>
        <button className="primary-button" type="submit" disabled={busyActorId === "new"}>새 계정 생성</button>
      </form>
      <form className="member-create-form existing-account-form" onSubmit={attach}>
        <h3>기존 계정을 이 Workspace에 연결</h3>
        <label>기존 로그인 아이디<input name="loginId" autoComplete="off" required /></label>
        <label>역할<select name="role" defaultValue="operator"><ParticipantRoleOptions /></select></label>
        <button className="primary-button" type="submit" disabled={busyActorId === "existing"}>기존 계정 연결</button>
      </form>
      <section className="member-list" aria-busy={loading}>
        <h3>현재 멤버</h3>
        {loading
          ? <p>멤버 목록을 불러오는 중입니다…</p>
          : members.map((member) => <article key={member.actorId} className={!member.active ? "inactive" : ""}>
            <div>
              <strong>{member.displayName}</strong>
              <span>{member.relationship === "owner" ? "Owner" : `Participant · ${member.role}`}</span>
              {member.accountId === account.id && <small>현재 계정</small>}
            </div>
            {member.role !== "owner" && <div className="member-actions">
              <select
                aria-label={`${member.displayName} 역할`}
                value={member.role}
                disabled={busyActorId === member.actorId}
                onChange={(event) => void mutate(member.actorId, () => updateWorkspaceMember(
                  workspace.id,
                  member.actorId,
                  { role: event.currentTarget.value as Exclude<WorkspaceRole, "owner"> },
                  csrfToken,
                ))}
              ><ParticipantRoleOptions /></select>
              <button type="button" disabled={busyActorId === member.actorId} onClick={() => void mutate(
                member.actorId,
                () => updateWorkspaceMember(workspace.id, member.actorId, { active: !member.active }, csrfToken),
              )}>{member.active ? "비활성화" : "활성화"}</button>
              {member.active && <button type="button" disabled={busyActorId === member.actorId} onClick={() => void mutate(
                member.actorId,
                () => transferWorkspaceOwnership(workspace.id, member.actorId, csrfToken),
              )}>Owner 이전</button>}
              {member.accountId && member.accountId !== account.id && <button type="button" disabled={busyActorId === member.actorId} onClick={() => setResetPasswordActorId((current) => current === member.actorId ? undefined : member.actorId)}>암호 재설정</button>}
              {member.accountId && member.accountId !== account.id && <button type="button" className="danger-button" disabled={busyActorId === member.actorId} onClick={() => void mutate(
                member.actorId,
                () => disableMemberAccount(workspace.id, member.actorId, csrfToken),
              )}>계정 비활성화</button>}
              <button type="button" className="danger-button" disabled={busyActorId === member.actorId} onClick={() => void mutate(
                member.actorId,
                () => removeWorkspaceMember(workspace.id, member.actorId, csrfToken),
              )}>제거</button>
            </div>}
            {resetPasswordActorId === member.actorId && <form className="password-reset-form" onSubmit={(event) => void resetPassword(event, member)}>
              <label>새 암호<input name="newPassword" type="password" autoComplete="new-password" minLength={15} required /></label>
              <button type="submit" disabled={busyActorId === member.actorId}>암호 재설정 및 기존 session 폐기</button>
            </form>}
          </article>)}
      </section>
    </section>
  </div>;
}

function ParticipantRoleOptions() {
  return <>
    <option value="viewer">Viewer</option>
    <option value="operator">Operator</option>
    <option value="approver">Approver</option>
  </>;
}

function roleLabel(membership: WorkspaceMembership) {
  return membership.relationship === "owner"
    ? "Owner"
    : `Participant · ${membership.role[0]?.toUpperCase()}${membership.role.slice(1)}`;
}

function errorMessage(reason: unknown) {
  if (reason instanceof APIError && reason.status === 409) {
    return "다른 변경과 충돌했습니다. 최신 멤버 상태를 다시 불러왔습니다.";
  }
  if (reason instanceof APIError && reason.status === 403) {
    return "이 작업을 수행할 Owner 권한이 없습니다.";
  }
  return reason instanceof Error ? reason.message : "멤버 작업을 완료하지 못했습니다.";
}
