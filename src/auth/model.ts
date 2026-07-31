export type WorkspaceRole = "viewer" | "operator" | "approver" | "owner";
export type WorkspaceRelationship = "owner" | "participant";

export type Account = {
  id: string;
  actorId: string;
  loginId: string;
  displayName: string;
};

export type WorkspaceMembership = {
  id: string;
  name: string;
  state: string;
  revision: number;
  role: WorkspaceRole;
  relationship: WorkspaceRelationship;
  capabilities: string[];
};

export type WorkspaceMember = {
  actorId: string;
  accountId?: string;
  displayName: string;
  role: WorkspaceRole;
  relationship: WorkspaceRelationship;
  active: boolean;
};

export type AuthSession = {
  account: Account;
  csrfToken: string;
  expiresAt: string;
};

export type AuthState =
  | { status: "booting" }
  | { status: "anonymous" }
  | { status: "unavailable"; message: string }
  | {
    status: "authenticated";
    mode: "legacy" | "enforced";
    account: Account;
    csrfToken: string;
    expiresAt: string;
    memberships: WorkspaceMembership[];
  };
