package domain

import (
	"sort"
	"strings"
	"time"
)

type DatedTaskRecord struct {
	Record     TaskRecord
	ObservedAt time.Time
}

type DatedCommitReference struct {
	Commit     CommitReference
	ObservedAt time.Time
}

type BriefTask struct {
	TaskID        string
	PublicID      int
	Title         string
	Status        TaskStatus
	Actionability TaskActionability
	Reasons       []ActionabilityReason
	Blocked       bool
	BlockerReason string
	NextAction    string
}

type BriefEvidence struct {
	Kind              string
	ID                string
	TaskID            string
	RepositoryID      string
	ObservedAt        time.Time
	Summary           string
	RelativePath      string
	CommitSHA         string
	RecordType        RecordType
	RecordState       RecordState
	RunKind           RunKind
	RunStatus         RunStatus
	CommitRelation    CommitRelation
	VerificationState CommitVerificationState
}

type BriefGateParticipation struct {
	GateID           string
	Status           GateStatus
	TaskIDs          []string
	DecisionRequired string
}

type BriefDecision struct {
	Action     string
	EntityType string
	EntityID   string
}

type BriefSource struct {
	EntityType string
	EntityID   string
	ObservedAt time.Time
}

type LaneBrief struct {
	WorkspaceID       string
	LaneID            string
	Goal              string
	CurrentSummary    string
	OpenTasks         []BriefTask
	Blockers          []BriefTask
	NextActions       []BriefTask
	RecentEvidence    []BriefEvidence
	GateParticipation []BriefGateParticipation
	Decisions         []BriefDecision
	Warnings          []Diagnostic
	Stale             bool
	StalenessKnown    bool
	StaleSources      []BriefSource
	LatestObservedAt  time.Time
	Sources           []BriefSource
}

type LaneBriefInput struct {
	Workspace               Workspace
	Lane                    Lane
	Phases                  []Phase
	Tasks                   []Task
	Dependencies            []Dependency
	Runs                    []Run
	Records                 []DatedTaskRecord
	Commits                 []DatedCommitReference
	Gates                   []Gate
	GateConditions          map[string][]GateTaskCondition
	WorkspaceObservedAt     time.Time
	LaneObservedAt          time.Time
	PhaseObservedAt         map[string]time.Time
	TaskObservedAt          map[string]time.Time
	DependencyObservedAt    time.Time
	GateObservedAt          map[string]time.Time
	GateConditionObservedAt map[string]time.Time
	Now                     time.Time
	StaleAfter              time.Duration
	RecentLimit             int
}

func BuildLaneBrief(input LaneBriefInput) (LaneBrief, Evaluation) {
	brief := LaneBrief{WorkspaceID: input.Workspace.ID, LaneID: input.Lane.ID, Goal: strings.TrimSpace(input.Lane.Goal), CurrentSummary: strings.TrimSpace(input.Lane.Summary)}
	evaluation := Evaluation{}
	if input.Lane.ID == "" || input.Lane.WorkspaceID != input.Workspace.ID || input.Now.IsZero() || input.StaleAfter < 0 || input.RecentLimit < 0 {
		evaluation.Errors = []Diagnostic{{Code: CodeInvalidStateTransition, EntityID: input.Lane.ID}}
		return LaneBrief{}, evaluation
	}
	selection, child := SelectActionable(ActionableSelectionInput{
		Workspace: input.Workspace, Phases: input.Phases, Tasks: input.Tasks, Dependencies: input.Dependencies,
		Runs: input.Runs, Gates: input.Gates, GateConditions: input.GateConditions, LaneID: input.Lane.ID,
	})
	if child.HasErrors() {
		return LaneBrief{}, child
	}
	taskByID := make(map[string]Task, len(input.Tasks))
	for _, task := range input.Tasks {
		taskByID[task.ID] = task
	}
	for _, selected := range selection.Tasks {
		task := taskByID[selected.TaskID]
		if task.Status == TaskConfirmed || task.Status == TaskDiscarded {
			continue
		}
		item := BriefTask{
			TaskID: task.ID, PublicID: task.PublicID, Title: task.Title, Status: task.Status,
			Actionability: selected.Actionability, Reasons: append([]ActionabilityReason(nil), selected.Reasons...),
			Blocked: task.BlockedAt != nil, BlockerReason: task.BlockerReason, NextAction: strings.TrimSpace(task.NextAction),
		}
		brief.OpenTasks = append(brief.OpenTasks, item)
		if item.Actionability == TaskBlocked || hasActionabilityReason(item.Reasons, ReasonUnresolvedDependency) {
			brief.Blockers = append(brief.Blockers, item)
		}
		if item.NextAction != "" && (item.Actionability == TaskExecutable || item.Actionability == TaskPlanningOnly) {
			brief.NextActions = append(brief.NextActions, item)
		}
		if selected.DecisionRequired != "" {
			brief.Decisions = append(brief.Decisions, BriefDecision{Action: selected.DecisionRequired, EntityType: "task", EntityID: task.ID})
		}
	}
	for _, gateDecision := range selection.GateDecisions {
		brief.Decisions = append(brief.Decisions, BriefDecision{Action: gateDecision.DecisionRequired, EntityType: "gate", EntityID: gateDecision.GateID})
	}
	gateTaskIDs := []string{}
	for _, conditions := range input.GateConditions {
		for _, condition := range conditions {
			gateTaskIDs = append(gateTaskIDs, condition.TaskID)
		}
	}
	graph, graphEvaluation := NewWorkspaceGraph(input.Tasks, input.Dependencies, gateTaskIDs)
	if graphEvaluation.HasErrors() {
		return LaneBrief{}, graphEvaluation
	}
	for _, task := range brief.OpenTasks {
		if graph.isDangling(task.TaskID) {
			brief.Warnings = append(brief.Warnings, Diagnostic{Code: CodeDanglingPath, EntityID: task.TaskID})
		}
	}
	sortDiagnostics(brief.Warnings)

	for _, gate := range input.Gates {
		conditions := normalizedBriefConditions(gate, input.GateConditions[gate.ID], taskByID)
		taskIDs := []string{}
		for _, condition := range conditions {
			if taskByID[condition.TaskID].LaneID == input.Lane.ID {
				taskIDs = append(taskIDs, condition.TaskID)
			}
		}
		if len(taskIDs) == 0 {
			continue
		}
		sort.Strings(taskIDs)
		participation := BriefGateParticipation{GateID: gate.ID, Status: GateStatusFor(gate, conditions), TaskIDs: taskIDs}
		if input.Workspace.State == WorkspaceActive && input.Workspace.ActivePhaseID == gate.FromPhaseID && participation.Status == GateReadyStatus {
			participation.DecisionRequired = "gate.pass"
		}
		brief.GateParticipation = append(brief.GateParticipation, participation)
	}

	laneTask := func(taskID string) bool { return taskByID[taskID].LaneID == input.Lane.ID }
	evidenceIDs := map[string]bool{}
	for _, run := range input.Runs {
		if !laneTask(run.TaskID) {
			continue
		}
		if !validBriefRun(run) || run.StartedAt.After(input.Now) || evidenceIDs["run:"+run.ID] {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: run.ID})
			continue
		}
		evidenceIDs["run:"+run.ID] = true
		summary := run.ResultSummary
		if summary == "" {
			summary = run.ErrorSummary
		}
		observedAt := run.StartedAt
		if run.EndedAt != nil {
			observedAt = *run.EndedAt
		}
		if observedAt.After(input.Now) {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: run.ID})
			continue
		}
		brief.RecentEvidence = append(brief.RecentEvidence, BriefEvidence{Kind: "run", ID: run.ID, TaskID: run.TaskID, ObservedAt: observedAt, Summary: summary, RunKind: run.Kind, RunStatus: run.Status})
	}
	for _, dated := range input.Records {
		if dated.Record.WorkspaceID != input.Workspace.ID || taskByID[dated.Record.TaskID].ID == "" || !validBriefRecord(dated.Record) || dated.ObservedAt.IsZero() || dated.ObservedAt.After(input.Now) || evidenceIDs["record:"+dated.Record.ID] {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: dated.Record.ID})
			continue
		}
		if !laneTask(dated.Record.TaskID) {
			continue
		}
		evidenceIDs["record:"+dated.Record.ID] = true
		brief.RecentEvidence = append(brief.RecentEvidence, BriefEvidence{Kind: "record", ID: dated.Record.ID, TaskID: dated.Record.TaskID, RepositoryID: dated.Record.RepositoryID, ObservedAt: dated.ObservedAt, Summary: strings.TrimSpace(dated.Record.ShortSummary), RelativePath: dated.Record.RelativePath, RecordType: dated.Record.Type, RecordState: dated.Record.State})
	}
	for _, dated := range input.Commits {
		if dated.Commit.WorkspaceID != input.Workspace.ID || taskByID[dated.Commit.TaskID].ID == "" || !validBriefCommit(dated.Commit) || dated.ObservedAt.IsZero() || dated.ObservedAt.After(input.Now) || evidenceIDs["commit:"+dated.Commit.ID] {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: dated.Commit.ID})
			continue
		}
		if !laneTask(dated.Commit.TaskID) {
			continue
		}
		evidenceIDs["commit:"+dated.Commit.ID] = true
		brief.RecentEvidence = append(brief.RecentEvidence, BriefEvidence{Kind: "commit", ID: dated.Commit.ID, TaskID: dated.Commit.TaskID, RepositoryID: dated.Commit.RepositoryID, ObservedAt: dated.ObservedAt, CommitSHA: dated.Commit.CommitSHA, CommitRelation: dated.Commit.Relation, VerificationState: dated.Commit.VerificationState})
	}
	if evaluation.HasErrors() {
		evaluation.sort()
		return LaneBrief{}, evaluation
	}
	sort.Slice(brief.RecentEvidence, func(i, j int) bool {
		if !brief.RecentEvidence[i].ObservedAt.Equal(brief.RecentEvidence[j].ObservedAt) {
			return brief.RecentEvidence[i].ObservedAt.After(brief.RecentEvidence[j].ObservedAt)
		}
		if brief.RecentEvidence[i].Kind != brief.RecentEvidence[j].Kind {
			return brief.RecentEvidence[i].Kind < brief.RecentEvidence[j].Kind
		}
		return brief.RecentEvidence[i].ID < brief.RecentEvidence[j].ID
	})
	limit := input.RecentLimit
	if limit == 0 {
		limit = 10
	}
	if len(brief.RecentEvidence) > limit {
		brief.RecentEvidence = brief.RecentEvidence[:limit]
	}

	contextSources := []BriefSource{
		{EntityType: "workspace", EntityID: input.Workspace.ID, ObservedAt: input.WorkspaceObservedAt},
		{EntityType: "lane", EntityID: input.Lane.ID, ObservedAt: input.LaneObservedAt},
	}
	relevantTaskIDs := make(map[string]bool)
	relevantPhaseIDs := make(map[string]bool)
	for _, task := range input.Tasks {
		if task.LaneID == input.Lane.ID {
			relevantTaskIDs[task.ID] = true
			relevantPhaseIDs[task.PhaseID] = true
		}
	}
	for _, dependency := range input.Dependencies {
		if relevantTaskIDs[dependency.ToTaskID] {
			relevantTaskIDs[dependency.FromTaskID] = true
			if predecessor, ok := taskByID[dependency.FromTaskID]; ok {
				relevantPhaseIDs[predecessor.PhaseID] = true
			}
		}
	}
	for _, participation := range brief.GateParticipation {
		for _, gate := range input.Gates {
			if gate.ID == participation.GateID {
				relevantPhaseIDs[gate.FromPhaseID], relevantPhaseIDs[gate.ToPhaseID] = true, true
				break
			}
		}
		for _, condition := range input.GateConditions[participation.GateID] {
			relevantTaskIDs[condition.TaskID] = true
			if task, ok := taskByID[condition.TaskID]; ok {
				relevantPhaseIDs[task.PhaseID] = true
			}
		}
	}
	for phaseID := range relevantPhaseIDs {
		contextSources = append(contextSources, BriefSource{EntityType: "phase", EntityID: phaseID, ObservedAt: input.PhaseObservedAt[phaseID]})
	}
	for taskID := range relevantTaskIDs {
		contextSources = append(contextSources, BriefSource{EntityType: "task", EntityID: taskID, ObservedAt: input.TaskObservedAt[taskID]})
	}
	if len(input.Dependencies) > 0 {
		contextSources = append(contextSources, BriefSource{EntityType: "dependency_graph", EntityID: input.Workspace.ID, ObservedAt: input.DependencyObservedAt})
	}
	for _, participation := range brief.GateParticipation {
		contextSources = append(contextSources, BriefSource{EntityType: "gate", EntityID: participation.GateID, ObservedAt: input.GateObservedAt[participation.GateID]})
		for _, condition := range input.GateConditions[participation.GateID] {
			contextSources = append(contextSources, BriefSource{EntityType: "gate_condition", EntityID: condition.LinkID, ObservedAt: input.GateConditionObservedAt[condition.LinkID]})
		}
	}
	for _, source := range contextSources {
		if source.ObservedAt.After(input.Now) {
			evaluation.Errors = append(evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: source.EntityID})
		}
		addBriefSource(&brief, source.EntityType, source.EntityID, source.ObservedAt)
	}
	for _, evidence := range brief.RecentEvidence {
		addBriefSource(&brief, evidence.Kind, evidence.ID, evidence.ObservedAt)
	}
	if evaluation.HasErrors() {
		evaluation.sort()
		return LaneBrief{}, evaluation
	}
	for _, source := range brief.Sources {
		if source.ObservedAt.After(brief.LatestObservedAt) {
			brief.LatestObservedAt = source.ObservedAt
		}
	}
	brief.StalenessKnown = len(contextSources) > 0 && input.StaleAfter > 0
	for _, source := range contextSources {
		if source.ObservedAt.IsZero() {
			brief.StalenessKnown = false
			continue
		}
		if input.StaleAfter > 0 && input.Now.Sub(source.ObservedAt) > input.StaleAfter {
			brief.Stale = true
			brief.StaleSources = append(brief.StaleSources, source)
		}
	}
	sort.Slice(brief.Sources, func(i, j int) bool {
		if brief.Sources[i].EntityType != brief.Sources[j].EntityType {
			return brief.Sources[i].EntityType < brief.Sources[j].EntityType
		}
		return brief.Sources[i].EntityID < brief.Sources[j].EntityID
	})
	sort.Slice(brief.StaleSources, func(i, j int) bool {
		if brief.StaleSources[i].EntityType != brief.StaleSources[j].EntityType {
			return brief.StaleSources[i].EntityType < brief.StaleSources[j].EntityType
		}
		return brief.StaleSources[i].EntityID < brief.StaleSources[j].EntityID
	})
	sort.Slice(brief.GateParticipation, func(i, j int) bool { return brief.GateParticipation[i].GateID < brief.GateParticipation[j].GateID })
	sort.Slice(brief.Decisions, func(i, j int) bool {
		if brief.Decisions[i].Action != brief.Decisions[j].Action {
			return brief.Decisions[i].Action < brief.Decisions[j].Action
		}
		if brief.Decisions[i].EntityType != brief.Decisions[j].EntityType {
			return brief.Decisions[i].EntityType < brief.Decisions[j].EntityType
		}
		return brief.Decisions[i].EntityID < brief.Decisions[j].EntityID
	})
	return brief, evaluation
}

func normalizedBriefConditions(gate Gate, conditions []GateTaskCondition, tasks map[string]Task) []GateTaskCondition {
	result := append([]GateTaskCondition(nil), conditions...)
	for index := range result {
		if task, ok := tasks[result[index].TaskID]; ok && result[index].GateID == gate.ID {
			result[index].TaskStatus = task.Status
		}
	}
	return result
}

func hasActionabilityReason(reasons []ActionabilityReason, expected ActionabilityReason) bool {
	for _, reason := range reasons {
		if reason == expected {
			return true
		}
	}
	return false
}

func addBriefSource(brief *LaneBrief, entityType, entityID string, observedAt time.Time) {
	brief.Sources = append(brief.Sources, BriefSource{EntityType: entityType, EntityID: entityID, ObservedAt: observedAt})
}

func validBriefRecord(record TaskRecord) bool {
	if record.ID == "" || record.TaskID == "" || record.RepositoryID == "" || strings.TrimSpace(record.ShortSummary) == "" || !validRecordType(record.Type) || !validWorkingTreeHash(record.WorkingTreeHash) {
		return false
	}
	normalizedPath, err := normalizeRepositoryRelative(record.RelativePath)
	if err != nil || normalizedPath != record.RelativePath {
		return false
	}
	switch record.State {
	case RecordReportedUncommitted:
		return record.CommitSHA == "" && record.BlobSHA == ""
	case RecordCommittedUnverified, RecordVerified:
		return validGitObjectID(record.CommitSHA) && validGitObjectID(record.BlobSHA) && len(record.CommitSHA) == len(record.BlobSHA)
	default:
		return false
	}
}

func validBriefCommit(commit CommitReference) bool {
	if commit.ID == "" || commit.TaskID == "" || commit.RepositoryID == "" || !validGitObjectID(commit.CommitSHA) || !containsCommitRelation(commit.Relation) {
		return false
	}
	return commit.VerificationState == CommitReported || commit.VerificationState == CommitRemoteVerified
}

func validBriefRun(run Run) bool {
	if run.ID == "" || run.TaskID == "" || !validRunKind(run.Kind) || run.StartedAt.IsZero() || !knownRunStatus(run.Status) {
		return false
	}
	if run.Status == RunRunning {
		return run.EndedAt == nil
	}
	if run.EndedAt == nil || run.EndedAt.Before(run.StartedAt) {
		return false
	}
	if run.Status == RunSucceeded {
		return strings.TrimSpace(run.ResultSummary) != "" && run.ErrorSummary == ""
	}
	return strings.TrimSpace(run.ErrorSummary) != "" && run.ResultSummary == ""
}
