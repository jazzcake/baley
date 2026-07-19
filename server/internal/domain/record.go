package domain

import (
	"strings"
	"unicode/utf8"
)

type RecordType string

const (
	RecordDetailedPlan      RecordType = "detailed-plan"
	RecordHandoff           RecordType = "handoff"
	RecordIndependentReview RecordType = "independent-agent-review"
	RecordReviewResponse    RecordType = "review-response"
	RecordCompletionReport  RecordType = "completion-report"
)

var RecordTypes = []RecordType{RecordDetailedPlan, RecordHandoff, RecordIndependentReview, RecordReviewResponse, RecordCompletionReport}

type RecordState string

const (
	RecordReportedUncommitted RecordState = "reported_uncommitted"
	RecordCommittedUnverified RecordState = "committed_unverified"
	RecordVerified            RecordState = "verified"
)

var RecordStates = []RecordState{RecordReportedUncommitted, RecordCommittedUnverified, RecordVerified}

type TaskRecord struct {
	ID                 string
	WorkspaceID        string
	TaskID             string
	RunID              string
	Type               RecordType
	RepositoryID       string
	RelativePath       string
	WorkingTreeHash    string
	CommitSHA          string
	BlobSHA            string
	State              RecordState
	ShortSummary       string
	SupersedesRecordID string
}

type RecordRegistration struct {
	ID                 string
	WorkspaceID        string
	TaskID             string
	RunID              string
	Type               RecordType
	RepositoryID       string
	RelativePath       string
	WorkingTreeHash    string
	ShortSummary       string
	SupersedesRecordID string
}

func NewTaskRecord(taskRecordsRoot string, input RecordRegistration, existingRecords map[string]TaskRecord) (TaskRecord, error) {
	normalizedPath, pathErr := NormalizeRecordPath(taskRecordsRoot, input.RelativePath)
	if pathErr != nil {
		return TaskRecord{}, pathErr
	}
	if strings.TrimSpace(input.ID) == "" || strings.TrimSpace(input.WorkspaceID) == "" || strings.TrimSpace(input.TaskID) == "" || strings.TrimSpace(input.RepositoryID) == "" || !validRecordType(input.Type) || strings.TrimSpace(input.ShortSummary) == "" || utf8.RuneCountInString(strings.TrimSpace(input.ShortSummary)) > 500 || input.SupersedesRecordID == input.ID || !validWorkingTreeHash(input.WorkingTreeHash) {
		return TaskRecord{}, &Violation{Code: CodeInvalidStateTransition}
	}
	if err := validateRecordSupersession(input, existingRecords); err != nil {
		return TaskRecord{}, err
	}
	return TaskRecord{
		ID: input.ID, WorkspaceID: input.WorkspaceID, TaskID: input.TaskID, RunID: input.RunID,
		Type: input.Type, RepositoryID: input.RepositoryID, RelativePath: normalizedPath,
		WorkingTreeHash: strings.ToLower(input.WorkingTreeHash), State: RecordReportedUncommitted,
		ShortSummary: strings.TrimSpace(input.ShortSummary), SupersedesRecordID: input.SupersedesRecordID,
	}, nil
}

func CompareRecordRegistration(taskRecordsRoot string, existing TaskRecord, requested RecordRegistration) error {
	normalizedPath, err := NormalizeRecordPath(taskRecordsRoot, requested.RelativePath)
	if err != nil {
		return err
	}
	if existing.ID != requested.ID {
		return &Violation{Code: CodeIdempotencyConflict}
	}
	if existing.RelativePath != normalizedPath || existing.WorkingTreeHash != strings.ToLower(requested.WorkingTreeHash) {
		return &Violation{Code: CodeRecordHashConflict}
	}
	if existing.WorkspaceID != requested.WorkspaceID || existing.TaskID != requested.TaskID || existing.RunID != requested.RunID || existing.Type != requested.Type || existing.RepositoryID != requested.RepositoryID || existing.ShortSummary != strings.TrimSpace(requested.ShortSummary) || existing.SupersedesRecordID != requested.SupersedesRecordID {
		return &Violation{Code: CodeIdempotencyConflict}
	}
	return nil
}

func (r TaskRecord) AttachCommit(commitSHA, blobSHA string) (TaskRecord, RunTransitionOutcome, error) {
	commitSHA, blobSHA = strings.ToLower(strings.TrimSpace(commitSHA)), strings.ToLower(strings.TrimSpace(blobSHA))
	if !validGitObjectID(commitSHA) || !validGitObjectID(blobSHA) || len(commitSHA) != len(blobSHA) {
		return r, "", &Violation{Code: CodeInvalidStateTransition}
	}
	if r.CommitSHA != "" || r.BlobSHA != "" {
		if r.CommitSHA == commitSHA && r.BlobSHA == blobSHA {
			return r, RunTransitionIdempotent, nil
		}
		return r, "", &Violation{Code: CodeRecordHashConflict}
	}
	if r.State != RecordReportedUncommitted {
		return r, "", &Violation{Code: CodeInvalidStateTransition}
	}
	next := r
	next.CommitSHA, next.BlobSHA, next.State = commitSHA, blobSHA, RecordCommittedUnverified
	return next, RunTransitionApplied, nil
}

func (r TaskRecord) MarkVerified() (TaskRecord, error) {
	if r.State != RecordCommittedUnverified || r.CommitSHA == "" || r.BlobSHA == "" {
		return r, &Violation{Code: CodeInvalidStateTransition}
	}
	next := r
	next.State = RecordVerified
	return next, nil
}

func validRecordType(value RecordType) bool {
	for _, candidate := range RecordTypes {
		if value == candidate {
			return true
		}
	}
	return false
}

func validWorkingTreeHash(value string) bool {
	if value == "" {
		return true
	}
	if !strings.HasPrefix(value, "sha256:") {
		return false
	}
	return validHexLength(strings.TrimPrefix(value, "sha256:"), 64)
}

func validateRecordSupersession(input RecordRegistration, existing map[string]TaskRecord) error {
	if input.SupersedesRecordID == "" {
		return nil
	}
	seen := map[string]bool{input.ID: true}
	currentID := input.SupersedesRecordID
	for currentID != "" {
		if seen[currentID] {
			return &Violation{Code: CodeInvalidStateTransition}
		}
		seen[currentID] = true
		current, ok := existing[currentID]
		if !ok || current.WorkspaceID != input.WorkspaceID || current.TaskID != input.TaskID || current.Type != input.Type {
			return &Violation{Code: CodeInvalidStateTransition}
		}
		currentID = current.SupersedesRecordID
	}
	return nil
}
