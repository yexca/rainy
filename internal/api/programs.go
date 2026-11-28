package api

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"rainy/internal/events"
	"rainy/internal/model"
	"rainy/internal/store"
)

// maxBookmarkIDs bounds GET /api/bookmarks?trackIds=.
const maxBookmarkIDs = maxIDsPerRequest

// routesPrograms registers the program shows and the bookmarks used to resume them
// (docs/architecture/contract.md §7.2); mounted behind auth.RequireUser.
func (a *API) routesPrograms(r chi.Router) {
	r.Get("/programs", a.programShows)
	r.Get("/bookmarks", a.bookmarkList)
	r.Put("/bookmarks/{trackId}", a.bookmarkSet)
	r.Delete("/bookmarks/{trackId}", a.bookmarkDelete)
}

// browsePrograms returns the Programs filter of a browse list. Program episodes and shows
// are left out unless the request narrows the list (given ids, an album, a folder, the
// starred items) or, in an order other than random, asks for them with
// ?programs=include|only.
func browsePrograms(r *http.Request, narrowed bool, sort string) string {
	if narrowed {
		return store.ProgramsAll
	}
	if sort == "random" {
		return store.ProgramsExclude
	}
	switch r.URL.Query().Get("programs") {
	case "include":
		return store.ProgramsAll
	case "only":
		return store.ProgramsOnly
	}
	return store.ProgramsExclude
}

func (a *API) programShows(w http.ResponseWriter, r *http.Request) {
	shows, err := a.app.Store.ListProgramShows(r.Context(), userFrom(r).ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(shows))
}

// bookmarkList returns the user's bookmarks of ?trackIds (readable tracks only).
func (a *API) bookmarkList(w http.ResponseWriter, r *http.Request) {
	ids := queryList(r, "trackIds")
	if len(ids) > maxBookmarkIDs {
		writeErr(w, r, badRequest("at most %d track ids per request", maxBookmarkIDs))
		return
	}
	bms, err := a.app.Store.TrackBookmarks(r.Context(), userFrom(r).ID, ids)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(bms))
}

// bookmarkSet saves the user's position in a readable track (404 otherwise).
func (a *API) bookmarkSet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PositionMs *int64 `json:"positionMs"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.PositionMs == nil || *body.PositionMs < 0 {
		writeErr(w, r, badRequest("positionMs must be a non-negative number"))
		return
	}
	ctx, uid, id := r.Context(), userFrom(r).ID, chi.URLParam(r, "trackId")
	if _, err := a.app.Store.GetTrack(ctx, id, ""); err != nil {
		writeErr(w, r, err)
		return
	}
	b, err := a.app.Store.SetBookmarkPosition(ctx, uid, id, *body.PositionMs)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// bookmarkDelete removes the user's bookmark of a readable track; removing one that does
// not exist succeeds.
func (a *API) bookmarkDelete(w http.ResponseWriter, r *http.Request) {
	ctx, uid, id := r.Context(), userFrom(r).ID, chi.URLParam(r, "trackId")
	if _, err := a.app.Store.GetTrack(ctx, id, ""); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.app.Store.DeleteBookmark(ctx, uid, id); err != nil && !errors.Is(err, store.ErrNotFound) {
		writeErr(w, r, err)
		return
	}
	writeNoContent(w)
}

// ---- program folders (/api/manage, library managers)

type programFolderBody struct {
	LibraryID int64  `json:"libraryId"`
	Path      string `json:"path"`
}

func (a *API) manageProgramFolders(w http.ResponseWriter, r *http.Request) {
	programs, err := a.app.Store.ListPrograms(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, programs)
}

// manageMarkProgramFolder marks (PUT) or unmarks (DELETE) a program folder and answers
// with the program libraries and folders.
func (a *API) manageMarkProgramFolder(w http.ResponseWriter, r *http.Request) {
	var body programFolderBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.LibraryID <= 0 {
		writeErr(w, r, badRequest("libraryId is required"))
		return
	}
	if _, ok := model.CleanFolderPath(body.Path); !ok {
		writeErr(w, r, badRequest("path must be a folder of the library"))
		return
	}
	ctx := r.Context()
	var err error
	if r.Method == http.MethodDelete {
		err = a.app.Store.RemoveProgramFolder(ctx, body.LibraryID, body.Path)
	} else {
		err = a.app.Store.AddProgramFolder(ctx, body.LibraryID, body.Path)
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	a.app.Bus.Publish(events.Library("programs"))
	a.manageProgramFolders(w, r)
}
