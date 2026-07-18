package authz

import (
	"strings"

	"github.com/jazzcake/baley/server/internal/domain"
)

type Subject struct {
	ActorID    string
	Kind       ActorKind
	Credential CredentialKind
	Scopes     []Capability
}

type Membership struct {
	ActorID     string
	WorkspaceID string
	Role        Role
	Active      bool
}

type DenyReason string

const (
	ReasonAllowed             DenyReason = "allowed"
	ReasonInvalidSubject      DenyReason = "invalid_subject"
	ReasonMissingMembership   DenyReason = "missing_membership"
	ReasonInactiveMembership  DenyReason = "inactive_membership"
	ReasonCrossWorkspace      DenyReason = "cross_workspace"
	ReasonWorkspaceUnresolved DenyReason = "entity_workspace_unresolved"
	ReasonRoleDenied          DenyReason = "role_denied"
	ReasonScopeDenied         DenyReason = "scope_denied"
	ReasonApprovalMismatch    DenyReason = "approval_subject_mismatch"
	ReasonExecutorDenied      DenyReason = "executor_denied"
)

type Decision struct {
	Allowed            bool
	Reason             DenyReason
	RequiredCapability Capability
	HumanApproval      bool
}

type AuthorizationInput struct {
	Subject           Subject
	Membership        *Membership
	WorkspaceID       string
	EntityWorkspaceID string
	Capability        Capability
}

func Authorize(input AuthorizationInput) Decision {
	decision := Decision{Reason: ReasonInvalidSubject, RequiredCapability: input.Capability}
	if strings.TrimSpace(input.Subject.ActorID) == "" || strings.TrimSpace(input.WorkspaceID) == "" || !knownCapability(input.Capability) {
		return decision
	}
	if strings.TrimSpace(input.EntityWorkspaceID) == "" {
		decision.Reason = ReasonWorkspaceUnresolved
		return decision
	}
	if input.EntityWorkspaceID != input.WorkspaceID {
		decision.Reason = ReasonCrossWorkspace
		return decision
	}
	if input.Membership == nil || input.Membership.ActorID != input.Subject.ActorID || input.Membership.WorkspaceID != input.WorkspaceID {
		decision.Reason = ReasonMissingMembership
		return decision
	}
	if !input.Membership.Active {
		decision.Reason = ReasonInactiveMembership
		return decision
	}
	roleCapabilities, err := ResolveRole(input.Membership.Role, input.Subject.Kind, input.Subject.Credential)
	if err != nil {
		decision.Reason = ReasonRoleDenied
		return decision
	}
	if !capabilitySet(roleCapabilities)[input.Capability] {
		decision.Reason = ReasonRoleDenied
		return decision
	}
	if !capabilitySet(input.Subject.Scopes)[input.Capability] {
		decision.Reason = ReasonScopeDenied
		return decision
	}
	decision.Allowed, decision.Reason = true, ReasonAllowed
	return decision
}

type CommandState struct {
	FromPhaseActive bool
}

type CommandAuthorizationInput struct {
	Command                 string
	State                   CommandState
	WorkspaceID             string
	EntityWorkspaceID       string
	AuthenticatedSubject    Subject
	AuthenticatedMembership *Membership
	Executor                Subject
	ExecutorMembership      *Membership
	ApprovedByActorID       string
}

func AuthorizeCommand(input CommandAuthorizationInput) Decision {
	required, approval, ok := commandRequirement(input.Command, input.State)
	if !ok {
		return Decision{Reason: ReasonInvalidSubject}
	}
	return authorizeResolvedCommand(input, required, approval)
}

// AuthorizePlannedCommand composes authorization with a mutation plan derived
// from the current locked Workspace snapshot.
func AuthorizePlannedCommand(input CommandAuthorizationInput, plan domain.DomainMutationPlan) Decision {
	if plan.Command != input.Command || plan.Evaluation.HasErrors() || !knownCapability(Capability(plan.RequiredCapability)) {
		return Decision{Reason: ReasonInvalidSubject}
	}
	approval := plan.HumanApproval == domain.ApprovalAlways || plan.HumanApproval == domain.ApprovalAlwaysOwner || plan.HumanApproval == domain.ApprovalWhenFromPhaseActive
	return authorizeResolvedCommand(input, Capability(plan.RequiredCapability), approval)
}

func authorizeResolvedCommand(input CommandAuthorizationInput, required Capability, approval bool) Decision {
	result := Decision{RequiredCapability: required, HumanApproval: approval, Reason: ReasonInvalidSubject}
	if !approval {
		if !sameSubject(input.AuthenticatedSubject, input.Executor) || !sameMembership(input.AuthenticatedMembership, input.ExecutorMembership) || input.ApprovedByActorID != "" {
			result.Reason = ReasonExecutorDenied
			return result
		}
		decision := Authorize(AuthorizationInput{Subject: input.AuthenticatedSubject, Membership: input.AuthenticatedMembership, WorkspaceID: input.WorkspaceID, EntityWorkspaceID: input.EntityWorkspaceID, Capability: required})
		decision.HumanApproval = false
		return decision
	}
	if input.AuthenticatedSubject.Kind != ActorHuman || input.AuthenticatedSubject.Credential != HumanSession || input.ApprovedByActorID != input.AuthenticatedSubject.ActorID {
		result.Reason = ReasonApprovalMismatch
		return result
	}
	approver := Authorize(AuthorizationInput{Subject: input.AuthenticatedSubject, Membership: input.AuthenticatedMembership, WorkspaceID: input.WorkspaceID, EntityWorkspaceID: input.EntityWorkspaceID, Capability: required})
	approver.HumanApproval = true
	if !approver.Allowed {
		return approver
	}
	if input.Command == "workspace.close" && input.AuthenticatedMembership.Role != RoleOwner {
		result.Reason = ReasonRoleDenied
		return result
	}
	if input.Executor.ActorID == input.AuthenticatedSubject.ActorID {
		if !sameSubject(input.AuthenticatedSubject, input.Executor) || !sameMembership(input.AuthenticatedMembership, input.ExecutorMembership) {
			result.Reason = ReasonExecutorDenied
			return result
		}
	} else {
		executor := Authorize(AuthorizationInput{Subject: input.Executor, Membership: input.ExecutorMembership, WorkspaceID: input.WorkspaceID, EntityWorkspaceID: input.EntityWorkspaceID, Capability: WorkspaceOperate})
		if !executor.Allowed {
			result.Reason = ReasonExecutorDenied
			return result
		}
	}
	return approver
}

func sameSubject(left, right Subject) bool {
	return left.ActorID == right.ActorID && left.Kind == right.Kind && left.Credential == right.Credential && sameCapabilitySet(left.Scopes, right.Scopes)
}

func sameMembership(left, right *Membership) bool {
	return left != nil && right != nil && *left == *right
}

func commandRequirement(command string, state CommandState) (Capability, bool, bool) {
	for _, policy := range domain.MutationPolicies {
		if policy.Name != command {
			continue
		}
		capability := Capability(policy.Capability)
		approval := policy.HumanApproval == domain.ApprovalAlways || policy.HumanApproval == domain.ApprovalAlwaysOwner
		if policy.HumanApproval == domain.ApprovalWhenFromPhaseActive && state.FromPhaseActive {
			capability, approval = Capability(policy.ActiveCapability), true
		}
		return capability, approval, knownCapability(capability)
	}
	return "", false, false
}

func knownCapability(capability Capability) bool {
	for _, candidate := range Capabilities {
		if candidate == capability {
			return true
		}
	}
	return false
}
