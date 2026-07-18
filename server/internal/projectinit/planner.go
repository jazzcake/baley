package projectinit

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/jazzcake/baley/server/internal/config"
	"github.com/jazzcake/baley/server/internal/domain"
)

type FileAction string

const (
	FileCreate   FileAction = "create"
	FileKeep     FileAction = "keep"
	FileMerge    FileAction = "merge"
	FileConflict FileAction = "conflict"
)

type BootstrapArguments struct {
	ClientProjectID string `json:"clientProjectId"`
	WorkspaceID     string `json:"workspaceId"`
	WorkspaceName   string `json:"workspaceName"`
	RepositoryID    string `json:"repositoryId"`
	RepositoryName  string `json:"repositoryName"`
	RemoteURL       string `json:"remoteUrl"`
	TaskRecordsRoot string `json:"taskRecordsRoot"`
}

type BootstrapEnvelope struct {
	IdempotencyKey     string `json:"idempotencyKey"`
	InitiatedByActorID string `json:"initiatedByActorId,omitempty"`
	ExecutedByActorID  string `json:"executedByActorId"`
}

type BootstrapRequest struct {
	Name      string             `json:"name"`
	Arguments BootstrapArguments `json:"arguments"`
	Envelope  BootstrapEnvelope  `json:"envelope"`
}

type FilePlan struct {
	RelativePath        string     `json:"relativePath"`
	ConflictingPath     string     `json:"conflictingPath,omitempty"`
	Action              FileAction `json:"action"`
	DesiredContent      string     `json:"desiredContent,omitempty"`
	ExpectedExistingSHA string     `json:"expectedExistingSha256,omitempty"`
	DesiredSHA          string     `json:"desiredSha256"`
}

type RecoveryStep struct {
	Action string   `json:"action"`
	Paths  []string `json:"paths,omitempty"`
}

type Input struct {
	ClientProjectID    string
	Server             string
	WorkspaceID        string
	WorkspaceName      string
	RepositoryID       string
	RepositoryName     string
	RemoteURL          string
	RecordRepositoryID string
	TaskRecordsRoot    string
	InitiatedByActorID string
	ExecutedByActorID  string
	BootstrapCompleted bool
	ExistingFiles      map[string]string
}

type Plan struct {
	Bootstrap BootstrapRequest `json:"bootstrap"`
	Files     []FilePlan       `json:"files"`
	Recovery  []RecoveryStep   `json:"recovery"`
	Ready     bool             `json:"ready"`
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

const recoveryStatePath = ".baley-init-state.json"

type recoveryState struct {
	Version                int              `json:"version"`
	Server                 string           `json:"server"`
	RecordRepositoryID     string           `json:"recordRepositoryId,omitempty"`
	Bootstrap              BootstrapRequest `json:"bootstrap"`
	BootstrapRequestSHA256 string           `json:"bootstrapRequestSha256"`
}

func Build(input Input) (Plan, error) {
	if recovered, ok := parseRecoveryState(input.ExistingFiles[recoveryStatePath]); ok {
		input = hydrateInput(input, recovered)
	}
	clientProjectID := strings.ToLower(strings.TrimSpace(input.ClientProjectID))
	if !validUUID(clientProjectID) || strings.TrimSpace(input.WorkspaceName) == "" || strings.TrimSpace(input.RepositoryName) == "" || strings.TrimSpace(input.ExecutedByActorID) == "" {
		return Plan{}, ErrInvalidInput
	}
	repository, err := domain.NewRepository(domain.Repository{
		ID: input.RepositoryID, WorkspaceID: input.WorkspaceID, Name: input.RepositoryName, RemoteURL: input.RemoteURL,
		IsRecordRepository: true, TaskRecordsRoot: input.TaskRecordsRoot,
	})
	if err != nil {
		return Plan{}, ErrInvalidInput
	}
	configValue, configErr := config.Normalize(config.BaleyConfig{
		Version: config.CurrentVersion, Server: input.Server, WorkspaceID: input.WorkspaceID, RepositoryID: input.RepositoryID,
		RecordRepositoryID: input.RecordRepositoryID, TaskRecords: config.TaskRecordsConfig{Root: input.TaskRecordsRoot},
	})
	if configErr != nil || configValue.TaskRecords.Root != repository.TaskRecordsRoot {
		return Plan{}, ErrInvalidInput
	}
	if configValue.RecordRepositoryID != "" && configValue.RecordRepositoryID != configValue.RepositoryID || !safeInitRoot(configValue.TaskRecords.Root) {
		return Plan{}, ErrInvalidInput
	}
	configData, configErr := config.Marshal(configValue)
	if configErr != nil {
		return Plan{}, ErrInvalidInput
	}
	if err := validateExistingPaths(input.ExistingFiles); err != nil {
		return Plan{}, err
	}
	existingByFold := make(map[string]string, len(input.ExistingFiles))
	for relativePath := range input.ExistingFiles {
		existingByFold[strings.ToLower(relativePath)] = relativePath
	}
	plan := Plan{Bootstrap: BootstrapRequest{
		Name: "project.bootstrap",
		Arguments: BootstrapArguments{
			ClientProjectID: clientProjectID, WorkspaceID: configValue.WorkspaceID, WorkspaceName: strings.TrimSpace(input.WorkspaceName),
			RepositoryID: configValue.RepositoryID, RepositoryName: repository.Name, RemoteURL: repository.RemoteURL,
			TaskRecordsRoot: configValue.TaskRecords.Root,
		},
		Envelope: BootstrapEnvelope{
			IdempotencyKey: clientProjectID, InitiatedByActorID: strings.TrimSpace(input.InitiatedByActorID),
			ExecutedByActorID: strings.TrimSpace(input.ExecutedByActorID),
		},
	}}
	desired := desiredFiles(configValue.TaskRecords.Root, string(configData))
	desired[recoveryStatePath] = recoveryContent(plan.Bootstrap, configValue.Server, configValue.RecordRepositoryID)
	for _, relativePath := range sortedKeys(desired) {
		desiredContent := desired[relativePath]
		existingContent, exists := input.ExistingFiles[relativePath]
		file := FilePlan{RelativePath: relativePath, DesiredContent: desiredContent, DesiredSHA: contentSHA(desiredContent)}
		caseCollision := false
		if !exists {
			if existingPath, collision := existingByFold[strings.ToLower(relativePath)]; collision {
				existingContent, exists = input.ExistingFiles[existingPath], true
				file.ConflictingPath, caseCollision = existingPath, existingPath != relativePath
			}
		}
		switch {
		case caseCollision:
			file.Action = FileConflict
			file.ExpectedExistingSHA = contentSHA(existingContent)
		case !exists:
			file.Action = FileCreate
		case existingContent == desiredContent:
			file.Action = FileKeep
			file.ExpectedExistingSHA = contentSHA(existingContent)
		case relativePath == ".rgignore" && !strings.ContainsRune(existingContent, 0):
			merged, changed := appendIgnoreRule(existingContent, configValue.TaskRecords.Root+"/**")
			file.ExpectedExistingSHA = contentSHA(existingContent)
			if changed {
				file.Action, file.DesiredContent, file.DesiredSHA = FileMerge, merged, contentSHA(merged)
			} else {
				file.Action, file.DesiredContent, file.DesiredSHA = FileKeep, existingContent, contentSHA(existingContent)
			}
		default:
			file.Action = FileConflict
			file.ExpectedExistingSHA = contentSHA(existingContent)
		}
		plan.Files = append(plan.Files, file)
	}

	pending, conflicts := []string{}, []string{}
	identityPending := false
	for _, file := range plan.Files {
		switch file.Action {
		case FileCreate, FileMerge:
			if file.RelativePath == recoveryStatePath {
				identityPending = true
			} else {
				pending = append(pending, file.RelativePath)
			}
		case FileConflict:
			conflicts = append(conflicts, file.RelativePath)
		}
	}
	if len(conflicts) != 0 {
		plan.Recovery = append(plan.Recovery, RecoveryStep{Action: "resolve_without_overwrite", Paths: conflicts})
		plan.Recovery = append(plan.Recovery, RecoveryStep{Action: "verify_manifest_and_server_binding"})
		plan.Ready = false
		return plan, nil
	}
	if identityPending {
		plan.Recovery = append(plan.Recovery, RecoveryStep{Action: "persist_retry_identity", Paths: []string{recoveryStatePath}})
	}
	if !input.BootstrapCompleted {
		plan.Recovery = append(plan.Recovery, RecoveryStep{Action: "execute_or_retry_project_bootstrap"})
	}
	if len(pending) != 0 {
		plan.Recovery = append(plan.Recovery, RecoveryStep{Action: "apply_non_conflicting_files", Paths: pending})
	}
	plan.Recovery = append(plan.Recovery, RecoveryStep{Action: "verify_manifest_and_server_binding"})
	plan.Ready = len(conflicts) == 0
	return plan, nil
}

type plannerError string

func (e plannerError) Error() string { return string(e) }

const ErrInvalidInput plannerError = "invalid project init input"

func desiredFiles(root, configContent string) map[string]string {
	files := map[string]string{
		"baley.yaml":                 configContent,
		".rgignore":                  root + "/**\n",
		path.Join(root, "README.md"): "# Baley Task Records\n\nTask Record 원문과 version history는 Git이 보관합니다. 일반 검색에서는 제외하고 현재 Task에 연결된 정확한 경로만 읽습니다.\n",
	}
	templates := []struct{ name, title, recordType string }{
		{"detailed-plan.md", "상세계획", "detailed-plan"},
		{"handoff.md", "Handoff", "handoff"},
		{"independent-agent-review.md", "독립 Agent 리뷰", "independent-agent-review"},
		{"review-response.md", "리뷰 반영", "review-response"},
		{"completion-report.md", "완료보고", "completion-report"},
	}
	for _, template := range templates {
		files[path.Join(root, "_templates", template.name)] = fmt.Sprintf("---\nbaley_record: 1\nrecord_id: \"{{record_id}}\"\ntask_id: {{task_id}}\nrecord_type: %s\nrun_id: {{run_id}}\ncreated_at: \"{{created_at}}\"\ncreated_by: \"{{created_by}}\"\nsupersedes: null\n---\n\n# %s\n", template.recordType, template.title)
	}
	return files
}

func appendIgnoreRule(existing, rule string) (string, bool) {
	for _, line := range strings.Split(strings.ReplaceAll(existing, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == rule {
			return existing, false
		}
	}
	merged := existing
	if merged != "" && !strings.HasSuffix(merged, "\n") {
		merged += "\n"
	}
	return merged + rule + "\n", true
}

func validateExistingPaths(files map[string]string) error {
	seenFolded := make(map[string]bool, len(files))
	for relativePath := range files {
		if relativePath == "" || strings.TrimSpace(relativePath) != relativePath || strings.ContainsRune(relativePath, 0) || strings.Contains(relativePath, `\`) || strings.HasPrefix(relativePath, "/") || path.Clean(relativePath) != relativePath || relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, "../") {
			return ErrInvalidInput
		}
		folded := strings.ToLower(relativePath)
		if seenFolded[folded] {
			return ErrInvalidInput
		}
		seenFolded[folded] = true
	}
	return nil
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func contentSHA(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validUUID(value string) bool {
	return uuidPattern.MatchString(value)
}

func safeInitRoot(root string) bool {
	for _, character := range root {
		if unicode.IsControl(character) || strings.ContainsRune(`*?[]!#`, character) {
			return false
		}
	}
	for index, segment := range strings.Split(root, "/") {
		reserved := strings.EqualFold(segment, "baley.yaml") || strings.EqualFold(segment, ".rgignore") || strings.EqualFold(segment, recoveryStatePath)
		if strings.EqualFold(segment, ".git") || index == 0 && reserved {
			return false
		}
	}
	return true
}

func parseRecoveryState(content string) (recoveryState, bool) {
	var state recoveryState
	if json.Unmarshal([]byte(content), &state) != nil || state.Version != 1 || !validUUID(state.Bootstrap.Arguments.ClientProjectID) || state.Server == "" || state.BootstrapRequestSHA256 == "" {
		return recoveryState{}, false
	}
	if state.BootstrapRequestSHA256 != recoveryFingerprint(state.Bootstrap, state.Server, state.RecordRepositoryID) {
		return recoveryState{}, false
	}
	return state, true
}

func recoveryContent(request BootstrapRequest, server, recordRepositoryID string) string {
	state := recoveryState{
		Version: 1, Server: server, RecordRepositoryID: recordRepositoryID, Bootstrap: request,
		BootstrapRequestSHA256: recoveryFingerprint(request, server, recordRepositoryID),
	}
	encoded, _ := json.MarshalIndent(state, "", "  ")
	return string(encoded) + "\n"
}

func recoveryFingerprint(request BootstrapRequest, server, recordRepositoryID string) string {
	payload, _ := json.Marshal(struct {
		Server             string           `json:"server"`
		RecordRepositoryID string           `json:"recordRepositoryId,omitempty"`
		Bootstrap          BootstrapRequest `json:"bootstrap"`
	}{server, recordRepositoryID, request})
	return contentSHA(string(payload))
}

func hydrateInput(input Input, state recoveryState) Input {
	arguments, envelope := state.Bootstrap.Arguments, state.Bootstrap.Envelope
	fill := func(target *string, recovered string) {
		if strings.TrimSpace(*target) == "" {
			*target = recovered
		}
	}
	fill(&input.ClientProjectID, arguments.ClientProjectID)
	fill(&input.Server, state.Server)
	fill(&input.WorkspaceID, arguments.WorkspaceID)
	fill(&input.WorkspaceName, arguments.WorkspaceName)
	fill(&input.RepositoryID, arguments.RepositoryID)
	fill(&input.RepositoryName, arguments.RepositoryName)
	fill(&input.RemoteURL, arguments.RemoteURL)
	fill(&input.RecordRepositoryID, state.RecordRepositoryID)
	fill(&input.TaskRecordsRoot, arguments.TaskRecordsRoot)
	fill(&input.InitiatedByActorID, envelope.InitiatedByActorID)
	fill(&input.ExecutedByActorID, envelope.ExecutedByActorID)
	return input
}
