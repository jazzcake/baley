package collab

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jazzcake/baley/server/internal/domain"
)

type NotificationKind string

const (
	NotifyStaleLane       NotificationKind = "stale_lane"
	NotifyLongBlocker     NotificationKind = "long_blocker"
	NotifyReadyGate       NotificationKind = "ready_gate"
	NotifyTaskDecision    NotificationKind = "implemented_task_decision"
	NotifyExpiredRunLease NotificationKind = "expired_run_lease"
)

type ObservedLane struct {
	Lane      domain.Lane
	UpdatedAt time.Time
}

type ObservedTask struct {
	Task       domain.Task
	StateSince time.Time
}

type ObservedGate struct {
	Gate                 domain.Gate
	Conditions           []domain.GateTaskCondition
	ConditionSatisfiedAt map[string]time.Time
	ReadySince           time.Time
}

type NotificationInput struct {
	WorkspaceID    string
	Now            time.Time
	LaneStaleAfter time.Duration
	BlockerAfter   time.Duration
	Lanes          []ObservedLane
	Tasks          []ObservedTask
	Gates          []ObservedGate
	Runs           []domain.Run
}

type NotificationCandidate struct {
	Kind        NotificationKind `json:"kind"`
	WorkspaceID string           `json:"workspaceId"`
	EntityType  string           `json:"entityType"`
	EntityID    string           `json:"entityId"`
	Since       time.Time        `json:"since"`
	Fingerprint string           `json:"fingerprint"`
}

func DetectNotifications(input NotificationInput) ([]NotificationCandidate, error) {
	if !cleanIdentifier(input.WorkspaceID) || input.Now.IsZero() || input.LaneStaleAfter <= 0 || input.BlockerAfter <= 0 {
		return nil, ErrInvalidNotificationInput
	}

	result := []NotificationCandidate{}
	add := func(kind NotificationKind, entityType, id string, since time.Time, discriminator any) {
		payload := fmt.Sprintf("%s|%s|%s|%s|%s|%v", kind, input.WorkspaceID, entityType, id, since.UTC().Format(time.RFC3339Nano), discriminator)
		digest := sha256.Sum256([]byte(payload))
		result = append(result, NotificationCandidate{
			Kind:        kind,
			WorkspaceID: input.WorkspaceID,
			EntityType:  entityType,
			EntityID:    id,
			Since:       since,
			Fingerprint: "sha256:" + hex.EncodeToString(digest[:]),
		})
	}

	laneIDs := map[string]bool{}
	for _, observed := range input.Lanes {
		if !validObservedLane(input, observed) || laneIDs[observed.Lane.ID] {
			return nil, ErrInvalidNotificationInput
		}
		laneIDs[observed.Lane.ID] = true
		if observed.Lane.State == domain.LaneActive && input.Now.Sub(observed.UpdatedAt) >= input.LaneStaleAfter {
			add(NotifyStaleLane, "lane", observed.Lane.ID, observed.UpdatedAt, observed.Lane.State)
		}
	}

	taskIDs := map[string]bool{}
	tasksByID := map[string]ObservedTask{}
	for _, observed := range input.Tasks {
		if !validObservedTask(input, observed) || taskIDs[observed.Task.ID] {
			return nil, ErrInvalidNotificationInput
		}
		taskIDs[observed.Task.ID] = true
		tasksByID[observed.Task.ID] = observed
		if observed.Task.BlockedAt != nil && input.Now.Sub(*observed.Task.BlockedAt) >= input.BlockerAfter {
			add(NotifyLongBlocker, "task", observed.Task.ID, *observed.Task.BlockedAt, strings.TrimSpace(observed.Task.BlockerReason))
		} else if observed.Task.Status == domain.TaskImplemented {
			add(NotifyTaskDecision, "task", observed.Task.ID, observed.StateSince, observed.Task.Status)
		}
	}

	gateIDs := map[string]bool{}
	for _, observed := range input.Gates {
		if !validObservedGate(input, observed, tasksByID) || gateIDs[observed.Gate.ID] {
			return nil, ErrInvalidNotificationInput
		}
		gateIDs[observed.Gate.ID] = true
		if observed.Gate.PassedAt == nil && domain.GateStatusFor(observed.Gate, observed.Conditions) == domain.GateReadyStatus {
			add(NotifyReadyGate, "gate", observed.Gate.ID, observed.ReadySince, observed.Gate.CriteriaRevision)
		}
	}

	runIDs := map[string]bool{}
	for _, run := range input.Runs {
		if !validObservedRun(input, run, tasksByID) || runIDs[run.ID] {
			return nil, ErrInvalidNotificationInput
		}
		runIDs[run.ID] = true
		if run.Status == domain.RunRunning && run.IsLeaseExpired(input.Now) {
			add(NotifyExpiredRunLease, "run", run.ID, run.LeaseExpiresAt, run.Version)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Kind != result[j].Kind {
			return result[i].Kind < result[j].Kind
		}
		return result[i].EntityID < result[j].EntityID
	})
	return result, nil
}

func validObservedLane(input NotificationInput, observed ObservedLane) bool {
	return cleanIdentifier(observed.Lane.ID) && observed.Lane.WorkspaceID == input.WorkspaceID &&
		(observed.Lane.State == domain.LaneActive || observed.Lane.State == domain.LaneClosedOut || observed.Lane.State == domain.LaneDiscarded) &&
		!observed.UpdatedAt.IsZero() && !observed.UpdatedAt.After(input.Now)
}

func validObservedTask(input NotificationInput, observed ObservedTask) bool {
	if !cleanIdentifier(observed.Task.ID) || observed.Task.WorkspaceID != input.WorkspaceID || !knownNotificationTaskStatus(observed.Task.Status) || observed.StateSince.IsZero() || observed.StateSince.After(input.Now) {
		return false
	}
	if observed.Task.BlockedAt == nil {
		return strings.TrimSpace(observed.Task.BlockerReason) == ""
	}
	return (observed.Task.Status == domain.TaskPending || observed.Task.Status == domain.TaskInProgress) &&
		!observed.Task.BlockedAt.IsZero() && !observed.Task.BlockedAt.After(input.Now) && strings.TrimSpace(observed.Task.BlockerReason) != ""
}

func validObservedGate(input NotificationInput, observed ObservedGate, tasksByID map[string]ObservedTask) bool {
	if !cleanIdentifier(observed.Gate.ID) || observed.Gate.WorkspaceID != input.WorkspaceID || observed.Gate.CriteriaRevision <= 0 {
		return false
	}
	if observed.Gate.PassedAt != nil && (observed.Gate.PassedAt.IsZero() || observed.Gate.PassedAt.After(input.Now)) {
		return false
	}
	links, tasks := map[string]bool{}, map[string]bool{}
	latestSatisfiedAt := time.Time{}
	for _, condition := range observed.Conditions {
		canonical, exists := tasksByID[condition.TaskID]
		if condition.WorkspaceID != input.WorkspaceID || condition.GateID != observed.Gate.ID || !cleanIdentifier(condition.LinkID) || !cleanIdentifier(condition.TaskID) || links[condition.LinkID] || tasks[condition.TaskID] || !knownNotificationTaskStatus(condition.TaskStatus) || !exists || canonical.Task.WorkspaceID != input.WorkspaceID || canonical.Task.Status != condition.TaskStatus {
			return false
		}
		if condition.Passed != (strings.TrimSpace(condition.PassReason) != "") {
			return false
		}
		satisfied := condition.Passed || condition.TaskStatus == domain.TaskConfirmed
		satisfiedAt, hasSatisfiedAt := observed.ConditionSatisfiedAt[condition.LinkID]
		if satisfied != hasSatisfiedAt || hasSatisfiedAt && (satisfiedAt.IsZero() || satisfiedAt.After(input.Now) || !condition.Passed && !satisfiedAt.Equal(canonical.StateSince)) {
			return false
		}
		if hasSatisfiedAt && satisfiedAt.After(latestSatisfiedAt) {
			latestSatisfiedAt = satisfiedAt
		}
		links[condition.LinkID], tasks[condition.TaskID] = true, true
	}
	for linkID := range observed.ConditionSatisfiedAt {
		if !links[linkID] {
			return false
		}
	}
	snapshotReady := domain.GateReady(observed.Conditions)
	if snapshotReady && (observed.ReadySince.IsZero() || !observed.ReadySince.Equal(latestSatisfiedAt)) || !snapshotReady && !observed.ReadySince.IsZero() {
		return false
	}
	if observed.Gate.PassedAt != nil && (!snapshotReady || observed.ReadySince.After(*observed.Gate.PassedAt)) {
		return false
	}
	return !observed.ReadySince.After(input.Now)
}

func validObservedRun(input NotificationInput, run domain.Run, tasksByID map[string]ObservedTask) bool {
	canonical, taskExists := tasksByID[run.TaskID]
	if !cleanIdentifier(run.ID) || !cleanIdentifier(run.TaskID) || run.WorkspaceID != input.WorkspaceID || !taskExists || canonical.Task.WorkspaceID != input.WorkspaceID || !knownNotificationRunStatus(run.Status) || !knownNotificationRunKind(run.Kind) || run.Version <= 0 || run.StartedAt.IsZero() || run.StartedAt.After(input.Now) || run.HeartbeatAt.IsZero() || run.HeartbeatAt.Before(run.StartedAt) || run.HeartbeatAt.After(input.Now) || run.LeaseExpiresAt.IsZero() || !run.LeaseExpiresAt.After(run.HeartbeatAt) {
		return false
	}
	if run.Status == domain.RunRunning {
		return run.EndedAt == nil && strings.TrimSpace(run.ResultSummary) == "" && strings.TrimSpace(run.ErrorSummary) == ""
	}
	validEnd := run.EndedAt != nil && !run.EndedAt.IsZero() && !run.EndedAt.Before(run.StartedAt) && !run.EndedAt.Before(run.HeartbeatAt) && !run.EndedAt.After(input.Now)
	if run.Status == domain.RunSucceeded {
		return validEnd && strings.TrimSpace(run.ResultSummary) != "" && strings.TrimSpace(run.ErrorSummary) == ""
	}
	return validEnd && strings.TrimSpace(run.ResultSummary) == "" && strings.TrimSpace(run.ErrorSummary) != ""
}

func cleanIdentifier(value string) bool { return value != "" && value == strings.TrimSpace(value) }

func knownNotificationTaskStatus(value domain.TaskStatus) bool {
	for _, candidate := range domain.TaskStatuses {
		if value == candidate {
			return true
		}
	}
	return false
}

func knownNotificationRunStatus(value domain.RunStatus) bool {
	for _, candidate := range domain.RunStatuses {
		if value == candidate {
			return true
		}
	}
	return false
}

func knownNotificationRunKind(value domain.RunKind) bool {
	for _, candidate := range domain.RunKinds {
		if value == candidate {
			return true
		}
	}
	return false
}

const ErrInvalidNotificationInput conflictError = "invalid notification input"
