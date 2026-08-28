import { useEffect, useRef, useState } from "react";
import { Check, ChevronDown, LayoutGrid, LogOut, Plus, Settings, X } from "lucide-react";
import { useNavigate } from "react-router-dom";
import {
	attachExistingAccount,
	beginOIDCLink,
  approveMCPConnection,
  createWorkspace,
  createWorkspaceMember,
  disableMemberAccount,
	fetchOIDCProviders,
  fetchWorkspaceMembers,
  fetchMCPConnection,
  removeWorkspaceMember,
  resetMemberPassword,
  transferWorkspaceOwnership,
	updateWorkspaceMember,
	oidcLoginURL,
} from "../api/auth";
import type { MCPConnection, OIDCProvider } from "../api/auth";
import { APIError } from "../api/http";
import { traceViewer } from "../debug/viewer-trace";
import type {
  Account,
  WorkspaceMember,
  WorkspaceMembership,
  WorkspaceRole,
} from "../auth/model";

type LoginScreenProps = {
  onLogin: (loginId: string, password: string) => Promise<void>;
};

export function MCPConnectionApproval({
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
  const [connection, setConnection] = useState<MCPConnection>();
  const [busy, setBusy] = useState(false);
  const [approved, setApproved] = useState(false);
  const [error, setError] = useState<string>();
  const closeRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const controller = new AbortController();
    traceViewer("mcp-connect:request", {
      event: "load",
      targetWorkspaceId: workspace.id,
      connectionId,
      authState: "authenticated",
    });
    void fetchMCPConnection(workspace.id, connectionId, controller.signal)
      .then((next) => {
        setConnection(next);
        setApproved(next.status === "approved");
        traceViewer("mcp-connect:state", {
          targetWorkspaceId: workspace.id,
          connectionId,
          status: next.status,
        });
        window.requestAnimationFrame(() => traceViewer("mcp-connect:dom", {
          targetWorkspaceId: workspace.id,
          connectionId,
          dialogPresent: Boolean(document.querySelector("[data-mcp-connection-id]")),
        }));
      })
      .catch((reason: unknown) => {
        if (!controller.signal.aborted) setError(reason instanceof Error ? reason.message : "연결 요청을 불러올 수 없습니다.");
      });
    window.setTimeout(() => closeRef.current?.focus(), 0);
    return () => controller.abort();
  }, [workspace.id, connectionId]);

  const approve = async () => {
    setBusy(true);
    setError(undefined);
    traceViewer("mcp-connect:approve", {
      event: "click",
      targetWorkspaceId: workspace.id,
      connectionId,
      calculatedTargetState: "approved",
      currentState: connection?.status,
    });
    try {
      await approveMCPConnection(workspace.id, connectionId, csrfToken);
      setApproved(true);
      setConnection((current) => current ? { ...current, status: "approved" } : current);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : "연결을 승인하지 못했습니다.");
    } finally {
      setBusy(false);
    }
  };

  return <div className="admin-overlay">
    <section className="mcp-connect-dialog" role="dialog" aria-modal="true" aria-labelledby="mcp-connect-title" data-mcp-connection-id={connectionId}>
      <header>
        <div><span>WORKSPACE CONNECTION</span><h2 id="mcp-connect-title">Codex Operator 연결</h2></div>
        <button ref={closeRef} className="icon-button" type="button" aria-label="연결 창 닫기" onClick={onClose}><X size={18} /></button>
      </header>
      {error && <div className="form-error" role="alert">{error}</div>}
      {!error && !connection && <p>연결 요청을 확인하는 중입니다.</p>}
      {connection && !approved && <>
        <p><strong>{workspace.name}</strong>에 Codex를 Operator로 연결합니다.</p>
        <p className="mcp-connect-scope">Task와 Backlog를 실행할 수 있지만, Task 확인이나 Gate 통과 같은 사람 전용 승인은 할 수 없습니다.</p>
        <dl><div><dt>권한</dt><dd>Operator</dd></div><div><dt>Agent</dt><dd><code>{connection.agentActorId}</code></dd></div></dl>
        <button className="primary-button" type="button" disabled={busy} onClick={() => void approve()}>Operator 연결 승인</button>
      </>}
      {approved && <div className="mcp-connect-success" role="status">
        <Check size={22} /><div><strong>연결되었습니다.</strong><p>LLM 세션으로 돌아가 같은 요청을 다시 실행하면 됩니다.</p></div>
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

export function LoginScreen({ onLogin }: LoginScreenProps) {
  const navigate = useNavigate();
  const passwordRef = useRef<HTMLInputElement>(null);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string>();
	const [oidcProviders, setOIDCProviders] = useState<OIDCProvider[]>([]);

	useEffect(() => {
		const controller = new AbortController();
		void fetchOIDCProviders().then((items) => {
			if (!controller.signal.aborted) setOIDCProviders(items);
		}).catch(() => undefined);
		return () => controller.abort();
	}, []);

	const startOIDC = (provider: OIDCProvider) => {
		traceViewer("oidc-login:event", { providerId: provider.id, authState: "anonymous", calculatedTarget: "authorization-redirect" });
		window.location.assign(oidcLoginURL(provider.id));
	};

  const submit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (submitting) return;
    const data = new FormData(event.currentTarget);
    const loginId = String(data.get("loginId") ?? "");
    const password = String(data.get("password") ?? "");
    if (passwordRef.current) passwordRef.current.value = "";
    setSubmitting(true);
    setError(undefined);
    try {
      await onLogin(loginId, password);
      navigate("/workspaces", { replace: true });
    } catch (reason) {
      const retry = reason instanceof APIError && reason.status === 429 && reason.retryAfter
        ? ` 다시 시도 가능: ${reason.retryAfter}`
        : "";
      setError(`아이디 또는 암호를 확인해주세요.${retry}`);
    } finally {
      setSubmitting(false);
    }
  };

  return <main className="auth-shell">
    <section className="auth-card" aria-labelledby="login-title">
      <div className="brand-mark auth-brand">B</div>
      <span className="auth-kicker">BALEY ACCOUNT</span>
      <h1 id="login-title">로그인</h1>
      <p>소속된 Workspace와 권한으로 안전하게 작업을 확인합니다.</p>
      <form onSubmit={submit}>
        <label htmlFor="login-id">아이디</label>
        <input id="login-id" name="loginId" autoComplete="username" required />
        <label htmlFor="login-password">암호</label>
        <input
          ref={passwordRef}
          id="login-password"
          name="password"
          type="password"
          autoComplete="current-password"
          required
        />
        {error && <div className="form-error" role="alert">{error}</div>}
        <button className="primary-button" type="submit" disabled={submitting}>
          {submitting ? "확인 중…" : "로그인"}
        </button>
      </form>
		{oidcProviders.length > 0 && <div className="oidc-login-options" aria-label="OIDC login options">
			{oidcProviders.map((provider) => <button key={provider.id} type="button" className="secondary-button" onClick={() => startOIDC(provider)}>
				{provider.id === "google" ? "Google로 계속" : `${provider.label}로 계속`}
			</button>)}
		</div>}
    </section>
  </main>;
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
}: {
	account: Account;
	memberships: WorkspaceMembership[];
	csrfToken: string;
	onLogout: () => Promise<void>;
	onMembershipsChanged: () => Promise<void>;
}) {
  const navigate = useNavigate();
  const [creating, setCreating] = useState(false);
  const [createBusy, setCreateBusy] = useState(false);
	const [createError, setCreateError] = useState<string>();
	const [logoutBusy, setLogoutBusy] = useState(false);
	const [logoutError, setLogoutError] = useState<string>();
  const createTriggerRef = useRef<HTMLButtonElement>(null);
  const createNameRef = useRef<HTMLInputElement>(null);

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
		<button type="button" className="account-menu-logout workspace-chooser-logout" disabled={logoutBusy} onClick={() => {
			if (logoutBusy) return;
			setLogoutBusy(true);
			setLogoutError(undefined);
			traceViewer("workspace-chooser-logout:event", { accountId: account.id, currentState: "authenticated", calculatedTargetState: "anonymous" });
			void onLogout().catch((reason: unknown) => {
				setLogoutError(reason instanceof Error ? reason.message : "로그아웃하지 못했습니다.");
			}).finally(() => setLogoutBusy(false));
		}}><LogOut size={16} />{logoutBusy ? "로그아웃 중…" : "로그아웃"}</button>
      </div>
    </header>
    {logoutError && <div className="form-error workspace-chooser-logout-error" role="alert">{logoutError}</div>}
    {memberships.length === 0
      ? <section className="chooser-empty"><h2>소속된 Workspace가 없습니다</h2><p>Owner에게 참여 요청을 보내주세요.</p></section>
      : <ul className="workspace-card-grid">
        {memberships.map((membership) => <li key={membership.id}>
          <button type="button" onClick={() => {
            traceViewer("workspace-select:event", {
              event: "chooser-click",
              targetWorkspaceId: membership.id,
              authState: "authenticated",
              route: location.pathname,
            });
            navigate(`/workspaces/${encodeURIComponent(membership.id)}`);
          }}>
            <span>{membership.relationship === "owner" ? "OWNER" : "PARTICIPANT"}</span>
            <strong>{membership.name}</strong>
            <small>{roleLabel(membership)}</small>
          </button>
        </li>)}
      </ul>}
  </main>;
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
  const [accountMenuOpen, setAccountMenuOpen] = useState(false);
  const [accountMenuError, setAccountMenuError] = useState<string>();
	const [logoutBusy, setLogoutBusy] = useState(false);
	const [oidcProviders, setOIDCProviders] = useState<OIDCProvider[]>([]);
	const [oidcLinkBusy, setOIDCLinkBusy] = useState(false);
  const accountMenuRootRef = useRef<HTMLDivElement>(null);
  const accountMenuTriggerRef = useRef<HTMLButtonElement>(null);
  const accountMenuRef = useRef<HTMLDivElement>(null);
	const canAdmin = membership.capabilities.includes("workspace:admin") || membership.role === "owner";

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
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
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
      {memberships.map((item, index) => <button
        type="button"
        role="menuitemradio"
        aria-checked={item.id === membership.id}
        tabIndex={index === 0 ? 0 : -1}
        key={item.id}
        onClick={() => selectWorkspace(item.id)}
      >
        <span>
          <strong>{item.name}</strong>
          <small>{roleLabel(item)}</small>
        </span>
        {item.id === membership.id && <Check size={16} aria-label="현재 Workspace" />}
      </button>)}
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
