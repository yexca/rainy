package manage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// errExists is returned (wrapped in store.ErrConflict) when a move target already exists.
var errExists = fmt.Errorf("%w: target file already exists", store.ErrConflict)

// isCrossDevice reports whether err is a rename failure across file systems.
func isCrossDevice(err error) bool {
	if errors.Is(err, syscall.EXDEV) {
		return true
	}
	var errno syscall.Errno
	// ERROR_NOT_SAME_DEVICE (17) on Windows.
	return runtime.GOOS == "windows" && errors.As(err, &errno) && errno == 17
}

// exists reports whether a file or directory exists at p (without following a final
// symlink).
func exists(p string) bool {
	_, err := os.Lstat(p)
	return err == nil
}

// moveFile moves src to dst, never overwriting an existing dst. Parent directories of
// dst must exist. Across file systems it falls back to copy + remove.
func moveFile(src, dst string) error {
	if exists(dst) && !sameFile(src, dst) {
		return errExists
	}
	err := os.Rename(src, dst)
	if err == nil || !isCrossDevice(err) {
		return err
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		_ = os.Remove(dst) // keep exactly one copy
		return err
	}
	return nil
}

// sameFile reports whether a and b name the same existing file (e.g. a case-only rename
// on a case-insensitive file system).
func sameFile(a, b string) bool {
	fa, err1 := os.Stat(a)
	fb, err2 := os.Stat(b)
	return err1 == nil && err2 == nil && os.SameFile(fa, fb)
}

// renameCaseOnly renames src to dst when both differ only in letter case: via a
// temporary name so that case-insensitive file systems apply the new spelling.
func renameCaseOnly(src, dst string) error {
	if exists(dst) && !sameFile(src, dst) {
		return errExists
	}
	tmp := src + ".rainy-" + randSuffix()
	if err := os.Rename(src, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Rename(tmp, src)
		return err
	}
	return nil
}

func randSuffix() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// copyFile copies src to a new file dst (O_EXCL), preserving mode and modification time.
func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fi.Mode().Perm()|0o200)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return errExists
		}
		return err
	}
	defer func() {
		if err != nil {
			_ = out.Close()
			_ = os.Remove(dst)
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	if err = out.Sync(); err != nil {
		return err
	}
	if err = out.Close(); err != nil {
		return err
	}
	_ = os.Chtimes(dst, fi.ModTime(), fi.ModTime())
	return nil
}

// junkFiles are OS metadata files that do not keep a directory "in use" (plus AppleDouble
// "._*" files); junkDirs are NAS thumbnail caches that may be deleted with their content.
var (
	junkFiles = map[string]bool{".ds_store": true, "thumbs.db": true, "ehthumbs.db": true, "desktop.ini": true}
	junkDirs  = map[string]bool{"@eadir": true, ".@__thumb": true}
)

// isJunk reports whether a directory entry is disposable metadata. Only regular files
// (never symlinks) and the known thumbnail-cache directories qualify, so a folder that
// merely has a junk-looking name is never deleted with its content.
func isJunk(e fs.DirEntry) bool {
	name := strings.ToLower(e.Name())
	switch {
	case e.IsDir():
		return junkDirs[name]
	case !e.Type().IsRegular():
		return false
	}
	return junkFiles[name] || strings.HasPrefix(name, "._")
}

// removeEmptyDirs removes dir and then its parents while they contain nothing but
// OS/NAS metadata (".DS_Store", "Thumbs.db", "@eaDir", …). It never removes stop (the
// library or trash root) or anything outside it.
func removeEmptyDirs(stop, dir string) {
	stop = filepath.Clean(stop)
	for dir = filepath.Clean(dir); dir != stop && util.IsWithin(stop, dir); dir = filepath.Dir(dir) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !isJunk(e) {
				return
			}
		}
		for _, e := range entries {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
		if err := os.Remove(dir); err != nil {
			return
		}
	}
}

// DirWritable reports whether the server can create files in dir (probed by creating
// and removing a temporary file).
func DirWritable(dir string) bool {
	f, err := os.CreateTemp(dir, ".rainy-probe-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// fileWritable reports whether path can be opened for writing (nothing is modified).
func fileWritable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// checkWritable opens path for writing (without modifying it) so that permission and
// read-only-mount problems surface as *ReadonlyError before a tag library reports a
// generic "could not save" error.
func checkWritable(rel, path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fsErr(rel, err)
	}
	return f.Close()
}

// findSidecar returns the path of the .lrc sidecar of an audio file (matched
// case-insensitively: "01 Song.lrc", "01 Song.LRC"), or "".
func findSidecar(audioAbs string) string {
	dir := filepath.Dir(audioAbs)
	want := stem(filepath.Base(audioAbs)) + ".lrc"
	if p := filepath.Join(dir, want); exists(p) {
		return p
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), want) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// ownSidecar returns the .lrc sidecar that moves with an audio file (rename, trash), or
// "". A sidecar shared with another audio file of the same stem ("Song.mp3" and
// "Song.flac" next to "Song.lrc") stays where it is, with the file that remains.
func ownSidecar(audioAbs string) string {
	lrc := findSidecar(audioAbs)
	if lrc == "" {
		return ""
	}
	entries, err := os.ReadDir(filepath.Dir(audioAbs))
	if err != nil {
		return ""
	}
	self, want := filepath.Base(audioAbs), stem(filepath.Base(audioAbs))
	for _, e := range entries {
		if n := e.Name(); n != self && !e.IsDir() && tags.IsAudioFile(n) && strings.EqualFold(stem(n), want) {
			return ""
		}
	}
	return lrc
}

// stemTaken reports whether a directory entry other than p itself has p's stem
// (case-insensitively), e.g. "Song.flac" or "Song.lrc" for "Song.mp3".
func stemTaken(p string) bool {
	entries, err := os.ReadDir(filepath.Dir(p))
	if err != nil {
		return false
	}
	self, want := filepath.Base(p), stem(filepath.Base(p))
	for _, e := range entries {
		if n := e.Name(); !strings.EqualFold(n, self) && strings.EqualFold(stem(n), want) {
			return true
		}
	}
	return false
}

// isRegularFile reports whether p is a regular file (a symlink is not, even if it points
// to one: sidecars are only read or written when they cannot lead outside the library).
func isRegularFile(p string) bool {
	fi, err := os.Lstat(p)
	return err == nil && fi.Mode().IsRegular()
}

// writeFileAtomic replaces (or creates) the regular file p with data via a temporary file
// in the same directory, so a crash never leaves a truncated file and an existing symlink
// at p is replaced instead of followed.
func writeFileAtomic(p string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(p), ".rainy-tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(name)
		return err
	}
	_ = os.Chmod(name, 0o644)
	if err := os.Rename(name, p); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

// stem returns name without its extension.
func stem(name string) string { return strings.TrimSuffix(name, filepath.Ext(name)) }

// nameIndex answers "is this name taken in that directory?" case-insensitively,
// including names reserved by earlier items of the same batch.
type nameIndex struct {
	dirs map[string]map[string]bool // abs dir → lower-case names
}

func newNameIndex() *nameIndex { return &nameIndex{dirs: map[string]map[string]bool{}} }

func (x *nameIndex) dir(absDir string) map[string]bool {
	absDir = filepath.Clean(absDir)
	m, ok := x.dirs[absDir]
	if ok {
		return m
	}
	m = map[string]bool{}
	if entries, err := os.ReadDir(absDir); err == nil {
		for _, e := range entries {
			m[strings.ToLower(e.Name())] = true
		}
	}
	x.dirs[absDir] = m
	return m
}

// taken reports whether abs (compared case-insensitively) exists or was reserved.
func (x *nameIndex) taken(abs string) bool {
	return x.dir(filepath.Dir(abs))[strings.ToLower(filepath.Base(abs))]
}

func (x *nameIndex) reserve(abs string) {
	x.dir(filepath.Dir(abs))[strings.ToLower(filepath.Base(abs))] = true
}

// unique returns abs, or the first "<stem> (n)<ext>" variant that is not taken, and
// reserves it. Stems are shortened when needed to keep the name within maxComponent.
func (x *nameIndex) unique(abs string) string {
	dir, name := filepath.Split(abs)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	cand := abs
	for n := 1; x.taken(cand); n++ {
		suffix := " (" + strconv.Itoa(n) + ")"
		cand = filepath.Join(dir, truncateBytes(base, maxComponent-len(suffix)-len(ext))+suffix+ext)
	}
	x.reserve(cand)
	return cand
}
