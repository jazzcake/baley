import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import {
  fetchSession,
  fetchWorkspaces,
  login as requestLogin,
  logout as requestLogout,
} from "../api/auth";
import { APIError } from "../api/http";
import { traceViewer } from "../debug/viewer-trace";
import type { AuthState } from "./model";

type AuthContextValue = {
  state: AuthState;
  login: (loginId: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  retryBootstrap: () => void;
  refreshWorkspaces: () => Promise<void>;
  expireSession: () => void;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<AuthState>({ status: "booting" });
  const [bootstrapGeneration, setBootstrapGeneration] = useState(0);

  const commitAuthenticated = useCallback(async (
    session: Awaited<ReturnType<typeof fetchSession>>,
    signal?: AbortSignal,
  ) => {
    const memberships = await fetchWorkspaces(signal);
    setState({
      status: "authenticated",
      mode: "enforced",
      account: session.account,
      csrfToken: session.csrfToken,
      expiresAt: session.expiresAt,
      memberships,
    });
    traceViewer("auth:committed", {
      authState: "authenticated",
      accountId: session.account.id,
      membershipCount: memberships.length,
    });
  }, []);

  useEffect(() => {
    const authMode = resolveAuthMode(import.meta.env.VITE_BALEY_AUTH_MODE, import.meta.env.PROD);
    if (authMode !== "enforced") {
      const workspaceId = import.meta.env.VITE_BALEY_WORKSPACE_ID || "00000000-0000-4000-8000-000000000001";
      setState({
        status: "authenticated",
        mode: "legacy",
        account: {
          id: "legacy-local-account",
          actorId: "00000000-0000-4000-8000-000000000002",
          loginId: "legacy",
          displayName: "Local Viewer",
        },
        csrfToken: "",
        expiresAt: "",
        memberships: [{
          id: workspaceId,
          name: import.meta.env.VITE_BALEY_WORKSPACE_NAME || "Baley Workspace",
          state: "active",
          revision: 0,
          role: "operator",
          relationship: "participant",
          capabilities: ["workspace:read"],
        }],
      });
      traceViewer("auth:committed", {
        authState: "authenticated",
        mode: "legacy",
        membershipCount: 1,
      });
      return;
    }
    const controller = new AbortController();
    traceViewer("auth:bootstrap-request", { generation: bootstrapGeneration });
    void fetchSession(controller.signal)
      .then((session) => commitAuthenticated(session, controller.signal))
      .catch((error: unknown) => {
        if (controller.signal.aborted) return;
        if (error instanceof APIError && error.status === 401) {
          setState({ status: "anonymous" });
          traceViewer("auth:committed", { authState: "anonymous" });
          return;
        }
        setState({
          status: "unavailable",
          message: error instanceof Error ? error.message : "Authentication service unavailable",
        });
        traceViewer("auth:committed", { authState: "unavailable" });
      });
    return () => controller.abort();
  }, [bootstrapGeneration, commitAuthenticated]);

  const login = useCallback(async (loginId: string, password: string) => {
    traceViewer("login:request", { authState: state.status });
    const session = await requestLogin(loginId, password);
    await commitAuthenticated(session);
  }, [commitAuthenticated, state.status]);

  const logout = useCallback(async () => {
    if (state.status !== "authenticated") return;
    if (state.mode === "legacy") return;
    const csrfToken = state.csrfToken;
    await requestLogout(csrfToken);
    setState({ status: "anonymous" });
    traceViewer("auth:committed", { authState: "anonymous", reason: "logout" });
  }, [state]);

  const refreshWorkspaces = useCallback(async () => {
    if (state.status !== "authenticated") return;
    if (state.mode === "legacy") return;
    const memberships = await fetchWorkspaces();
    setState((current) => current.status === "authenticated"
      ? { ...current, memberships }
      : current);
  }, [state.status]);

  const retryBootstrap = useCallback(() => {
    setState({ status: "booting" });
    setBootstrapGeneration((generation) => generation + 1);
  }, []);

  const expireSession = useCallback(() => {
    setState({ status: "anonymous" });
    traceViewer("auth:committed", { authState: "anonymous", reason: "session-expired" });
  }, []);

  const value = useMemo<AuthContextValue>(() => ({
    state,
    login,
    logout,
    retryBootstrap,
    refreshWorkspaces,
    expireSession,
  }), [state, login, logout, retryBootstrap, refreshWorkspaces, expireSession]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function resolveAuthMode(configured: string | undefined, production: boolean): "legacy" | "enforced" {
  if (configured === "legacy" || configured === "enforced") return configured;
  return "enforced";
}

export function useAuth() {
  const value = useContext(AuthContext);
  if (!value) throw new Error("useAuth must be used within AuthProvider");
  return value;
}
