package store

import (
	"context"
	"slices"
	"strconv"
	"strings"

	"rainy/internal/model"
)

// Library scope (docs/architecture/contract.md §5.4): a request context may carry the
// libraries and folders its user is allowed to read. Library reads made with such a
// context only see tracks of those libraries and of those folders (and every folder below
// them), and the albums, artists, genres and playlist entries derived from them; everything
// else behaves as if it did not exist (ErrNotFound, left out of lists, counts computed from
// the visible tracks). A library is visible when any of its tracks may be. A context
// without a scope reads every library: administrators, managers, the scanner and
// background jobs. Writes that index files (scanner, manage) never consult the scope.

type libraryScopeKey struct{}

type libraryScope struct {
	libraries []int64               // whole libraries, sorted and distinct
	folders   []model.LibraryFolder // folders of other libraries
}

// WithLibraryScope returns a context whose library reads only see the given whole
// libraries and folders (none: no library at all). Folders must be valid
// (model.CleanFolderPath); invalid ones are ignored.
func WithLibraryScope(ctx context.Context, ids []int64, folders ...model.LibraryFolder) context.Context {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if ids == nil {
		ids = []int64{}
	}
	sc := libraryScope{libraries: ids}
	for _, f := range folders {
		if p, ok := model.CleanFolderPath(f.Path); ok && p == f.Path && !slices.Contains(ids, f.LibraryID) {
			sc.folders = append(sc.folders, f)
		}
	}
	return context.WithValue(ctx, libraryScopeKey{}, sc)
}

// LibraryScope returns the libraries ctx may read at least in part (sorted) and true, or
// nil and false when it may read every library.
func LibraryScope(ctx context.Context) ([]int64, bool) {
	sc, ok := ctx.Value(libraryScopeKey{}).(libraryScope)
	if !ok {
		return nil, false
	}
	return sc.visibleLibraries(), true
}

func (sc libraryScope) visibleLibraries() []int64 {
	ids := slices.Clone(sc.libraries)
	for _, f := range sc.folders {
		ids = append(ids, f.LibraryID)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// CanReadLibrary reports whether ctx may read at least part of the library id.
func CanReadLibrary(ctx context.Context, id int64) bool {
	ids, restricted := LibraryScope(ctx)
	return !restricted || slices.Contains(ids, id)
}

// inList returns "col IN (1, 2)", or "0" for no id. The ids are integers, so they are
// inlined and the condition adds no bind parameters.
func inList(col string, ids []int64) string {
	if len(ids) == 0 {
		return "0"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return col + " IN (" + strings.Join(parts, ", ") + ")"
}

// sqlString quotes s as an SQLite string literal. Folder paths are inlined (like library
// ids) so that the scope conditions add no bind parameters.
func sqlString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// libraryCond returns the condition on a libraries id column for a scoped context ("0"
// when no library is readable) and "" otherwise.
func libraryCond(ctx context.Context, col string) string {
	ids, restricted := LibraryScope(ctx)
	if !restricted {
		return ""
	}
	return inList(col, ids)
}

// trackCond returns the condition on the tracks columns <prefix>library_id and <prefix>dir
// for a scoped context ("0" when nothing is readable) and "" otherwise. A folder matches
// its own dir and every dir below it; the range test (dir >= 'a/' AND dir < 'a0', '0'
// following '/') keeps the (library_id, dir) index usable and does not match siblings
// such as "a b" or "ab". The folder terms are joined as a balanced OR tree, since SQLite
// rejects expressions nested deeper than 1000 levels and a flat chain nests one per folder.
func trackCond(ctx context.Context, prefix string) string {
	sc, restricted := ctx.Value(libraryScopeKey{}).(libraryScope)
	if !restricted {
		return ""
	}
	lib, dir := prefix+"library_id", prefix+"dir"
	var parts []string
	if len(sc.libraries) > 0 {
		parts = append(parts, inList(lib, sc.libraries))
	}
	for _, f := range sc.folders {
		parts = append(parts, "("+lib+" = "+strconv.FormatInt(f.LibraryID, 10)+" AND ("+dir+" = "+sqlString(f.Path)+
			" OR ("+dir+" >= "+sqlString(f.Path+"/")+" AND "+dir+" < "+sqlString(f.Path+"0")+")))")
	}
	if len(parts) == 0 {
		return "0"
	}
	return orTree(parts)
}

// orTree joins non-empty parts with OR, nesting the halves so the expression is only
// about log2(len(parts)) levels deep.
func orTree(parts []string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	mid := len(parts) / 2
	return "(" + orTree(parts[:mid]) + " OR " + orTree(parts[mid:]) + ")"
}

// andLibrary returns " AND " + libraryCond, or "" for an unscoped context.
func andLibrary(ctx context.Context, col string) string {
	if c := libraryCond(ctx, col); c != "" {
		return " AND " + c
	}
	return ""
}

// andTrack returns " AND " + trackCond, or "" for an unscoped context.
func andTrack(ctx context.Context, prefix string) string {
	if c := trackCond(ctx, prefix); c != "" {
		return " AND " + c
	}
	return ""
}

// TracksFrom returns the tracks table as seen by ctx, to be used as "FROM <expr> t": the
// plain table, or a sub-select of the readable tracks.
func TracksFrom(ctx context.Context) string {
	c := trackCond(ctx, "")
	if c == "" {
		return "tracks"
	}
	return "(SELECT * FROM tracks WHERE " + c + ")"
}

// AlbumsFrom returns the albums table as seen by ctx: albums with at least one readable
// track. Their stored aggregates (song count, duration) cover every track of the album.
func AlbumsFrom(ctx context.Context) string {
	c := trackCond(ctx, "st.")
	if c == "" {
		return "albums"
	}
	return "(SELECT * FROM albums WHERE id IN (SELECT st.album_id FROM tracks st WHERE " + c + "))"
}

// ArtistsFrom returns the artists table as seen by ctx: artists with a readable track,
// with album_count and song_count computed from the readable present tracks (the same
// definitions RefreshArtists stores).
func ArtistsFrom(ctx context.Context) string {
	c := trackCond(ctx, "st.")
	if c == "" {
		return "artists"
	}
	return `(SELECT a.id, a.name, a.sort_name, a.index_key,
			(SELECT COUNT(DISTINCT st.album_id) FROM tracks st
				WHERE st.album_artist_id = a.id AND st.missing = 0 AND ` + c + `) AS album_count,
			(SELECT COUNT(*) FROM tracks st
				WHERE (st.artist_id = a.id OR st.album_artist_id = a.id) AND st.missing = 0 AND ` + c + `) AS song_count,
			a.mbz_artist_id, a.image_path, a.search_text, a.created_at, a.updated_at
		FROM artists a
		WHERE a.id IN (SELECT st.artist_id FROM tracks st WHERE ` + c + `)
			OR a.id IN (SELECT st.album_artist_id FROM tracks st WHERE ` + c + `))`
}

// libraryVisible returns a condition that holds for rows of libraries ctx reads at least in
// part ("1" for an unscoped context); col is a libraries id column.
func libraryVisible(ctx context.Context, col string) string {
	if c := libraryCond(ctx, col); c != "" {
		return "(" + c + ")"
	}
	return "1"
}

// trackVisible returns a condition that holds for readable rows of the tracks columns
// <prefix>library_id and <prefix>dir ("1" for an unscoped context).
func trackVisible(ctx context.Context, prefix string) string {
	if c := trackCond(ctx, prefix); c != "" {
		return "(" + c + ")"
	}
	return "1"
}
