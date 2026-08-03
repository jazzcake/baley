import type {
  AuthSession,
  WorkspaceMember,
  WorkspaceMembership,
  WorkspaceRole,
} from "../auth/model";
import { requestJSON } from "./http";

type SessionDTO = AuthSession & { authenticated?: true };
export type CommandRequest = {
  name: string;
  arguments: Record<string, unknown>;
  envelope: {
    idempotencyKey: string;
    expectedWorkspaceRevision?: number;
    initiatedByActorId?: string;
    executedByActorId?: string;
    acknowledgedWarningCodes?: string[];
    proceedReason?: string;
    humanApprovalAttestation?: unknown;
    approvalGrantToken?: string;
  };
};
export type Diagnostic = { code: string; message: string; details?: unknown };
export type CommandPreview = {
  commandHash: string;
  expectedWorkspaceRevision: number;
  requiredCapability: string;
  projectedDiff: unknown;
  errors: Diagnostic[];
  warnings: Diagnostic[];
  advisories: Diagnostic[];
  decisionSnapshotHash?: string;
  entityType?: string;
  entityId?: string;
};
export type ApprovalGrant = {
  id: string;
  grantToken: string;
  expiresAt: string;
  commandHash: string;
  workspaceRevision: number;
};
export type MCPConnection = {
  id: string;
  workspaceId: string;
  agentActorId: string;
  status: "pending" | "approved";
  expiresAt: string;
};

export function fetchSession(signal?: AbortSignal): Promise<AuthSession> {
  return requestJSON<SessionDTO>("/v1/auth/session", { signal });
}

export function login(loginId: string, password: string): Promise<AuthSession> {
  return requestJSON<AuthSession>("/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ loginId, password }),
  });
}

export function logout(csrfToken: string): Promise<void> {
  return requestJSON<void>("/v1/auth/logout", { method: "POST" }, csrfToken);
}

export async function fetchWorkspaces(signal?: AbortSignal): Promise<WorkspaceMembership[]> {
  const result = await requestJSON<{ items: WorkspaceMembership[] }>("/v1/workspaces", { signal });
  return result.items;
}

export function createWorkspace(
  input: { workspaceId: string; name: string },
  csrfToken: string,
): Promise<WorkspaceMembership> {
  return requestJSON<WorkspaceMembership>(
    "/v1/workspaces",
    { method: "POST", body: JSON.stringify(input) },
    csrfToken,
  );
}

export async function fetchWorkspaceMembers(
  workspaceId: string,
  signal?: AbortSignal,
): Promise<WorkspaceMember[]> {
  const result = await requestJSON<{ items: WorkspaceMember[] }>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/members`,
    { signal },
  );
  return result.items;
}

export function createWorkspaceMember(
  workspaceId: string,
  input: { loginId: string; displayName: string; initialPassword: string; role: Exclude<WorkspaceRole, "owner"> },
  csrfToken: string,
): Promise<WorkspaceMember> {
  return requestJSON<WorkspaceMember>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/members`,
    { method: "POST", body: JSON.stringify(input) },
    csrfToken,
  );
}

export function updateWorkspaceMember(
  workspaceId: string,
  actorId: string,
  input: { role?: Exclude<WorkspaceRole, "owner">; active?: boolean },
  csrfToken: string,
): Promise<WorkspaceMember> {
  return requestJSON<WorkspaceMember>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/members/${encodeURIComponent(actorId)}`,
    { method: "PATCH", body: JSON.stringify(input) },
    csrfToken,
  );
}

export function removeWorkspaceMember(
  workspaceId: string,
  actorId: string,
  csrfToken: string,
): Promise<void> {
  return requestJSON<void>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/members/${encodeURIComponent(actorId)}`,
    { method: "DELETE" },
    csrfToken,
  );
}

export function transferWorkspaceOwnership(
  workspaceId: string,
  targetActorId: string,
  csrfToken: string,
): Promise<void> {
  return requestJSON<void>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/owner-transfer`,
    {
      method: "POST",
      body: JSON.stringify({ targetActorId, previousOwnerRole: "operator" }),
    },
    csrfToken,
  );
}

export function attachExistingAccount(
  workspaceId: string,
  input: { loginId: string; role: WorkspaceRole },
  csrfToken: string,
): Promise<WorkspaceMember> {
  return requestJSON<WorkspaceMember>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/memberships`,
    { method: "POST", body: JSON.stringify(input) },
    csrfToken,
  );
}

export function disableMemberAccount(
  workspaceId: string,
  actorId: string,
  csrfToken: string,
): Promise<void> {
  return requestJSON<void>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/members/${encodeURIComponent(actorId)}/disable-account`,
    { method: "POST", body: "{}" },
    csrfToken,
  );
}

export function resetMemberPassword(
  workspaceId: string,
  actorId: string,
  newPassword: string,
  csrfToken: string,
): Promise<void> {
  return requestJSON<void>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/members/${encodeURIComponent(actorId)}/reset-password`,
    { method: "POST", body: JSON.stringify({ newPassword }) },
    csrfToken,
  );
}

export function previewCommand(command: CommandRequest, csrfToken: string): Promise<CommandPreview> {
  return requestJSON<CommandPreview>(
    "/v1/commands/preview",
    { method: "POST", body: JSON.stringify(command) },
    csrfToken,
  );
}

export function issueApprovalGrant(
  workspaceId: string,
  input: {
    command: CommandRequest;
    acknowledgedWarningCodes: string[];
    proceedReason: string;
  },
  csrfToken: string,
): Promise<ApprovalGrant> {
  return requestJSON<ApprovalGrant>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/approval-grants`,
    { method: "POST", body: JSON.stringify(input) },
    csrfToken,
  );
}

export function revokeApprovalGrant(
  workspaceId: string,
  grantId: string,
  csrfToken: string,
): Promise<void> {
  return requestJSON<void>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/approval-grants/${encodeURIComponent(grantId)}`,
    { method: "DELETE" },
    csrfToken,
  );
}

export function fetchMCPConnection(
  workspaceId: string,
  connectionId: string,
  signal?: AbortSignal,
): Promise<MCPConnection> {
  return requestJSON<MCPConnection>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/mcp-connections/${encodeURIComponent(connectionId)}`,
    { signal },
  );
}

export function approveMCPConnection(
  workspaceId: string,
  connectionId: string,
  csrfToken: string,
): Promise<void> {
  return requestJSON<void>(
    `/v1/workspaces/${encodeURIComponent(workspaceId)}/mcp-connections/${encodeURIComponent(connectionId)}/approve`,
    { method: "POST", body: "{}" },
    csrfToken,
  );
}
