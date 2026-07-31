package projectinit

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// LoadExistingFiles reads only the deterministic project-init manifest paths.
// Symlinks and non-regular files are rejected so planning cannot follow data
// outside the selected project root.
func LoadExistingFiles(projectRoot, taskRecordsRoot string) (map[string]string, error) {
	root, err := verifiedRoot(projectRoot)
	if err != nil || !safeInitRoot(taskRecordsRoot) {
		return nil, ErrInvalidInput
	}
	result := map[string]string{}
	for _, relativePath := range desiredPaths(taskRecordsRoot) {
		target, err := safeTarget(root, relativePath)
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(target)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return nil, ErrInvalidInput
		}
		content, err := os.ReadFile(target)
		if err != nil {
			return nil, err
		}
		result[relativePath] = string(content)
	}
	return result, nil
}

// Apply writes only create and merge actions after rechecking every precondition.
// Conflicts are never written, and a stale existing hash aborts the whole apply
// before the first file mutation.
func Apply(projectRoot string, plan Plan) error {
	if !plan.Ready {
		return ErrInvalidInput
	}
	root, err := verifiedRoot(projectRoot)
	if err != nil {
		return err
	}
	type pendingWrite struct {
		file   FilePlan
		target string
	}
	pending := []pendingWrite{}
	for _, file := range plan.Files {
		target, targetErr := safeTarget(root, file.RelativePath)
		if targetErr != nil || contentSHA(file.DesiredContent) != file.DesiredSHA {
			return ErrInvalidInput
		}
		switch file.Action {
		case FileKeep:
			info, statErr := os.Lstat(target)
			if statErr != nil || !info.Mode().IsRegular() {
				return ErrInvalidInput
			}
			content, readErr := os.ReadFile(target)
			if readErr != nil || contentSHA(string(content)) != file.DesiredSHA {
				return ErrInvalidInput
			}
		case FileCreate:
			if _, statErr := os.Lstat(target); !errors.Is(statErr, fs.ErrNotExist) {
				return ErrInvalidInput
			}
			pending = append(pending, pendingWrite{file, target})
		case FileMerge:
			info, statErr := os.Lstat(target)
			if statErr != nil || !info.Mode().IsRegular() {
				return ErrInvalidInput
			}
			content, readErr := os.ReadFile(target)
			if readErr != nil || contentSHA(string(content)) != file.ExpectedExistingSHA {
				return ErrInvalidInput
			}
			pending = append(pending, pendingWrite{file, target})
		default:
			return ErrInvalidInput
		}
	}
	for _, write := range pending {
		if err = ensureSafeParents(root, filepath.Dir(write.target)); err != nil {
			return err
		}
		if write.file.Action == FileCreate {
			handle, openErr := os.OpenFile(write.target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if openErr != nil {
				return openErr
			}
			_, writeErr := handle.WriteString(write.file.DesiredContent)
			closeErr := handle.Close()
			if writeErr != nil {
				return writeErr
			}
			if closeErr != nil {
				return closeErr
			}
			continue
		}
		handle, openErr := os.OpenFile(write.target, os.O_RDWR, 0)
		if openErr != nil {
			return openErr
		}
		content, readErr := os.ReadFile(write.target)
		if readErr != nil || contentSHA(string(content)) != write.file.ExpectedExistingSHA {
			handle.Close()
			return ErrInvalidInput
		}
		if truncateErr := handle.Truncate(0); truncateErr != nil {
			handle.Close()
			return truncateErr
		}
		if _, seekErr := handle.Seek(0, 0); seekErr != nil {
			handle.Close()
			return seekErr
		}
		_, writeErr := handle.WriteString(write.file.DesiredContent)
		closeErr := handle.Close()
		if writeErr != nil {
			return writeErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func desiredPaths(taskRecordsRoot string) []string {
	paths := sortedKeys(desiredFiles(taskRecordsRoot, ""))
	return append([]string{recoveryStatePath}, paths...)
}

func verifiedRoot(projectRoot string) (string, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrInvalidInput
	}
	return filepath.Clean(root), nil
}

func safeTarget(root, relativePath string) (string, error) {
	if err := validateExistingPaths(map[string]string{relativePath: ""}); err != nil {
		return "", err
	}
	target := filepath.Clean(filepath.Join(root, filepath.FromSlash(relativePath)))
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrInvalidInput
	}
	return target, nil
}

func ensureSafeParents(root, parent string) error {
	relative, err := filepath.Rel(root, parent)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ErrInvalidInput
	}
	current := root
	if relative == "." {
		return nil
	}
	for _, segment := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, segment)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, fs.ErrNotExist) {
			if mkdirErr := os.Mkdir(current, 0o700); mkdirErr != nil {
				return mkdirErr
			}
			continue
		}
		if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrInvalidInput
		}
	}
	return nil
}
