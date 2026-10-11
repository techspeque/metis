//go:build !windows

package launch

import (
	"os/exec"
	"syscall"
	"time"
)

// setProcessGroup starts the agent in its own process group, so a timeout
// terminates every process it spawned, not just the CLI itself.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	// SIGTERM first, then SIGKILL for whatever in the group ignored it.
	pgid := cmd.Process.Pid
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		return cmd.Process.Kill()
	}
	go func() {
		time.Sleep(5 * time.Second)
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}()
	return nil
}
