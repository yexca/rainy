package manage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"rainy/internal/artwork"
	"rainy/internal/lyrics"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

const (
	maxSubfolderPeeks  = 128
	maxFolderImageSize = 32 << 20
	maxFolderTextSize  = 256 << 10
)

// FolderEntry is a sub-directory in a folder listing.
type FolderEntry struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	TrackCount    int    `json:"trackCount"`
	HasSubfolders bool   `json:"hasSubfolders"`
}

// FileEntry is a file in a folder listing.
type FileEntry struct {
	Name    string       `json:"name"`
	Path    string       `json:"path"`
	Size    int64        `json:"size"`
	Mtime   int64        `json:"mtime"`
	IsAudio bool         `json:"isAudio"`
	IsImage bool         `json:"isImage"`
	TrackID *string      `json:"trackId"`
	Track   *model.Track `json:"track"`
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

// FolderTextPreview is the response of GET /api/manage/folders/text.
type FolderTextPreview struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	Mtime     int64  `json:"mtime"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
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
func (s *Service) ListFolder(ctx context.Context, libraryID int64, dir, userID string) (*FolderListing, error) {
	libraryID, err := s.defaultLibrary(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	dir = cleanDir(dir)
	abs, err := s.resolveDir(ctx, libraryID, dir)
	if err != nil {
		return nil, err
	}
	lib, err := s.libs().get(ctx, libraryID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fsErr(dir, err)
	}
	tracksByPath, err := s.folderTracks(ctx, libraryID, dir, userID)
	if err != nil {
		return nil, err
	}
	trackCounts, indexedSub, err := s.subfolderStats(ctx, libraryID, dir)
	if err != nil {
		return nil, err
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
	peeks := 0
	for _, e := range entries {
		name := e.Name()
		if util.IsHiddenOrSystem(name) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		entryAbs := filepath.Join(abs, name)
		if info.Mode()&fs.ModeSymlink != 0 {
			// Follow symlinks only when they stay inside the library.
			if info, err = os.Stat(entryAbs); err != nil {
				continue
			}
			if util.EnsureWithinRoot(lib.Path, entryAbs) != nil {
				continue
			}
		}
		rel := join(name)
		if info.IsDir() {
			hasSub := indexedSub[name]
			if !hasSub {
				if peeks < maxSubfolderPeeks {
					peeks++
					hasSub = hasVisibleSubdir(lib.Path, entryAbs)
				} else {
					hasSub = true
				}
			}
			out.Folders = append(out.Folders, FolderEntry{
				Name:          name,
				Path:          rel,
				TrackCount:    trackCounts[name],
				HasSubfolders: hasSub,
			})
			continue
		}
		fe := FileEntry{
			Name: name, Path: rel, Size: info.Size(), Mtime: info.ModTime().UnixMilli(),
			IsAudio: tags.IsAudioFile(name),
			IsImage: isFolderImageSuffix(strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))),
		}
		if t, ok := tracksByPath[rel]; ok {
			id := t.ID
			fe.TrackID = &id
			fe.Track = t
		}
		out.Files = append(out.Files, fe)
	}
	sort.SliceStable(out.Folders, func(i, j int) bool { return naturalLess(out.Folders[i].Name, out.Folders[j].Name) })
	sort.SliceStable(out.Files, func(i, j int) bool { return naturalLess(out.Files[i].Name, out.Files[j].Name) })
	return out, nil
}

func (s *Service) folderTracks(ctx context.Context, libraryID int64, dir, userID string) (map[string]*model.Track, error) {
	var ids []string
	if err := s.st.DB().R.SelectContext(ctx, &ids,
		`SELECT id FROM tracks WHERE library_id = ? AND dir = ?`, libraryID, dir); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	out := make(map[string]*model.Track, len(ids))
	const batch = 500
	for start := 0; start < len(ids); start += batch {
		end := min(start+batch, len(ids))
		tracks, _, err := s.st.ListTracks(ctx, store.TrackQuery{
			UserID:  userID,
			IDs:     ids[start:end],
			Missing: "include",
		})
		if err != nil {
			return nil, err
		}
		for i := range tracks {
			out[tracks[i].Path] = &tracks[i]
		}
	}
	return out, nil
}

func (s *Service) subfolderStats(ctx context.Context, libraryID int64, dir string) (map[string]int, map[string]bool, error) {
	type dirCount struct {
		Dir   string `db:"dir"`
		Count int    `db:"n"`
	}
	var rows []dirCount
	var err error
	if dir == "" {
		err = s.st.DB().R.SelectContext(ctx, &rows,
			`SELECT dir, COUNT(*) AS n FROM tracks WHERE library_id = ? AND missing = 0 AND dir != '' GROUP BY dir`,
			libraryID)
	} else {
		err = s.st.DB().R.SelectContext(ctx, &rows,
			`SELECT dir, COUNT(*) AS n FROM tracks WHERE library_id = ? AND missing = 0 AND dir LIKE ? ESCAPE '\' GROUP BY dir`,
			libraryID, util.EscapeLike(dir)+"/%")
	}
	if err != nil {
		return nil, nil, err
	}
	counts := make(map[string]int, len(rows))
	hasSub := make(map[string]bool, len(rows))
	prefix := ""
	if dir != "" {
		prefix = dir + "/"
	}
	for _, r := range rows {
		rest := strings.TrimPrefix(r.Dir, prefix)
		if rest == "" {
			continue
		}
		first, sub, ok := strings.Cut(rest, "/")
		counts[first] += r.Count
		if ok && sub != "" {
			hasSub[first] = true
		}
	}
	return counts, hasSub, nil
}

func hasVisibleSubdir(root, abs string) bool {
	entries, err := os.ReadDir(abs)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if util.IsHiddenOrSystem(name) {
			continue
		}
		if e.IsDir() {
			return true
		}
		if e.Type()&fs.ModeSymlink != 0 {
			sub := filepath.Join(abs, name)
			if fi, err := os.Stat(sub); err == nil && fi.IsDir() && util.EnsureWithinRoot(root, sub) == nil {
				return true
			}
		}
	}
	return false
}

func (s *Service) resolveFolderFile(ctx context.Context, libraryID int64, rel string) (int64, string, fs.FileInfo, error) {
	libraryID, err := s.defaultLibrary(ctx, libraryID)
	if err != nil {
		return 0, "", nil, err
	}
	rel = cleanDir(rel)
	if rel == "" {
		return 0, "", nil, fmt.Errorf("%w: file path is required", store.ErrInvalid)
	}
	if err := checkDirParts(rel); err != nil {
		return 0, "", nil, err
	}
	lib, err := s.libs().get(ctx, libraryID)
	if err != nil {
		return 0, "", nil, err
	}
	abs, err := libPath(lib.Path, rel)
	if err != nil {
		return 0, "", nil, err
	}
	fi, err := os.Stat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return 0, "", nil, fmt.Errorf("%w: file does not exist", store.ErrNotFound)
	case err != nil:
		return 0, "", nil, fsErr(rel, err)
	case fi.IsDir():
		return 0, "", nil, fmt.Errorf("%w: path is a folder", store.ErrInvalid)
	}
	return libraryID, abs, fi, nil
}

func isFolderImageSuffix(ext string) bool {
	return util.IsImageSuffix(ext) || ext == "gif" || ext == "bmp"
}

// FolderImage reads a raster image file inside a library folder for manager preview.
func (s *Service) FolderImage(ctx context.Context, libraryID int64, rel string) ([]byte, string, int64, int64, error) {
	rel = cleanDir(rel)
	_, abs, fi, err := s.resolveFolderFile(ctx, libraryID, rel)
	if err != nil {
		return nil, "", 0, 0, err
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fi.Name()), "."))
	if !isFolderImageSuffix(ext) {
		return nil, "", 0, 0, fmt.Errorf("%w: unsupported image file", store.ErrInvalid)
	}
	if fi.Size() <= 0 {
		return nil, "", 0, 0, store.ErrNotFound
	}
	if fi.Size() > maxFolderImageSize {
		return nil, "", 0, 0, fmt.Errorf("%w: image exceeds maximum preview size", store.ErrInvalid)
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, "", 0, 0, fsErr(rel, err)
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxFolderImageSize+1))
	if err != nil {
		return nil, "", 0, 0, fsErr(rel, err)
	}
	if int64(len(data)) > maxFolderImageSize {
		return nil, "", 0, 0, fmt.Errorf("%w: image exceeds maximum preview size", store.ErrInvalid)
	}
	mt, _, _ := mime.ParseMediaType(http.DetectContentType(data))
	switch mt {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp":
	default:
		return nil, "", 0, 0, fmt.Errorf("%w: unsupported image content", store.ErrInvalid)
	}
	return data, mt, int64(len(data)), fi.ModTime().UnixMilli(), nil
}

func isPreviewTextSuffix(ext string) bool {
	switch ext {
	case "lrc", "txt", "cue", "log", "md", "nfo", "m3u", "m3u8":
		return true
	}
	return false
}

// FolderText reads a bounded text or lyrics file inside a library folder for manager preview.
func (s *Service) FolderText(ctx context.Context, libraryID int64, rel string) (*FolderTextPreview, error) {
	rel = cleanDir(rel)
	_, abs, fi, err := s.resolveFolderFile(ctx, libraryID, rel)
	if err != nil {
		return nil, err
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fi.Name()), "."))
	if !isPreviewTextSuffix(ext) {
		return nil, fmt.Errorf("%w: unsupported text file", store.ErrInvalid)
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, fsErr(rel, err)
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, maxFolderTextSize+1))
	if err != nil {
		return nil, fsErr(rel, err)
	}
	truncated := int64(len(raw)) > maxFolderTextSize
	if truncated {
		raw = raw[:maxFolderTextSize]
	}
	hasUTF16 := bytes.HasPrefix(raw, []byte{0xFF, 0xFE}) || bytes.HasPrefix(raw, []byte{0xFE, 0xFF})
	if !hasUTF16 && bytes.IndexByte(raw, 0) >= 0 {
		return nil, fmt.Errorf("%w: binary file cannot be previewed as text", store.ErrInvalid)
	}
	content := lyrics.DecodeText(raw)
	if truncated {
		content = strings.TrimRight(content, "\uFFFD")
	}
	return &FolderTextPreview{
		Name:      fi.Name(),
		Path:      rel,
		Size:      fi.Size(),
		Mtime:     fi.ModTime().UnixMilli(),
		Content:   content,
		Truncated: truncated,
	}, nil
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
