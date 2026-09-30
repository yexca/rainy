package api

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"rainy/internal/config"
	"rainy/internal/manage"
	"rainy/internal/model"
)

// routesManage registers the library management endpoints under /api/manage (paths
// relative, e.g. "/tracks/{id}/tags"); mounted behind auth.RequireManager.
//
// Owner: manage agent (D) (docs/architecture/contract.md §7.6).
func (a *API) routesManage(r chi.Router) {
	r.Get("/tracks/{id}/tags", a.manageTrackTags)
	r.Post("/tags", a.manageSaveTags)
	r.Post("/tags/rebuild", a.manageRebuildTags)
	r.Get("/tracks/{id}/picture", a.managePicture)
	r.Post("/cover", a.manageSetCover)
	r.Delete("/cover", a.manageRemoveCover)
	r.Put("/tracks/{id}/lyrics", a.manageSetLyrics)
	r.Post("/rename/preview", a.manageRenamePreview)
	r.Post("/rename", a.manageRename)
	r.Post("/upload", a.manageUpload)
	r.Post("/delete", a.manageDelete)
	r.Get("/trash", a.manageTrash)
	r.Post("/trash/restore", a.manageRestore)
	r.Post("/trash/purge", a.managePurgeTrash)
	r.Post("/missing/purge", a.managePurgeMissing)
	r.Get("/folders", a.manageFolders)
	r.Post("/folders/rescan", a.manageRescanFolder)
	r.Get("/issues/summary", a.manageIssueSummary)
	r.Get("/issues", a.manageIssues)
	r.Post("/encoding", a.manageEncoding)
	r.Get("/log", a.manageLog)
	r.Get("/metadata", a.manageMetadataStatus)
	r.Get("/metadata/search", a.manageMetadataSearch)
	r.Get("/metadata/lyrics", a.manageMetadataLyrics)
	r.Get("/metadata/cover", a.manageMetadataCover)
	r.Get("/downloads", a.manageDownloads)
	r.Post("/downloads", a.manageStartDownload)
	r.Delete("/downloads/{id}", a.manageRemoveDownload)
}

// maxUploadRequest caps a whole upload request (the per-file limit is
// manage.MaxUploadFileSize); files are staged in the data directory, so this bounds how
// much of it one request can fill.
var maxUploadRequest int64 = 32 << 30

// writeManageErr maps manage errors (readonly with its explanatory message, 413 for an
// oversized request body) and falls back to writeErr.
func writeManageErr(w http.ResponseWriter, r *http.Request, err error) {
	var re *manage.ReadonlyError
	if errors.As(err, &re) {
		writeErr(w, r, newError(http.StatusConflict, CodeReadonly, "%s", re.Error()))
		return
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		writeErr(w, r, newError(http.StatusRequestEntityTooLarge, CodeBadRequest,
			"the request is too large (max %d GiB per upload; send the files in several batches)", mbe.Limit>>30))
		return
	}
	writeErr(w, r, err)
}

// pictureContentType restricts the type of an embedded picture (sniffed from untrusted
// file content) to raster images; anything else is served as an opaque download so it
// can never render as a document in the app's origin.
func pictureContentType(sniffed string) string {
	switch mt, _, _ := mime.ParseMediaType(sniffed); mt {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp":
		return mt
	}
	return "application/octet-stream"
}

func (a *API) manageTrackTags(w http.ResponseWriter, r *http.Request) {
	tt, err := a.app.Manage.TrackTags(r.Context(), chi.URLParam(r, "id"), userFrom(r).ID)
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tt)
}

func (a *API) manageSaveTags(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Edits []manage.TagEdit `json:"edits"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	res, err := a.app.Manage.SaveTags(r.Context(), userFrom(r), body.Edits)
	writeBatch(w, r, res, err)
}

func (a *API) manageRebuildTags(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TrackIDs []string `json:"trackIds"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	res, err := a.app.Manage.RebuildTags(r.Context(), userFrom(r), body.TrackIDs)
	writeBatch(w, r, res, err)
}

func writeBatch(w http.ResponseWriter, r *http.Request, res *manage.BatchResult, err error) {
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) managePicture(w http.ResponseWriter, r *http.Request) {
	data, ctype, err := a.app.Manage.Picture(r.Context(), chi.URLParam(r, "id"))
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

// formBool parses an optional boolean form value.
func formBool(v string, def bool) (bool, error) {
	if strings.TrimSpace(v) == "" {
		return def, nil
	}
	return config.ParseBool(v)
}

func splitList(v string) []string {
	var out []string
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// isMultipart reports whether the request body is multipart/form-data.
func isMultipart(r *http.Request) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && mt == "multipart/form-data"
}

func (a *API) manageSetCover(w http.ResponseWriter, r *http.Request) {
	if !isMultipart(r) {
		writeErr(w, r, badRequest("expected multipart/form-data with a \"file\" part"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, manage.MaxCoverUpload+1<<20)
	mr, err := r.MultipartReader()
	if err != nil {
		writeErr(w, r, badRequest("invalid multipart body: %v", err))
		return
	}
	req := manage.CoverRequest{}
	fields := map[string]string{}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeErr(w, r, badRequest("invalid multipart body: %v", err))
			return
		}
		switch name := part.FormName(); name {
		case "file":
			b, err := io.ReadAll(io.LimitReader(part, manage.MaxCoverUpload+1))
			_ = part.Close()
			if err != nil {
				writeErr(w, r, badRequest("reading image: %v", err))
				return
			}
			if len(b) > manage.MaxCoverUpload {
				writeErr(w, r, newError(http.StatusRequestEntityTooLarge, CodeBadRequest, "image too large (max %d MiB)", manage.MaxCoverUpload>>20))
				return
			}
			req.Image = b
		case "trackIds", "albumId", "embed", "saveToFolder":
			b, err := io.ReadAll(io.LimitReader(part, 1<<20))
			_ = part.Close()
			if err != nil {
				writeErr(w, r, badRequest("reading field %s: %v", name, err))
				return
			}
			fields[name] = string(b)
		default:
			_ = part.Close()
		}
	}
	if req.Image == nil {
		writeErr(w, r, badRequest("missing \"file\" part"))
		return
	}
	req.TrackIDs = splitList(fields["trackIds"])
	req.AlbumID = strings.TrimSpace(fields["albumId"])
	if req.Embed, err = formBool(fields["embed"], true); err != nil {
		writeErr(w, r, badRequest("invalid embed flag"))
		return
	}
	if req.SaveToFolder, err = formBool(fields["saveToFolder"], false); err != nil {
		writeErr(w, r, badRequest("invalid saveToFolder flag"))
		return
	}
	if len(req.TrackIDs) == 0 && req.AlbumID == "" {
		writeErr(w, r, badRequest("trackIds or albumId is required"))
		return
	}
	res, err := a.app.Manage.SetCover(r.Context(), userFrom(r), req)
	writeBatch(w, r, res, err)
}

func (a *API) manageRemoveCover(w http.ResponseWriter, r *http.Request) {
	var body manage.RemoveCoverRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if len(body.TrackIDs) == 0 && body.AlbumID == "" {
		writeErr(w, r, badRequest("trackIds or albumId is required"))
		return
	}
	res, err := a.app.Manage.RemoveCover(r.Context(), userFrom(r), body)
	writeBatch(w, r, res, err)
}

func (a *API) manageSetLyrics(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Text   string `json:"text"`
		Target string `json:"target"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	t, err := a.app.Manage.SetLyrics(r.Context(), userFrom(r), chi.URLParam(r, "id"), body.Text, body.Target)
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

type renameBody struct {
	TrackIDs []string `json:"trackIds"`
	Pattern  string   `json:"pattern"`
}

func (a *API) manageRenamePreview(w http.ResponseWriter, r *http.Request) {
	var body renameBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	plans, err := a.app.Manage.PreviewRename(r.Context(), body.TrackIDs, body.Pattern)
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": nonNil(plans)})
}

func (a *API) manageRename(w http.ResponseWriter, r *http.Request) {
	var body renameBody
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	res, err := a.app.Manage.Rename(r.Context(), userFrom(r), body.TrackIDs, body.Pattern)
	writeBatch(w, r, res, err)
}

func (a *API) manageUpload(w http.ResponseWriter, r *http.Request) {
	if !isMultipart(r) {
		writeErr(w, r, badRequest("expected multipart/form-data"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadRequest)
	mr, err := r.MultipartReader()
	if err != nil {
		writeErr(w, r, badRequest("invalid multipart body: %v", err))
		return
	}
	res, err := a.app.Manage.Upload(r.Context(), userFrom(r), mr)
	writeBatch(w, r, res, err)
}

func (a *API) manageDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TrackIDs []string `json:"trackIds"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	res, err := a.app.Manage.Delete(r.Context(), userFrom(r), body.TrackIDs)
	writeBatch(w, r, res, err)
}

func (a *API) manageTrash(w http.ResponseWriter, r *http.Request) {
	entries, err := a.app.Manage.ListTrash(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, nonNil(entries))
}

func (a *API) manageRestore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	res, err := a.app.Manage.Restore(r.Context(), userFrom(r), body.IDs)
	writeBatch(w, r, res, err)
}

func (a *API) managePurgeTrash(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	n, err := a.app.Manage.PurgeTrash(r.Context(), userFrom(r), body.IDs)
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"purged": n})
}

func (a *API) managePurgeMissing(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TrackIDs []string `json:"trackIds"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	n, err := a.app.Manage.PurgeMissing(r.Context(), userFrom(r), body.TrackIDs)
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"purged": n})
}

func (a *API) manageFolders(w http.ResponseWriter, r *http.Request) {
	listing, err := a.app.Manage.ListFolder(r.Context(), queryInt64(r, "libraryId", 0), r.URL.Query().Get("dir"))
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, listing)
}

func (a *API) manageRescanFolder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LibraryID int64  `json:"libraryId"`
		Dir       string `json:"dir"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := a.app.Manage.RescanFolder(r.Context(), body.LibraryID, body.Dir); err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (a *API) manageIssueSummary(w http.ResponseWriter, r *http.Request) {
	sum, err := a.app.Manage.IssueSummary(r.Context())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, sum)
}

func (a *API) manageIssues(w http.ResponseWriter, r *http.Request) {
	offset, limit, _, _ := pageParams(r, 50, 200)
	items, total, err := a.app.Manage.Issues(r.Context(), userFrom(r).ID, r.URL.Query().Get("type"), offset, limit)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writePage(w, items, total)
}

func (a *API) manageEncoding(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TrackIDs []string `json:"trackIds"`
		Apply    bool     `json:"apply"`
	}
	if err := decodeJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	res, err := a.app.Manage.Encoding(r.Context(), userFrom(r), body.TrackIDs, body.Apply)
	if err != nil {
		writeManageErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *API) manageLog(w http.ResponseWriter, r *http.Request) {
	offset, limit, _, _ := pageParams(r, 50, 1000)
	entries, total, err := a.app.Manage.EditLog(r.Context(), strings.TrimSpace(r.URL.Query().Get("trackId")), offset, limit)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writePage[model.EditLogEntry](w, entries, total)
}
