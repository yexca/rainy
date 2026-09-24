package manage

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// Issue types of the library doctor.
const (
	IssueMissingTags  = "missing_tags"
	IssueNoCover      = "no_cover"
	IssueDuplicates   = "duplicates"
	IssueMissingFiles = "missing_files"
	IssueEncoding     = "encoding"
)

// IssueTypes lists every issue type (summary order).
var IssueTypes = []string{IssueMissingTags, IssueNoCover, IssueDuplicates, IssueMissingFiles, IssueEncoding}

// Issue is one finding of the library doctor.
type Issue struct {
	Key     string        `json:"key"`
	Type    string        `json:"type"`
	Message string        `json:"message"`
	Tracks  []model.Track `json:"tracks"`
	Album   *model.Album  `json:"album,omitempty"`
}

// EncodingFix is the mojibake repair proposed for one track.
type EncodingFix struct {
	TrackID  string            `json:"trackId"`
	Path     string            `json:"path"`
	Encoding string            `json:"encoding"`
	Changes  map[string]Change `json:"changes"`
}

// EncodingResult is the response of POST /api/manage/encoding.
type EncodingResult struct {
	Items  []EncodingFix `json:"items"`
	Result *BatchResult  `json:"result,omitempty"`
}

const (
	unknownAlbum  = "[Unknown Album]"
	unknownArtist = "[Unknown Artist]"
	// duplicateTolerance is the maximum duration difference (seconds) of duplicates.
	duplicateTolerance = 2.0
)

// missingTagsWhere selects tracks with fallback values or no track number.
const missingTagsWhere = `missing = 0 AND (album = '` + unknownAlbum + `' OR artist = '` + unknownArtist + `'
	OR track_number = 0 OR title = substr(filename, 1, length(filename) - length(suffix) - 1))`

const noCoverWhere = `cover_path = '' AND cover_track_id = ''`

const missingFilesWhere = `missing = 1 AND id NOT IN (SELECT track_id FROM trash WHERE track_id != '')`

// IssueSummary counts the issues of every type.
func (s *Service) IssueSummary(ctx context.Context) (map[string]int, error) {
	out := make(map[string]int, len(IssueTypes))
	r := s.st.DB().R
	for typ, q := range map[string]string{
		IssueMissingTags:  `SELECT COUNT(*) FROM tracks WHERE ` + missingTagsWhere,
		IssueNoCover:      `SELECT COUNT(*) FROM albums WHERE ` + noCoverWhere,
		IssueMissingFiles: `SELECT COUNT(*) FROM tracks WHERE ` + missingFilesWhere,
	} {
		var n int
		if err := r.GetContext(ctx, &n, q); err != nil {
			return nil, fmt.Errorf("counting %s: %w", typ, err)
		}
		out[typ] = n
	}
	dups, err := s.duplicateGroups(ctx)
	if err != nil {
		return nil, err
	}
	out[IssueDuplicates] = len(dups)
	enc, err := s.encodingIssues(ctx)
	if err != nil {
		return nil, err
	}
	out[IssueEncoding] = len(enc)
	return out, nil
}

// Issues returns a page of issues of one type.
func (s *Service) Issues(ctx context.Context, userID, typ string, offset, limit int) ([]Issue, int, error) {
	switch typ {
	case IssueMissingTags:
		return s.trackIssues(ctx, userID, typ, missingTagsWhere, offset, limit, missingTagsMessage)
	case IssueMissingFiles:
		return s.trackIssues(ctx, userID, typ, missingFilesWhere, offset, limit, func(t *model.Track) string {
			return "The file is missing: " + t.Path
		})
	case IssueNoCover:
		return s.noCoverIssues(ctx, userID, offset, limit)
	case IssueDuplicates:
		return s.duplicateIssues(ctx, userID, offset, limit)
	case IssueEncoding:
		return s.encodingIssuePage(ctx, userID, offset, limit)
	}
	return nil, 0, fmt.Errorf("%w: unknown issue type %q (%s)", store.ErrInvalid, typ, strings.Join(IssueTypes, ", "))
}

func (s *Service) trackIssues(ctx context.Context, userID, typ, where string, offset, limit int, msg func(*model.Track) string) ([]Issue, int, error) {
	r := s.st.DB().R
	var total int
	if err := r.GetContext(ctx, &total, `SELECT COUNT(*) FROM tracks WHERE `+where); err != nil {
		return nil, 0, err
	}
	var ids []string
	if err := r.SelectContext(ctx, &ids, `SELECT id FROM tracks WHERE `+where+` ORDER BY path, id LIMIT ? OFFSET ?`, limit, offset); err != nil {
		return nil, 0, err
	}
	tracks, err := s.st.GetTracks(ctx, ids, userID)
	if err != nil {
		return nil, 0, err
	}
	out := make([]Issue, 0, len(tracks))
	for i := range tracks {
		t := &tracks[i]
		out = append(out, Issue{Key: typ + ":" + t.ID, Type: typ, Message: msg(t), Tracks: []model.Track{*t}})
	}
	return out, total, nil
}

func missingTagsMessage(t *model.Track) string {
	var miss []string
	if t.Title == stem(t.Filename) {
		miss = append(miss, "title")
	}
	if t.Artist == unknownArtist {
		miss = append(miss, "artist")
	}
	if t.Album == unknownAlbum {
		miss = append(miss, "album")
	}
	if t.TrackNumber == 0 {
		miss = append(miss, "track number")
	}
	if len(miss) == 0 {
		return "Incomplete tags"
	}
	return "Missing " + strings.Join(miss, ", ")
}

func (s *Service) noCoverIssues(ctx context.Context, userID string, offset, limit int) ([]Issue, int, error) {
	r := s.st.DB().R
	var total int
	if err := r.GetContext(ctx, &total, `SELECT COUNT(*) FROM albums WHERE `+noCoverWhere); err != nil {
		return nil, 0, err
	}
	var ids []string
	if err := r.SelectContext(ctx, &ids, `SELECT id FROM albums WHERE `+noCoverWhere+` ORDER BY sort_name, id LIMIT ? OFFSET ?`, limit, offset); err != nil {
		return nil, 0, err
	}
	out := make([]Issue, 0, len(ids))
	for _, id := range ids {
		al, err := s.st.GetAlbum(ctx, id, userID)
		if err != nil {
			continue
		}
		tracks, _, err := s.st.ListTracks(ctx, store.TrackQuery{UserID: userID, AlbumID: id, Sort: "track"})
		if err != nil {
			return nil, 0, err
		}
		out = append(out, Issue{Key: IssueNoCover + ":" + id, Type: IssueNoCover,
			Message: "No cover art for " + al.Name, Tracks: tracks, Album: al})
	}
	return out, total, nil
}

// ---- duplicates

type dupRow struct {
	ID       string  `db:"id"`
	Artist   string  `db:"artist"`
	Title    string  `db:"title"`
	Duration float64 `db:"duration"`
}

// duplicateGroups returns groups of ≥ 2 tracks with the same normalized artist + title
// whose durations are within duplicateTolerance of each other, ordered by artist/title.
func (s *Service) duplicateGroups(ctx context.Context) ([][]string, error) {
	var rows []dupRow
	if err := s.st.DB().R.SelectContext(ctx, &rows,
		`SELECT id, artist, title, duration FROM tracks WHERE missing = 0`); err != nil {
		return nil, err
	}
	byKey := map[string][]dupRow{}
	for _, r := range rows {
		title := util.NormalizeSearch(r.Title)
		if title == "" {
			continue
		}
		k := util.NormalizeSearch(r.Artist) + "\x00" + title
		byKey[k] = append(byKey[k], r)
	}
	keys := make([]string, 0, len(byKey))
	for k, g := range byKey {
		if len(g) > 1 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var groups [][]string
	for _, k := range keys {
		g := byKey[k]
		sort.Slice(g, func(i, j int) bool {
			if g[i].Duration != g[j].Duration {
				return g[i].Duration < g[j].Duration
			}
			return g[i].ID < g[j].ID
		})
		cluster := []string{g[0].ID}
		for i := 1; i <= len(g); i++ {
			if i < len(g) && g[i].Duration-g[i-1].Duration <= duplicateTolerance {
				cluster = append(cluster, g[i].ID)
				continue
			}
			if len(cluster) > 1 {
				groups = append(groups, cluster)
			}
			if i < len(g) {
				cluster = []string{g[i].ID}
			}
		}
	}
	return groups, nil
}

func (s *Service) duplicateIssues(ctx context.Context, userID string, offset, limit int) ([]Issue, int, error) {
	groups, err := s.duplicateGroups(ctx)
	if err != nil {
		return nil, 0, err
	}
	total := len(groups)
	groups = page(groups, offset, limit)
	out := make([]Issue, 0, len(groups))
	for _, g := range groups {
		tracks, err := s.st.GetTracks(ctx, g, userID)
		if err != nil {
			return nil, 0, err
		}
		if len(tracks) < 2 {
			continue
		}
		msg := fmt.Sprintf("%d copies of \"%s\" by %s", len(tracks), tracks[0].Title, tracks[0].Artist)
		out = append(out, Issue{Key: IssueDuplicates + ":" + g[0], Type: IssueDuplicates, Message: msg, Tracks: tracks})
	}
	return out, total, nil
}

func page[T any](v []T, offset, limit int) []T {
	if offset >= len(v) {
		return nil
	}
	v = v[offset:]
	if limit > 0 && limit < len(v) {
		v = v[:limit]
	}
	return v
}

// ---- encoding (mojibake)

// encCacheEntry caches the raw-tag analysis of a file (keyed by path; valid while size
// and mtime are unchanged).
type encCacheEntry struct {
	size, mtime int64
	fix         *EncodingFix // nil = nothing to fix
}

// id3Suffixes are formats whose tags may carry legacy (non-UTF-8) text, i.e. ID3 / ID3v1
// capable containers. Other formats store UTF-8 by specification.
var id3Suffixes = map[string]bool{"mp3": true, "mp2": true, "aif": true, "aiff": true, "wav": true,
	"ape": true, "wv": true, "mpc": true, "tta": true}

// encRow holds the stored text fields of a track checked for mojibake.
type encRow struct {
	ID          string `db:"id"`
	LibraryID   int64  `db:"library_id"`
	Path        string `db:"path"`
	Suffix      string `db:"suffix"`
	Title       string `db:"title"`
	Artist      string `db:"artist"`
	Album       string `db:"album"`
	AlbumArtist string `db:"album_artist"`
	Composer    string `db:"composer"`
	Genre       string `db:"genre"`
}

func (r *encRow) fields() map[string]string {
	return map[string]string{"TITLE": r.Title, "ARTIST": r.Artist, "ALBUM": r.Album,
		"ALBUMARTIST": r.AlbumArtist, "COMPOSER": r.Composer, "GENRE": r.Genre}
}

func hasNonASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return true
		}
	}
	return false
}

// dbFix checks the stored text fields (these reflect raw tags when FixEncodingOnScan is
// off).
func (r *encRow) dbFix() *EncodingFix {
	fix := &EncodingFix{TrackID: r.ID, Path: r.Path, Changes: map[string]Change{}}
	for key, v := range r.fields() {
		if fixed, enc, changed := tags.FixMojibake(v); changed {
			fix.Changes[key] = Change{Old: []string{v}, New: []string{fixed}}
			if fix.Encoding == "" {
				fix.Encoding = enc
			}
		}
	}
	if len(fix.Changes) == 0 {
		return nil
	}
	return fix
}

// rawFix analyses the raw tags of a file (every value of every key).
func rawFix(raw map[string][]string) (map[string]Change, string) {
	changes := map[string]Change{}
	encoding := ""
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		vals := raw[k]
		nv := make([]string, len(vals))
		changedAny := false
		for i, v := range vals {
			fixed, enc, changed := tags.FixMojibake(v)
			nv[i] = fixed
			if changed {
				changedAny = true
				if encoding == "" {
					encoding = enc
				}
			}
		}
		if changedAny {
			changes[k] = Change{Old: append([]string{}, vals...), New: nv}
		}
	}
	return changes, encoding
}

// fileFix analyses one file's raw tags, using the cache.
func (s *Service) fileFix(abs string, r *encRow) (*EncodingFix, error) {
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	s.encMu.Lock()
	e, ok := s.encCache[abs]
	s.encMu.Unlock()
	if ok && e.size == fi.Size() && e.mtime == fi.ModTime().UnixNano() {
		return e.fix, nil
	}
	raw, err := tags.ReadRaw(abs)
	if err != nil {
		return nil, err
	}
	var fix *EncodingFix
	if changes, enc := rawFix(raw); len(changes) > 0 {
		fix = &EncodingFix{TrackID: r.ID, Path: r.Path, Encoding: enc, Changes: changes}
	}
	s.encMu.Lock()
	s.encCache[abs] = encCacheEntry{size: fi.Size(), mtime: fi.ModTime().UnixNano(), fix: fix}
	s.encMu.Unlock()
	return fix, nil
}

// forgetEncoding drops the cached analysis of a file (after writing its tags).
func (s *Service) forgetEncoding(abs string) {
	s.encMu.Lock()
	delete(s.encCache, abs)
	s.encMu.Unlock()
}

// encodingIssues finds tracks whose tags look like mojibake: stored fields are checked
// directly; with FixEncodingOnScan on (stored fields already repaired) the raw tags of
// ID3-capable files with non-ASCII text are analysed, cached by path + mtime.
func (s *Service) encodingIssues(ctx context.Context) ([]EncodingFix, error) {
	var rows []encRow
	if err := s.st.DB().R.SelectContext(ctx, &rows, `SELECT id, library_id, path, suffix, title, artist, album,
		album_artist, composer, genre FROM tracks WHERE missing = 0 ORDER BY path, id`); err != nil {
		return nil, err
	}
	checkRaw := s.settings(ctx).FixEncodingOnScan
	libs := s.libs()
	results := make([]*EncodingFix, len(rows))
	type job struct {
		i   int
		abs string
	}
	var jobs []job
	for i := range rows {
		r := &rows[i]
		if fix := r.dbFix(); fix != nil {
			results[i] = fix
			continue
		}
		if !checkRaw || !id3Suffixes[r.Suffix] {
			continue
		}
		nonASCII := false
		for _, v := range r.fields() {
			if hasNonASCII(v) {
				nonASCII = true
				break
			}
		}
		if !nonASCII {
			continue
		}
		lib, err := libs.get(ctx, r.LibraryID)
		if err != nil {
			continue
		}
		abs, err := util.SafeJoin(lib.Path, r.Path)
		if err != nil {
			continue
		}
		jobs = append(jobs, job{i, abs})
	}
	if len(jobs) > 0 {
		ch := make(chan job)
		var wg sync.WaitGroup
		for range min(runtime.NumCPU(), 8, len(jobs)) {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := range ch {
					fix, err := s.fileFix(j.abs, &rows[j.i])
					if err != nil {
						slog.Debug("manage: reading raw tags for encoding check", "path", rows[j.i].Path, "err", err)
						continue
					}
					results[j.i] = fix
				}
			}()
		}
	feed:
		for _, j := range jobs {
			select {
			case ch <- j:
			case <-ctx.Done():
				break feed
			}
		}
		close(ch)
		wg.Wait()
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	out := []EncodingFix{}
	for _, f := range results {
		if f != nil {
			out = append(out, *f)
		}
	}
	return out, nil
}

func (s *Service) encodingIssuePage(ctx context.Context, userID string, offset, limit int) ([]Issue, int, error) {
	fixes, err := s.encodingIssues(ctx)
	if err != nil {
		return nil, 0, err
	}
	total := len(fixes)
	fixes = page(fixes, offset, limit)
	ids := make([]string, len(fixes))
	for i, f := range fixes {
		ids[i] = f.TrackID
	}
	tracks, err := s.st.GetTracks(ctx, ids, userID)
	if err != nil {
		return nil, 0, err
	}
	byID := map[string]model.Track{}
	for _, t := range tracks {
		byID[t.ID] = t
	}
	out := make([]Issue, 0, len(fixes))
	for _, f := range fixes {
		t, ok := byID[f.TrackID]
		if !ok {
			continue
		}
		keys := make([]string, 0, len(f.Changes))
		for k := range f.Changes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		msg := fmt.Sprintf("%s text shown as mojibake in %s", strings.ToUpper(f.Encoding), strings.Join(keys, ", "))
		out = append(out, Issue{Key: IssueEncoding + ":" + t.ID, Type: IssueEncoding, Message: msg, Tracks: []model.Track{t}})
	}
	return out, total, nil
}

// Encoding previews (apply=false) or applies mojibake repairs to the raw tags of tracks
// (written back as UTF-8). trackIDs == nil (omitted in the request) uses every detected
// encoding issue; an empty non-nil list does nothing.
func (s *Service) Encoding(ctx context.Context, u *model.User, trackIDs []string, apply bool) (*EncodingResult, error) {
	all := trackIDs == nil
	trackIDs = dedupe(trackIDs)
	if len(trackIDs) == 0 && !all {
		out := &EncodingResult{Items: []EncodingFix{}}
		if apply {
			out.Result = newBatch()
		}
		return out, nil
	}
	if all {
		fixes, err := s.encodingIssues(ctx)
		if err != nil {
			return nil, err
		}
		for _, f := range fixes {
			trackIDs = append(trackIDs, f.TrackID)
		}
		if len(trackIDs) == 0 {
			out := &EncodingResult{Items: []EncodingFix{}}
			if apply {
				out.Result = newBatch()
			}
			return out, nil
		}
	}
	if err := checkBatch(len(trackIDs), "trackIds"); err != nil {
		return nil, err
	}
	tracks, err := s.st.GetTracks(ctx, trackIDs, "")
	if err != nil {
		return nil, err
	}
	libs := s.libs()
	out := &EncodingResult{Items: []EncodingFix{}}
	res := newBatch()
	var todo []model.Track
	for _, t := range tracks {
		_, abs, err := s.trackFile(ctx, libs, &t)
		if err != nil {
			res.fail(t.ID, t.Path, err)
			continue
		}
		raw, err := tags.ReadRaw(abs)
		if err != nil {
			res.fail(t.ID, t.Path, fsErr(t.Path, err))
			continue
		}
		changes, enc := rawFix(raw)
		if len(changes) == 0 {
			continue
		}
		out.Items = append(out.Items, EncodingFix{TrackID: t.ID, Path: t.Path, Encoding: enc, Changes: changes})
		todo = append(todo, t)
	}
	if !apply {
		return out, nil
	}
	out.Result = res
	if len(todo) == 0 {
		return out, nil
	}

	unlock, err := s.lock(ctx)
	if err != nil {
		return nil, err
	}
	defer unlock()
	ctx = context.WithoutCancel(ctx)
	type done struct {
		t    model.Track
		diff map[string]Change
		enc  string
	}
	var ok []done
	for i, t := range todo {
		// Re-analyse under the lock: the tags may have been edited since the preview, and
		// stale "fixed" values must not overwrite those edits.
		enc := out.Items[i].Encoding
		write := TagMap{}
		if _, abs, err := s.trackFile(ctx, libs, &t); err == nil {
			if raw, err := tags.ReadRaw(abs); err == nil {
				var changes map[string]Change
				changes, enc = rawFix(raw)
				for k, c := range changes {
					write[k] = c.New
				}
			}
		}
		diff, err := s.writeTags(ctx, libs, &t, write)
		if err != nil {
			res.fail(t.ID, t.Path, err)
			continue
		}
		if len(diff) == 0 {
			continue // already fixed meanwhile
		}
		ok = append(ok, done{t, diff, enc})
	}

	byLib := map[int64][]string{}
	ids := make([]string, 0, len(ok))
	for _, d := range ok {
		byLib[d.t.LibraryID] = append(byLib[d.t.LibraryID], d.t.Path)
		ids = append(ids, d.t.ID)
	}
	for lib, paths := range byLib {
		s.rescan(ctx, lib, paths)
	}
	for _, d := range ok {
		s.logEdit(ctx, u, "encoding", d.t.ID, d.t.Path, map[string]any{"encoding": d.enc, "changes": d.diff})
	}
	res.Updated = s.tracksByID(ctx, ids, u.ID)
	if len(ok) > 0 {
		s.publish("encoding")
	}
	if err := res.failure(); err != nil {
		return nil, err
	}
	return out, nil
}

// EditLog returns a page of the edit log (optionally for one track), newest first.
func (s *Service) EditLog(ctx context.Context, trackID string, offset, limit int) ([]model.EditLogEntry, int, error) {
	entries, total, err := s.st.ListEditLog(ctx, trackID, offset, limit)
	if entries == nil {
		entries = []model.EditLogEntry{}
	}
	return entries, total, err
}
