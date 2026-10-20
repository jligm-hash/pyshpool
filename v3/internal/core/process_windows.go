//go:build windows

package core

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}

// ConfigureProcess is a no-op on Windows; there is no process-group flag.
func ConfigureProcess(cmd *exec.Cmd) { configureProcess(cmd) }

// KillProcess terminates the direct child on Windows.
func KillProcess(cmd *exec.Cmd) { killProcess(cmd) }

func killProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Kill()
}
