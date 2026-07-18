package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const CurrentVersion = 1

type ErrorCode string

const (
	CodeInvalidConfig      ErrorCode = "invalid_config"
	CodeUnsupportedVersion ErrorCode = "unsupported_version"
	CodeSensitiveField     ErrorCode = "sensitive_field"
	CodeInvalidPath        ErrorCode = "invalid_path"
)

type ConfigError struct {
	Code  ErrorCode
	Field string
}

func (e *ConfigError) Error() string {
	if e.Field == "" {
		return "Baley config error: " + string(e.Code)
	}
	return fmt.Sprintf("Baley config error: %s (%s)", e.Code, e.Field)
}

type TaskRecordsConfig struct {
	Root string `yaml:"root"`
}

type BaleyConfig struct {
	Version            int               `yaml:"version"`
	Server             string            `yaml:"server"`
	WorkspaceID        string            `yaml:"workspace_id"`
	RepositoryID       string            `yaml:"repository_id"`
	RecordRepositoryID string            `yaml:"record_repository_id,omitempty"`
	TaskRecords        TaskRecordsConfig `yaml:"task_records"`
}

var (
	uuidPattern   = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	schemePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

func Parse(data []byte) (BaleyConfig, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return BaleyConfig{}, &ConfigError{Code: CodeInvalidConfig}
	}
	if forbiddenYAMLFeature(&document) {
		return BaleyConfig{}, &ConfigError{Code: CodeInvalidConfig}
	}
	if field := sensitiveField(&document); field != "" {
		return BaleyConfig{}, &ConfigError{Code: CodeSensitiveField, Field: field}
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var result BaleyConfig
	if err := decoder.Decode(&result); err != nil {
		return BaleyConfig{}, &ConfigError{Code: CodeInvalidConfig}
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return BaleyConfig{}, &ConfigError{Code: CodeInvalidConfig}
	}
	return Normalize(result)
}

func Normalize(value BaleyConfig) (BaleyConfig, error) {
	if value.Version != CurrentVersion {
		return BaleyConfig{}, &ConfigError{Code: CodeUnsupportedVersion, Field: "version"}
	}
	server, err := normalizeServer(value.Server)
	if err != nil {
		return BaleyConfig{}, err
	}
	workspaceID := strings.ToLower(strings.TrimSpace(value.WorkspaceID))
	repositoryID := strings.ToLower(strings.TrimSpace(value.RepositoryID))
	recordRepositoryID := strings.ToLower(strings.TrimSpace(value.RecordRepositoryID))
	if !uuidPattern.MatchString(workspaceID) {
		return BaleyConfig{}, &ConfigError{Code: CodeInvalidConfig, Field: "workspace_id"}
	}
	if !uuidPattern.MatchString(repositoryID) {
		return BaleyConfig{}, &ConfigError{Code: CodeInvalidConfig, Field: "repository_id"}
	}
	if recordRepositoryID != "" && !uuidPattern.MatchString(recordRepositoryID) {
		return BaleyConfig{}, &ConfigError{Code: CodeInvalidConfig, Field: "record_repository_id"}
	}
	root, err := normalizeRelative(value.TaskRecords.Root)
	if err != nil {
		return BaleyConfig{}, err
	}
	value.Server = server
	value.WorkspaceID = workspaceID
	value.RepositoryID = repositoryID
	value.RecordRepositoryID = recordRepositoryID
	value.TaskRecords.Root = root
	return value, nil
}

func Marshal(value BaleyConfig) ([]byte, error) {
	normalized, err := Normalize(value)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(normalized)
}

func (c BaleyConfig) EffectiveRecordRepositoryID() string {
	if c.RecordRepositoryID != "" {
		return c.RecordRepositoryID
	}
	return c.RepositoryID
}

func normalizeServer(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "" && parsed.Path != "/" {
		return "", &ConfigError{Code: CodeInvalidConfig, Field: "server"}
	}
	if port := parsed.Port(); port != "" {
		value, conversionErr := strconv.Atoi(port)
		if conversionErr != nil || value < 1 || value > 65535 {
			return "", &ConfigError{Code: CodeInvalidConfig, Field: "server"}
		}
	}
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")
	return parsed.String(), nil
}

func normalizeRelative(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.ContainsRune(trimmed, 0) || strings.Contains(trimmed, `\`) || strings.HasPrefix(trimmed, "/") || schemePattern.MatchString(trimmed) {
		return "", &ConfigError{Code: CodeInvalidPath, Field: "task_records.root"}
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == ".." {
			return "", &ConfigError{Code: CodeInvalidPath, Field: "task_records.root"}
		}
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", &ConfigError{Code: CodeInvalidPath, Field: "task_records.root"}
	}
	return cleaned, nil
}

func sensitiveField(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	if node.Kind == yaml.MappingNode {
		for index := 0; index+1 < len(node.Content); index += 2 {
			key := node.Content[index]
			if isSensitiveName(key.Value) {
				return key.Value
			}
			if nested := sensitiveField(node.Content[index+1]); nested != "" {
				return nested
			}
		}
		return ""
	}
	for _, child := range node.Content {
		if nested := sensitiveField(child); nested != "" {
			return nested
		}
	}
	return ""
}

func forbiddenYAMLFeature(node *yaml.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == yaml.AliasNode || node.Anchor != "" {
		return true
	}
	if node.Kind == yaml.MappingNode {
		for index := 0; index+1 < len(node.Content); index += 2 {
			if node.Content[index].Value == "<<" || forbiddenYAMLFeature(node.Content[index]) || forbiddenYAMLFeature(node.Content[index+1]) {
				return true
			}
		}
		return false
	}
	for _, child := range node.Content {
		if forbiddenYAMLFeature(child) {
			return true
		}
	}
	return false
}

func isSensitiveName(value string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(value), "-", "_"), ".", "_"))
	for _, fragment := range []string{"token", "secret", "password", "credential", "api_key", "private_key", "authorization"} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}
