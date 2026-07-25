//go:build !windows

package agent

import (
	"os"
	"os/exec"
	"syscall"
	"time"
)

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateProcessGroup(proc *os.Process) {
	if proc == nil {
		return
	}
	pgid, err := syscall.Getpgid(proc.Pid)
	if err != nil {
		_ = proc.Kill()
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	time.Sleep(100 * time.Millisecond)
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}
