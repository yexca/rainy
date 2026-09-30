package manage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"rainy/internal/config"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

const (
	// MaxUploadFileSize is the per-file upload limit.
	MaxUploadFileSize int64 = 2 << 30
	maxUploadFiles          = 1000
	maxUploadField          = 4 << 10
)

// staged is an uploaded file waiting in the tmp dir.
type staged struct {
	name  string // client-supplied relative name (sanitized, "/"-separated)
	tmp   string // abs path in cfg.TmpDir()
	size  int64
	title string // downloads: the title reported by the site (edit log)
}

// uploadForm is the parsed multipart request.
type uploadForm struct {
	libraryID int64
	dir       string
	organize  bool
	files     []staged
}

// Upload streams the multipart parts "files" (repeated), "libraryId", "dir" and
// "organize" to the staging directory, then places each file under dir (keeping the
// client's relative sub-directories) or, with organize, where settings.RenamePattern puts
// it (see place). New files are scanned immediately.
func (s *Service) Upload(ctx context.Context, u *model.User, mr *multipart.Reader) (*BatchResult, error) {
	res := newBatch()
	form, err := s.readUpload(mr, res)
	defer func() {
		for _, f := range form.files {
			if f.tmp != "" {
				_ = os.Remove(f.tmp)
			}
		}
	}()
	if err != nil {
		return nil, err
	}
	if form.libraryID <= 0 {
		return nil, fmt.Errorf("%w: libraryId is required", store.ErrInvalid)
	}
	lib, err := s.libs().get(ctx, form.libraryID)
	if err != nil {
		return nil, err
	}
	baseDir := cleanDir(form.dir)
	if err := checkDirParts(baseDir); err != nil {
		return nil, err
	}
	if _, err := libPath(lib.Path, baseDir); err != nil {
		return nil, err
	}
	if len(form.files) == 0 {
		if len(res.Errors) == 0 {
			return nil, fmt.Errorf("%w: no files uploaded", store.ErrInvalid)
		}
		return res, nil
	}

	if err := s.place(ctx, u, lib, baseDir, form.organize, form.files, res, "upload", func(f *staged) map[string]any {
		return map[string]any{"name": f.name, "size": f.size}
	}); err != nil {
		return nil, err
	}
	if err := res.failure(); err != nil {
		return nil, err
	}
	return res, nil
}

// place moves staged files into lib: under baseDir (keeping their relative names) or, with
// organize, where settings.RenamePattern puts them. Existing files are never overwritten
// (" (1)" is appended). It holds the library lock across the moves and the rescan, writes
// one edit_log row per placed file (action, with details(f)), adds the new tracks to
// res.Updated and publishes a library event. Placed files get f.tmp = "". Only a failure
// to decide the targets or to take the lock is returned; per-file failures go to res.
func (s *Service) place(ctx context.Context, u *model.User, lib *model.Library, baseDir string, organize bool,
	files []staged, res *BatchResult, action string, details func(f *staged) map[string]any) error {
	// Decide targets (tags are read before taking the lock).
	targets, err := s.uploadTargets(ctx, baseDir, organize, files)
	if err != nil {
		return err
	}

	unlock, err := s.lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	ctx = context.WithoutCancel(ctx)
	idx := newNameIndex()
	type placed struct {
		f   *staged
		rel string
	}
	var done []placed
	for i := range files {
		f := &files[i]
		rel := targets[i]
		abs, err := libPath(lib.Path, rel)
		if err != nil {
			res.fail("", f.name, err)
			continue
		}
		abs = idx.unique(abs)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			res.fail("", f.name, fsErr(rel, err))
			continue
		}
		if err := util.EnsureWithinRoot(lib.Path, abs); err != nil {
			res.fail("", f.name, err)
			continue
		}
		if err := moveFile(f.tmp, abs); err != nil {
			removeEmptyDirs(lib.Path, filepath.Dir(abs)) // folders created for nothing
			res.fail("", f.name, fsErr(rel, err))
			continue
		}
		_ = os.Chmod(abs, 0o644)
		f.tmp = ""
		rel, _ = util.ToRel(lib.Path, abs)
		done = append(done, placed{f, rel})
	}

	paths := make([]string, len(done))
	for i, d := range done {
		paths[i] = d.rel
	}
	s.rescan(ctx, lib.ID, paths)
	var ids []string
	for _, d := range done {
		trackID := ""
		if t, err := s.st.GetTrackByPath(ctx, lib.ID, d.rel); err == nil {
			trackID = t.ID
			ids = append(ids, t.ID)
		}
		s.logEdit(ctx, u, action, trackID, d.rel, details(d.f))
	}
	res.Updated = append(res.Updated, s.tracksByID(ctx, ids, u.ID)...)
	if len(done) > 0 {
		s.publish(action)
	}
	return nil
}

// readUpload consumes the multipart stream. Files are copied to the staging dir as they
// arrive (never buffered in memory); oversized or non-audio files become item errors.
func (s *Service) readUpload(mr *multipart.Reader, res *BatchResult) (uploadForm, error) {
	var form uploadForm
	if err := os.MkdirAll(s.cfg.TmpDir(), 0o755); err != nil {
		return form, err
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return form, nil
		}
		if err != nil {
			if tooLarge(err) {
				return form, err
			}
			return form, fmt.Errorf("%w: reading upload: %v", store.ErrInvalid, err)
		}
		name := part.FormName()
		switch name {
		case "files", "file":
			if len(form.files)+len(res.Errors) >= maxUploadFiles {
				_ = part.Close()
				return form, fmt.Errorf("%w: too many files (max %d per request)", store.ErrInvalid, maxUploadFiles)
			}
			f, err := s.stagePart(part)
			_ = part.Close()
			if err != nil {
				var ie itemError
				if errors.As(err, &ie) {
					res.fail("", f.name, err)
					continue
				}
				return form, err
			}
			form.files = append(form.files, f)
		case "libraryId", "dir", "organize":
			v, err := readField(part)
			_ = part.Close()
			if err != nil {
				return form, err
			}
			switch name {
			case "libraryId":
				id, err := strconv.ParseInt(v, 10, 64)
				if err != nil {
					return form, fmt.Errorf("%w: invalid libraryId", store.ErrInvalid)
				}
				form.libraryID = id
			case "dir":
				form.dir = v
			case "organize":
				b, err := config.ParseBool(v)
				if err != nil {
					return form, fmt.Errorf("%w: invalid organize flag", store.ErrInvalid)
				}
				form.organize = b
			}
		default:
			_ = part.Close() // unknown parts are skipped
		}
	}
}

// itemError marks per-file problems that do not abort the upload.
type itemError struct{ msg string }

func (e itemError) Error() string { return e.msg }

func readField(p *multipart.Part) (string, error) {
	b, err := io.ReadAll(io.LimitReader(p, maxUploadField+1))
	if err != nil {
		return "", fmt.Errorf("%w: reading form field: %v", store.ErrInvalid, err)
	}
	if len(b) > maxUploadField {
		return "", fmt.Errorf("%w: form field %s is too long", store.ErrInvalid, p.FormName())
	}
	return strings.TrimSpace(string(b)), nil
}

// uploadName returns the client's file name including relative directories (Go's
// Part.FileName strips them), sanitized into "/"-separated safe components, and whether
// the client's own base name is a hidden / system file (e.g. a macOS "._x.mp3").
func uploadName(p *multipart.Part) (name string, hidden bool) {
	raw := ""
	if _, params, err := mime.ParseMediaType(p.Header.Get("Content-Disposition")); err == nil {
		raw = params["filename"]
	}
	if raw == "" {
		raw = p.FileName()
	}
	var parts []string
	comps := strings.FieldsFunc(raw, func(r rune) bool { return r == '/' || r == '\\' })
	if len(comps) > 0 {
		hidden = util.IsHiddenOrSystem(strings.TrimSpace(comps[len(comps)-1]))
	}
	for _, c := range comps {
		if c = sanitizeComponent(c, maxComponent); c != "" && c != "." && c != ".." {
			parts = append(parts, c)
		}
	}
	return strings.Join(parts, "/"), hidden
}

// stagePart copies one file part to the staging directory.
func (s *Service) stagePart(p *multipart.Part) (staged, error) {
	name, hidden := uploadName(p)
	f := staged{name: name}
	if f.name == "" {
		return f, itemError{"missing file name"}
	}
	base := f.name[strings.LastIndexByte(f.name, '/')+1:]
	if hidden || !tags.IsAudioFile(base) {
		return f, itemError{"unsupported file type (audio files only)"}
	}
	tmp, err := os.CreateTemp(s.cfg.TmpDir(), "upload-*"+filepath.Ext(base))
	if err != nil {
		return f, err
	}
	n, err := io.Copy(tmp, io.LimitReader(p, MaxUploadFileSize+1))
	cerr := tmp.Close()
	if err == nil {
		err = cerr
	}
	if err != nil || n > MaxUploadFileSize || n == 0 {
		_ = os.Remove(tmp.Name())
		switch {
		case err != nil && (isLocalIOError(err) || tooLarge(err)):
			return f, err
		case err != nil:
			return f, itemError{"upload interrupted: " + err.Error()}
		case n == 0:
			return f, itemError{"the file is empty"}
		default:
			return f, itemError{fmt.Sprintf("file is larger than %d GiB", MaxUploadFileSize>>30)}
		}
	}
	// Reject files that are not playable audio before they reach the library.
	if _, err := tags.Read(tmp.Name(), tags.ReadOptions{}); err != nil && !errors.Is(err, errors.ErrUnsupported) {
		_ = os.Remove(tmp.Name())
		return f, itemError{"not a valid audio file"}
	}
	f.tmp, f.size = tmp.Name(), n
	return f, nil
}

// tooLarge reports whether err is the request body limit (http.MaxBytesReader) being hit.
func tooLarge(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}

// isLocalIOError reports whether err comes from writing the staging file (as opposed to
// reading the request body).
func isLocalIOError(err error) bool {
	var pe *os.PathError
	return errors.As(err, &pe)
}

// uploadTargets returns the library-relative target path of every staged file.
func (s *Service) uploadTargets(ctx context.Context, baseDir string, organize bool, files []staged) ([]string, error) {
	out := make([]string, len(files))
	join := func(parts ...string) string {
		var keep []string
		for _, p := range parts {
			if p != "" {
				keep = append(keep, p)
			}
		}
		return strings.Join(keep, "/")
	}
	if !organize {
		for i, f := range files {
			out[i] = join(baseDir, f.name)
		}
		return out, nil
	}
	set := s.settings(ctx)
	pattern, err := ParsePattern(set.RenamePattern)
	if err != nil {
		return nil, fmt.Errorf("the configured rename pattern is invalid: %w", err)
	}
	vals := make([]Values, len(files))
	discs := map[string]map[int]bool{} // album id → disc numbers in this upload
	for i, f := range files {
		md, err := tags.Read(f.tmp, tags.ReadOptions{FixEncoding: set.FixEncodingOnScan})
		if err != nil {
			slog.Warn("manage: reading tags of upload", "name", f.name, "err", err)
			md = &tags.Metadata{}
		}
		vals[i] = metadataValues(md, f.name)
		id := util.AlbumID(vals[i].AlbumArtist, vals[i].Album)
		if discs[id] == nil {
			discs[id] = map[int]bool{}
		}
		discs[id][vals[i].Disc] = true
	}
	for i, f := range files {
		v := vals[i]
		id := util.AlbumID(v.AlbumArtist, v.Album)
		if len(discs[id]) > 1 {
			v.MultiDisc = true
		} else if al, err := s.st.GetAlbum(ctx, id, ""); err == nil && al.DiscCount > 1 {
			v.MultiDisc = true
		}
		ext := strings.TrimPrefix(filepath.Ext(f.name), ".")
		rel, err := renderPath(pattern, v, ext)
		if err != nil {
			rel = join("[Unknown Artist]", "[Unknown Album]", f.name[strings.LastIndexByte(f.name, '/')+1:])
		}
		out[i] = join(baseDir, rel)
	}
	return out, nil
}

// metadataValues applies the scanner's fallbacks (docs/architecture/contract.md §5.8) to raw metadata.
func metadataValues(md *tags.Metadata, name string) Values {
	base := name[strings.LastIndexByte(name, '/')+1:]
	v := Values{
		Title: md.Title, Artist: md.Artist, Album: md.Album, AlbumArtist: md.AlbumArtist,
		Composer: md.Composer, Track: md.TrackNumber, Disc: md.DiscNumber, Year: md.Year,
		MultiDisc: md.DiscTotal > 1,
	}
	if len(md.Genres) > 0 {
		v.Genre = md.Genres[0]
	}
	if strings.TrimSpace(v.Title) == "" {
		v.Title = stem(base)
	}
	if strings.TrimSpace(v.Album) == "" {
		v.Album = "[Unknown Album]"
	}
	if strings.TrimSpace(v.Artist) == "" {
		v.Artist = "[Unknown Artist]"
	}
	if strings.TrimSpace(v.AlbumArtist) == "" {
		if md.Compilation {
			v.AlbumArtist = "Various Artists"
		} else {
			v.AlbumArtist = v.Artist
		}
	}
	return v
}
