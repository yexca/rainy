package manage

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"syscall"
	"testing"

	"rainy/internal/store"
	"rainy/internal/util"
)

// Item, staged-item and job errors reach clients, so errorText must never carry a host path
// or a raw filesystem error, while messages composed for users stay as they are.
func TestErrorTextHidesHostPaths(t *testing.T) {
	const host = "/srv/rainy"
	cases := map[string]struct {
		err  error
		want string
	}{
		"move": {
			&os.LinkError{Op: "rename", Old: host + "/data/staging/x/audio.m4a", New: host + "/music/A/01 T.m4a", Err: syscall.ENOSPC},
			"the server's disk is full",
		},
		"wrapped stat": {
			fmt.Errorf("holding the file for review: %w", &fs.PathError{Op: "stat", Path: host + "/data/tmp/x", Err: fs.ErrNotExist}),
			"the file no longer exists",
		},
		"other fs error": {
			&fs.PathError{Op: "open", Path: host + "/music/x", Err: syscall.EIO},
			"a file operation failed on the server",
		},
		"symlink escape": {
			fmt.Errorf("%w: %q resolves outside the library", util.ErrUnsafePath, host+"/elsewhere"),
			"invalid or unsafe path",
		},
		"user message": {
			fmt.Errorf("%w: the library folder is not accessible", store.ErrConflict),
			"conflict: the library folder is not accessible",
		},
	}
	for name, c := range cases {
		got := errorText(c.err)
		if got != c.want || strings.Contains(got, host) {
			t.Errorf("%s: errorText = %q, want %q", name, got, c.want)
		}
	}
	readonly := fsErr("A/01 T.m4a", &fs.PathError{Op: "open", Path: host + "/music/A/01 T.m4a", Err: fs.ErrPermission})
	if got := errorText(readonly); !errors.As(readonly, new(*ReadonlyError)) || strings.Contains(got, host) || !strings.HasSuffix(got, "(A/01 T.m4a)") {
		t.Errorf("read-only: errorText = %q", got)
	}
}
