// Package buildinfo exposes version information injected at build time via -ldflags:
//
//	go build -ldflags "-X rainy/internal/buildinfo.Version=1.2.3 \
//	  -X rainy/internal/buildinfo.Commit=abc123 -X rainy/internal/buildinfo.BuildDate=2026-01-01T00:00:00Z"
//
// When the values are not injected, Commit and BuildDate fall back to the VCS information
// recorded by the Go toolchain (if any).
package buildinfo

import (
	"runtime"
	"runtime/debug"
)

var (
	// Version is the release version (e.g. "1.2.3"); "dev" for local builds.
	Version = "dev"
	// Commit is the VCS revision the binary was built from.
	Commit = ""
	// BuildDate is the RFC3339 build timestamp.
	BuildDate = ""
)

func init() {
	if Commit != "" && BuildDate != "" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if Commit == "" {
				Commit = s.Value
				if len(Commit) > 12 {
					Commit = Commit[:12]
				}
			}
		case "vcs.time":
			if BuildDate == "" {
				BuildDate = s.Value
			}
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if dirty && Commit != "" {
		Commit += "-dirty"
	}
}

// GoVersion returns the Go runtime version the binary was built with.
func GoVersion() string { return runtime.Version() }

// String returns a one-line human readable description, e.g. "rainy 1.2.3 (abc123, 2026-01-01)".
func String() string {
	s := "rainy " + Version
	extra := ""
	if Commit != "" {
		extra = Commit
	}
	if BuildDate != "" {
		if extra != "" {
			extra += ", "
		}
		extra += BuildDate
	}
	if extra != "" {
		s += " (" + extra + ")"
	}
	return s + " " + runtime.Version() + " " + runtime.GOOS + "/" + runtime.GOARCH
}
