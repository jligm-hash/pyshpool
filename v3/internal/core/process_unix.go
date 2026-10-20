//go:build !windows

package core

import (
	"os/exec"
	"syscall"
)

// configureProcess puts each task in its own process group so a timeout can
// terminate the whole pipeline, not only the shell that started it.
func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// ConfigureProcess puts a command in its own process group. It is exported
// for packages that need the same timeout/cancellation behavior as the runner.
func ConfigureProcess(cmd *exec.Cmd) { configureProcess(cmd) }

// KillProcess terminates the process group rooted at cmd.
func KillProcess(cmd *exec.Cmd) { killProcess(cmd) }

// killProcess terminates the process group rooted at cmd.
func killProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	// Negative pid means "the process group". Fall back to killing the
	// direct child if the group has already disappeared.
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}
