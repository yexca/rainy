//go:build !windows

package ytdlp

import (
	"os/exec"
	"syscall"
)

// setProcessGroup starts the command in its own process group and makes cancellation
// signal the whole group: the PyInstaller build runs Python as a child process, and yt-dlp
// starts ffmpeg, so killing only the direct child would leave them running.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
}

// killProcessGroup kills whatever is left of the group after the command exited.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
