package domain

import (
	"testing"
	"time"
)

func TestBuildLaneBriefSupportsEmptyLane(t *testing.T) {
	input := laneBriefFixture()
	input.Tasks, input.Runs = nil, nil
	brief, evaluation := BuildLaneBrief(input)
	if evaluation.HasErrors() || len(brief.OpenTasks) != 0 || len(brief.RecentEvidence) != 0 || brief.StalenessKnown {
		t.Fatalf("empty Lane brief wrong: %+v %+v", brief, evaluation)
	}
}

func TestBuildLaneBriefProjectsDisconnectedTasksBlockersAndNextActions(t *testing.T) {
	input := laneBriefFixture()
	blockedAt := input.Now.Add(-time.Hour)
	input.Tasks = append(input.Tasks,
		Task{ID: "blocked", PublicID: 2, WorkspaceID: "workspace", LaneID: "lane", PhaseID: "phase", Title: "Blocked", Status: TaskInProgress, BlockedAt: &blockedAt, BlockerReason: "waiting", NextAction: "retry"},
		Task{ID: "disconnected", PublicID: 3, WorkspaceID: "workspace", LaneID: "lane", PhaseID: "phase", Title: "Independent", Status: TaskPending, NextAction: "start"},
	)
	brief, evaluation := BuildLaneBrief(input)
	if evaluation.HasErrors() || len(brief.OpenTasks) != 3 || len(brief.Blockers) != 1 || len(brief.NextActions) != 2 {
		t.Fatalf("Task brief projection wrong: %+v %+v", brief, evaluation)
	}
}

func TestBuildLaneBriefPreservesMultiRepositoryAndUnverifiedCommitEvidence(t *testing.T) {
	input := laneBriefFixture()
	input.Records = []DatedTaskRecord{
		{Record: TaskRecord{ID: "record-a", WorkspaceID: "workspace", TaskID: "task", RepositoryID: "repo-a", RelativePath: "task-records/task/plan.md", Type: RecordDetailedPlan, State: RecordVerified, CommitSHA: repeatHex("a", 40), BlobSHA: repeatHex("b", 40), ShortSummary: "plan"}, ObservedAt: input.Now.Add(-2 * time.Hour)},
		{Record: TaskRecord{ID: "record-b", WorkspaceID: "workspace", TaskID: "task", RepositoryID: "repo-b", RelativePath: "task-records/task/report.md", Type: RecordCompletionReport, State: RecordReportedUncommitted, ShortSummary: "report"}, ObservedAt: input.Now.Add(-time.Hour)},
	}
	input.Commits = []DatedCommitReference{{Commit: CommitReference{ID: "commit", WorkspaceID: "workspace", TaskID: "task", RepositoryID: "repo-b", CommitSHA: repeatHex("c", 40), Relation: CommitProduced, VerificationState: CommitReported}, ObservedAt: input.Now.Add(-30 * time.Minute)}}
	brief, evaluation := BuildLaneBrief(input)
	if evaluation.HasErrors() || len(brief.RecentEvidence) != 3 || brief.RecentEvidence[0].RepositoryID != "repo-b" || brief.RecentEvidence[0].VerificationState != CommitReported {
		t.Fatalf("multi-repository evidence lost: %+v %+v", brief.RecentEvidence, evaluation)
	}
}

func TestBuildLaneBriefProjectsGateAndHumanDecisions(t *testing.T) {
	input := laneBriefFixture()
	input.Tasks[0].Status = TaskImplemented
	input.Phases = append(input.Phases, Phase{ID: "future", WorkspaceID: "workspace", Position: 2, State: PhasePlanned})
	input.Gates = []Gate{{ID: "gate", WorkspaceID: "workspace", FromPhaseID: "phase", ToPhaseID: "future"}}
	input.GateConditions = map[string][]GateTaskCondition{"gate": {{WorkspaceID: "workspace", GateID: "gate", LinkID: "link", TaskID: "task", Passed: true, PassReason: "waived"}}}
	brief, evaluation := BuildLaneBrief(input)
	if evaluation.HasErrors() || len(brief.GateParticipation) != 1 || brief.GateParticipation[0].DecisionRequired != "gate.pass" || len(brief.Decisions) != 2 {
		t.Fatalf("Gate/Task decisions missing: %+v %+v", brief, evaluation)
	}
}

func TestBuildLaneBriefStalenessUsesLatestKnownSource(t *testing.T) {
	input := laneBriefFixture()
	input.LaneObservedAt = input.Now.Add(-72 * time.Hour)
	input.TaskObservedAt = map[string]time.Time{"task": input.Now.Add(-48 * time.Hour)}
	input.StaleAfter = 24 * time.Hour
	brief, evaluation := BuildLaneBrief(input)
	if evaluation.HasErrors() || !brief.StalenessKnown || !brief.Stale {
		t.Fatalf("old brief not stale: %+v %+v", brief, evaluation)
	}
	startedAt, endedAt := input.Now.Add(-time.Hour), input.Now.Add(-30*time.Minute)
	input.Runs = []Run{{ID: "recent", WorkspaceID: "workspace", TaskID: "task", Kind: RunImplementation, Status: RunSucceeded, StartedAt: startedAt, EndedAt: &endedAt, ResultSummary: "done"}}
	brief, evaluation = BuildLaneBrief(input)
	if evaluation.HasErrors() || !brief.Stale || brief.LatestObservedAt != input.Now {
		t.Fatalf("recent evidence masked stale context: %+v %+v", brief, evaluation)
	}
	input.LaneObservedAt = input.Now.Add(-time.Hour)
	input.TaskObservedAt["task"] = input.Now.Add(-time.Hour)
	brief, evaluation = BuildLaneBrief(input)
	if evaluation.HasErrors() || brief.Stale || !brief.StalenessKnown {
		t.Fatalf("fresh context reported stale: %+v %+v", brief, evaluation)
	}
}

func TestBuildLaneBriefStalenessIncludesPredecessorGateWorkspaceAndPhase(t *testing.T) {
	input := laneBriefFixture()
	input.Tasks = append(input.Tasks, Task{ID: "predecessor", PublicID: 2, WorkspaceID: "workspace", LaneID: "other", PhaseID: "phase", Status: TaskConfirmed})
	input.Dependencies = []Dependency{{FromTaskID: "predecessor", ToTaskID: "task"}}
	input.TaskObservedAt["predecessor"] = input.Now.Add(-48 * time.Hour)
	input.DependencyObservedAt = input.Now
	input.StaleAfter = 24 * time.Hour
	brief, evaluation := BuildLaneBrief(input)
	if evaluation.HasErrors() || !brief.StalenessKnown || !brief.Stale || !hasBriefSource(brief.StaleSources, "task", "predecessor") {
		t.Fatalf("terminal predecessor omitted from staleness: %+v %+v", brief, evaluation)
	}
	input.TaskObservedAt["predecessor"] = input.Now
	input.WorkspaceObservedAt = input.Now.Add(-48 * time.Hour)
	brief, evaluation = BuildLaneBrief(input)
	if evaluation.HasErrors() || !hasBriefSource(brief.StaleSources, "workspace", "workspace") {
		t.Fatalf("Workspace omitted from staleness: %+v %+v", brief, evaluation)
	}
	input.WorkspaceObservedAt = input.Now
	input.PhaseObservedAt["phase"] = input.Now.Add(-48 * time.Hour)
	brief, evaluation = BuildLaneBrief(input)
	if evaluation.HasErrors() || !hasBriefSource(brief.StaleSources, "phase", "phase") {
		t.Fatalf("Phase omitted from staleness: %+v %+v", brief, evaluation)
	}
}

func TestBuildLaneBriefStalenessIncludesEveryParticipatingGateTask(t *testing.T) {
	input := laneBriefFixture()
	input.Tasks[0].Status = TaskImplemented
	input.Tasks = append(input.Tasks, Task{ID: "gate-task", PublicID: 2, WorkspaceID: "workspace", LaneID: "other", PhaseID: "phase", Status: TaskConfirmed})
	input.TaskObservedAt["gate-task"] = input.Now.Add(-48 * time.Hour)
	input.Phases = append(input.Phases, Phase{ID: "future", WorkspaceID: "workspace", Position: 2, State: PhasePlanned})
	input.PhaseObservedAt["future"] = input.Now
	input.Gates = []Gate{{ID: "gate", WorkspaceID: "workspace", FromPhaseID: "phase", ToPhaseID: "future"}}
	input.GateObservedAt = map[string]time.Time{"gate": input.Now}
	input.GateConditions = map[string][]GateTaskCondition{"gate": {
		{WorkspaceID: "workspace", GateID: "gate", LinkID: "lane-link", TaskID: "task", Passed: true, PassReason: "waived"},
		{WorkspaceID: "workspace", GateID: "gate", LinkID: "other-link", TaskID: "gate-task"},
	}}
	input.GateConditionObservedAt = map[string]time.Time{"lane-link": input.Now, "other-link": input.Now}
	input.StaleAfter = 24 * time.Hour
	brief, evaluation := BuildLaneBrief(input)
	if evaluation.HasErrors() || !brief.StalenessKnown || !brief.Stale || !hasBriefSource(brief.StaleSources, "task", "gate-task") {
		t.Fatalf("participating Gate Task omitted from staleness: %+v %+v", brief, evaluation)
	}
}

func TestBuildLaneBriefRejectsForeignEvidence(t *testing.T) {
	input := laneBriefFixture()
	input.Records = []DatedTaskRecord{{Record: TaskRecord{ID: "foreign", WorkspaceID: "other", TaskID: "task"}, ObservedAt: input.Now}}
	if _, evaluation := BuildLaneBrief(input); !evaluation.HasErrors() {
		t.Fatal("foreign Record evidence silently accepted")
	}
	input.Records = nil
	input.Commits = []DatedCommitReference{{Commit: CommitReference{ID: "missing-task", WorkspaceID: "workspace", TaskID: "missing"}, ObservedAt: input.Now}}
	if _, evaluation := BuildLaneBrief(input); !evaluation.HasErrors() {
		t.Fatal("commit for unknown Task silently accepted")
	}
}

func TestBuildLaneBriefReportsDanglingAndAllBlockedOpenTasks(t *testing.T) {
	input := laneBriefFixture()
	brief, evaluation := BuildLaneBrief(input)
	if evaluation.HasErrors() || !hasDiagnostic(brief.Warnings, CodeDanglingPath) {
		t.Fatalf("dangling Lane Task warning missing: %+v %+v", brief, evaluation)
	}
	input.Workspace.State, input.Workspace.ActivePhaseID = WorkspaceClosed, ""
	input.Phases[0].State = PhaseCompleted
	brief, evaluation = BuildLaneBrief(input)
	if evaluation.HasErrors() || len(brief.Blockers) != 1 || brief.Blockers[0].Reasons[0] != ReasonWorkspaceClosed {
		t.Fatalf("closed Workspace Task missing from blockers: %+v %+v", brief, evaluation)
	}
}

func TestBuildLaneBriefRejectsImpossibleEvidenceAndFutureSources(t *testing.T) {
	input := laneBriefFixture()
	input.Records = []DatedTaskRecord{{Record: TaskRecord{ID: "false-verified", WorkspaceID: "workspace", TaskID: "task", RepositoryID: "repo", RelativePath: "task-records/report.md", Type: RecordCompletionReport, State: RecordVerified, ShortSummary: "done"}, ObservedAt: input.Now}}
	if _, evaluation := BuildLaneBrief(input); !evaluation.HasErrors() {
		t.Fatal("verified Record without commit/blob accepted")
	}
	input.Records = nil
	input.Commits = []DatedCommitReference{{Commit: CommitReference{ID: "false-remote", WorkspaceID: "workspace", TaskID: "task", RepositoryID: "repo", Relation: CommitProduced, VerificationState: CommitRemoteVerified}, ObservedAt: input.Now}}
	if _, evaluation := BuildLaneBrief(input); !evaluation.HasErrors() {
		t.Fatal("remote verified commit without SHA accepted")
	}
	input.Commits = nil
	input.LaneObservedAt = input.Now.Add(time.Minute)
	if _, evaluation := BuildLaneBrief(input); !evaluation.HasErrors() {
		t.Fatal("future context timestamp accepted")
	}
}

func laneBriefFixture() LaneBriefInput {
	now := time.Date(2026, time.July, 18, 12, 0, 0, 0, time.UTC)
	return LaneBriefInput{
		Workspace:           Workspace{ID: "workspace", State: WorkspaceActive, ActivePhaseID: "phase"},
		Lane:                Lane{ID: "lane", WorkspaceID: "workspace", Name: "Server", Goal: "Ship", Summary: "Working", State: LaneActive},
		Phases:              []Phase{{ID: "phase", WorkspaceID: "workspace", Position: 1, State: PhaseActive}},
		Tasks:               []Task{{ID: "task", PublicID: 1, WorkspaceID: "workspace", LaneID: "lane", PhaseID: "phase", Title: "Build", Status: TaskInProgress, NextAction: "implement"}},
		WorkspaceObservedAt: now,
		LaneObservedAt:      now,
		PhaseObservedAt:     map[string]time.Time{"phase": now},
		TaskObservedAt:      map[string]time.Time{"task": now},
		Now:                 now,
	}
}

func hasBriefSource(sources []BriefSource, entityType, entityID string) bool {
	for _, source := range sources {
		if source.EntityType == entityType && source.EntityID == entityID {
			return true
		}
	}
	return false
}
