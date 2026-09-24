// Package store is Rainy's repository layer: every SQL statement shared between services
// lives here. Reads go through the reader pool (db.R), writes through the single writer
// connection (db.W / db.Tx).
//
// Conventions:
//   - Missing rows yield ErrNotFound (callers map it to HTTP 404 / Subsonic error 70).
//   - Unique-constraint violations yield ErrConflict, bad arguments ErrInvalid.
//   - Every returned track/album/artist/playlist has its computed fields filled
//     (Starred, CoverArt, ContentType, HasLyrics, Genres, …).
//   - Per-user fields (starred/rating/play counts) come from a LEFT JOIN on annotations for
//     the given userID; an empty userID yields zero values.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"

	"rainy/internal/db"
	"rainy/internal/model"
	"rainy/internal/util"
)

var (
	// ErrNotFound is returned when a requested row does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned when a write violates a uniqueness constraint
	// (duplicate username, library path, …).
	ErrConflict = errors.New("conflict")
	// ErrInvalid is returned (wrapped) for invalid arguments such as an unknown item type.
	ErrInvalid = errors.New("invalid argument")
)

// Store is the repository. It is safe for concurrent use.
type Store struct {
	db *db.DB
}

// New creates a Store on an opened, migrated database.
func New(d *db.DB) *Store { return &Store{db: d} }

// DB exposes the database for package-local specialised queries elsewhere (read via R,
// write via W / Tx).
func (s *Store) DB() *db.DB { return s.db }

// maxVars bounds the number of bound parameters per statement when expanding IN lists.
const maxVars = 500

// placeholders returns "?, ?, ?" with n placeholders.
func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat("?, ", n-1) + "?"
}

// chunks splits ids into slices of at most maxVars elements.
func chunks[T any](ids []T) [][]T {
	var out [][]T
	for len(ids) > maxVars {
		out = append(out, ids[:maxVars])
		ids = ids[maxVars:]
	}
	if len(ids) > 0 {
		out = append(out, ids)
	}
	return out
}

func anys[T any](v []T) []any {
	out := make([]any, len(v))
	for i := range v {
		out[i] = v[i]
	}
	return out
}

// dedupe returns the distinct non-empty strings of v, order preserved.
func dedupe(v []string) []string {
	seen := make(map[string]bool, len(v))
	out := make([]string, 0, len(v))
	for _, s := range v {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// notFound maps sql.ErrNoRows to ErrNotFound.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// constraint maps SQLite unique/primary-key violations to ErrConflict (wrapping the
// original message) and returns other errors unchanged.
func constraint(err error, what string) error {
	var se *sqlite.Error
	if errors.As(err, &se) {
		switch se.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return fmt.Errorf("%w: %s already exists", ErrConflict, what)
		}
	}
	return err
}

// requireAffected returns ErrNotFound when res affected no rows.
func requireAffected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// where accumulates SQL conditions and their arguments.
type where struct {
	conds []string
	args  []any
}

func (w *where) add(cond string, args ...any) {
	w.conds = append(w.conds, cond)
	w.args = append(w.args, args...)
}

// in adds "col IN (...)" for a non-empty list; an empty (but non-nil) list matches nothing.
func (w *where) in(col string, ids []string) {
	if len(ids) == 0 {
		w.add("0")
		return
	}
	w.add(col+" IN ("+placeholders(len(ids))+")", anys(ids)...)
}

// tokens adds one LIKE condition per search token on col.
func (w *where) tokens(col, q string) {
	for _, tok := range util.SearchTokens(q) {
		w.add(col+` LIKE ? ESCAPE '\'`, "%"+util.EscapeLike(tok)+"%")
	}
}

func (w *where) sql() string {
	if len(w.conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(w.conds, " AND ")
}

// limitClause renders LIMIT/OFFSET; limit <= 0 means unlimited.
func limitClause(offset, limit int) string {
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		if offset == 0 {
			return ""
		}
		return fmt.Sprintf(" LIMIT -1 OFFSET %d", offset)
	}
	return fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
}

// sortSpec describes an ORDER BY for one sort key.
type sortSpec struct {
	expr      string   // primary expression
	desc      bool     // natural direction is descending
	nullsLast bool     // keep NULLs last regardless of direction
	then      []string // secondary expressions (always ascending)
}

// orderBy renders "ORDER BY ..." for spec with order "asc"|"desc"|"" (natural) and a
// stable tiebreak column.
func orderBy(spec sortSpec, order, tiebreak string) string {
	if spec.expr == "RANDOM()" {
		return " ORDER BY RANDOM()"
	}
	desc := spec.desc
	switch strings.ToLower(order) {
	case "asc":
		desc = false
	case "desc":
		desc = true
	}
	dir := " ASC"
	if desc {
		dir = " DESC"
	}
	parts := []string{spec.expr + dir}
	if spec.nullsLast {
		parts[0] += " NULLS LAST"
	}
	parts = append(parts, spec.then...)
	parts = append(parts, tiebreak)
	return " ORDER BY " + strings.Join(parts, ", ")
}

// validItemType checks an annotation item type.
func validItemType(t string) error {
	switch t {
	case "track", "album", "artist":
		return nil
	}
	return fmt.Errorf("%w: item type %q (want track|album|artist)", ErrInvalid, t)
}

// annotationCols selects the per-user annotation columns from alias an.
const annotationCols = `an.starred_at AS starred_at, COALESCE(an.rating, 0) AS rating,
	COALESCE(an.play_count, 0) AS play_count, COALESCE(an.played_at, 0) AS played_at`

// annotationJoin joins annotations for item alias/idCol; takes the userID argument.
func annotationJoin(itemType, idCol string) string {
	return ` LEFT JOIN annotations an ON an.user_id = ? AND an.item_type = '` + itemType + `' AND an.item_id = ` + idCol
}

// fillTrack computes a track's derived fields.
func fillTrack(t *model.Track) {
	t.Fill()
	t.ContentType = util.MimeType(t.Suffix)
	if t.HasCover {
		t.CoverArt = util.CoverArtID(util.CoverKindTrack, t.ID, t.UpdatedAt)
	} else {
		v := t.AlbumUpdatedAt
		if v == 0 {
			v = t.UpdatedAt
		}
		t.CoverArt = util.CoverArtID(util.CoverKindAlbum, t.AlbumID, v)
	}
}

func fillTracks(ts []model.Track) []model.Track {
	if ts == nil {
		return []model.Track{}
	}
	for i := range ts {
		fillTrack(&ts[i])
	}
	return ts
}

func fillAlbum(a *model.Album) {
	a.Fill()
	a.CoverArt = util.CoverArtID(util.CoverKindAlbum, a.ID, a.UpdatedAt)
}

func fillAlbums(as []model.Album) []model.Album {
	if as == nil {
		return []model.Album{}
	}
	for i := range as {
		fillAlbum(&as[i])
	}
	return as
}

func fillArtist(a *model.Artist) {
	a.Fill()
	a.CoverArt = util.CoverArtID(util.CoverKindArtist, a.ID, a.UpdatedAt)
}

func fillArtists(as []model.Artist) []model.Artist {
	if as == nil {
		return []model.Artist{}
	}
	for i := range as {
		fillArtist(&as[i])
	}
	return as
}

func fillPlaylist(p *model.Playlist) {
	// The mosaic is made of album covers: its version also follows those albums.
	p.CoverArt = util.CoverArtID(util.CoverKindPlaylist, p.ID, max(p.UpdatedAt, p.AlbumsUpdatedAt))
}

// queryer is implemented by *sqlx.DB and *sqlx.Tx.
type queryer interface {
	sqlx.QueryerContext
	sqlx.ExecerContext
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
}

var (
	_ queryer = (*sqlx.DB)(nil)
	_ queryer = (*sqlx.Tx)(nil)
)

// bumpVersion is an SQL expression that sets updated_at to now (arg) while guaranteeing it
// strictly increases, so cover-art versions always change. Takes two args: now, now.
const bumpVersion = `CASE WHEN ? > updated_at THEN ? ELSE updated_at + 1 END`
