package scanner

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"

	"rainy/internal/artwork"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// offlineDirMinTracks: a directory that held at least this many tracks and is now completely
// empty is taken for an offline mount point (see libScan.guardEmptyDirs).
const offlineDirMinTracks = 20

// IgnoreFile marks a directory (and its subtree) as excluded from scanning.
const IgnoreFile = ".rainyignore"

// fileInfo is one audio file found on disk.
type fileInfo struct {
	rel    string // library-relative, forward slashes
	abs    string
	size   int64
	mtime  int64 // unix ms
	hasLrc bool
}

// walkResult is the outcome of walking a (sub)tree.
type walkResult struct {
	files  []fileInfo
	failed []string // relative dirs that could not be read (their tracks are left alone)
	empty  []string // relative dirs (below the start) without any entry at all
}

// underFailed reports whether rel lies inside a directory that could not be read.
func (w *walkResult) underFailed(rel string) bool {
	for _, d := range w.failed {
		if d == "" || rel == d || strings.HasPrefix(rel, d+"/") {
			return true
		}
	}
	return false
}

// dirCache remembers the image files of directories (absolute path → image names; nil for
// a directory that could not be listed) so cover detection after a walk needs no extra I/O.
type dirCache struct{ images map[string][]string }

func newDirCache() *dirCache { return &dirCache{images: map[string][]string{}} }

// set records the image names of a directory that was listed successfully.
func (c *dirCache) set(absDir string, images []string) {
	if images == nil {
		images = []string{} // nil marks an unreadable directory
	}
	c.images[filepath.Clean(absDir)] = images
}

// find returns the absolute path of the first image in absDir matching patterns. ok is
// false when absDir cannot be listed (share offline, permissions): the caller must then
// keep what it knows instead of concluding that there is no image.
func (c *dirCache) find(absDir string, patterns []string) (match string, ok bool) {
	absDir = filepath.Clean(absDir)
	names, cached := c.images[absDir]
	if !cached {
		var err error
		if names, err = listImages(absDir); err != nil {
			names = nil
		} else if names == nil {
			names = []string{}
		}
		c.images[absDir] = names
	}
	if names == nil {
		return "", false
	}
	if n := artwork.MatchFolderImage(names, patterns); n != "" {
		return filepath.Join(absDir, n), true
	}
	return "", true
}

// listImages returns the names of the image files (regular or symlinked) in dir.
func listImages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, e := range entries {
		if util.IsImageSuffix(path.Ext(e.Name())) && isFile(dir, e) {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

func isFile(dir string, e fs.DirEntry) bool {
	if e.Type().IsRegular() {
		return true
	}
	if e.Type()&fs.ModeSymlink != 0 {
		fi, err := os.Stat(filepath.Join(dir, e.Name()))
		return err == nil && fi.Mode().IsRegular()
	}
	return false
}

// walker walks a library subtree.
type walker struct {
	root    string
	cache   *dirCache
	visited map[string]bool // real paths of visited dirs (symlink loop protection)
	res     walkResult
	onDir   func(files int) // progress callback (audio files found in a dir)
}

// walk walks root/rel. A missing or unreadable start directory is an error.
func (w *walker) walk(ctx context.Context, rel string) error {
	start, err := util.SafeJoin(w.root, rel)
	if err != nil {
		return err
	}
	w.visited = map[string]bool{}
	return w.dir(ctx, start, strings.Trim(rel, "/"), true)
}

func (w *walker) dir(ctx context.Context, abs, rel string, isStart bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		if w.visited[real] {
			return nil
		}
		w.visited[real] = true
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		if isStart {
			return err
		}
		slog.Warn("scanner: cannot read directory; its tracks are left unchanged", "dir", abs, "err", err)
		w.res.failed = append(w.res.failed, rel)
		return nil
	}
	if len(entries) == 0 && rel != "" {
		w.res.empty = append(w.res.empty, rel)
	}
	lrc := map[string]bool{}
	for _, e := range entries {
		if e.Name() == IgnoreFile {
			return nil // excluded subtree
		}
		if ext := path.Ext(e.Name()); ext == ".lrc" || ext == ".LRC" {
			lrc[e.Name()] = true
		}
	}

	var images []string
	var subdirs []fs.DirEntry
	found := 0
	for _, e := range entries {
		name := e.Name()
		if util.IsHiddenOrSystem(name) {
			continue
		}
		if !addressable(name) {
			slog.Warn(`scanner: skipping a name containing a backslash (library paths treat / and \ as separators)`,
				"path", filepath.Join(abs, name))
			continue
		}
		typ := e.Type()
		// Symlinks, and on Windows directory junctions (reported as irregular files), are
		// followed.
		if typ&(fs.ModeSymlink|fs.ModeIrregular) != 0 {
			fi, err := os.Stat(filepath.Join(abs, name))
			if err != nil {
				// A dangling link is indistinguishable from a link to a share that is
				// offline: leave whatever the database has under it alone.
				slog.Warn("scanner: cannot follow link; its tracks are left unchanged", "path", filepath.Join(abs, name), "err", err)
				w.res.failed = append(w.res.failed, joinRel(rel, name))
				continue
			}
			typ = fi.Mode().Type()
		}
		switch {
		case typ.IsDir():
			subdirs = append(subdirs, e)
		case typ.IsRegular():
			if util.IsImageSuffix(path.Ext(name)) {
				images = append(images, name)
			}
			if !tags.IsAudioFile(name) {
				continue
			}
			fabs := filepath.Join(abs, name)
			fi, err := os.Stat(fabs)
			if err != nil {
				slog.Warn("scanner: cannot stat file", "path", fabs, "err", err)
				w.res.failed = append(w.res.failed, joinRel(rel, name))
				continue
			}
			base := strings.TrimSuffix(name, path.Ext(name))
			w.res.files = append(w.res.files, fileInfo{
				rel:    joinRel(rel, name),
				abs:    fabs,
				size:   fi.Size(),
				mtime:  fi.ModTime().UnixMilli(),
				hasLrc: lrc[base+".lrc"] || lrc[base+".LRC"],
			})
			found++
		}
	}
	if w.cache != nil {
		w.cache.set(abs, images)
	}
	if w.onDir != nil {
		w.onDir(found)
	}
	for _, e := range subdirs {
		if err := w.dir(ctx, filepath.Join(abs, e.Name()), joinRel(rel, e.Name()), false); err != nil {
			return err
		}
	}
	return nil
}

// addressable reports whether a directory entry name can be part of a library-relative
// path: util.SafeJoin and util.PathParts treat `\` as a separator on every OS, so a Linux
// file named `AC\DC.mp3` could never be opened again through its stored path.
func addressable(name string) bool { return !strings.ContainsRune(name, '\\') }

func joinRel(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// checkRoot verifies that a library root exists and is a readable directory.
func checkRoot(root string) error {
	fi, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: %s does not exist (is the share mounted?)", ErrLibraryUnavailable, root)
		}
		return fmt.Errorf("%w: %v", ErrLibraryUnavailable, err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrLibraryUnavailable, root)
	}
	f, err := os.Open(root)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrLibraryUnavailable, err)
	}
	_ = f.Close()
	return nil
}
