//go:build windows

package application

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

const createNewProcessGroup = 0x00000200

func configureProcessTree(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewProcessGroup}
	command.Cancel = func() error {
		if command.Process != nil {
			_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(command.Process.Pid)).Run()
			_ = command.Process.Kill()
		}
		return nil
	}
	command.WaitDelay = 2 * time.Second
}
