package scanner

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"sort"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

// artistImagePatterns are the file names used for artist images.
var artistImagePatterns = []string{"artist.*"}

// discDirRe matches per-disc sub-folders ("CD1", "Disc 2", "disk-3", "DVD 1 - Live"), whose
// parent folder usually holds the album cover.
var discDirRe = regexp.MustCompile(`(?i)^(cd|disc|disk|dvd|vinyl)[\s._-]*\d{1,3}(\D.*)?$`)

type albumDirRow struct {
	AlbumID       string `db:"album_id"`
	AlbumArtistID string `db:"album_artist_id"`
	LibraryID     int64  `db:"library_id"`
	Dir           string `db:"dir"`
	N             int    `db:"n"`
}

// albumDirs returns the directories holding non-missing tracks of the given albums (nil =
// all albums), most populated first.
func (s *Scanner) albumDirs(ctx context.Context, albumIDs []string, byArtist bool) ([]albumDirRow, error) {
	q := `SELECT album_id, album_artist_id, library_id, dir, COUNT(*) AS n FROM tracks WHERE missing = 0`
	var rows []albumDirRow
	if albumIDs == nil {
		if err := s.st.DB().R.SelectContext(ctx, &rows, q+` GROUP BY album_id, album_artist_id, library_id, dir`); err != nil {
			return nil, err
		}
	} else {
		col := "album_id"
		if byArtist {
			col = "album_artist_id"
		}
		for _, chunk := range chunkStrings(albumIDs, 400) {
			var part []albumDirRow
			args := make([]any, len(chunk))
			for i, id := range chunk {
				args[i] = id
			}
			if err := s.st.DB().R.SelectContext(ctx, &part, q+` AND `+col+` IN (`+placeholders(len(chunk))+`)
				GROUP BY album_id, album_artist_id, library_id, dir`, args...); err != nil {
				return nil, err
			}
			rows = append(rows, part...)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].N != rows[j].N {
			return rows[i].N > rows[j].N
		}
		return rows[i].Dir < rows[j].Dir
	})
	return rows, nil
}

// updateCovers sets albums.cover_path (folder image in the album's directories, or in the
// parent of per-disc folders) and artists.image_path (artist.* in the parent directory of
// the album folders, else in the album folder) for the given albums (nil = all) and their
// album artists. Libraries in skipLibs (whose scan failed) and libraries whose root is
// unavailable are left untouched.
func (s *Scanner) updateCovers(ctx context.Context, albumIDs []string, set model.Settings, cache *dirCache, skipLibs map[int64]bool) error {
	if albumIDs != nil && len(albumIDs) == 0 {
		return nil
	}
	libs, err := s.st.ListLibraries(ctx)
	if err != nil {
		return err
	}
	// Libraries whose root is unavailable (share offline) are left alone: their albums keep
	// their current cover paths instead of being cleared.
	roots := make(map[int64]string, len(libs))
	for _, l := range libs {
		if !skipLibs[l.ID] && checkRoot(l.Path) == nil {
			roots[l.ID] = l.Path
		}
	}
	unavailable := func(libID int64) bool { _, ok := roots[libID]; return !ok }
	abs := func(libID int64, rel string) string {
		root, ok := roots[libID]
		if !ok {
			return ""
		}
		p, err := util.SafeJoin(root, rel)
		if err != nil {
			return ""
		}
		return p
	}
	patterns := set.CoverArtPatterns()
	curCovers, err := s.pathColumn(ctx, `SELECT id, cover_path AS p FROM albums`)
	if err != nil {
		return err
	}
	curImages, err := s.pathColumn(ctx, `SELECT id, image_path AS p FROM artists`)
	if err != nil {
		return err
	}

	rows, err := s.albumDirs(ctx, albumIDs, false)
	if err != nil {
		return fmt.Errorf("listing album folders: %w", err)
	}
	type albumInfo struct {
		artist string
		cands  []string // absolute directories in priority order
		seen   map[string]bool
		skip   bool // some tracks live in an unavailable library
	}
	albums := map[string]*albumInfo{}
	var order []string
	artistSet := map[string]bool{}
	for _, r := range rows {
		a := albums[r.AlbumID]
		if a == nil {
			a = &albumInfo{artist: r.AlbumArtistID, seen: map[string]bool{}}
			albums[r.AlbumID] = a
			order = append(order, r.AlbumID)
		}
		artistSet[r.AlbumArtistID] = true
		if unavailable(r.LibraryID) {
			a.skip = true
			continue
		}
		add := func(rel string) {
			if p := abs(r.LibraryID, rel); p != "" && !a.seen[p] {
				a.seen[p] = true
				a.cands = append(a.cands, p)
			}
		}
		add(r.Dir)
		if r.Dir != "" && discDirRe.MatchString(path.Base(r.Dir)) {
			if parent := path.Dir(r.Dir); parent != "." {
				add(parent)
			}
		}
	}
	// Albums explicitly requested but without tracks any more have been deleted by the
	// refresh; nothing to do for them.
	var errs []error
	for _, id := range order {
		if err := ctx.Err(); err != nil {
			return err
		}
		a := albums[id]
		if a.skip {
			continue
		}
		cover, known := findFirst(cache, a.cands, patterns)
		if cur, ok := curCovers[id]; !ok || cur == cover || !known {
			continue
		}
		if err := s.st.SetAlbumCoverPath(ctx, id, cover); err != nil && !errors.Is(err, store.ErrNotFound) {
			errs = append(errs, err)
		}
	}

	// Artist images: consider every album of the affected album artists.
	artistRows := rows
	if albumIDs != nil {
		ids := make([]string, 0, len(artistSet))
		for id := range artistSet {
			ids = append(ids, id)
		}
		if artistRows, err = s.albumDirs(ctx, ids, true); err != nil {
			return fmt.Errorf("listing artist folders: %w", err)
		}
	}
	type artistInfo struct {
		parents, own []string
		seen         map[string]bool
		skip         bool
	}
	artists := map[string]*artistInfo{}
	var artistOrder []string
	for _, r := range artistRows {
		if r.AlbumArtistID == "" {
			continue
		}
		a := artists[r.AlbumArtistID]
		if a == nil {
			a = &artistInfo{seen: map[string]bool{}}
			artists[r.AlbumArtistID] = a
			artistOrder = append(artistOrder, r.AlbumArtistID)
		}
		if unavailable(r.LibraryID) {
			a.skip = true
			continue
		}
		albumDir := r.Dir
		if albumDir != "" && discDirRe.MatchString(path.Base(albumDir)) {
			albumDir = path.Dir(albumDir)
			if albumDir == "." {
				albumDir = ""
			}
		}
		if albumDir == "" {
			continue // never use images at the library root
		}
		if p := abs(r.LibraryID, albumDir); p != "" && !a.seen[p] {
			a.seen[p] = true
			a.own = append(a.own, p)
		}
		if parent := path.Dir(albumDir); parent != "." && parent != "" {
			if p := abs(r.LibraryID, parent); p != "" && !a.seen[p] {
				a.seen[p] = true
				a.parents = append(a.parents, p)
			}
		}
	}
	for _, id := range artistOrder {
		if err := ctx.Err(); err != nil {
			return err
		}
		a := artists[id]
		if a.skip {
			continue
		}
		img, known := findFirst(cache, append(a.parents, a.own...), artistImagePatterns)
		if cur, ok := curImages[id]; !ok || cur == img || !known {
			continue
		}
		if err := s.st.SetArtistImagePath(ctx, id, img); err != nil && !errors.Is(err, store.ErrNotFound) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// findFirst returns the first image matching patterns in dirs (in priority order). known
// is false when no image was found and a directory that could have held one was
// unreadable (share offline): the current value must then be kept, not cleared — nor
// replaced by a lower-priority match.
func findFirst(cache *dirCache, dirs, patterns []string) (img string, known bool) {
	for _, dir := range dirs {
		m, ok := cache.find(dir, patterns)
		if !ok {
			return "", false
		}
		if m != "" {
			return m, true
		}
	}
	return "", true
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, 0, n*3)
	for i := 0; i < n; i++ {
		if i > 0 {
			b = append(b, ", "...)
		}
		b = append(b, '?')
	}
	return string(b)
}

func chunkStrings(v []string, n int) [][]string {
	var out [][]string
	for len(v) > n {
		out = append(out, v[:n])
		v = v[n:]
	}
	if len(v) > 0 {
		out = append(out, v)
	}
	return out
}

// pathColumn loads an id → path map (query must select columns id and p).
func (s *Scanner) pathColumn(ctx context.Context, query string) (map[string]string, error) {
	var rows []struct {
		ID string `db:"id"`
		P  string `db:"p"`
	}
	if err := s.st.DB().R.SelectContext(ctx, &rows, query); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.ID] = r.P
	}
	return out, nil
}
