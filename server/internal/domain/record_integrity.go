package domain

import (
	"sort"
	"strings"
)

type RecordIntegrityIssue string

const (
	RecordMissing        RecordIntegrityIssue = "missing"
	RecordUnregistered   RecordIntegrityIssue = "unregistered"
	RecordModified       RecordIntegrityIssue = "modified"
	RecordUncommitted    RecordIntegrityIssue = "uncommitted"
	RecordCommitMismatch RecordIntegrityIssue = "commit_mismatch"
)

var RecordIntegrityIssues = []RecordIntegrityIssue{RecordMissing, RecordUnregistered, RecordModified, RecordUncommitted, RecordCommitMismatch}

// RecordFileObservation is a client-produced, repository-relative snapshot.
// It intentionally contains no local absolute path or file-reading capability.
type RecordFileObservation struct {
	RecordID        string
	RepositoryID    string
	RelativePath    string
	WorkingTreeHash string
	CommitSHA       string
	BlobSHA         string
}

type RecordIntegrityInput struct {
	Repository       Repository
	Registered       []TaskRecord
	Observed         []RecordFileObservation
	SnapshotComplete bool
}

type RecordIntegrityResult struct {
	RecordID                  string
	RepositoryID              string
	RelativePath              string
	RegisteredState           RecordState
	RegisteredWorkingTreeHash string
	ObservedWorkingTreeHash   string
	RegisteredCommitSHA       string
	ObservedCommitSHA         string
	RegisteredBlobSHA         string
	ObservedBlobSHA           string
	Issues                    []RecordIntegrityIssue
}

type RecordIntegrityReport struct {
	Results    []RecordIntegrityResult
	Evaluation Evaluation
}

func DetectRecordIntegrity(input RecordIntegrityInput) RecordIntegrityReport {
	report := RecordIntegrityReport{}
	repository, err := NewRepository(input.Repository)
	if err != nil || !repository.IsRecordRepository || !input.SnapshotComplete {
		report.Evaluation.Errors = append(report.Evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: input.Repository.ID})
		return report
	}

	registeredByPath := make(map[string]TaskRecord, len(input.Registered))
	registeredIDs := make(map[string]bool, len(input.Registered))
	for _, record := range input.Registered {
		normalized, pathErr := NormalizeRecordPath(repository.TaskRecordsRoot, record.RelativePath)
		if pathErr != nil || normalized != record.RelativePath || record.WorkspaceID != repository.WorkspaceID || record.RepositoryID != repository.ID || registeredIDs[record.ID] || registeredByPath[record.RelativePath].ID != "" || !validIntegrityRecord(record) {
			report.Evaluation.Errors = append(report.Evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: record.ID})
			continue
		}
		registeredIDs[record.ID] = true
		registeredByPath[record.RelativePath] = record
	}

	observedPaths := make(map[string]bool, len(input.Observed))
	observedIDs := make(map[string]bool, len(input.Observed))
	matchedRecordIDs := make(map[string]bool, len(input.Observed))
	for _, observation := range input.Observed {
		normalized, pathErr := NormalizeRecordPath(repository.TaskRecordsRoot, observation.RelativePath)
		observation = normalizeRecordObservation(observation)
		if pathErr != nil || normalized != observation.RelativePath || observation.RecordID == "" || observation.RepositoryID != repository.ID || observation.WorkingTreeHash == "" || !validWorkingTreeHash(observation.WorkingTreeHash) || !validObservedCommitPair(observation.CommitSHA, observation.BlobSHA) || observedPaths[observation.RelativePath] || observedIDs[observation.RecordID] {
			report.Evaluation.Errors = append(report.Evaluation.Errors, Diagnostic{Code: CodeInvalidStateTransition, EntityID: observation.RecordID})
			continue
		}
		observedPaths[observation.RelativePath] = true
		observedIDs[observation.RecordID] = true
		record, exists := registeredByPath[observation.RelativePath]
		if !exists || record.ID != observation.RecordID {
			report.Results = append(report.Results, RecordIntegrityResult{
				RecordID: observation.RecordID, RepositoryID: observation.RepositoryID, RelativePath: observation.RelativePath,
				ObservedWorkingTreeHash: observation.WorkingTreeHash, ObservedCommitSHA: observation.CommitSHA, ObservedBlobSHA: observation.BlobSHA,
				Issues: []RecordIntegrityIssue{RecordUnregistered},
			})
			continue
		}
		matchedRecordIDs[record.ID] = true
		result := compareRecordObservation(record, observation)
		report.Results = append(report.Results, result)
		if hasRecordIntegrityIssue(result.Issues, RecordModified) {
			report.Evaluation.Warnings = append(report.Evaluation.Warnings, Diagnostic{Code: CodeRecordHashChanged, EntityID: record.ID, Details: map[string]any{"registeredHash": record.WorkingTreeHash, "observedHash": observation.WorkingTreeHash}})
		}
		if record.State == RecordCommittedUnverified {
			report.Evaluation.Advisories = append(report.Evaluation.Advisories, Diagnostic{Code: CodeCommitRemoteUnverified, EntityID: record.ID})
		}
	}
	for _, record := range input.Registered {
		if registeredIDs[record.ID] && !matchedRecordIDs[record.ID] {
			report.Results = append(report.Results, RecordIntegrityResult{
				RecordID: record.ID, RepositoryID: record.RepositoryID, RelativePath: record.RelativePath, RegisteredState: record.State,
				RegisteredWorkingTreeHash: record.WorkingTreeHash, RegisteredCommitSHA: record.CommitSHA, RegisteredBlobSHA: record.BlobSHA,
				Issues: []RecordIntegrityIssue{RecordMissing},
			})
		}
	}

	report.Evaluation.sort()
	if report.Evaluation.HasErrors() {
		report.Results = nil
		return report
	}
	sort.Slice(report.Results, func(i, j int) bool {
		if report.Results[i].RelativePath != report.Results[j].RelativePath {
			return report.Results[i].RelativePath < report.Results[j].RelativePath
		}
		return report.Results[i].RecordID < report.Results[j].RecordID
	})
	return report
}

func validIntegrityRecord(record TaskRecord) bool {
	if record.ID == "" || record.TaskID == "" || !validRecordType(record.Type) || strings.TrimSpace(record.ShortSummary) == "" || !validWorkingTreeHash(record.WorkingTreeHash) {
		return false
	}
	if record.WorkingTreeHash != strings.ToLower(record.WorkingTreeHash) || record.CommitSHA != strings.ToLower(record.CommitSHA) || record.BlobSHA != strings.ToLower(record.BlobSHA) {
		return false
	}
	switch record.State {
	case RecordReportedUncommitted:
		return record.CommitSHA == "" && record.BlobSHA == ""
	case RecordCommittedUnverified, RecordVerified:
		return validObservedCommitPair(record.CommitSHA, record.BlobSHA) && record.CommitSHA != ""
	default:
		return false
	}
}

func normalizeRecordObservation(value RecordFileObservation) RecordFileObservation {
	value.RecordID = strings.TrimSpace(value.RecordID)
	value.RepositoryID = strings.TrimSpace(value.RepositoryID)
	value.RelativePath = strings.TrimSpace(value.RelativePath)
	value.WorkingTreeHash = strings.ToLower(strings.TrimSpace(value.WorkingTreeHash))
	value.CommitSHA = strings.ToLower(strings.TrimSpace(value.CommitSHA))
	value.BlobSHA = strings.ToLower(strings.TrimSpace(value.BlobSHA))
	return value
}

func validObservedCommitPair(commitSHA, blobSHA string) bool {
	if commitSHA == "" && blobSHA == "" {
		return true
	}
	return validGitObjectID(commitSHA) && validGitObjectID(blobSHA) && len(commitSHA) == len(blobSHA)
}

func compareRecordObservation(record TaskRecord, observation RecordFileObservation) RecordIntegrityResult {
	result := RecordIntegrityResult{
		RecordID: record.ID, RepositoryID: record.RepositoryID, RelativePath: record.RelativePath, RegisteredState: record.State,
		RegisteredWorkingTreeHash: record.WorkingTreeHash, ObservedWorkingTreeHash: observation.WorkingTreeHash,
		RegisteredCommitSHA: record.CommitSHA, ObservedCommitSHA: observation.CommitSHA,
		RegisteredBlobSHA: record.BlobSHA, ObservedBlobSHA: observation.BlobSHA,
	}
	if record.WorkingTreeHash != "" && record.WorkingTreeHash != observation.WorkingTreeHash {
		result.Issues = append(result.Issues, RecordModified)
	}
	if record.State == RecordReportedUncommitted || observation.CommitSHA == "" {
		result.Issues = append(result.Issues, RecordUncommitted)
	}
	if record.CommitSHA != "" && (record.CommitSHA != observation.CommitSHA || record.BlobSHA != observation.BlobSHA) {
		result.Issues = append(result.Issues, RecordCommitMismatch)
	}
	return result
}

func hasRecordIntegrityIssue(issues []RecordIntegrityIssue, expected RecordIntegrityIssue) bool {
	for _, issue := range issues {
		if issue == expected {
			return true
		}
	}
	return false
}
