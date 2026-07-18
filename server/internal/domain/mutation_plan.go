package domain

import (
	"strings"
	"time"
)

type HumanApprovalMode string

const (
	ApprovalNone                HumanApprovalMode = "none"
	ApprovalAlways              HumanApprovalMode = "always"
	ApprovalAlwaysOwner         HumanApprovalMode = "always_owner"
	ApprovalWhenFromPhaseActive HumanApprovalMode = "when_from_phase_active"
)

type MutationPolicy struct {
	Name, Capability, ActiveCapability string
	HumanApproval                      HumanApprovalMode
	EventType                          string
	OperationalNoEvent                 bool
}

var MutationPolicies = []MutationPolicy{
	{Name: "project.bootstrap", Capability: "workspace:operate", EventType: "project.bootstrapped"},
	{Name: "repository.register", Capability: "workspace:operate", EventType: "repository.registered"},
	{Name: "workspace.create", Capability: "workspace:operate", EventType: "workspace.created"},
	{Name: "workspace.activate", Capability: "workspace:operate", EventType: "workspace.activated"},
	{Name: "workspace.close", Capability: "workspace:close", HumanApproval: ApprovalAlwaysOwner, EventType: "workspace.closed"},
	{Name: "phase.create", Capability: "workspace:operate", EventType: "phase.created"},
	{Name: "lane.create", Capability: "workspace:operate", EventType: "lane.created"}, {Name: "lane.update", Capability: "workspace:operate", EventType: "lane.updated"},
	{Name: "lane.close_out", Capability: "lane:approve", HumanApproval: ApprovalAlways, EventType: "lane.closed_out"}, {Name: "lane.discard", Capability: "lane:approve", HumanApproval: ApprovalAlways, EventType: "lane.discarded"},
	{Name: "gate.create", Capability: "workspace:operate", EventType: "gate.created"},
	{Name: "task.create", Capability: "workspace:operate", EventType: "task.created"}, {Name: "task.update", Capability: "workspace:operate", EventType: "task.updated"},
	{Name: "task.set_terminal", Capability: "workspace:operate", EventType: "task.terminal_set"}, {Name: "task.clear_terminal", Capability: "workspace:operate", EventType: "task.terminal_cleared"},
	{Name: "task.block", Capability: "workspace:operate", EventType: "task.blocked"}, {Name: "task.unblock", Capability: "workspace:operate", EventType: "task.unblocked"},
	{Name: "task.report_implemented", Capability: "workspace:operate", EventType: "task.implemented_reported"}, {Name: "task.confirm", Capability: "task:approve", HumanApproval: ApprovalAlways, EventType: "task.confirmed"},
	{Name: "task.discard", Capability: "task:approve", HumanApproval: ApprovalAlways, EventType: "task.discarded"}, {Name: "task.rework", Capability: "workspace:operate", EventType: "task.rework_started"},
	{Name: "dependency.connect", Capability: "workspace:operate", EventType: "dependency.connected"}, {Name: "dependency.disconnect", Capability: "workspace:operate", EventType: "dependency.disconnected"}, {Name: "dependency.patch", Capability: "workspace:operate", EventType: "dependency.patched"},
	{Name: "gate.attach_task", Capability: "workspace:operate", ActiveCapability: "gate:approve", HumanApproval: ApprovalWhenFromPhaseActive, EventType: "gate.task_attached"},
	{Name: "gate.detach_task", Capability: "workspace:operate", EventType: "gate.task_detached"}, {Name: "gate.pass_task", Capability: "gate:approve", HumanApproval: ApprovalAlways, EventType: "gate.task_passed"}, {Name: "gate.revoke_task_pass", Capability: "gate:approve", HumanApproval: ApprovalAlways, EventType: "gate.task_pass_revoked"}, {Name: "gate.pass", Capability: "gate:approve", HumanApproval: ApprovalAlways, EventType: "gate.passed"},
	{Name: "run.start", Capability: "run:operate", EventType: "run.started"}, {Name: "run.heartbeat", Capability: "run:operate", OperationalNoEvent: true}, {Name: "run.succeed", Capability: "run:operate", EventType: "run.succeeded"}, {Name: "run.fail", Capability: "run:operate", EventType: "run.failed"}, {Name: "run.cancel", Capability: "run:operate", EventType: "run.cancelled"}, {Name: "run.interrupt", Capability: "run:operate", EventType: "run.interrupted"}, {Name: "run.correct", Capability: "run:operate", EventType: "run.corrected"},
	{Name: "record.register", Capability: "record:operate", EventType: "record.registered"}, {Name: "record.attach_commit", Capability: "record:operate", EventType: "record.commit_attached"}, {Name: "commit.attach", Capability: "record:operate", EventType: "commit.attached"}, {Name: "git.observe", Capability: "record:operate", EventType: "git.observed"},
}

type DomainMutationPlan struct {
	Command, RequiredCapability string
	HumanApproval               HumanApprovalMode
	CommandHash                 string
	DecisionSnapshotHash        string
	ProjectedDiff               any
	Events                      []PlannedEvent
	Evaluation                  Evaluation
}

type Workspace struct {
	ID, Name, ActivePhaseID string
	State                   WorkspaceState
	Revision                int64
}

func PlanWorkspaceActivate(workspace Workspace, phases []Phase) (Workspace, []Phase, DomainMutationPlan) {
	plan := newDomainPlan("workspace.activate", false)
	if workspace.State != WorkspaceDraft || len(phases) == 0 {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: workspace.ID}}
		return workspace, phases, plan
	}
	nextPhases := append([]Phase(nil), phases...)
	firstIndex := -1
	positions := map[int]bool{}
	for index, phase := range nextPhases {
		if phase.WorkspaceID != workspace.ID || phase.State != PhasePlanned || phase.Position <= 0 || positions[phase.Position] {
			plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: phase.ID}}
			return workspace, phases, plan
		}
		positions[phase.Position] = true
		if firstIndex < 0 || phase.Position < nextPhases[firstIndex].Position {
			firstIndex = index
		}
	}
	if nextPhases[firstIndex].State != PhasePlanned {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: nextPhases[firstIndex].ID}}
		return workspace, phases, plan
	}
	next := workspace
	next.State = WorkspaceActive
	next.ActivePhaseID = nextPhases[firstIndex].ID
	nextPhases[firstIndex].State = PhaseActive
	plan.ProjectedDiff = map[string]any{"workspaceId": next.ID, "workspaceState": next.State, "activePhaseId": next.ActivePhaseID}
	plan.Events = []PlannedEvent{{Type: "workspace.activated", EntityType: "workspace", EntityID: workspace.ID, Payload: plan.ProjectedDiff.(map[string]any)}, {Type: "phase.activated", EntityType: "phase", EntityID: next.ActivePhaseID, Payload: map[string]any{"phaseId": next.ActivePhaseID}}}
	return next, nextPhases, plan
}

func PlanPhaseCreate(workspace Workspace, existing []Phase, phase Phase) DomainMutationPlan {
	plan := newDomainPlan("phase.create", false)
	max := 0
	for _, item := range existing {
		if item.WorkspaceID != workspace.ID || item.ID == phase.ID {
			plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: item.ID}}
			return plan
		}
		if item.Position > max {
			max = item.Position
		}
	}
	if workspace.State == WorkspaceClosed || phase.WorkspaceID != workspace.ID || phase.ID == "" || phase.Position != max+1 || phase.State != PhasePlanned {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: phase.ID}}
		return plan
	}
	plan.ProjectedDiff = phase
	plan.Events = []PlannedEvent{{Type: "phase.created", EntityType: "phase", EntityID: phase.ID, Payload: map[string]any{"phaseId": phase.ID, "position": phase.Position}}}
	return plan
}

func PlanTaskCreate(workspace Workspace, lane Lane, phase Phase, graph *WorkspaceGraph, task Task, initial DependencyPatch) DomainMutationPlan {
	plan := newDomainPlan("task.create", false)
	if workspace.State != WorkspaceActive || graph == nil || lane.WorkspaceID != workspace.ID || phase.WorkspaceID != workspace.ID || lane.State != LaneActive || phase.State == PhaseCompleted || task.ID == "" || task.PublicID <= 0 || strings.TrimSpace(task.Title) == "" || task.WorkspaceID != workspace.ID || task.LaneID != lane.ID || task.PhaseID != phase.ID || task.Status != TaskPending {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: task.ID}}
		return plan
	}
	if task.ParentTaskID != "" {
		parent, ok := graph.Tasks[task.ParentTaskID]
		if !ok || parent.WorkspaceID != task.WorkspaceID || parent.LaneID != task.LaneID || parent.PhaseID != task.PhaseID {
			plan.Evaluation.Errors = []Diagnostic{{Code: CodeNotFound, EntityID: task.ParentTaskID}}
			return plan
		}
	}
	candidate := graph.clone()
	if _, exists := candidate.Tasks[task.ID]; exists {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: task.ID}}
		return plan
	}
	candidate.Tasks[task.ID] = task
	preview := candidate.PreviewPatch(initial)
	plan.Evaluation = preview.Evaluation
	plan.ProjectedDiff = map[string]any{"task": task, "relations": preview.Diff}
	if !plan.Evaluation.HasErrors() {
		plan.Events = []PlannedEvent{{Type: "task.created", EntityType: "task", EntityID: task.ID, Payload: plan.ProjectedDiff.(map[string]any)}}
	}
	return plan
}

func PlanTaskUpdate(workspace Workspace, task Task, title, description, summary, nextAction string) (Task, DomainMutationPlan) {
	plan := newDomainPlan("task.update", false)
	if workspace.State == WorkspaceClosed || task.WorkspaceID != workspace.ID {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: task.WorkspaceID}}
		return task, plan
	}
	next, err := task.Update(title, description, summary, nextAction)
	if err != nil {
		plan.Evaluation.Errors = []Diagnostic{{Code: violationCode(err), EntityID: task.ID}}
		return task, plan
	}
	plan.ProjectedDiff = map[string]any{"before": task, "after": next}
	plan.Events = []PlannedEvent{{Type: "task.updated", EntityType: "task", EntityID: task.ID, Payload: map[string]any{"taskId": task.ID}}}
	return next, plan
}

func PlanGateCreate(workspace Workspace, gate Gate, from, to Phase, existing []Gate) DomainMutationPlan {
	plan := newDomainPlan("gate.create", false)
	if workspace.State == WorkspaceClosed || gate.ID == "" || gate.WorkspaceID != workspace.ID || gate.CriteriaRevision != 0 || gate.PassedAt != nil || from.WorkspaceID != workspace.ID || to.WorkspaceID != workspace.ID || gate.FromPhaseID != from.ID || gate.ToPhaseID != to.ID || to.Position != from.Position+1 {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: gate.ID}}
		return plan
	}
	for _, item := range existing {
		if item.FromPhaseID == from.ID || item.ToPhaseID == to.ID {
			plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: gate.ID}}
			return plan
		}
	}
	plan.ProjectedDiff = gate
	plan.Events = []PlannedEvent{{Type: "gate.created", EntityType: "gate", EntityID: gate.ID, Payload: map[string]any{"gateId": gate.ID, "fromPhaseId": from.ID, "toPhaseId": to.ID}}}
	return plan
}

func PlanLaneCreate(workspace Workspace, lane Lane) DomainMutationPlan {
	plan := newDomainPlan("lane.create", false)
	if workspace.State == WorkspaceClosed || lane.ID == "" || lane.WorkspaceID != workspace.ID || strings.TrimSpace(lane.Name) == "" || lane.State != LaneActive {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: lane.ID}}
		return plan
	}
	plan.ProjectedDiff = lane
	plan.Events = []PlannedEvent{{Type: "lane.created", EntityType: "lane", EntityID: lane.ID, Payload: map[string]any{"laneId": lane.ID}}}
	return plan
}
func PlanLaneUpdate(workspace Workspace, lane Lane, name, goal, summary string) (Lane, DomainMutationPlan) {
	plan := newDomainPlan("lane.update", false)
	if workspace.State == WorkspaceClosed || lane.WorkspaceID != workspace.ID {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: lane.WorkspaceID}}
		return lane, plan
	}
	if lane.State != LaneActive || strings.TrimSpace(name) == "" {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: lane.ID}}
		return lane, plan
	}
	next := lane
	next.Name = strings.TrimSpace(name)
	next.Goal = strings.TrimSpace(goal)
	next.Summary = strings.TrimSpace(summary)
	plan.ProjectedDiff = map[string]any{"before": lane, "after": next}
	plan.Events = []PlannedEvent{{Type: "lane.updated", EntityType: "lane", EntityID: lane.ID, Payload: map[string]any{"laneId": lane.ID}}}
	return next, plan
}

func policyFor(name string, activeGate bool) (MutationPolicy, bool) {
	for _, policy := range MutationPolicies {
		if policy.Name == name {
			if activeGate && policy.ActiveCapability != "" {
				policy.Capability = policy.ActiveCapability
			}
			return policy, true
		}
	}
	return MutationPolicy{}, false
}
func newDomainPlan(name string, activeGate bool) DomainMutationPlan {
	policy, ok := policyFor(name, activeGate)
	if !ok {
		return DomainMutationPlan{Command: name, Evaluation: Evaluation{Errors: []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: name}}}}
	}
	approval := policy.HumanApproval
	if approval == "" || approval == ApprovalWhenFromPhaseActive && !activeGate {
		approval = ApprovalNone
	}
	return DomainMutationPlan{Command: name, RequiredCapability: policy.Capability, HumanApproval: approval}
}

func PlanTaskMutation(workspace Workspace, command string, task Task, reason string, now time.Time) (Task, DomainMutationPlan) {
	plan := newDomainPlan(command, false)
	if workspace.State == WorkspaceClosed || task.WorkspaceID != workspace.ID {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: task.WorkspaceID}}
		return task, plan
	}
	next := task
	var err error
	switch command {
	case "task.block":
		next, err = task.Block(now, reason)
	case "task.unblock":
		next, err = task.Unblock(reason)
	case "task.rework":
		next, err = task.Rework(reason)
	case "task.discard":
		next, err = task.Discard(reason)
	default:
		err = &Violation{Code: CodeInvalidStateTransition}
	}
	if err != nil {
		plan.Evaluation.Errors = []Diagnostic{{Code: violationCode(err), EntityID: task.ID}}
		return task, plan
	}
	plan.ProjectedDiff = map[string]any{"taskId": task.ID, "before": task, "after": next}
	policy, _ := policyFor(command, false)
	plan.Events = []PlannedEvent{{Type: policy.EventType, EntityType: "task", EntityID: task.ID, Payload: map[string]any{"taskId": task.ID, "reason": strings.TrimSpace(reason)}}}
	return next, plan
}

func PlanDependencyMutation(workspace Workspace, command string, graph *WorkspaceGraph, patch DependencyPatch, runningTaskIDs map[string]bool) DomainMutationPlan {
	plan := newDomainPlan(command, false)
	if workspace.State == WorkspaceClosed || graph == nil {
		return invalidPlan(plan, "workspace_graph", CodeInvalidStateTransition)
	}
	if plan.Evaluation.HasErrors() {
		return plan
	}
	for _, task := range graph.Tasks {
		if task.WorkspaceID != workspace.ID {
			return invalidPlan(plan, task.ID, CodeInvalidStateTransition)
		}
	}
	if command == "dependency.connect" && (len(patch.Add) != 1 || len(patch.Remove) != 0 || len(patch.TerminalUpdates) != 0) || command == "dependency.disconnect" && (len(patch.Remove) != 1 || len(patch.Add) != 0 || len(patch.TerminalUpdates) != 0) || command == "dependency.patch" && len(patch.Add)+len(patch.Remove)+len(patch.TerminalUpdates) == 0 {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidDependencyPatch, EntityID: command}}
		return plan
	}
	preview := graph.PreviewPatch(patch)
	plan.ProjectedDiff = preview.Diff
	plan.Evaluation = preview.Evaluation
	if !preview.Evaluation.HasErrors() {
		for _, edge := range patch.Add {
			if runningTaskIDs[edge.ToTaskID] {
				plan.Evaluation.Warnings = append(plan.Evaluation.Warnings, Diagnostic{Code: CodeRunningTaskDependencyAdded, EntityID: edgeID(edge)})
			}
		}
		plan.Evaluation.sort()
		policy, _ := policyFor(command, false)
		plan.Events = []PlannedEvent{{Type: policy.EventType, EntityType: "workspace_graph", EntityID: "workspace_graph", Payload: map[string]any{"diff": preview.Diff}}}
	}
	return plan
}

type LaneState string

const (
	LaneActive    LaneState = "active"
	LaneClosedOut LaneState = "closed_out"
	LaneDiscarded LaneState = "discarded"
)

type Lane struct {
	ID, WorkspaceID, Name, Goal, Summary string
	State                                LaneState
}

func PlanLaneTermination(workspace Workspace, command string, lane Lane, reason string) (Lane, DomainMutationPlan) {
	plan := newDomainPlan(command, false)
	if workspace.State == WorkspaceClosed || lane.WorkspaceID != workspace.ID {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: lane.WorkspaceID}}
		return lane, plan
	}
	if lane.State != LaneActive || strings.TrimSpace(reason) == "" {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: lane.ID}}
		return lane, plan
	}
	next := lane
	if command == "lane.close_out" {
		next.State = LaneClosedOut
	} else if command == "lane.discard" {
		next.State = LaneDiscarded
	} else {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: lane.ID}}
		return lane, plan
	}
	plan.ProjectedDiff = map[string]any{"laneId": lane.ID, "before": lane.State, "after": next.State}
	policy, _ := policyFor(command, false)
	plan.Events = []PlannedEvent{{Type: policy.EventType, EntityType: "lane", EntityID: lane.ID, Payload: map[string]any{"laneId": lane.ID, "reason": strings.TrimSpace(reason)}}}
	return next, plan
}

func PlanGateTaskAttachment(workspace Workspace, gate Gate, fromPhase Phase, task Task, existing []GateTaskCondition, attach, clearTerminal bool) DomainMutationPlan {
	command := "gate.attach_task"
	if !attach {
		command = "gate.detach_task"
	}
	active := workspace.State == WorkspaceActive && workspace.ActivePhaseID == fromPhase.ID && fromPhase.State == PhaseActive
	plan := newDomainPlan(command, active)
	if workspace.State == WorkspaceClosed || gate.WorkspaceID != workspace.ID || fromPhase.WorkspaceID != workspace.ID || task.WorkspaceID != workspace.ID || gate.PassedAt != nil || gate.CriteriaRevision < 0 || gate.FromPhaseID != fromPhase.ID || task.PhaseID != fromPhase.ID {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeGateTaskWrongPhase, EntityID: task.ID}}
		return plan
	}
	found := false
	for _, condition := range existing {
		if condition.TaskID == task.ID {
			found = true
		}
	}
	if attach && found || !attach && !found {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: task.ID}}
		return plan
	}
	if !attach && active {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeActiveGateDetachForbidden, EntityID: gate.ID}}
		return plan
	}
	if attach && task.TerminalReason != "" && !clearTerminal {
		plan.Evaluation.Errors = []Diagnostic{{Code: CodeTerminalPathConflict, EntityID: task.ID}}
		return plan
	}
	policy, _ := policyFor(command, active)
	plan.ProjectedDiff = map[string]any{"gateId": gate.ID, "taskId": task.ID, "attached": attach, "clearTerminalReason": clearTerminal, "criteriaRevisionBefore": gate.CriteriaRevision, "criteriaRevisionAfter": gate.CriteriaRevision + 1}
	plan.Events = []PlannedEvent{{Type: policy.EventType, EntityType: "gate", EntityID: gate.ID, Payload: plan.ProjectedDiff.(map[string]any)}}
	return plan
}
