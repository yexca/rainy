package util

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// ErrUnsafePath is returned (wrapped) by SafeJoin, ToRel and EnsureWithinRoot for paths
// that could escape the root.
var ErrUnsafePath = errors.New("unsafe path")

// SafeJoin joins a library-relative path (forward or back slashes) to root and returns the
// absolute OS path. It rejects NUL bytes, absolute or volume paths ("/x", `C:\x`, "C:x",
// `\\server\share`), any ".." component and — on Windows — components the OS would
// normalise into something else (only dots/spaces) or alternate data streams ("a:b").
// The result is root itself or lexically inside it (see EnsureWithinRoot for symlinks).
// rel "" or "." yields root.
func SafeJoin(root, rel string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("%w: empty root", ErrUnsafePath)
	}
	if strings.ContainsRune(rel, 0) || strings.ContainsRune(root, 0) {
		return "", fmt.Errorf("%w: NUL byte", ErrUnsafePath)
	}
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, `\`) || filepath.IsAbs(rel) ||
		filepath.VolumeName(rel) != "" || hasDriveLetter(rel) {
		return "", fmt.Errorf("%w: absolute path %q", ErrUnsafePath, rel)
	}
	parts := strings.FieldsFunc(rel, func(r rune) bool { return r == '/' || r == '\\' })
	elems := make([]string, 0, len(parts)+1)
	root = filepath.Clean(root)
	elems = append(elems, root)
	for _, p := range parts {
		if p == "." {
			continue
		}
		if p == ".." {
			return "", fmt.Errorf("%w: parent reference in %q", ErrUnsafePath, rel)
		}
		// Windows: reject components the OS rewrites (only dots/spaces), alternate data
		// streams ("a:b") and reserved device names ("NUL", "CON", "COM1", "aux.mp3", …),
		// which would address a device instead of a file under root.
		if runtime.GOOS == "windows" && (strings.Trim(p, ". ") == "" || strings.ContainsRune(p, ':') || !filepath.IsLocal(p)) {
			return "", fmt.Errorf("%w: invalid component %q", ErrUnsafePath, p)
		}
		elems = append(elems, p)
	}
	joined := filepath.Join(elems...)
	if !within(root, joined) {
		return "", fmt.Errorf("%w: %q escapes root", ErrUnsafePath, rel)
	}
	return joined, nil
}

func hasDriveLetter(s string) bool {
	return len(s) >= 2 && s[1] == ':' && ((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z'))
}

// within reports whether p is root or lexically inside it (both already cleaned).
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// IsWithin reports whether abs is root or lexically inside root.
func IsWithin(root, abs string) bool { return within(filepath.Clean(root), filepath.Clean(abs)) }

// EnsureWithinRoot resolves symlinks of root and of abs (or of its deepest existing
// ancestor when abs does not exist yet) and returns an ErrUnsafePath error if abs resolves
// outside root. Use it before writing or deleting files obtained via SafeJoin when
// symlinked directories inside a library are a concern.
func EnsureWithinRoot(root, abs string) error {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolving root: %w", err)
	}
	p := filepath.Clean(abs)
	var suffix []string
	for {
		real, err := filepath.EvalSymlinks(p)
		if err == nil {
			full := filepath.Join(append([]string{real}, suffix...)...)
			if !within(filepath.Clean(realRoot), full) {
				return fmt.Errorf("%w: %q resolves outside the library", ErrUnsafePath, abs)
			}
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("resolving %q: %w", p, err)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return fmt.Errorf("%w: %q has no existing ancestor", ErrUnsafePath, abs)
		}
		suffix = append([]string{filepath.Base(p)}, suffix...)
		p = parent
	}
}

// ToRel converts an absolute path under root to a library-relative forward-slash path
// ("" for root itself).
func ToRel(root, abs string) (string, error) {
	root, abs = filepath.Clean(root), filepath.Clean(abs)
	if !within(root, abs) {
		return "", fmt.Errorf("%w: %q is not inside %q", ErrUnsafePath, abs, root)
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return "", err
	}
	if rel == "." {
		return "", nil
	}
	return filepath.ToSlash(rel), nil
}

// PathParts splits a library-relative path into its directory ("" for the root), file
// name and lower-case suffix without dot: "A/B/01 x.FLAC" → "A/B", "01 x.FLAC", "flac".
// Back slashes are treated as separators.
func PathParts(rel string) (dir, filename, suffix string) {
	rel = strings.Trim(strings.ReplaceAll(rel, `\`, "/"), "/")
	dir, filename = path.Split(rel)
	dir = strings.TrimSuffix(dir, "/")
	suffix = strings.ToLower(strings.TrimPrefix(path.Ext(filename), "."))
	return dir, filename, suffix
}

var systemNames = map[string]bool{
	"@eadir":                    true,
	"#recycle":                  true,
	"#snapshot":                 true,
	"@recycle":                  true,
	"$recycle.bin":              true,
	"lost+found":                true,
	"system volume information": true,
}

// IsHiddenOrSystem reports whether a file or directory name should be skipped when walking
// a library: dot-files (".*", incl. ".@__thumb"), Synology "@eaDir", "#recycle",
// "#snapshot", QNAP "@Recycle", "$RECYCLE.BIN", "lost+found", "System Volume Information"
// (case-insensitive).
func IsHiddenOrSystem(name string) bool {
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	return systemNames[strings.ToLower(name)]
}
