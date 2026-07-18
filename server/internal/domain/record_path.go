package domain

import (
	"path"
	"regexp"
	"strings"
)

var pathSchemePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

func NormalizeRecordPath(root, relativePath string) (string, error) {
	normalizedRoot, err := normalizeRepositoryRelative(root)
	if err != nil {
		return "", err
	}
	normalizedPath, err := normalizeRepositoryRelative(relativePath)
	if err != nil {
		return "", err
	}
	if normalizedPath == normalizedRoot || !strings.HasPrefix(normalizedPath, normalizedRoot+"/") {
		return "", &Violation{Code: CodeInvalidRecordPath}
	}
	return normalizedPath, nil
}

func normalizeRepositoryRelative(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.ContainsRune(trimmed, 0) || strings.Contains(trimmed, `\`) || strings.HasPrefix(trimmed, "/") || pathSchemePattern.MatchString(trimmed) {
		return "", &Violation{Code: CodeInvalidRecordPath}
	}
	for _, segment := range strings.Split(trimmed, "/") {
		if segment == ".." {
			return "", &Violation{Code: CodeInvalidRecordPath}
		}
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", &Violation{Code: CodeInvalidRecordPath}
	}
	return cleaned, nil
}
