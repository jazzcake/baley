package runtimeconfig

import (
	"fmt"
	"os"
	"strings"
)

const maxSecretFileBytes = 64 * 1024

// Load returns a configuration value from NAME or NAME_FILE. Exactly one source
// may be configured. File values may end with newline characters so they work
// with Docker secrets and systemd credentials.
func Load(name string) (value string, configured bool, err error) {
	direct := os.Getenv(name)
	filePath := strings.TrimSpace(os.Getenv(name + "_FILE"))
	if direct != "" && filePath != "" {
		return "", false, fmt.Errorf("%s and %s_FILE cannot both be configured", name, name)
	}
	if direct != "" {
		if strings.IndexByte(direct, 0) >= 0 {
			return "", false, fmt.Errorf("%s contains a NUL byte", name)
		}
		return direct, true, nil
	}
	if filePath == "" {
		return "", false, nil
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return "", false, fmt.Errorf("read %s_FILE: %w", name, err)
	}
	if !info.Mode().IsRegular() || info.Size() > maxSecretFileBytes {
		return "", false, fmt.Errorf("%s_FILE must reference a regular file no larger than %d bytes", name, maxSecretFileBytes)
	}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return "", false, fmt.Errorf("read %s_FILE: %w", name, err)
	}
	value = strings.TrimRight(string(raw), "\r\n")
	if value == "" {
		return "", false, fmt.Errorf("%s_FILE is empty", name)
	}
	if strings.IndexByte(value, 0) >= 0 {
		return "", false, fmt.Errorf("%s_FILE contains a NUL byte", name)
	}
	return value, true, nil
}
