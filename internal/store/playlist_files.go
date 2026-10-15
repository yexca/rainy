package store

import (
	"context"
	"fmt"

	"rainy/internal/util"
)

type PlaylistImportEntry struct {
	Line    int    `json:"line"`
	Path    string `json:"path"`
	Status  string `json:"status"` // matched | missing | ambiguous | invalid
	TrackID string `json:"trackId"`
}

type PlaylistImportLibrary struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type PlaylistImportPreview struct {
	Entries   []PlaylistImportEntry   `json:"entries"`
	Libraries []PlaylistImportLibrary `json:"libraries"`
}

// PreviewPlaylistImport matches exact indexed paths only, without reading media or
// fetching URLs. No playlist changes until the user submits the matched ids.
func (s *Store) PreviewPlaylistImport(ctx context.Context, text, baseDir string, libraryID int64) (*PlaylistImportPreview, error) {
	parsed, err := util.ParseM3U(text, baseDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrInvalid, err)
	}
	out := &PlaylistImportPreview{Entries: []PlaylistImportEntry{}, Libraries: []PlaylistImportLibrary{}}
	if err := s.db.R.SelectContext(ctx, &out.Libraries, "SELECT id, name FROM libraries ORDER BY name, id"); err != nil {
		return nil, err
	}
	if libraryID != 0 {
		found := false
		for _, lib := range out.Libraries {
			if lib.ID == libraryID {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("%w: unknown library", ErrInvalid)
		}
	}
	paths := []string{}
	hints := []string{}
	for _, entry := range parsed {
		if entry.Valid {
			paths = append(paths, entry.Path)
			if entry.TrackID != "" {
				hints = append(hints, entry.TrackID)
			}
		}
	}
	type match struct {
		Path, ID string
		Count    int
	}
	matches := map[string]match{}
	for _, batch := range chunks(dedupe(paths)) {
		query := "SELECT path, MIN(id) AS id, COUNT(*) AS count FROM tracks WHERE missing = 0 AND path IN (" + placeholders(len(batch)) + ")"
		args := anys(batch)
		if libraryID != 0 {
			query += " AND library_id = ?"
			args = append(args, libraryID)
		}
		var rows []match
		if err := s.db.R.SelectContext(ctx, &rows, query+" GROUP BY path", args...); err != nil {
			return nil, err
		}
		for _, row := range rows {
			matches[row.Path] = row
		}
	}
	hintPaths := map[string]string{}
	for _, batch := range chunks(dedupe(hints)) {
		query := "SELECT id, path FROM tracks WHERE missing = 0 AND id IN (" + placeholders(len(batch)) + ")"
		args := anys(batch)
		if libraryID != 0 {
			query += " AND library_id = ?"
			args = append(args, libraryID)
		}
		var rows []struct{ ID, Path string }
		if err := s.db.R.SelectContext(ctx, &rows, query, args...); err != nil {
			return nil, err
		}
		for _, row := range rows {
			hintPaths[row.ID] = row.Path
		}
	}
	for _, entry := range parsed {
		result := PlaylistImportEntry{Line: entry.Line, Path: entry.Path, Status: "invalid"}
		if entry.Valid {
			result.Status = "missing"
			m := matches[entry.Path]
			switch {
			case entry.TrackID != "" && hintPaths[entry.TrackID] == entry.Path:
				result.Status, result.TrackID = "matched", entry.TrackID
			case m.Count == 1:
				result.Status, result.TrackID = "matched", m.ID
			case m.Count > 1:
				result.Status = "ambiguous"
			}
		}
		out.Entries = append(out.Entries, result)
	}
	return out, nil
}
