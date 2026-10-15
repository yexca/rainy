package api

import (
	"mime"
	"net/http"

	"rainy/internal/util"
)

func (a *API) plImportPreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text      string `json:"text"`
		BaseDir   string `json:"baseDir"`
		LibraryID int64  `json:"libraryId"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	preview, err := a.app.Store.PreviewPlaylistImport(r.Context(), body.Text, body.BaseDir, body.LibraryID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, preview)
}

func (a *API) plExport(w http.ResponseWriter, r *http.Request) {
	p, err := a.loadPlaylist(r, false)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	tracks, err := a.app.Store.PlaylistTracks(r.Context(), p.ID, userFrom(r).ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	entries := make([]util.M3UTrack, 0, len(tracks))
	for _, t := range tracks {
		entries = append(entries, util.M3UTrack{ID: t.ID, Path: t.Path, Artist: t.Artist, Title: t.Title, Duration: t.Duration})
	}
	text, err := util.FormatM3U(entries)
	if err != nil {
		writeErr(w, r, badRequest("%s", err))
		return
	}
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl; charset=utf-8")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": "playlist-" + p.ID + ".m3u8"}))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte(text))
}
