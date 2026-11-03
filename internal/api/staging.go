package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"rainy/internal/manage"
)

// Downloads held for review (docs/architecture/contract.md §7.6 "staging"), registered by
// routesManage.
func (a *API) routesStaging(r chi.Router) {
	r.Get("/staging", a.manageStaging)
	r.Patch("/staging/{id}", a.manageUpdateStaged)
	r.Get("/staging/{id}/picture", a.manageStagedPicture)
	r.Post("/staging/import", a.manageImportStaged)
	r.Post("/staging/discard", a.manageDiscardStaged)
}

func (a *API) manageStaging(w http.ResponseWriter, r *http.Request) {
	items, err := a.app.Manage.ListStaged(r.Context())
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": nonNil(items)})
}

func (a *API) manageUpdateStaged(w http.ResponseWriter, r *http.Request) {
	var body manage.StagedUpdate
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	item, err := a.app.Manage.UpdateStaged(r.Context(), chi.URLParam(r, "id"), body)
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (a *API) manageStagedPicture(w http.ResponseWriter, r *http.Request) {
	data, ctype, err := a.app.Manage.StagedPicture(chi.URLParam(r, "id"))
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", pictureContentType(ctype))
	h.Set("Content-Length", strconv.Itoa(len(data)))
	h.Set("Cache-Control", "private, no-cache")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (a *API) manageImportStaged(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs        []string `json:"ids"`
		OnConflict string   `json:"onConflict"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	res, err := a.app.Manage.ImportStaged(r.Context(), userFrom(r), body.IDs, body.OnConflict)
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) manageDiscardStaged(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	n, err := a.app.Manage.DiscardStaged(body.IDs)
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"discarded": n})
}
