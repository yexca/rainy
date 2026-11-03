package manage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// Downloads held for review (docs/architecture/contract.md §7.6 "staging").
//
// A link or online download job started with review (settings.ReviewDownloads when the
// request does not say) does not import its files: it moves each prepared file into
// <data>/staging/<id>/ next to a manifest (item.json) that remembers the destination and
// the edit-log details. Managers then correct the tags of the staged file (written through
// TagLib), see where it would be placed and what it may duplicate, and import it — exactly
// like an upload (place) — or discard it. Staged items survive restarts; the staging
// directory is part of the data directory, not of the library.

const (
	stagedManifest   = "item.json"
	stagedNewPrefix  = ".new-" // an item being written; removed at startup
	maxStagedBatch   = 500     // ids per import or discard request
	stagedSimilarMax = 5       // matches reported per kind
)

// Conflict policies of ImportStaged.
const (
	ConflictKeep = "keep" // import next to an existing file as "<name> (n)"
	ConflictSkip = "skip" // leave the item staged when its target already exists
)

// stagedMeta is the manifest of a staged item.
type stagedMeta struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`   // JobKindLink | JobKindOnline
	Source    string         `json:"source"` // the link, or the song's catalogue page
	Site      string         `json:"site"`   // link downloads: youtube | bilibili
	Title     string         `json:"title"`  // the title the site or catalogue reported
	Name      string         `json:"name"`   // library-relative name when not organized
	File      string         `json:"file"`   // the audio file inside the item directory
	LibraryID int64          `json:"libraryId"`
	Dir       string         `json:"dir"`
	Organize  bool           `json:"organize"`
	CreatedBy string         `json:"createdBy"`
	CreatedAt int64          `json:"createdAt"`
	Details   map[string]any `json:"details"` // the edit-log details of the import
}

// StagedItem is a download waiting for review (native API shape).
type StagedItem struct {
	ID        string         `json:"id"`
	Kind      string         `json:"kind"`
	Source    string         `json:"source"`
	Site      string         `json:"site"`
	Title     string         `json:"title"`
	Name      string         `json:"name"`
	Format    string         `json:"format"` // file extension without dot
	Size      int64          `json:"size"`
	Duration  float64        `json:"duration"`
	LibraryID int64          `json:"libraryId"`
	Dir       string         `json:"dir"`
	Organize  bool           `json:"organize"`
	CreatedBy string         `json:"createdBy"`
	CreatedAt int64          `json:"createdAt"`
	Tags      TagMap         `json:"tags"`
	HasCover  bool           `json:"hasCover"`
	Target    string         `json:"target"` // library-relative path an import would use now ("" with Error)
	Error     string         `json:"error"`  // why the target cannot be decided now
	Conflict  StagedConflict `json:"conflict"`
}

// StagedConflict tells what an import of a staged item may collide with.
type StagedConflict struct {
	Exists     bool          `json:"exists"`     // a file already has the target name (keep adds " (n)", skip leaves the item)
	SameSource []StagedMatch `json:"sameSource"` // tracks downloaded from the same link or page before
	Similar    []StagedMatch `json:"similar"`    // tracks with the same artist and title and a duration within 2 s
}

// StagedMatch is a library track a staged item may duplicate.
type StagedMatch struct {
	TrackID   string `json:"trackId"`
	LibraryID int64  `json:"libraryId"`
	Path      string `json:"path"`
}

// StagedUpdate is the body of PATCH /api/manage/staging/{id}: tags to write (other keys
// untouched, [] deletes) and a new destination.
type StagedUpdate struct {
	Tags      TagMap  `json:"tags"`
	LibraryID *int64  `json:"libraryId"`
	Dir       *string `json:"dir"`
	Organize  *bool   `json:"organize"`
}

// StagedImportResult is the response of POST /api/manage/staging/import.
type StagedImportResult struct {
	Imported []string      `json:"imported"` // staged ids now in the library
	Skipped  []string      `json:"skipped"`  // still staged: the target exists (skip policy)
	Errors   []StagedError `json:"errors"`   // still staged: the import failed
	Updated  []model.Track `json:"updated"`  // the new tracks
	Untagged []string      `json:"untagged"` // the new tracks without an artist or album tag
}

// StagedError reports a staged item that could not be imported.
type StagedError struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// review reports whether a download job holds its files for review.
func (s *Service) review(ctx context.Context, requested *bool) bool {
	if requested != nil {
		return *requested
	}
	return s.settings(ctx).ReviewDownloads
}

// validStagedID accepts the ids util.NewID makes, so an id can never name another path.
func validStagedID(id string) bool {
	if len(id) != util.IDLength {
		return false
	}
	for _, r := range id {
		digit, upper, lower := r >= '0' && r <= '9', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z'
		if !digit && !upper && !lower {
			return false
		}
	}
	return true
}

func (s *Service) stagedDir(id string) string { return filepath.Join(s.cfg.StagingDir(), id) }

// removeStaleStaging deletes items a crash left half-written.
func removeStaleStaging(dir string) {
	stale, _ := filepath.Glob(filepath.Join(dir, stagedNewPrefix+"*"))
	for _, p := range stale {
		if err := os.RemoveAll(p); err != nil {
			slog.Warn("manage: removing a half-written staged download", "name", filepath.Base(p), "err", err)
		}
	}
}

// stageFiles moves the prepared files of a finished download job into the staging area
// instead of importing them. It returns the new item ids; per-file failures go to res.
func (s *Service) stageFiles(j *downloadJob, files []staged, res *BatchResult, details func(f *staged) map[string]any) []string {
	root := s.cfg.StagingDir()
	if err := os.MkdirAll(root, 0o755); err != nil {
		for _, f := range files {
			res.fail("", f.name, err)
		}
		return nil
	}
	var ids []string
	for i := range files {
		f := &files[i]
		m := stagedMeta{
			ID: util.NewID(), Kind: j.Kind, Source: j.URL, Site: j.Site, Title: f.title, Name: f.name,
			File: "audio" + strings.ToLower(filepath.Ext(f.name)), LibraryID: j.LibraryID, Dir: j.Dir, Organize: j.Organize,
			CreatedBy: j.CreatedBy, CreatedAt: util.NowMs(), Details: details(f),
		}
		delete(m.Details, "size") // the size at import time is logged
		if err := s.writeStaged(root, f.tmp, m); err != nil {
			res.fail("", f.name, fmt.Errorf("holding the file for review: %w", err))
			continue
		}
		f.tmp = ""
		ids = append(ids, m.ID)
	}
	return ids
}

// writeStaged assembles an item in a hidden directory and renames it into place, so a
// listed item is always complete.
func (s *Service) writeStaged(root, src string, m stagedMeta) error {
	tmp := filepath.Join(root, stagedNewPrefix+m.ID)
	if err := os.MkdirAll(tmp, 0o700); err != nil {
		return err
	}
	fail := func(err error) error {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := moveFile(src, filepath.Join(tmp, m.File)); err != nil {
		return fail(err)
	}
	if err := writeManifest(tmp, m); err != nil {
		return fail(err)
	}
	s.stageMu.Lock()
	defer s.stageMu.Unlock()
	if err := os.Rename(tmp, filepath.Join(root, m.ID)); err != nil {
		return fail(err)
	}
	return nil
}

func writeManifest(dir string, m stagedMeta) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, stagedManifest+".tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, stagedManifest))
}

// readStaged loads the manifest of an item (store.ErrNotFound when there is none).
func (s *Service) readStaged(id string) (stagedMeta, error) {
	var m stagedMeta
	if !validStagedID(id) {
		return m, store.ErrNotFound
	}
	b, err := os.ReadFile(filepath.Join(s.stagedDir(id), stagedManifest))
	if errors.Is(err, fs.ErrNotExist) {
		return m, store.ErrNotFound
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("reading staged download %s: %w", id, err)
	}
	if m.ID != id || m.File != filepath.Base(m.File) || !strings.HasPrefix(m.File, "audio") {
		return m, fmt.Errorf("staged download %s has an invalid manifest", id)
	}
	if m.Details == nil {
		m.Details = map[string]any{}
	}
	return m, nil
}

func (s *Service) stagedFile(m stagedMeta) string { return filepath.Join(s.stagedDir(m.ID), m.File) }

// stagedIDs lists the complete items in the staging directory.
func (s *Service) stagedIDs() ([]string, error) {
	entries, err := os.ReadDir(s.cfg.StagingDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if e.IsDir() && validStagedID(e.Name()) {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

// ListStaged returns the items waiting for review, newest first, with where each would be
// imported now and what it may duplicate.
func (s *Service) ListStaged(ctx context.Context) ([]StagedItem, error) {
	s.stageMu.Lock()
	ids, err := s.stagedIDs()
	var metas []stagedMeta
	for _, id := range ids {
		m, err := s.readStaged(id)
		if err != nil {
			slog.Warn("manage: skipping a staged download", "id", id, "err", err)
			continue
		}
		metas = append(metas, m)
	}
	s.stageMu.Unlock()
	if err != nil {
		return nil, err
	}
	sort.Slice(metas, func(i, j int) bool {
		if metas[i].CreatedAt != metas[j].CreatedAt {
			return metas[i].CreatedAt > metas[j].CreatedAt
		}
		return metas[i].ID < metas[j].ID
	})
	out := make([]StagedItem, 0, len(metas))
	if len(metas) == 0 {
		return out, nil
	}
	look, err := s.newDupLookup(ctx)
	if err != nil {
		return nil, err
	}
	libs := s.libs()
	for _, m := range metas {
		out = append(out, s.describeStaged(ctx, libs, look, m))
	}
	return out, nil
}

// GetStaged returns one item.
func (s *Service) GetStaged(ctx context.Context, id string) (*StagedItem, error) {
	m, err := s.readStaged(id)
	if err != nil {
		return nil, err
	}
	look, err := s.newDupLookup(ctx)
	if err != nil {
		return nil, err
	}
	it := s.describeStaged(ctx, s.libs(), look, m)
	return &it, nil
}

// describeStaged reads the staged file and works out its target and conflicts.
func (s *Service) describeStaged(ctx context.Context, libs *libraries, look *dupLookup, m stagedMeta) StagedItem {
	it := StagedItem{
		ID: m.ID, Kind: m.Kind, Source: m.Source, Site: m.Site, Title: m.Title, Name: m.Name,
		Format: strings.TrimPrefix(filepath.Ext(m.File), "."), LibraryID: m.LibraryID, Dir: m.Dir, Organize: m.Organize,
		CreatedBy: m.CreatedBy, CreatedAt: m.CreatedAt, Tags: TagMap{},
		Conflict: StagedConflict{SameSource: []StagedMatch{}, Similar: []StagedMatch{}},
	}
	path := s.stagedFile(m)
	if fi, err := os.Stat(path); err == nil {
		it.Size = fi.Size()
	}
	md, err := tags.Read(path, tags.ReadOptions{})
	if err != nil {
		it.Error = "the staged file cannot be read"
		slog.Warn("manage: reading a staged download", "id", m.ID, "err", err)
		return it
	}
	it.Tags = cloneTags(md.Raw)
	delete(it.Tags, "LYRICS") // long; shown by the tag editor after the import
	it.Duration, it.HasCover = md.Duration, md.HasPicture
	it.Conflict.SameSource = look.sameSource(ctx, m.Source)
	it.Conflict.Similar = look.similar(md.Artist, md.Title, md.Duration)

	lib, err := libs.get(ctx, m.LibraryID)
	if err != nil {
		it.Error = errorText(err)
		return it
	}
	targets, err := s.uploadTargets(ctx, m.Dir, m.Organize, []staged{{name: m.Name, tmp: path}})
	if err != nil {
		it.Error = errorText(err)
		return it
	}
	abs, err := libPath(lib.Path, targets[0])
	if err != nil {
		it.Error = errorText(err)
		return it
	}
	it.Target = targets[0]
	it.Conflict.Exists = newNameIndex().taken(abs)
	return it
}

// dupLookup finds library tracks a staged item may duplicate.
type dupLookup struct {
	s       *Service
	byKey   map[string][]dupTrack
	sources map[string][]StagedMatch
}

type dupTrack struct {
	StagedMatch
	duration float64
}

func (s *Service) newDupLookup(ctx context.Context) (*dupLookup, error) {
	var rows []struct {
		ID        string  `db:"id"`
		LibraryID int64   `db:"library_id"`
		Path      string  `db:"path"`
		Artist    string  `db:"artist"`
		Title     string  `db:"title"`
		Duration  float64 `db:"duration"`
	}
	if err := s.st.DB().R.SelectContext(ctx, &rows,
		`SELECT id, library_id, path, artist, title, duration FROM tracks WHERE missing = 0`); err != nil {
		return nil, err
	}
	l := &dupLookup{s: s, byKey: map[string][]dupTrack{}, sources: map[string][]StagedMatch{}}
	for _, r := range rows {
		if k := dupKey(r.Artist, r.Title); k != "" {
			l.byKey[k] = append(l.byKey[k], dupTrack{StagedMatch{r.ID, r.LibraryID, r.Path}, r.Duration})
		}
	}
	return l, nil
}

// dupKey is the Library Doctor's duplicate key: normalized artist and title.
func dupKey(artist, title string) string {
	title = util.NormalizeSearch(title)
	if title == "" {
		return ""
	}
	return util.NormalizeSearch(artist) + "\x00" + title
}

func (l *dupLookup) similar(artist, title string, duration float64) []StagedMatch {
	out := []StagedMatch{}
	for _, t := range l.byKey[dupKey(artist, title)] {
		if len(out) == stagedSimilarMax {
			break
		}
		if d := t.duration - duration; d <= duplicateTolerance && d >= -duplicateTolerance {
			out = append(out, t.StagedMatch)
		}
	}
	return out
}

// sameSource returns the tracks still in a library that an earlier download of source
// imported (from the edit log).
func (l *dupLookup) sameSource(ctx context.Context, source string) []StagedMatch {
	if source == "" {
		return []StagedMatch{}
	}
	if m, ok := l.sources[source]; ok {
		return m
	}
	out := []StagedMatch{}
	ids, err := l.s.st.DownloadedTrackIDs(ctx, source, stagedSimilarMax)
	if err != nil {
		slog.Warn("manage: looking up earlier downloads", "err", err)
	}
	for _, id := range ids {
		t, err := l.s.st.GetTrack(ctx, id, "")
		if err != nil || t.Missing {
			continue
		}
		out = append(out, StagedMatch{TrackID: t.ID, LibraryID: t.LibraryID, Path: t.Path})
	}
	l.sources[source] = out
	return out
}

// UpdateStaged writes tags into a staged file and changes its destination.
func (s *Service) UpdateStaged(ctx context.Context, id string, req StagedUpdate) (*StagedItem, error) {
	changes, err := normalizeTagMap(req.Tags)
	if err != nil {
		return nil, err
	}
	s.stageMu.Lock()
	m, err := s.readStaged(id)
	if err != nil {
		s.stageMu.Unlock()
		return nil, err
	}
	err = s.updateStagedLocked(ctx, &m, changes, req)
	s.stageMu.Unlock()
	if err != nil {
		return nil, err
	}
	return s.GetStaged(ctx, id)
}

func (s *Service) updateStagedLocked(ctx context.Context, m *stagedMeta, changes TagMap, req StagedUpdate) error {
	moved := false
	if req.LibraryID != nil && *req.LibraryID != m.LibraryID {
		if *req.LibraryID <= 0 {
			return fmt.Errorf("%w: libraryId is required", store.ErrInvalid)
		}
		m.LibraryID, moved = *req.LibraryID, true
	}
	if req.Dir != nil {
		m.Dir, moved = cleanDir(*req.Dir), true
	}
	if moved {
		lib, err := s.libs().get(ctx, m.LibraryID)
		if err != nil {
			return err
		}
		if err := checkDirParts(m.Dir); err != nil {
			return err
		}
		if _, err := libPath(lib.Path, m.Dir); err != nil {
			return err
		}
	}
	if req.Organize != nil {
		m.Organize, moved = *req.Organize, true
	}
	if len(changes) > 0 {
		path := s.stagedFile(*m)
		raw, err := tags.ReadRaw(path)
		if err != nil {
			return fmt.Errorf("reading the tags of the staged file: %w", err)
		}
		if write, _ := tagDiff(raw, changes); len(write) > 0 {
			if err := tags.Write(path, write); err != nil {
				return fmt.Errorf("writing the tags of the staged file: %w", err)
			}
		}
	}
	if moved {
		return writeManifest(s.stagedDir(m.ID), *m)
	}
	return nil
}

// StagedPicture returns the cover embedded in a staged file.
func (s *Service) StagedPicture(id string) ([]byte, string, error) {
	m, err := s.readStaged(id)
	if err != nil {
		return nil, "", err
	}
	pic, err := tags.ReadPicture(s.stagedFile(m))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, "", store.ErrNotFound
		}
		return nil, "", err
	}
	if len(pic) == 0 {
		return nil, "", store.ErrNotFound
	}
	return pic, http.DetectContentType(pic), nil
}

func checkStagedIDs(ids []string) error {
	if len(ids) == 0 {
		return fmt.Errorf("%w: choose at least one item", store.ErrInvalid)
	}
	if len(ids) > maxStagedBatch {
		return fmt.Errorf("%w: at most %d items at a time", store.ErrInvalid, maxStagedBatch)
	}
	return nil
}

// ImportStaged imports staged items like an upload (place): grouped by destination,
// library lock, rescan, one "download" edit-log row per file and a library event. With
// ConflictSkip an item whose target already exists stays staged; ConflictKeep (the
// default) adds " (n)". Imported items leave the staging area.
func (s *Service) ImportStaged(ctx context.Context, u *model.User, ids []string, onConflict string) (*StagedImportResult, error) {
	if err := checkStagedIDs(ids); err != nil {
		return nil, err
	}
	switch onConflict {
	case "":
		onConflict = ConflictKeep
	case ConflictKeep, ConflictSkip:
	default:
		return nil, fmt.Errorf("%w: onConflict must be keep or skip", store.ErrInvalid)
	}
	out := &StagedImportResult{Imported: []string{}, Skipped: []string{}, Errors: []StagedError{}, Updated: []model.Track{}, Untagged: []string{}}

	s.stageMu.Lock()
	defer s.stageMu.Unlock()

	type group struct {
		lib      *model.Library
		dir      string
		organize bool
		metas    []stagedMeta
	}
	var groups []*group
	byKey := map[string]*group{}
	libs := s.libs()
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		m, err := s.readStaged(id)
		if err != nil {
			out.Errors = append(out.Errors, StagedError{ID: id, Error: errorText(notFoundStaged(err))})
			continue
		}
		lib, err := libs.get(ctx, m.LibraryID)
		if err != nil {
			out.Errors = append(out.Errors, StagedError{ID: id, Error: errorText(err)})
			continue
		}
		key := fmt.Sprintf("%d\x00%s\x00%t", m.LibraryID, m.Dir, m.Organize)
		g := byKey[key]
		if g == nil {
			g = &group{lib: lib, dir: m.Dir, organize: m.Organize}
			byKey[key] = g
			groups = append(groups, g)
		}
		g.metas = append(g.metas, m)
	}

	for _, g := range groups {
		files := make([]staged, len(g.metas))
		for i, m := range g.metas {
			files[i] = staged{name: m.Name, tmp: s.stagedFile(m), title: m.Title, skipExisting: onConflict == ConflictSkip}
			if fi, err := os.Stat(files[i].tmp); err == nil {
				files[i].size = fi.Size()
			}
		}
		res := newBatch()
		err := s.place(ctx, u, g.lib, g.dir, g.organize, files, res, "download", func(f *staged) map[string]any {
			for i := range files {
				if &files[i] == f {
					d := map[string]any{}
					for k, v := range g.metas[i].Details {
						d[k] = v
					}
					d["size"], d["reviewed"] = f.size, true
					return d
				}
			}
			return map[string]any{"size": f.size, "reviewed": true}
		})
		for i, m := range g.metas {
			f := files[i]
			switch {
			case f.tmp == "":
				out.Imported = append(out.Imported, m.ID)
				if rmErr := os.RemoveAll(s.stagedDir(m.ID)); rmErr != nil {
					slog.Warn("manage: removing an imported staged download", "id", m.ID, "err", rmErr)
				}
			case errors.Is(f.err, errExists) && f.skipExisting:
				out.Skipped = append(out.Skipped, m.ID)
			case f.err != nil:
				out.Errors = append(out.Errors, StagedError{ID: m.ID, Error: errorText(f.err)})
			case err != nil:
				out.Errors = append(out.Errors, StagedError{ID: m.ID, Error: errorText(err)})
			default:
				out.Errors = append(out.Errors, StagedError{ID: m.ID, Error: "the file could not be imported"})
			}
		}
		out.Updated = append(out.Updated, res.Updated...)
		out.Untagged = append(out.Untagged, res.Untagged...)
	}
	return out, nil
}

func notFoundStaged(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return errors.New("this item is no longer waiting for review")
	}
	return err
}

// DiscardStaged deletes staged items and returns how many were removed.
func (s *Service) DiscardStaged(ids []string) (int, error) {
	if err := checkStagedIDs(ids); err != nil {
		return 0, err
	}
	s.stageMu.Lock()
	defer s.stageMu.Unlock()
	n := 0
	for _, id := range ids {
		if !validStagedID(id) {
			continue
		}
		dir := s.stagedDir(id)
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}
