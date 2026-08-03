package application

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jazzcake/baley/server/internal/domain"
)

// LaneBrief is a read-only recovery query. It deliberately builds candidates
// from the current PostgreSQL snapshot and never mutates inferred state.
func (s *Service) LaneBrief(ctx context.Context, workspaceID, laneID string) (domain.LaneBrief, error) {
	snapshot, err := s.repo.LoadSnapshot(ctx, workspaceID)
	if err != nil {
		return domain.LaneBrief{}, err
	}
	lane := findLane(snapshot.Lanes, laneID)
	if lane == nil {
		return domain.LaneBrief{}, &CommandError{Code: domain.CodeNotFound, Message: "lane not found"}
	}
	now := laneBriefEvaluationTime(s.now().UTC(), snapshot.Workspace.ObservedAt)
	phases := make([]domain.Phase, 0, len(snapshot.Phases))
	phaseObservedAt := map[string]time.Time{}
	for _, value := range snapshot.Phases {
		phases = append(phases, domainPhase(value, workspaceID))
		phaseObservedAt[value.ID] = value.ObservedAt
	}
	tasks := make([]domain.Task, 0, len(snapshot.Tasks))
	taskObservedAt := map[string]time.Time{}
	for _, value := range snapshot.Tasks {
		task := domainTask(value, workspaceID)
		if phase := findPhase(snapshot.Phases, value.PhaseID); phase != nil {
			task.PhasePosition = phase.Position
		}
		tasks = append(tasks, task)
		taskObservedAt[value.ID] = value.ObservedAt
	}
	dependencies := make([]domain.Dependency, 0, len(snapshot.Dependencies))
	dependencyObservedAt := time.Time{}
	for _, value := range snapshot.Dependencies {
		dependencies = append(dependencies, domain.Dependency{FromTaskID: value.FromTaskID, ToTaskID: value.ToTaskID})
		if value.ObservedAt.After(dependencyObservedAt) {
			dependencyObservedAt = value.ObservedAt
		}
	}
	runs := make([]domain.Run, 0, len(snapshot.Runs))
	for _, value := range snapshot.Runs {
		runs = append(runs, domainRun(value, workspaceID))
	}
	records := make([]domain.DatedTaskRecord, 0, len(snapshot.Records))
	gitRoot, gitRootErr := localGitRoot()
	for _, value := range snapshot.Records {
		status, reason := verifyLocalTaskRecord(gitRoot, gitRootErr, snapshot.Repositories, value)
		records = append(records, domain.DatedTaskRecord{
			Record: domainRecord(value, workspaceID), ObservedAt: value.ObservedAt,
			RepositoryStatus: status, MismatchReason: reason,
		})
	}
	commits := make([]domain.DatedCommitReference, 0, len(snapshot.Commits))
	for _, value := range snapshot.Commits {
		commits = append(commits, domain.DatedCommitReference{Commit: domain.CommitReference{
			ID: value.ID, WorkspaceID: workspaceID, TaskID: value.TaskID, RunID: value.RunID,
			RepositoryID: value.RepositoryID, CommitSHA: value.CommitSHA,
			Relation: domain.CommitRelation(value.Relation), VerificationState: domain.CommitVerificationState(value.VerificationState),
		}, ObservedAt: value.ObservedAt})
	}
	observations := make([]domain.RunGitObservation, 0, len(snapshot.GitObservations))
	for _, value := range snapshot.GitObservations {
		observations = append(observations, domain.RunGitObservation{
			ID: value.ID, WorkspaceID: workspaceID, RunID: value.RunID, RepositoryID: value.RepositoryID,
			ObservedAt: value.ObservedAt, HeadCommitSHA: value.HeadCommitSHA, BranchHint: value.BranchHint,
			WorktreeLabel: value.WorktreeLabel, Dirty: value.Dirty,
		})
	}
	gates := make([]domain.Gate, 0, len(snapshot.Gates))
	conditions := map[string][]domain.GateTaskCondition{}
	gateObservedAt := map[string]time.Time{}
	conditionObservedAt := map[string]time.Time{}
	for _, value := range snapshot.Gates {
		gates = append(gates, domainGate(value, workspaceID))
		gateObservedAt[value.ID] = value.ObservedAt
		for _, condition := range value.Conditions {
			task := findTask(snapshot.Tasks, condition.TaskID)
			status := domain.TaskPending
			if task != nil {
				status = domain.TaskStatus(task.Status)
			}
			passReason := ""
			if condition.PassReason != nil {
				passReason = *condition.PassReason
			}
			conditions[value.ID] = append(conditions[value.ID], domain.GateTaskCondition{
				WorkspaceID: workspaceID, GateID: value.ID, LinkID: condition.ID,
				TaskID: condition.TaskID, TaskStatus: status, Passed: condition.PassedAt != nil, PassReason: passReason,
			})
			conditionObservedAt[condition.ID] = condition.ObservedAt
		}
	}
	activePhaseID := ""
	if snapshot.Workspace.ActivePhaseID != nil {
		activePhaseID = *snapshot.Workspace.ActivePhaseID
	}
	brief, evaluation := domain.BuildLaneBrief(domain.LaneBriefInput{
		Workspace: domain.Workspace{
			ID: snapshot.Workspace.ID, Name: snapshot.Workspace.Name, State: domain.WorkspaceState(snapshot.Workspace.State),
			Revision: snapshot.Workspace.Revision, ActivePhaseID: activePhaseID,
		},
		Lane: domainLane(*lane, workspaceID), Phases: phases, Tasks: tasks, Dependencies: dependencies,
		Runs: runs, Records: records, Commits: commits, GitObservations: observations,
		Gates: gates, GateConditions: conditions,
		WorkspaceObservedAt: snapshot.Workspace.ObservedAt, LaneObservedAt: lane.ObservedAt,
		PhaseObservedAt: phaseObservedAt, TaskObservedAt: taskObservedAt,
		DependencyObservedAt: dependencyObservedAt, GateObservedAt: gateObservedAt,
		GateConditionObservedAt: conditionObservedAt, Now: now, StaleAfter: 72 * time.Hour, RecentLimit: 20,
	})
	if evaluation.HasErrors() {
		return domain.LaneBrief{}, fmt.Errorf("lane brief evaluation failed: %s (%s)", evaluation.Errors[0].Code, evaluation.Errors[0].EntityID)
	}
	return brief, nil
}

const laneBriefClockSkewTolerance = 5 * time.Second

// PostgreSQL (especially a local Docker VM) and the application clock can
// differ by a few seconds even when the snapshot was read synchronously.
// Accept only that bounded skew; larger future timestamps still reach the
// domain validator and are rejected as corrupt evidence.
func laneBriefEvaluationTime(now, workspaceObservedAt time.Time) time.Time {
	now, workspaceObservedAt = now.UTC(), workspaceObservedAt.UTC()
	if workspaceObservedAt.After(now) && workspaceObservedAt.Sub(now) <= laneBriefClockSkewTolerance {
		return workspaceObservedAt
	}
	return now
}

func localGitRoot() (string, error) {
	output, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func verifyLocalTaskRecord(root string, rootErr error, repositories []RepositoryProjection, record TaskRecordProjection) (string, string) {
	if rootErr != nil || root == "" {
		return "unverified", "local Git repository is unavailable"
	}
	var repository *RepositoryProjection
	for index := range repositories {
		if repositories[index].ID == record.RepositoryID {
			repository = &repositories[index]
			break
		}
	}
	if repository == nil || !repository.IsRecordRepository ||
		(repository.TaskRecordsRoot != "" && !strings.HasPrefix(record.RelativePath, strings.TrimSuffix(repository.TaskRecordsRoot, "/")+"/")) {
		return "missing", "Task Record repository binding is unavailable"
	}
	remote, err := exec.Command("git", "-C", root, "remote", "get-url", "origin").Output()
	if err != nil || normalizeGitRemote(string(remote)) == "" ||
		normalizeGitRemote(string(remote)) != normalizeGitRemote(repository.RemoteURL) {
		return "unverified", "local Git repository identity does not match the indexed repository"
	}
	path := filepath.Join(root, filepath.FromSlash(record.RelativePath))
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "missing", "Task Record path escapes the repository"
	}
	if _, err = os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return "missing", "Task Record path does not exist in the repository"
		}
		return "unverified", "Task Record path could not be inspected"
	}
	if record.State == string(domain.RecordReportedUncommitted) {
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return "unverified", "Task Record working-tree content could not be hashed"
		}
		digest := sha256.Sum256(content)
		if record.WorkingTreeHash == "" || record.WorkingTreeHash != "sha256:"+hex.EncodeToString(digest[:]) {
			return "stale", "Task Record working-tree content differs from its registered hash"
		}
		return "", ""
	}
	if record.CommitSHA == "" || record.BlobSHA == "" {
		return "unverified", "committed Task Record lacks commit/blob evidence"
	}
	if err = exec.Command("git", "-C", root, "cat-file", "-e", record.CommitSHA+"^{commit}").Run(); err != nil {
		return "stale", "Task Record commit is absent from the local repository"
	}
	committedBlob, err := exec.Command("git", "-C", root, "rev-parse", record.CommitSHA+":"+filepath.ToSlash(relative)).Output()
	if err != nil || strings.TrimSpace(string(committedBlob)) != record.BlobSHA {
		return "stale", "Task Record path/blob does not match the recorded commit"
	}
	workingBlob, err := exec.Command("git", "-C", root, "hash-object", path).Output()
	if err != nil || strings.TrimSpace(string(workingBlob)) != record.BlobSHA {
		return "stale", "Task Record working-tree content differs from the recorded blob"
	}
	return "", ""
}

func normalizeGitRemote(value string) string {
	value = strings.TrimSuffix(strings.TrimSuffix(strings.TrimSpace(value), "/"), ".git")
	if strings.HasPrefix(value, "git@") {
		value = strings.TrimPrefix(value, "git@")
		value = strings.Replace(value, ":", "/", 1)
		value = "https://" + value
	}
	return strings.ToLower(value)
}
