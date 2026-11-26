package store

import (
	"context"
	"slices"
	"strconv"
	"strings"
)

// Library scope (docs/architecture/contract.md §5.4): a request context may carry the
// libraries its user is allowed to read. Library reads made with such a context only see
// tracks of those libraries, and the albums, artists, genres and playlist entries derived
// from them; everything else behaves as if it did not exist (ErrNotFound, left out of
// lists, counts computed from the visible tracks). A context without a scope reads every
// library: administrators, managers, the scanner and background jobs. Writes that index
// files (scanner, manage) never consult the scope.

type libraryScopeKey struct{}

// WithLibraryScope returns a context whose library reads only see the given libraries
// (none: no library at all).
func WithLibraryScope(ctx context.Context, ids []int64) context.Context {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if ids == nil {
		ids = []int64{}
	}
	return context.WithValue(ctx, libraryScopeKey{}, ids)
}

// LibraryScope returns the libraries ctx may read and true, or nil and false when it may
// read every library.
func LibraryScope(ctx context.Context) ([]int64, bool) {
	ids, ok := ctx.Value(libraryScopeKey{}).([]int64)
	return ids, ok
}

// CanReadLibrary reports whether ctx may read the library id.
func CanReadLibrary(ctx context.Context, id int64) bool {
	ids, restricted := LibraryScope(ctx)
	return !restricted || slices.Contains(ids, id)
}

// libraryCond returns "col IN (1, 2)" ("0" when no library is readable) for a scoped
// context and "" otherwise. The ids are integers, so they are inlined and the condition
// adds no bind parameters.
func libraryCond(ctx context.Context, col string) string {
	ids, restricted := LibraryScope(ctx)
	if !restricted {
		return ""
	}
	if len(ids) == 0 {
		return "0"
	}
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return col + " IN (" + strings.Join(parts, ", ") + ")"
}

// andLibrary returns " AND " + libraryCond, or "" for an unscoped context.
func andLibrary(ctx context.Context, col string) string {
	if c := libraryCond(ctx, col); c != "" {
		return " AND " + c
	}
	return ""
}

// TracksFrom returns the tracks table as seen by ctx, to be used as "FROM <expr> t": the
// plain table, or a sub-select of the readable libraries' tracks.
func TracksFrom(ctx context.Context) string {
	c := libraryCond(ctx, "library_id")
	if c == "" {
		return "tracks"
	}
	return "(SELECT * FROM tracks WHERE " + c + ")"
}

// AlbumsFrom returns the albums table as seen by ctx: albums with at least one track in a
// readable library. Their stored aggregates (song count, duration) cover every library.
func AlbumsFrom(ctx context.Context) string {
	c := libraryCond(ctx, "st.library_id")
	if c == "" {
		return "albums"
	}
	return "(SELECT * FROM albums WHERE id IN (SELECT st.album_id FROM tracks st WHERE " + c + "))"
}

// ArtistsFrom returns the artists table as seen by ctx: artists with a track in a readable
// library, with album_count and song_count computed from those libraries' present tracks
// (the same definitions RefreshArtists stores).
func ArtistsFrom(ctx context.Context) string {
	c := libraryCond(ctx, "st.library_id")
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

// libraryVisible returns a condition that holds for rows of readable libraries ("1" for an
// unscoped context).
func libraryVisible(ctx context.Context, col string) string {
	if c := libraryCond(ctx, col); c != "" {
		return "(" + c + ")"
	}
	return "1"
}
