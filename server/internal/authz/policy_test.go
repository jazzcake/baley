package authz

import (
	"testing"

	"github.com/jazzcake/baley/server/internal/domain"
)

func TestAuthorizeWorkspaceMembershipMatrix(t *testing.T) {
	humanViewer := subject("human", ActorHuman, HumanSession, WorkspaceRead)
	viewer := &Membership{ActorID: "human", WorkspaceID: "workspace-a", Role: RoleViewer, Active: true}
	if decision := Authorize(AuthorizationInput{Subject: humanViewer, Membership: viewer, WorkspaceID: "workspace-a", EntityWorkspaceID: "workspace-a", Capability: WorkspaceRead}); !decision.Allowed {
		t.Fatalf("viewer read denied: %+v", decision)
	}
	if decision := Authorize(AuthorizationInput{Subject: humanViewer, Membership: viewer, WorkspaceID: "workspace-a", EntityWorkspaceID: "workspace-a", Capability: WorkspaceOperate}); decision.Allowed || decision.Reason != ReasonRoleDenied {
		t.Fatalf("viewer mutation allowed: %+v", decision)
	}
	viewer.Active = false
	if decision := Authorize(AuthorizationInput{Subject: humanViewer, Membership: viewer, WorkspaceID: "workspace-a", EntityWorkspaceID: "workspace-a", Capability: WorkspaceRead}); decision.Reason != ReasonInactiveMembership {
		t.Fatalf("inactive membership accepted: %+v", decision)
	}
	if decision := Authorize(AuthorizationInput{Subject: humanViewer, Membership: nil, WorkspaceID: "workspace-a", EntityWorkspaceID: "workspace-a", Capability: WorkspaceRead}); decision.Reason != ReasonMissingMembership {
		t.Fatalf("missing membership accepted: %+v", decision)
	}
}

func TestAuthorizeSeparatesSameActorRolesByWorkspaceAndRejectsCrossWorkspace(t *testing.T) {
	subject := subject("person", ActorHuman, HumanSession, Capabilities...)
	ownerA := &Membership{ActorID: "person", WorkspaceID: "a", Role: RoleOwner, Active: true}
	viewerB := &Membership{ActorID: "person", WorkspaceID: "b", Role: RoleViewer, Active: true}
	if !Authorize(AuthorizationInput{Subject: subject, Membership: ownerA, WorkspaceID: "a", EntityWorkspaceID: "a", Capability: WorkspaceAdmin}).Allowed {
		t.Fatal("Workspace A owner denied")
	}
	if decision := Authorize(AuthorizationInput{Subject: subject, Membership: viewerB, WorkspaceID: "b", EntityWorkspaceID: "b", Capability: WorkspaceAdmin}); decision.Allowed || decision.Reason != ReasonRoleDenied {
		t.Fatalf("Workspace B viewer escalated: %+v", decision)
	}
	if decision := Authorize(AuthorizationInput{Subject: subject, Membership: ownerA, WorkspaceID: "a", EntityWorkspaceID: "b", Capability: WorkspaceRead}); decision.Reason != ReasonCrossWorkspace {
		t.Fatalf("cross-Workspace entity accepted: %+v", decision)
	}
	if decision := Authorize(AuthorizationInput{Subject: subject, Membership: ownerA, WorkspaceID: "a", Capability: WorkspaceRead}); decision.Reason != ReasonWorkspaceUnresolved {
		t.Fatalf("unresolved entity Workspace accepted: %+v", decision)
	}
}

func TestAuthorizeEnforcesTokenScopeAndAgentOperatorBoundary(t *testing.T) {
	agent := subject("agent", ActorAgent, AgentToken, WorkspaceRead, WorkspaceOperate)
	membership := &Membership{ActorID: "agent", WorkspaceID: "workspace", Role: RoleOperator, Active: true}
	if !Authorize(AuthorizationInput{Subject: agent, Membership: membership, WorkspaceID: "workspace", EntityWorkspaceID: "workspace", Capability: WorkspaceOperate}).Allowed {
		t.Fatal("Agent operator mutation denied")
	}
	if decision := Authorize(AuthorizationInput{Subject: agent, Membership: membership, WorkspaceID: "workspace", EntityWorkspaceID: "workspace", Capability: RunOperate}); decision.Reason != ReasonScopeDenied {
		t.Fatalf("missing token scope ignored: %+v", decision)
	}
	membership.Role = RoleOwner
	agent.Scopes = append(agent.Scopes, GateApprove)
	if decision := Authorize(AuthorizationInput{Subject: agent, Membership: membership, WorkspaceID: "workspace", EntityWorkspaceID: "workspace", Capability: GateApprove}); decision.Allowed || decision.Reason != ReasonRoleDenied {
		t.Fatalf("Agent gained approval capability: %+v", decision)
	}
}

func TestAuthorizeCommandDynamicGateAndHumanApprovalMatrix(t *testing.T) {
	agent := subject("agent", ActorAgent, AgentToken, WorkspaceRead, WorkspaceOperate, RunOperate, RecordOperate)
	agentMembership := &Membership{ActorID: "agent", WorkspaceID: "workspace", Role: RoleOperator, Active: true}
	human := subject("human", ActorHuman, HumanSession, WorkspaceRead, TaskApprove, LaneApprove, GateApprove)
	approver := &Membership{ActorID: "human", WorkspaceID: "workspace", Role: RoleApprover, Active: true}

	future := CommandAuthorizationInput{Command: "gate.attach_task", WorkspaceID: "workspace", EntityWorkspaceID: "workspace", AuthenticatedSubject: agent, AuthenticatedMembership: agentMembership, Executor: agent, ExecutorMembership: agentMembership}
	if decision := AuthorizeCommand(future); !decision.Allowed || decision.RequiredCapability != WorkspaceOperate || decision.HumanApproval {
		t.Fatalf("future Gate attach policy wrong: %+v", decision)
	}
	active := future
	active.State.FromPhaseActive = true
	active.AuthenticatedSubject, active.AuthenticatedMembership = human, approver
	active.ApprovedByActorID = "human"
	if decision := AuthorizeCommand(active); !decision.Allowed || decision.RequiredCapability != GateApprove || !decision.HumanApproval {
		t.Fatalf("active Gate attach policy wrong: %+v", decision)
	}
	active.ApprovedByActorID = "other"
	if decision := AuthorizeCommand(active); decision.Reason != ReasonApprovalMismatch {
		t.Fatalf("attestation/authenticated subject mismatch accepted: %+v", decision)
	}
}

func TestAuthorizeCommandSeparatesAgentExecutorAndHumanOwner(t *testing.T) {
	agent := subject("agent", ActorAgent, AgentToken, WorkspaceRead, WorkspaceOperate, RunOperate, RecordOperate)
	agentMembership := &Membership{ActorID: "agent", WorkspaceID: "workspace", Role: RoleOperator, Active: true}
	owner := subject("owner", ActorHuman, HumanSession, Capabilities...)
	ownerMembership := &Membership{ActorID: "owner", WorkspaceID: "workspace", Role: RoleOwner, Active: true}
	input := CommandAuthorizationInput{
		Command: "workspace.close", WorkspaceID: "workspace", EntityWorkspaceID: "workspace",
		AuthenticatedSubject: owner, AuthenticatedMembership: ownerMembership,
		Executor: agent, ExecutorMembership: agentMembership, ApprovedByActorID: "owner",
	}
	if decision := AuthorizeCommand(input); !decision.Allowed || !decision.HumanApproval || decision.RequiredCapability != WorkspaceClose {
		t.Fatalf("human owner + Agent executor denied: %+v", decision)
	}
	ownerMembership.Role = RoleApprover
	if decision := AuthorizeCommand(input); decision.Allowed || decision.Reason != ReasonRoleDenied {
		t.Fatalf("non-owner closed Workspace: %+v", decision)
	}
}

func TestEveryHumanOnlyCommandRequiresAuthenticatedHumanCapability(t *testing.T) {
	agent := subject("agent", ActorAgent, AgentToken, WorkspaceRead, WorkspaceOperate, RunOperate, RecordOperate)
	agentMembership := &Membership{ActorID: "agent", WorkspaceID: "workspace", Role: RoleOperator, Active: true}
	human := subject("human", ActorHuman, HumanSession, Capabilities...)
	ownerMembership := &Membership{ActorID: "human", WorkspaceID: "workspace", Role: RoleOwner, Active: true}
	for _, policy := range domain.MutationPolicies {
		state := CommandState{}
		humanOnly := policy.HumanApproval == domain.ApprovalAlways || policy.HumanApproval == domain.ApprovalAlwaysOwner
		if policy.HumanApproval == domain.ApprovalWhenFromPhaseActive {
			state.FromPhaseActive, humanOnly = true, true
		}
		if !humanOnly {
			continue
		}
		input := CommandAuthorizationInput{
			Command: policy.Name, State: state, WorkspaceID: "workspace", EntityWorkspaceID: "workspace",
			AuthenticatedSubject: human, AuthenticatedMembership: ownerMembership,
			Executor: agent, ExecutorMembership: agentMembership, ApprovedByActorID: "human",
		}
		if decision := AuthorizeCommand(input); !decision.Allowed || !decision.HumanApproval {
			t.Errorf("human-only command %s denied: %+v", policy.Name, decision)
		}
		input.AuthenticatedSubject, input.AuthenticatedMembership = agent, agentMembership
		input.ApprovedByActorID = "agent"
		if decision := AuthorizeCommand(input); decision.Allowed || decision.Reason != ReasonApprovalMismatch {
			t.Errorf("Agent approved human-only command %s: %+v", policy.Name, decision)
		}
	}
}

func TestAuthorizePlannedCommandReevaluatesLockedGateState(t *testing.T) {
	workspace := domain.Workspace{ID: "workspace", State: domain.WorkspaceActive, ActivePhaseID: "current"}
	from := domain.Phase{ID: "future", WorkspaceID: "workspace", State: domain.PhasePlanned}
	gate := domain.Gate{ID: "gate", WorkspaceID: "workspace", FromPhaseID: "future", ToPhaseID: "later"}
	task := domain.Task{ID: "task", PublicID: 1, WorkspaceID: "workspace", PhaseID: "future", Status: domain.TaskPending}
	agent := subject("agent", ActorAgent, AgentToken, WorkspaceRead, WorkspaceOperate, RunOperate, RecordOperate)
	agentMembership := &Membership{ActorID: "agent", WorkspaceID: "workspace", Role: RoleOperator, Active: true}
	input := CommandAuthorizationInput{
		Command: "gate.attach_task", WorkspaceID: "workspace", EntityWorkspaceID: "workspace",
		AuthenticatedSubject: agent, AuthenticatedMembership: agentMembership,
		Executor: agent, ExecutorMembership: agentMembership,
	}
	futurePlan := domain.PlanGateTaskAttachment(workspace, gate, from, task, nil, true, false)
	if decision := AuthorizePlannedCommand(input, futurePlan); !decision.Allowed || decision.HumanApproval {
		t.Fatalf("future locked plan denied: %+v", decision)
	}
	workspace.ActivePhaseID, from.State = "future", domain.PhaseActive
	activePlan := domain.PlanGateTaskAttachment(workspace, gate, from, task, nil, true, false)
	if decision := AuthorizePlannedCommand(input, activePlan); decision.Allowed || decision.Reason != ReasonApprovalMismatch {
		t.Fatalf("stale future authorization survived active transition: %+v", decision)
	}
	human := subject("human", ActorHuman, HumanSession, WorkspaceRead, GateApprove)
	approver := &Membership{ActorID: "human", WorkspaceID: "workspace", Role: RoleApprover, Active: true}
	input.AuthenticatedSubject, input.AuthenticatedMembership, input.ApprovedByActorID = human, approver, "human"
	if decision := AuthorizePlannedCommand(input, activePlan); !decision.Allowed || !decision.HumanApproval || decision.RequiredCapability != GateApprove {
		t.Fatalf("active locked plan human approval denied: %+v", decision)
	}
}

func TestAuthorizeCommandRejectsContradictorySameIDExecutorProvenance(t *testing.T) {
	human := subject("human", ActorHuman, HumanSession, WorkspaceRead, TaskApprove)
	membership := &Membership{ActorID: "human", WorkspaceID: "workspace", Role: RoleApprover, Active: true}
	input := CommandAuthorizationInput{
		Command: "task.confirm", WorkspaceID: "workspace", EntityWorkspaceID: "workspace",
		AuthenticatedSubject: human, AuthenticatedMembership: membership, ApprovedByActorID: "human",
		Executor:           subject("human", ActorAgent, AgentToken, WorkspaceRead, WorkspaceOperate),
		ExecutorMembership: &Membership{ActorID: "human", WorkspaceID: "workspace", Role: RoleOperator, Active: true},
	}
	if decision := AuthorizeCommand(input); decision.Allowed || decision.Reason != ReasonExecutorDenied {
		t.Fatalf("contradictory same-ID executor accepted: %+v", decision)
	}
}

func subject(id string, kind ActorKind, credential CredentialKind, scopes ...Capability) Subject {
	return Subject{ActorID: id, Kind: kind, Credential: credential, Scopes: append([]Capability(nil), scopes...)}
}
