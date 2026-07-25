//go:build windows

package agent

import (
	"os"
	"os/exec"
)

func setProcessGroup(cmd *exec.Cmd) {}

func terminateProcessGroup(proc *os.Process) {
	if proc != nil {
		_ = proc.Kill()
	}
}
