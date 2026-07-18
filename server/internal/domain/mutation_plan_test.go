package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMutationPoliciesCoverCommandContract(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "v1", "commands.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract struct {
		Mutations map[string]struct{ Capability, HumanApproval, ActiveCapability string } `json:"mutations"`
	}
	if err = json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	policies := map[string]MutationPolicy{}
	for _, policy := range MutationPolicies {
		if _, duplicate := policies[policy.Name]; duplicate {
			t.Errorf("duplicate policy %s", policy.Name)
		}
		policies[policy.Name] = policy
	}
	if len(policies) != len(contract.Mutations) {
		t.Fatalf("policy count %d != contract %d", len(policies), len(contract.Mutations))
	}
	for name, literal := range contract.Mutations {
		policy, ok := policies[name]
		if !ok {
			t.Errorf("missing policy %s", name)
			continue
		}
		if policy.Capability != literal.Capability || string(policy.HumanApproval) != literal.HumanApproval && !(policy.HumanApproval == "" && literal.HumanApproval == "none") || policy.ActiveCapability != literal.ActiveCapability {
			t.Errorf("policy drift %s: %+v vs %+v", name, policy, literal)
		}
		if MutationHandlers[name] == nil {
			t.Errorf("mutation %s has no executable handler", name)
		}
	}
	if len(MutationHandlers) != len(contract.Mutations) {
		t.Fatalf("handler count %d != contract %d", len(MutationHandlers), len(contract.Mutations))
	}
}

func TestEveryMutationHandlerProducesValidPlanAndEvidence(t *testing.T) {
	for command := range MutationHandlers {
		t.Run(command, func(t *testing.T) {
			plan := PlanMutation(command, validMutationContext(t, command))
			if plan.Command != command || plan.RequiredCapability == "" || plan.Evaluation.HasErrors() {
				t.Fatalf("handler did not produce an executable plan: %+v", plan)
			}
			policy, _ := policyFor(command, false)
			if policy.OperationalNoEvent {
				if len(plan.Events) != 0 {
					t.Fatalf("operational exception emitted Event: %+v", plan.Events)
				}
				return
			}
			if len(plan.Events) == 0 {
				t.Fatal("mutation emitted no Event")
			}
			found := false
			for _, event := range plan.Events {
				if event.Type == policy.EventType {
					found = true
				}
				if command == "gate.pass" && event.Type == "gate.passed" {
					continue // Preview intentionally has no attestation ID; execute evidence is tested separately.
				}
				if evaluation := ValidateEventEvidence(event); evaluation.HasErrors() {
					t.Errorf("invalid Event evidence for %s: %+v", event.Type, evaluation)
				}
			}
			if !found {
				t.Errorf("expected Event %s not emitted: %+v", policy.EventType, plan.Events)
			}
		})
	}
}

func TestClosedWorkspaceRejectsEveryWorkspaceScopedHandler(t *testing.T) {
	allowedWithoutExistingWorkspace := map[string]bool{"project.bootstrap": true, "workspace.create": true}
	for command := range MutationHandlers {
		if allowedWithoutExistingWorkspace[command] || command == "workspace.close" {
			continue
		}
		t.Run(command, func(t *testing.T) {
			context := validMutationContext(t, command)
			context.Workspace.State = WorkspaceClosed
			plan := PlanMutation(command, context)
			if !plan.Evaluation.HasErrors() || len(plan.Events) != 0 {
				t.Fatalf("closed Workspace mutation accepted: %+v", plan)
			}
		})
	}
}

func TestExecuteRequiresAndRecordsBoundHumanApproval(t *testing.T) {
	context := validMutationContext(t, "task.confirm")
	context.Mode = MutationExecute
	context.ExecutedByActorID = "agent"
	plan := PlanMutation("task.confirm", context)
	if !hasDiagnostic(plan.Evaluation.Errors, CodeHumanApprovalRequired) || len(plan.Events) != 0 || plan.ProjectedDiff != nil {
		t.Fatalf("approval-free execution accepted: %+v", plan)
	}
	approveMutation(t, "task.confirm", &context, "member")
	plan = PlanMutation("task.confirm", context)
	if plan.Evaluation.HasErrors() || len(plan.Events) != 2 || plan.Events[1].Payload["action"] != "task_confirm" {
		t.Fatalf("bound approval execution rejected: %+v", plan)
	}
	context.Attestation.WorkspaceRevision--
	if plan = PlanMutation("task.confirm", context); !hasDiagnostic(plan.Evaluation.Errors, CodeHumanApprovalMismatch) {
		t.Fatalf("stale revision attestation accepted: %+v", plan)
	}
}

func TestExecuteDerivesActiveGateApprovalInsteadOfTrustingCallerFlag(t *testing.T) {
	context := validMutationContext(t, "gate.attach_task")
	context.Mode = MutationExecute
	context.Workspace.ActivePhaseID = context.FromPhase.ID
	context.FromPhase.State = PhaseActive
	context.ExecutedByActorID = "agent"
	plan := PlanMutation("gate.attach_task", context)
	if !hasDiagnostic(plan.Evaluation.Errors, CodeHumanApprovalRequired) {
		t.Fatalf("active Gate attach bypassed approval: %+v", plan)
	}
	approveMutation(t, "gate.attach_task", &context, "member")
	plan = PlanMutation("gate.attach_task", context)
	if plan.Evaluation.HasErrors() || plan.RequiredCapability != "gate:approve" || len(plan.Events) != 2 {
		t.Fatalf("approved active Gate attach rejected: %+v", plan)
	}
}

func TestExecuteRequiresExactWarningAcknowledgementWithoutRequiringReason(t *testing.T) {
	context := validMutationContext(t, "dependency.connect")
	context.Mode = MutationExecute
	context.ExecutedByActorID = "agent"
	context.RunningTaskIDs = map[string]bool{"other": true}
	plan := PlanMutation("dependency.connect", context)
	if !plan.Evaluation.HasErrors() {
		t.Fatalf("unacknowledged execution warning accepted: %+v", plan)
	}
	context.Acknowledgement.Codes = []string{CodeRunningTaskDependencyAdded}
	plan = PlanMutation("dependency.connect", context)
	if plan.Evaluation.HasErrors() || len(plan.Events) != 1 || plan.Events[0].Payload["proceedReason"] != "" {
		t.Fatalf("acknowledged warning execution rejected: %+v", plan)
	}
	if evaluation := ValidateEventEvidence(plan.Events[0]); evaluation.HasErrors() {
		t.Fatalf("warning audit evidence invalid: %+v", evaluation)
	}
}

func TestGatePassExecutionBindsSnapshotAttestationAndRevision(t *testing.T) {
	context := validMutationContext(t, "gate.pass")
	context.Mode = MutationExecute
	context.ExecutedByActorID = "agent"
	approveMutation(t, "gate.pass", &context, "member")
	plan := PlanMutation("gate.pass", context)
	if plan.Evaluation.HasErrors() || len(plan.Events) != 4 || plan.Events[0].Payload["workspaceRevision"] != int64(8) {
		t.Fatalf("bound Gate pass rejected: %+v", plan)
	}
	context.Attestation.DecisionSnapshotHash = "sha256:stale"
	plan = PlanMutation("gate.pass", context)
	if !hasDiagnostic(plan.Evaluation.Errors, CodeHumanApprovalMismatch) {
		t.Fatalf("stale Gate snapshot approval accepted: %+v", plan)
	}
}

func TestWorkspaceCloseRequiresLastPhaseAndProjectsResidualWarnings(t *testing.T) {
	context := validMutationContext(t, "workspace.close")
	context.Phases = []Phase{context.Phase, {ID: "future", WorkspaceID: context.Workspace.ID, Position: 2, State: PhasePlanned}}
	if plan := PlanMutation("workspace.close", context); !plan.Evaluation.HasErrors() {
		t.Fatalf("Workspace closed before last Phase: %+v", plan)
	}
	context.Phases = []Phase{context.Phase}
	context.Tasks = []Task{context.Task}
	context.Lanes = []Lane{context.Lane}
	context.Mode = MutationExecute
	context.ExecutedByActorID = "agent"
	context.Acknowledgement.Codes = []string{CodeWorkspaceCloseResidualTask, CodeWorkspaceCloseActiveLane}
	approveMutation(t, "workspace.close", &context, "member")
	if plan := PlanMutation("workspace.close", context); !hasDiagnostic(plan.Evaluation.Errors, CodeHumanApprovalMismatch) {
		t.Fatalf("non-owner Workspace close accepted: %+v", plan)
	}
	approveMutation(t, "workspace.close", &context, "owner")
	plan := PlanMutation("workspace.close", context)
	if plan.Evaluation.HasErrors() || len(plan.Evaluation.Warnings) != 2 || len(plan.Events) != 3 {
		t.Fatalf("approved close with recorded residuals rejected: %+v", plan)
	}
}

func TestImplementedRegistryDerivesDanglingWarningAndResultingRevision(t *testing.T) {
	context := validMutationContext(t, "task.report_implemented")
	delete(context.Graph.GateConditionTaskIDs, context.Task.ID)
	context.Acknowledgement = WarningAcknowledgement{Codes: []string{CodeDanglingPath}, Enforce: true}
	plan := PlanMutation("task.report_implemented", context)
	if plan.Evaluation.HasErrors() || !hasDiagnostic(plan.Evaluation.Warnings, CodeDanglingPath) {
		t.Fatalf("dangling implementation projection failed: %+v", plan)
	}
	diff, ok := plan.ProjectedDiff.(map[string]any)
	decision, decisionOK := diff["decision"].(TaskDecision)
	if !ok || !decisionOK || decision.ExpectedWorkspaceRevision != context.Workspace.Revision+1 {
		t.Fatalf("decision revision was not derived: %+v", plan.ProjectedDiff)
	}
}

func TestGateConditionMutationRejectsLinkFromAnotherGate(t *testing.T) {
	context := validMutationContext(t, "gate.pass_task")
	context.Condition.GateID = "other-gate"
	if plan := PlanMutation("gate.pass_task", context); !plan.Evaluation.HasErrors() {
		t.Fatalf("foreign Gate condition accepted: %+v", plan)
	}
}

func TestApprovalCommandHashBindsNormalizedReasonPayload(t *testing.T) {
	for _, command := range []string{"lane.close_out", "lane.discard", "task.discard", "gate.pass_task", "gate.revoke_task_pass"} {
		t.Run(command, func(t *testing.T) {
			context := validMutationContext(t, command)
			context.Reason = "first reason"
			first := PlanMutation(command, context)
			context.Reason = "second reason"
			second := PlanMutation(command, context)
			if first.Evaluation.HasErrors() || second.Evaluation.HasErrors() || first.CommandHash == second.CommandHash {
				t.Fatalf("reason was not bound to command hash: first=%+v second=%+v", first, second)
			}
			context.Reason = "  first reason  "
			normalized := PlanMutation(command, context)
			if first.CommandHash != normalized.CommandHash {
				t.Fatalf("equivalent normalized reason changed command hash: %s != %s", first.CommandHash, normalized.CommandHash)
			}
		})
	}
}

func validMutationContext(t *testing.T, command string) MutationContext {
	t.Helper()
	now := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	workspace := Workspace{ID: "workspace", Name: "Main", State: WorkspaceActive, ActivePhaseID: "p1", Revision: 7}
	p1 := Phase{ID: "p1", WorkspaceID: workspace.ID, Position: 1, State: PhaseActive}
	p2 := Phase{ID: "p2", WorkspaceID: workspace.ID, Position: 2, State: PhasePlanned}
	lane := Lane{ID: "lane", WorkspaceID: workspace.ID, Name: "Server", State: LaneActive}
	task := Task{ID: "task", PublicID: 1, WorkspaceID: workspace.ID, LaneID: lane.ID, PhaseID: p1.ID, PhasePosition: 1, Title: "Task", Status: TaskInProgress}
	graph, evaluation := NewWorkspaceGraph([]Task{task, {ID: "other", PublicID: 2, WorkspaceID: workspace.ID, LaneID: lane.ID, PhaseID: p1.ID, PhasePosition: 1, Title: "Other", Status: TaskPending}}, nil, nil)
	if evaluation.HasErrors() {
		t.Fatal(evaluation)
	}
	context := MutationContext{
		Mode: MutationPreview, ProjectID: "project", Workspace: workspace, Phase: p1, FromPhase: p1, ToPhase: p2,
		Phases: []Phase{p1}, Lane: lane, Gate: Gate{ID: "gate", WorkspaceID: workspace.ID, FromPhaseID: p1.ID, ToPhaseID: p2.ID},
		Task: task, Graph: graph, Now: now, Reason: "because", Title: "Updated", Description: "goal", Summary: "summary", NextAction: "next",
	}
	switch command {
	case "project.bootstrap":
		context.Workspace = Workspace{ID: workspace.ID, Name: "Main", State: WorkspaceDraft}
		context.Repository = Repository{ID: "repository", WorkspaceID: workspace.ID, Name: "Main", RemoteURL: "https://example.com/repo.git"}
	case "repository.register":
		context.Repository = Repository{ID: "repository", WorkspaceID: workspace.ID, Name: "Main", RemoteURL: "https://example.com/repo.git"}
	case "workspace.create":
		context.Workspace = Workspace{ID: "new-workspace", Name: "New", State: WorkspaceDraft}
	case "workspace.activate":
		context.Workspace = Workspace{ID: workspace.ID, Name: workspace.Name, State: WorkspaceDraft}
		context.Phases = []Phase{{ID: "p1", WorkspaceID: workspace.ID, Position: 1, State: PhasePlanned}}
	case "workspace.close":
	case "phase.create":
		context.Phase = p2
	case "lane.create", "lane.update", "lane.close_out", "lane.discard":
	case "gate.create":
	case "task.create":
		context.Task = Task{ID: "new", PublicID: 3, WorkspaceID: workspace.ID, LaneID: lane.ID, PhaseID: p1.ID, PhasePosition: 1, Title: "New", Status: TaskPending}
		context.InitialPatch = DependencyPatch{Add: []Dependency{{FromTaskID: "task", ToTaskID: "new"}}}
	case "task.update":
	case "task.set_terminal", "task.clear_terminal":
		if command == "task.clear_terminal" {
			withTerminal := task
			withTerminal.TerminalReason = "leaf"
			context.Task = withTerminal
			context.Graph, _ = NewWorkspaceGraph([]Task{withTerminal, graph.Tasks["other"]}, nil, nil)
		}
	case "task.block":
	case "task.unblock":
		blocked := task
		blocked.BlockedAt, blocked.BlockerReason = &now, "waiting"
		context.Task = blocked
	case "task.report_implemented":
		context.Graph.GateConditionTaskIDs[task.ID] = struct{}{}
		context.Assessment = "implemented"
		context.Records = []TaskRecord{
			{WorkspaceID: workspace.ID, TaskID: task.ID, Type: RecordDetailedPlan},
			{WorkspaceID: workspace.ID, TaskID: task.ID, Type: RecordIndependentReview},
			{WorkspaceID: workspace.ID, TaskID: task.ID, Type: RecordCompletionReport},
		}
	case "task.confirm":
		context.Task.Status = TaskImplemented
	case "task.discard":
	case "task.rework":
		context.Task.Status = TaskImplemented
	case "dependency.connect":
		context.DependencyPatch = DependencyPatch{Add: []Dependency{{FromTaskID: "task", ToTaskID: "other"}}}
	case "dependency.disconnect":
		context.Graph, _ = NewWorkspaceGraph([]Task{task, graph.Tasks["other"]}, []Dependency{{FromTaskID: "task", ToTaskID: "other"}}, nil)
		context.DependencyPatch = DependencyPatch{Remove: []Dependency{{FromTaskID: "task", ToTaskID: "other"}}}
	case "dependency.patch":
		context.DependencyPatch = DependencyPatch{Add: []Dependency{{FromTaskID: "task", ToTaskID: "other"}}}
	case "gate.attach_task":
		context.Workspace.ActivePhaseID = "other-phase"
		context.FromPhase.State = PhasePlanned
	case "gate.detach_task":
		context.Workspace.ActivePhaseID = "other-phase"
		context.FromPhase.State = PhasePlanned
		context.Conditions = []GateTaskCondition{{WorkspaceID: workspace.ID, GateID: "gate", LinkID: "link", TaskID: task.ID}}
	case "gate.pass_task":
		context.Condition = GateTaskCondition{WorkspaceID: workspace.ID, GateID: "gate", LinkID: "link", TaskID: task.ID, TaskStatus: TaskInProgress}
	case "gate.revoke_task_pass":
		context.Condition = GateTaskCondition{WorkspaceID: workspace.ID, GateID: "gate", LinkID: "link", TaskID: task.ID, TaskStatus: TaskInProgress, Passed: true, PassReason: "old"}
	case "gate.pass":
		context.Conditions = []GateTaskCondition{{WorkspaceID: workspace.ID, GateID: "gate", LinkID: "link", TaskID: task.ID, TaskStatus: TaskConfirmed}}
	case "run.start":
		context.Task.Status = TaskPending
		context.RunStartRequest = RunStartRequest{RunID: "run", Identity: RunStartIdentity{WorkspaceID: workspace.ID, TaskID: task.ID, ClientRunID: "client", Kind: RunImplementation}, OperatorActorID: "agent", LeaseToken: "secret", LeaseDuration: time.Minute, Now: now}
	case "run.heartbeat":
		context.Run = validRegistryRun(now)
		context.ExpectedVersion, context.LeaseToken, context.LeaseExtension = 1, "secret", time.Minute
	case "run.succeed", "run.fail", "run.cancel", "run.interrupt":
		context.Run = validRegistryRun(now)
		context.ExpectedVersion = 1
		context.Summary = "terminal summary"
	case "run.correct":
		ended := now.Add(-time.Minute)
		context.Run = validRegistryRun(now)
		context.Run.Status, context.Run.ErrorSummary, context.Run.EndedAt = RunFailed, "old failure", &ended
		context.ExpectedVersion, context.RunStatus, context.Summary = 1, RunSucceeded, "corrected"
	case "record.register":
		context.TaskRecordsRoot = "task-records"
		context.Registration = RecordRegistration{ID: "record", WorkspaceID: workspace.ID, TaskID: task.ID, RunID: "run", Type: RecordCompletionReport, RepositoryID: "repository", RelativePath: "task-records/task/report.md", WorkingTreeHash: "sha256:" + repeatHex("a", 64), ShortSummary: "done"}
	case "record.attach_commit":
		context.Record = TaskRecord{ID: "record", WorkspaceID: workspace.ID, TaskID: task.ID, RepositoryID: "repository", State: RecordReportedUncommitted}
		context.CommitSHA, context.BlobSHA = repeatHex("a", 40), repeatHex("b", 40)
	case "commit.attach":
		context.Commit = CommitReference{ID: "commit", WorkspaceID: workspace.ID, TaskID: task.ID, RepositoryID: "repository", CommitSHA: repeatHex("a", 40), Relation: CommitProduced}
	case "git.observe":
		context.GitObservation = RunGitObservation{ID: "observation", WorkspaceID: workspace.ID, RunID: "run", RepositoryID: "repository", ObservedAt: now}
	default:
		t.Fatalf("missing valid fixture for %s", command)
	}
	return context
}

func validRegistryRun(now time.Time) Run {
	return Run{ID: "run", WorkspaceID: "workspace", TaskID: "task", Status: RunRunning, Version: 1, LeaseTokenHash: HashLeaseToken("secret"), HeartbeatAt: now.Add(-time.Minute), LeaseExpiresAt: now.Add(time.Minute)}
}

func approveMutation(t *testing.T, command string, context *MutationContext, role string) {
	t.Helper()
	previewContext := *context
	previewContext.Mode = MutationPreview
	previewContext.Attestation = HumanApprovalAttestation{}
	preview := PlanMutation(command, previewContext)
	if preview.Evaluation.HasErrors() {
		t.Fatalf("approval preview failed: %+v", preview)
	}
	primary, ok := primaryMutationEvent(preview)
	if !ok {
		t.Fatalf("approval preview has no primary Event: %+v", preview)
	}
	context.Attestation = HumanApprovalAttestation{
		ID: "attestation", Action: canonicalApprovalAction(command), EntityType: primary.EntityType, EntityID: primary.EntityID,
		WorkspaceRevision: context.Workspace.Revision, ApprovedByActorID: "owner", ApproverRole: role,
		ApprovedCommandHash: preview.CommandHash, DecisionSnapshotHash: preview.DecisionSnapshotHash,
	}
}

func TestPlanTaskMutationUsesLifecycleAndApprovalPolicy(t *testing.T) {
	now := time.Now()
	task := Task{ID: "task", Status: TaskInProgress}
	workspace := Workspace{ID: task.WorkspaceID, State: WorkspaceActive}
	blocked, plan := PlanTaskMutation(workspace, "task.block", task, " waiting ", now)
	if plan.Evaluation.HasErrors() || blocked.BlockedAt == nil || plan.RequiredCapability != "workspace:operate" || plan.HumanApproval != ApprovalNone || plan.Events[0].Type != "task.blocked" {
		t.Fatalf("bad block plan: %+v %+v", blocked, plan)
	}
	discarded, plan := PlanTaskMutation(workspace, "task.discard", task, "obsolete", now)
	if plan.Evaluation.HasErrors() || discarded.Status != TaskDiscarded || plan.RequiredCapability != "task:approve" || plan.HumanApproval != ApprovalAlways {
		t.Fatalf("bad discard plan: %+v %+v", discarded, plan)
	}
}

func TestPlanDependencyMutationPreservesAtomicPreview(t *testing.T) {
	graph, evaluation := NewWorkspaceGraph([]Task{{ID: "a", WorkspaceID: "w"}, {ID: "b", WorkspaceID: "w"}, {ID: "c", WorkspaceID: "w"}}, []Dependency{{FromTaskID: "a", ToTaskID: "b"}}, nil)
	if evaluation.HasErrors() {
		t.Fatal(evaluation)
	}
	plan := PlanDependencyMutation(Workspace{ID: "w", State: WorkspaceActive}, "dependency.patch", graph, DependencyPatch{Remove: []Dependency{{FromTaskID: "a", ToTaskID: "b"}}, Add: []Dependency{{FromTaskID: "b", ToTaskID: "a"}, {FromTaskID: "a", ToTaskID: "c"}}}, nil)
	if plan.Evaluation.HasErrors() || len(plan.Events) != 1 || plan.Events[0].Type != "dependency.patched" {
		t.Fatalf("bad patch plan: %+v", plan)
	}
	if len(graph.DependencyList()) != 1 || graph.DependencyList()[0].FromTaskID != "a" {
		t.Fatal("preview mutated source graph")
	}
}

func TestPlanLaneTerminationRequiresHumanApproval(t *testing.T) {
	lane := Lane{ID: "lane", State: LaneActive}
	next, plan := PlanLaneTermination(Workspace{ID: lane.WorkspaceID, State: WorkspaceActive}, "lane.close_out", lane, "goal reached")
	if plan.Evaluation.HasErrors() || next.State != LaneClosedOut || plan.RequiredCapability != "lane:approve" || plan.HumanApproval != ApprovalAlways {
		t.Fatalf("bad lane plan: %+v %+v", next, plan)
	}
}

func TestPlanGateTaskAttachmentUsesDynamicCapabilityAndFreezeRules(t *testing.T) {
	workspace := Workspace{ID: "workspace", State: WorkspaceActive, ActivePhaseID: "phase"}
	gate := Gate{ID: "gate", WorkspaceID: "workspace", FromPhaseID: "phase"}
	task := Task{ID: "task", WorkspaceID: "workspace", PhaseID: "phase"}
	active := Phase{ID: "phase", WorkspaceID: "workspace", State: PhaseActive}
	plan := PlanGateTaskAttachment(workspace, gate, active, task, nil, true, false)
	if plan.Evaluation.HasErrors() || plan.RequiredCapability != "gate:approve" || plan.HumanApproval != ApprovalWhenFromPhaseActive {
		t.Fatalf("active attach policy wrong: %+v", plan)
	}
	planned := Phase{ID: "phase", WorkspaceID: "workspace", State: PhasePlanned}
	workspace.ActivePhaseID = "other"
	plan = PlanGateTaskAttachment(workspace, gate, planned, task, nil, true, false)
	if plan.Evaluation.HasErrors() || plan.RequiredCapability != "workspace:operate" || plan.HumanApproval != ApprovalNone {
		t.Fatalf("future attach policy wrong: %+v", plan)
	}
	workspace.ActivePhaseID = "phase"
	plan = PlanGateTaskAttachment(workspace, gate, active, task, []GateTaskCondition{{TaskID: "task"}}, false, false)
	if !hasDiagnostic(plan.Evaluation.Errors, CodeActiveGateDetachForbidden) {
		t.Fatalf("active detach accepted: %+v", plan)
	}
	task.TerminalReason = "intentional"
	workspace.ActivePhaseID = "other"
	plan = PlanGateTaskAttachment(workspace, gate, planned, task, nil, true, false)
	if !hasDiagnostic(plan.Evaluation.Errors, CodeTerminalPathConflict) {
		t.Fatalf("terminal attach accepted: %+v", plan)
	}
}

func TestWorkspacePhaseLaneAndGateCreationPlans(t *testing.T) {
	workspace := Workspace{ID: "workspace", State: WorkspaceDraft}
	phases := []Phase{{ID: "p1", WorkspaceID: "workspace", Position: 1, State: PhasePlanned}}
	active, nextPhases, plan := PlanWorkspaceActivate(workspace, phases)
	if plan.Evaluation.HasErrors() || active.State != WorkspaceActive || nextPhases[0].State != PhaseActive || len(plan.Events) != 2 {
		t.Fatalf("activation failed: %+v %+v", active, plan)
	}
	phase2 := Phase{ID: "p2", WorkspaceID: "workspace", Position: 2, State: PhasePlanned}
	plan = PlanPhaseCreate(active, nextPhases, phase2)
	if plan.Evaluation.HasErrors() || plan.Events[0].Type != "phase.created" {
		t.Fatalf("phase plan failed: %+v", plan)
	}
	lane := Lane{ID: "lane", WorkspaceID: "workspace", Name: "Server", State: LaneActive}
	plan = PlanLaneCreate(active, lane)
	if plan.Evaluation.HasErrors() {
		t.Fatalf("lane create failed: %+v", plan)
	}
	updated, plan := PlanLaneUpdate(active, lane, "Server API", "ship", "working")
	if plan.Evaluation.HasErrors() || updated.Goal != "ship" {
		t.Fatalf("lane update failed: %+v", plan)
	}
	gate := Gate{ID: "gate", WorkspaceID: "workspace", FromPhaseID: "p1", ToPhaseID: "p2"}
	plan = PlanGateCreate(active, gate, nextPhases[0], phase2, nil)
	if plan.Evaluation.HasErrors() {
		t.Fatalf("gate create failed: %+v", plan)
	}
	plan = PlanGateCreate(active, Gate{ID: "duplicate", WorkspaceID: "workspace", FromPhaseID: "p1", ToPhaseID: "p2"}, nextPhases[0], phase2, []Gate{gate})
	if !plan.Evaluation.HasErrors() {
		t.Fatal("duplicate Gate endpoints accepted")
	}
}

func TestPlanTaskCreateBuildsInitialRelationsAtomically(t *testing.T) {
	workspace := Workspace{ID: "workspace", State: WorkspaceActive}
	lane := Lane{ID: "lane", WorkspaceID: "workspace", State: LaneActive}
	phase := Phase{ID: "phase", WorkspaceID: "workspace", Position: 1, State: PhaseActive}
	graph, evaluation := NewWorkspaceGraph([]Task{{ID: "before", WorkspaceID: "workspace", LaneID: "lane", PhaseID: "phase", Status: TaskConfirmed}}, nil, nil)
	if evaluation.HasErrors() {
		t.Fatal(evaluation)
	}
	task := Task{ID: "new", PublicID: 2, WorkspaceID: "workspace", LaneID: "lane", PhaseID: "phase", PhasePosition: 1, Title: "Build", Status: TaskPending}
	plan := PlanTaskCreate(workspace, lane, phase, graph, task, DependencyPatch{Add: []Dependency{{FromTaskID: "before", ToTaskID: "new"}}})
	if plan.Evaluation.HasErrors() || len(plan.Events) != 1 || plan.Events[0].Type != "task.created" {
		t.Fatalf("task create failed: %+v", plan)
	}
	if _, exists := graph.Tasks["new"]; exists {
		t.Fatal("preview mutated source graph")
	}
	bad := task
	bad.ID = "bad"
	bad.PublicID = 3
	bad.ParentTaskID = "missing"
	plan = PlanTaskCreate(workspace, lane, phase, graph, bad, DependencyPatch{})
	if !hasDiagnostic(plan.Evaluation.Errors, CodeNotFound) {
		t.Fatalf("missing parent accepted: %+v", plan)
	}
}

func TestPlanTaskUpdateProtectsTerminalTask(t *testing.T) {
	task := Task{ID: "task", Title: "old", Status: TaskInProgress}
	workspace := Workspace{ID: task.WorkspaceID, State: WorkspaceActive}
	next, plan := PlanTaskUpdate(workspace, task, "new", "description", "summary", "next")
	if plan.Evaluation.HasErrors() || next.Title != "new" || plan.Events[0].Type != "task.updated" {
		t.Fatalf("update failed: %+v", plan)
	}
	task.Status = TaskConfirmed
	_, plan = PlanTaskUpdate(workspace, task, "new", "", "", "")
	if !plan.Evaluation.HasErrors() {
		t.Fatal("terminal Task updated")
	}
}
