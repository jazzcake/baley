package projectinit

import (
	"reflect"
	"strings"
	"testing"
)

const (
	projectID    = "018f4c18-7a3b-7cc1-8b1a-b9f8f03f4501"
	workspaceID  = "018f4c18-7a3b-7cc1-8b1a-b9f8f03f4502"
	repositoryID = "018f4c18-7a3b-7cc1-8b1a-b9f8f03f4503"
)

func TestBuildCreatesDeterministicBootstrapAndFileManifest(t *testing.T) {
	plan, err := Build(validInput())
	if err != nil || plan.Bootstrap.Name != "project.bootstrap" || plan.Bootstrap.Arguments.ClientProjectID != projectID || plan.Bootstrap.Envelope.IdempotencyKey != projectID || plan.Bootstrap.Envelope.ExecutedByActorID != "agent" || plan.Bootstrap.Envelope.InitiatedByActorID != "human" || len(plan.Files) != 9 || !plan.Ready {
		t.Fatalf("init manifest failed: %+v %v", plan, err)
	}
	wantPaths := []string{
		".baley-init-state.json", ".rgignore", "baley.yaml", "task-records/README.md", "task-records/_templates/completion-report.md",
		"task-records/_templates/detailed-plan.md", "task-records/_templates/handoff.md",
		"task-records/_templates/independent-agent-review.md", "task-records/_templates/review-response.md",
	}
	if got := filePaths(plan.Files); !reflect.DeepEqual(got, wantPaths) {
		t.Fatalf("manifest paths drift: %v", got)
	}
	for _, file := range plan.Files {
		if file.Action != FileCreate || file.DesiredContent == "" || file.DesiredSHA == "" {
			t.Errorf("invalid create plan: %+v", file)
		}
	}
}

func TestBuildUsesNonDestructiveMergeAndReportsConflicts(t *testing.T) {
	input := validInput()
	input.ExistingFiles = map[string]string{".rgignore": "node_modules/**", "baley.yaml": "user-owned: true\n"}
	plan, err := Build(input)
	ignore, config := fileByPath(plan.Files, ".rgignore"), fileByPath(plan.Files, "baley.yaml")
	if err != nil || plan.Ready || ignore.Action != FileMerge || ignore.DesiredContent != "node_modules/**\ntask-records/**\n" || config.Action != FileConflict || config.DesiredContent == input.ExistingFiles["baley.yaml"] {
		t.Fatalf("non-destructive classification failed: %+v %v", plan, err)
	}
	if !hasRecovery(plan.Recovery, "resolve_without_overwrite", "baley.yaml") {
		t.Fatalf("conflict recovery missing: %+v", plan.Recovery)
	}
}

func TestBuildDetectsCaseInsensitiveExistingFileCollisions(t *testing.T) {
	input := validInput()
	input.ExistingFiles = map[string]string{"BALEY.YAML": "user-owned: true\n"}
	plan, err := Build(input)
	config := fileByPath(plan.Files, "baley.yaml")
	if err != nil || plan.Ready || config.Action != FileConflict || config.ConflictingPath != "BALEY.YAML" || !hasRecovery(plan.Recovery, "resolve_without_overwrite", "baley.yaml") {
		t.Fatalf("case-insensitive collision missed: %+v %v", plan, err)
	}
	input.ExistingFiles["baley.yaml"] = "duplicate case entry"
	if _, err := Build(input); err != ErrInvalidInput {
		t.Fatalf("case-fold duplicate snapshot accepted: %v", err)
	}
}

func TestBuildRerunIsIdempotentAfterApplyingManifest(t *testing.T) {
	first, err := Build(validInput())
	if err != nil {
		t.Fatal(err)
	}
	input := validInput()
	input.BootstrapCompleted = true
	input.ExistingFiles = applyManifest(nil, first.Files)
	second, err := Build(input)
	if err != nil || second.Bootstrap != first.Bootstrap || !second.Ready || len(second.Recovery) != 1 || second.Recovery[0].Action != "verify_manifest_and_server_binding" {
		t.Fatalf("rerun drift: %+v %v", second, err)
	}
	for _, file := range second.Files {
		if file.Action != FileKeep {
			t.Errorf("rerun planned write: %+v", file)
		}
	}
}

func TestBuildResumesPartialFailureWithSameClientProjectID(t *testing.T) {
	initial, _ := Build(validInput())
	input := validInput()
	input.BootstrapCompleted = true
	input.ExistingFiles = map[string]string{
		"baley.yaml": fileByPath(initial.Files, "baley.yaml").DesiredContent,
		".rgignore":  "task-records/**\n",
	}
	resumed, err := Build(input)
	if err != nil || resumed.Bootstrap.Arguments.ClientProjectID != initial.Bootstrap.Arguments.ClientProjectID || fileByPath(resumed.Files, "baley.yaml").Action != FileKeep || fileByPath(resumed.Files, "task-records/README.md").Action != FileCreate || resumed.Recovery[0].Action != "persist_retry_identity" {
		t.Fatalf("partial failure did not resume: %+v %v", resumed, err)
	}
}

func TestBuildRecoversClientProjectIDAfterCrashFromDurableState(t *testing.T) {
	initial, err := Build(validInput())
	if err != nil {
		t.Fatal(err)
	}
	input := Input{ExistingFiles: map[string]string{recoveryStatePath: fileByPath(initial.Files, recoveryStatePath).DesiredContent}}
	resumed, err := Build(input)
	if err != nil || resumed.Bootstrap.Arguments.ClientProjectID != projectID || resumed.Bootstrap.Envelope.IdempotencyKey != projectID || resumed.Bootstrap.Arguments.WorkspaceID != workspaceID || resumed.Bootstrap.Envelope.ExecutedByActorID != "agent" || fileByPath(resumed.Files, recoveryStatePath).Action != FileKeep || resumed.Recovery[0].Action != "execute_or_retry_project_bootstrap" {
		t.Fatalf("durable retry identity not recovered: %+v %v", resumed, err)
	}
}

func TestBuildRejectsTamperedRecoveryState(t *testing.T) {
	initial, _ := Build(validInput())
	content := fileByPath(initial.Files, recoveryStatePath).DesiredContent
	content = strings.Replace(content, "https://baley.example.com", "https://attacker.example.com", 1)
	if _, err := Build(Input{ExistingFiles: map[string]string{recoveryStatePath: content}}); err != ErrInvalidInput {
		t.Fatalf("tampered retry state accepted: %v", err)
	}
}

func TestBuildRejectsInvalidIdentityConfigAndExistingPath(t *testing.T) {
	tests := []Input{validInput(), validInput(), validInput(), validInput(), validInput(), validInput()}
	tests[0].ClientProjectID = "not-uuid"
	tests[1].Server = "https://user:secret@example.com"
	tests[2].ExistingFiles = map[string]string{"../baley.yaml": "unsafe"}
	tests[3].RecordRepositoryID = "018f4c18-7a3b-7cc1-8b1a-b9f8f03f4599"
	tests[4].TaskRecordsRoot = "baley.yaml/records"
	tests[5].ExecutedByActorID = ""
	for _, input := range tests {
		if _, err := Build(input); err != ErrInvalidInput {
			t.Fatalf("invalid init input accepted: %+v %v", input, err)
		}
	}
}

func TestBuildRejectsUnsafeIgnoreAndGitMetadataRoots(t *testing.T) {
	for _, root := range []string{"*", "**", "!records", "#records", "records/[private]", ".git/records", "records/.GIT/private", "records\nprivate", "records\u0085private", recoveryStatePath + "/records", "BALEY.YAML/records", ".RGIGNORE/records", strings.ToUpper(recoveryStatePath) + "/records"} {
		input := validInput()
		input.TaskRecordsRoot = root
		if _, err := Build(input); err != ErrInvalidInput {
			t.Fatalf("unsafe Task Record root %q accepted: %v", root, err)
		}
	}
}

func validInput() Input {
	return Input{
		ClientProjectID: projectID, Server: "https://baley.example.com", WorkspaceID: workspaceID, WorkspaceName: "Baley",
		RepositoryID: repositoryID, RepositoryName: "Main", RemoteURL: "https://example.com/baley.git", TaskRecordsRoot: "task-records",
		InitiatedByActorID: "human", ExecutedByActorID: "agent",
	}
}

func filePaths(files []FilePlan) []string {
	result := make([]string, len(files))
	for index, file := range files {
		result[index] = file.RelativePath
	}
	return result
}

func fileByPath(files []FilePlan, relativePath string) FilePlan {
	for _, file := range files {
		if file.RelativePath == relativePath {
			return file
		}
	}
	return FilePlan{}
}

func hasRecovery(steps []RecoveryStep, action, path string) bool {
	for _, step := range steps {
		if step.Action == action {
			for _, candidate := range step.Paths {
				if candidate == path {
					return true
				}
			}
		}
	}
	return false
}

func applyManifest(existing map[string]string, files []FilePlan) map[string]string {
	result := map[string]string{}
	for key, value := range existing {
		result[key] = value
	}
	for _, file := range files {
		if file.Action == FileCreate || file.Action == FileMerge {
			result[file.RelativePath] = file.DesiredContent
		}
	}
	return result
}
