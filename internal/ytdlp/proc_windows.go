//go:build windows

package ytdlp

import "os/exec"

// setProcessGroup keeps the default behaviour on Windows (development only): cancellation
// kills the direct child.
func setProcessGroup(*exec.Cmd) {}

func killProcessGroup(*exec.Cmd) {}
