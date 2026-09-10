//go:build windows

package application

import (
	"fmt"
	"os/exec"
	"strings"
)

func processAlive(pid int) bool {
	output, err := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/FO", "CSV", "/NH").Output()
	return err == nil && strings.Contains(string(output), fmt.Sprintf("\"%d\"", pid))
}
