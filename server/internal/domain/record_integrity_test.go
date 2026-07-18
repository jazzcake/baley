package domain

import "testing"

func TestDetectRecordIntegrityMatchesRegisteredCommitAndHash(t *testing.T) {
	input := recordIntegrityFixture()
	report := DetectRecordIntegrity(input)
	if report.Evaluation.HasErrors() || len(report.Results) != 1 || len(report.Results[0].Issues) != 0 {
		t.Fatalf("matching Record reported inconsistent: %+v", report)
	}
}

func TestDetectRecordIntegrityReportsModifiedMissingAndUnregistered(t *testing.T) {
	input := recordIntegrityFixture()
	input.Registered = append(input.Registered, integrityRecord("missing", "task-records/task/missing.md"))
	input.Observed[0].WorkingTreeHash = "sha256:" + repeatHex("d", 64)
	input.Observed = append(input.Observed, RecordFileObservation{RecordID: "new", RepositoryID: "repo", RelativePath: "task-records/task/new.md", WorkingTreeHash: "sha256:" + repeatHex("e", 64)})
	report := DetectRecordIntegrity(input)
	if report.Evaluation.HasErrors() || !reportHasIssue(report, "record", RecordModified) || !reportHasIssue(report, "missing", RecordMissing) || !reportHasIssue(report, "new", RecordUnregistered) || !hasDiagnostic(report.Evaluation.Warnings, CodeRecordHashChanged) {
		t.Fatalf("integrity findings missing: %+v", report)
	}
}

func TestDetectRecordIntegrityReportsUncommittedAndCommitMismatch(t *testing.T) {
	input := recordIntegrityFixture()
	input.Observed[0].CommitSHA, input.Observed[0].BlobSHA = "", ""
	report := DetectRecordIntegrity(input)
	if report.Evaluation.HasErrors() || !reportHasIssue(report, "record", RecordUncommitted) || !reportHasIssue(report, "record", RecordCommitMismatch) {
		t.Fatalf("missing commit evidence accepted: %+v", report)
	}
	input = recordIntegrityFixture()
	input.Observed[0].CommitSHA = repeatHex("f", 40)
	report = DetectRecordIntegrity(input)
	if report.Evaluation.HasErrors() || !reportHasIssue(report, "record", RecordCommitMismatch) {
		t.Fatalf("commit mismatch not reported: %+v", report)
	}
}

func TestDetectRecordIntegrityPreventsFalseVerifiedAndReportsRemoteUnverified(t *testing.T) {
	input := recordIntegrityFixture()
	input.Registered[0].State = RecordVerified
	input.Observed[0].BlobSHA = repeatHex("f", 40)
	report := DetectRecordIntegrity(input)
	if report.Evaluation.HasErrors() || !reportHasIssue(report, "record", RecordCommitMismatch) {
		t.Fatalf("false verified Record accepted: %+v", report)
	}
	input = recordIntegrityFixture()
	input.Registered[0].State = RecordCommittedUnverified
	report = DetectRecordIntegrity(input)
	if report.Evaluation.HasErrors() || !hasDiagnostic(report.Evaluation.Advisories, CodeCommitRemoteUnverified) {
		t.Fatalf("unverified commit advisory missing: %+v", report)
	}
}

func TestDetectRecordIntegrityRejectsPartialInvalidOrAbsoluteObservations(t *testing.T) {
	input := recordIntegrityFixture()
	input.SnapshotComplete = false
	if report := DetectRecordIntegrity(input); !report.Evaluation.HasErrors() {
		t.Fatal("partial snapshot allowed to report missing files")
	}
	input = recordIntegrityFixture()
	input.Observed[0].RelativePath = "/Users/person/private/task.md"
	if report := DetectRecordIntegrity(input); !report.Evaluation.HasErrors() {
		t.Fatal("absolute local path accepted in observation")
	}
	input = recordIntegrityFixture()
	input.Observed[0].BlobSHA = repeatHex("f", 40)
	input.Observed[0].CommitSHA = ""
	if report := DetectRecordIntegrity(input); !report.Evaluation.HasErrors() {
		t.Fatal("partial commit evidence accepted")
	}
}

func recordIntegrityFixture() RecordIntegrityInput {
	record := integrityRecord("record", "task-records/task/report.md")
	return RecordIntegrityInput{
		Repository:       Repository{ID: "repo", WorkspaceID: "workspace", Name: "Records", RemoteURL: "https://example.com/repo.git", IsRecordRepository: true, TaskRecordsRoot: "task-records"},
		Registered:       []TaskRecord{record},
		Observed:         []RecordFileObservation{{RecordID: record.ID, RepositoryID: record.RepositoryID, RelativePath: record.RelativePath, WorkingTreeHash: record.WorkingTreeHash, CommitSHA: record.CommitSHA, BlobSHA: record.BlobSHA}},
		SnapshotComplete: true,
	}
}

func integrityRecord(id, relativePath string) TaskRecord {
	return TaskRecord{
		ID: id, WorkspaceID: "workspace", TaskID: "task", Type: RecordCompletionReport, RepositoryID: "repo", RelativePath: relativePath,
		WorkingTreeHash: "sha256:" + repeatHex("a", 64), CommitSHA: repeatHex("b", 40), BlobSHA: repeatHex("c", 40), State: RecordVerified, ShortSummary: "done",
	}
}

func reportHasIssue(report RecordIntegrityReport, recordID string, issue RecordIntegrityIssue) bool {
	for _, result := range report.Results {
		if result.RecordID == recordID && hasRecordIntegrityIssue(result.Issues, issue) {
			return true
		}
	}
	return false
}
