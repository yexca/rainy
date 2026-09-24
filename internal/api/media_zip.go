package api

import (
	"archive/zip"
	"context"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"rainy/internal/lyrics"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

// maxZipNameBytes bounds each path component inside album zips and download file names.
const maxZipNameBytes = 200

// sanitizeFileName makes s safe as a file or directory name on common file systems:
// characters illegal on Windows (<>:"/\|?*) and control characters become "_", leading and
// trailing dots/spaces are trimmed and the result is at most maxZipNameBytes bytes.
// An empty result becomes "_".
func sanitizeFileName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == utf8.RuneError || unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			b.WriteByte('_')
			continue
		}
		b.WriteRune(r)
	}
	out := strings.Trim(b.String(), " .")
	if len(out) > maxZipNameBytes {
		ext := path.Ext(out)
		if len(ext) > 16 {
			ext = ""
		}
		cut := maxZipNameBytes - len(ext)
		for cut > 0 && !utf8.RuneStart(out[cut]) {
			cut--
		}
		out = strings.TrimRight(out[:cut], " .") + ext
	}
	if out == "" {
		return "_"
	}
	return out
}

// commonDir returns the longest common directory prefix of library-relative dirs.
func commonDir(dirs []string) string {
	if len(dirs) == 0 {
		return ""
	}
	common := strings.Split(dirs[0], "/")
	for _, d := range dirs[1:] {
		parts := strings.Split(d, "/")
		n := 0
		for n < len(common) && n < len(parts) && common[n] == parts[n] {
			n++
		}
		common = common[:n]
	}
	return strings.Join(common, "/")
}

// zipEntry is one file to add to an album zip.
type zipEntry struct {
	abs     string
	name    string // path inside the zip, "/" separated
	modTime time.Time
}

// zipNames hands out unique entry names ("x.flac", "x (2).flac", …), case-insensitively.
type zipNames map[string]bool

func (z zipNames) unique(name string) string {
	candidate := name
	ext := path.Ext(name)
	for i := 2; z[strings.ToLower(candidate)]; i++ {
		candidate = strings.TrimSuffix(name, ext) + " (" + strconv.Itoa(i) + ")" + ext
	}
	z[strings.ToLower(candidate)] = true
	return candidate
}

// zipRelName converts a library-relative path below base into a sanitised zip path.
func zipRelName(base, rel string) string {
	rel = strings.TrimPrefix(strings.TrimPrefix(rel, base), "/")
	parts := strings.Split(rel, "/")
	for i, p := range parts {
		parts[i] = sanitizeFileName(p)
	}
	return strings.Join(parts, "/")
}

// albumZipEntries resolves the files of an album zip: the tracks (in album order), their
// sidecar .lrc files and the album's folder image, below a top-level folder named top.
// Files that are not on disk are skipped.
func (a *API) albumZipEntries(ctx context.Context, album *model.Album, tracks []model.Track, top string) ([]zipEntry, error) {
	libs := map[int64]*model.Library{}
	dirs := make([]string, 0, len(tracks))
	for _, t := range tracks {
		dirs = append(dirs, t.Dir)
	}
	base := commonDir(dirs)
	names := zipNames{}
	entries := make([]zipEntry, 0, len(tracks)+1)
	add := func(abs, rel string) {
		fi, err := os.Stat(abs)
		if err != nil || !fi.Mode().IsRegular() {
			return
		}
		entries = append(entries, zipEntry{abs: abs, name: names.unique(top + "/" + zipRelName(base, rel)), modTime: fi.ModTime()})
	}
	for _, t := range tracks {
		lib, ok := libs[t.LibraryID]
		if !ok {
			l, err := a.app.Store.GetLibrary(ctx, t.LibraryID)
			if err != nil {
				return nil, err
			}
			lib, libs[t.LibraryID] = l, l
		}
		abs, err := util.SafeJoin(lib.Path, t.Path)
		if err != nil {
			slog.Warn("album zip: unsafe track path", "track", t.ID, "err", err)
			continue
		}
		before := len(entries)
		add(abs, t.Path)
		if len(entries) == before {
			slog.Warn("album zip: file missing, skipped", "track", t.ID, "path", t.Path)
			continue
		}
		if t.HasLrc {
			// Same lookup order as lyrics.Load: ".lrc", then ".LRC" (case-sensitive systems).
			relBase, absBase := strings.TrimSuffix(t.Path, path.Ext(t.Path)), strings.TrimSuffix(lyrics.LrcPath(abs), ".lrc")
			for _, ext := range []string{".lrc", ".LRC"} {
				before := len(entries)
				if add(absBase+ext, relBase+ext); len(entries) > before {
					break
				}
			}
		}
	}
	if len(entries) == 0 {
		return nil, notFound("none of the album's files are available")
	}
	if e, ok := folderImageEntry(album, libs[album.LibraryID], names, top); ok {
		entries = append(entries, e)
	}
	return entries, nil
}

// folderImageEntry returns the zip entry of the album's folder image, when it exists and
// lives inside the album's library folder.
func folderImageEntry(album *model.Album, lib *model.Library, names zipNames, top string) (zipEntry, bool) {
	if album.CoverPath == "" || lib == nil || !util.IsWithin(lib.Path, album.CoverPath) {
		return zipEntry{}, false
	}
	fi, err := os.Stat(album.CoverPath)
	if err != nil || !fi.Mode().IsRegular() {
		return zipEntry{}, false
	}
	name := names.unique(top + "/" + sanitizeFileName(filepath.Base(album.CoverPath)))
	return zipEntry{abs: album.CoverPath, name: name, modTime: fi.ModTime()}, true
}

// mediaDownloadAlbum streams a zip (store method, no compression) of an album's files.
func (a *API) mediaDownloadAlbum(w http.ResponseWriter, r *http.Request) {
	if err := a.checkDownload(r); err != nil {
		writeErr(w, r, err)
		return
	}
	ctx, uid := r.Context(), userFrom(r).ID
	album, err := a.app.Store.GetAlbum(ctx, chi.URLParam(r, "albumId"), uid)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	tracks, _, err := a.app.Store.ListTracks(ctx, store.TrackQuery{UserID: uid, AlbumID: album.ID, Sort: "track"})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	top := sanitizeFileName(album.AlbumArtist + " - " + album.Name)
	entries, err := a.albumZipEntries(ctx, album, tracks, top)
	if err != nil {
		writeErr(w, r, err)
		return
	}

	h := w.Header()
	h.Set("Content-Type", "application/zip")
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": top + ".zip"}))
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	zw := zip.NewWriter(w)
	for _, e := range entries {
		if ctx.Err() != nil {
			return
		}
		f, err := os.Open(e.abs)
		if err != nil {
			// Removed since the entries were resolved: skip it rather than abort.
			slog.Warn("album zip: file vanished, skipped", "album", album.ID, "entry", e.name, "err", err)
			continue
		}
		err = writeZipEntry(zw, f, e)
		_ = f.Close()
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("album zip: writing entry failed", "album", album.ID, "entry", e.name, "err", err)
			}
			return
		}
	}
	if err := zw.Close(); err != nil && ctx.Err() == nil {
		slog.Warn("album zip: finishing archive", "album", album.ID, "err", err)
	}
}

// writeZipEntry adds the contents of f as entry e.
func writeZipEntry(zw *zip.Writer, f io.Reader, e zipEntry) error {
	ew, err := zw.CreateHeader(&zip.FileHeader{Name: e.name, Method: zip.Store, Modified: e.modTime})
	if err != nil {
		return err
	}
	_, err = io.Copy(ew, f)
	return err
}
