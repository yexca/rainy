package manage

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"rainy/internal/artwork"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// FolderEntry is a sub-directory in a folder listing.
type FolderEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// FileEntry is a file in a folder listing.
type FileEntry struct {
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	Size    int64   `json:"size"`
	Mtime   int64   `json:"mtime"`
	IsAudio bool    `json:"isAudio"`
	IsImage bool    `json:"isImage"`
	TrackID *string `json:"trackId"`
}

// FolderListing is the response of GET /api/manage/folders.
type FolderListing struct {
	LibraryID int64         `json:"libraryId"`
	Dir       string        `json:"dir"`
	Parent    *string       `json:"parent"`
	Writable  bool          `json:"writable"`
	Folders   []FolderEntry `json:"folders"`
	Files     []FileEntry   `json:"files"`
}

// cleanDir normalizes a library-relative directory ("" = root).
func cleanDir(dir string) string {
	return strings.Trim(strings.ReplaceAll(strings.TrimSpace(dir), `\`, "/"), "/")
}

// checkDirParts rejects client-supplied directories with hidden or NAS system components
// (".rainy", "@eaDir", "#recycle", …): the scanner ignores them, and they may hold data
// that is none of the library's business.
func checkDirParts(dir string) error {
	for _, c := range strings.Split(dir, "/") {
		if util.IsHiddenOrSystem(strings.TrimSpace(c)) {
			return fmt.Errorf("%w: hidden or system folder %q", store.ErrInvalid, c)
		}
	}
	return nil
}

// resolveDir validates a library directory and returns its absolute path.
func (s *Service) resolveDir(ctx context.Context, libraryID int64, dir string) (string, error) {
	if err := checkDirParts(dir); err != nil {
		return "", err
	}
	lib, err := s.libs().get(ctx, libraryID)
	if err != nil {
		return "", err
	}
	abs, err := libPath(lib.Path, dir)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("%w: folder %q does not exist", store.ErrNotFound, dir)
	case err != nil:
		return "", err
	case !fi.IsDir():
		return "", fmt.Errorf("%w: %q is not a folder", store.ErrInvalid, dir)
	}
	return abs, nil
}

// defaultLibrary returns libraryID, or the first library when it is 0.
func (s *Service) defaultLibrary(ctx context.Context, libraryID int64) (int64, error) {
	if libraryID > 0 {
		return libraryID, nil
	}
	libs, err := s.st.ListLibraries(ctx)
	if err != nil {
		return 0, err
	}
	if len(libs) == 0 {
		return 0, fmt.Errorf("%w: no library configured", store.ErrNotFound)
	}
	return libs[0].ID, nil
}

// ListFolder lists one directory of a library (hidden / NAS system entries skipped).
func (s *Service) ListFolder(ctx context.Context, libraryID int64, dir string) (*FolderListing, error) {
	libraryID, err := s.defaultLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	dir = cleanDir(dir)
	abs, err := s.resolveDir(ctx, libraryID, dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fsErr(dir, err)
	}
	var rows []idPath
	if err := s.st.DB().R.SelectContext(ctx, &rows,
		`SELECT id, path FROM tracks WHERE library_id = ? AND dir = ?`, libraryID, dir); err != nil {
		return nil, err
	}
	ids := make(map[string]string, len(rows))
	for _, r := range rows {
		ids[r.Path] = r.ID
	}
	out := &FolderListing{LibraryID: libraryID, Dir: dir, Writable: DirWritable(abs),
		Folders: []FolderEntry{}, Files: []FileEntry{}}
	if dir != "" {
		parent, _, _ := util.PathParts(dir)
		out.Parent = &parent
	}
	join := func(name string) string {
		if dir == "" {
			return name
		}
		return dir + "/" + name
	}
	for _, e := range entries {
		name := e.Name()
		if util.IsHiddenOrSystem(name) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			// Follow symlinks only when they stay inside the library.
			if info, err = os.Stat(filepath.Join(abs, name)); err != nil {
				continue
			}
			if lib, lerr := s.libs().get(ctx, libraryID); lerr != nil || util.EnsureWithinRoot(lib.Path, filepath.Join(abs, name)) != nil {
				continue
			}
		}
		rel := join(name)
		if info.IsDir() {
			out.Folders = append(out.Folders, FolderEntry{Name: name, Path: rel})
			continue
		}
		fe := FileEntry{
			Name: name, Path: rel, Size: info.Size(), Mtime: info.ModTime().UnixMilli(),
			IsAudio: tags.IsAudioFile(name),
			IsImage: util.IsImageSuffix(strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))),
		}
		if id, ok := ids[rel]; ok {
			fe.TrackID = &id
		}
		out.Files = append(out.Files, fe)
	}
	sort.SliceStable(out.Folders, func(i, j int) bool { return naturalLess(out.Folders[i].Name, out.Folders[j].Name) })
	sort.SliceStable(out.Files, func(i, j int) bool { return naturalLess(out.Files[i].Name, out.Files[j].Name) })
	return out, nil
}

// naturalLess compares case-insensitively with digit runs compared numerically
// ("2 x" < "10 x").
func naturalLess(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for a != "" && b != "" {
		da, db := digitPrefix(a), digitPrefix(b)
		if da != "" && db != "" {
			na, nb := strings.TrimLeft(da, "0"), strings.TrimLeft(db, "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			a, b = a[len(da):], b[len(db):]
			continue
		}
		ra, rb := []rune(a)[0], []rune(b)[0]
		if ra != rb {
			return ra < rb
		}
		a, b = a[len(string(ra)):], b[len(string(rb)):]
	}
	return len(a) < len(b)
}

func digitPrefix(s string) string {
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	return s[:i]
}

// RescanFolder validates the directory and quick-scans it in the background.
func (s *Service) RescanFolder(ctx context.Context, libraryID int64, dir string) error {
	libraryID, err := s.defaultLibrary(ctx, libraryID)
	if err != nil {
		return err
	}
	dir = cleanDir(dir)
	if _, err := s.resolveDir(ctx, libraryID, dir); err != nil {
		return err
	}
	go func() {
		// A folder rescan is a scan: hold the library lock so it never observes a
		// half-finished file operation (RescanDir does not take it itself).
		unlock := s.sc.LockLibrary()
		defer unlock()
		if err := s.sc.RescanDir(context.Background(), libraryID, dir); err != nil {
			slog.Warn("manage: folder rescan failed", "library", libraryID, "dir", dir, "err", err)
			return
		}
		s.publish("rescan")
	}()
	return nil
}

// refreshDirCovers re-resolves the folder image of the albums with tracks in the given
// library directories (after folder images were moved or restored) and busts their
// cover caches.
func (s *Service) refreshDirCovers(ctx context.Context, libraryID int64, dirs []string) {
	lib, err := s.libs().get(ctx, libraryID)
	if err != nil {
		return
	}
	patterns := s.settings(ctx).CoverArtPatterns()
	var touched []string
	for _, dir := range dirs {
		abs, err := libPath(lib.Path, dir)
		if err != nil {
			continue
		}
		var albumIDs []string
		if err := s.st.DB().R.SelectContext(ctx, &albumIDs,
			`SELECT DISTINCT album_id FROM tracks WHERE library_id = ? AND dir = ? AND missing = 0`, libraryID, dir); err != nil {
			slog.Warn("manage: listing albums of dir", "dir", dir, "err", err)
			continue
		}
		img := artwork.FindFolderImage(abs, patterns)
		for _, id := range albumIDs {
			al, err := s.st.GetAlbum(ctx, id, "")
			if err != nil {
				continue
			}
			switch {
			case img != "":
				err = s.st.SetAlbumCoverPath(ctx, id, img)
			case al.CoverPath != "" && !exists(al.CoverPath):
				err = s.st.SetAlbumCoverPath(ctx, id, "")
			}
			if err != nil {
				slog.Warn("manage: updating album cover path", "album", id, "err", err)
			}
			touched = append(touched, id)
		}
	}
	if len(touched) > 0 {
		if err := s.st.TouchAlbums(ctx, dedupe(touched)); err != nil {
			slog.Warn("manage: touching albums", "err", err)
		}
	}
}
