package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"rainy/internal/lyrics"
	"rainy/internal/model"
	"rainy/internal/transcode"
	"rainy/internal/util"
)

// coverSizes are the sizes /api/cover snaps requests to (bounded disk cache).
var coverSizes = []int{64, 128, 256, 512, 1024, 2048}

// routesMedia registers /stream, /download, /cover and /lyrics (docs/architecture/contract.md §7.5);
// mounted behind auth.RequireUser.
func (a *API) routesMedia(r chi.Router) {
	r.Get("/stream/{trackId}", a.mediaStream)
	r.Head("/stream/{trackId}", a.mediaStream)
	r.Get("/download/{trackId}", a.mediaDownload)
	r.Head("/download/{trackId}", a.mediaDownload)
	r.Get("/download/album/{albumId}", a.mediaDownloadAlbum)
	r.Get("/cover/{coverArtId}", a.mediaCover)
	r.Get("/lyrics/{trackId}", a.mediaLyrics)
}

// errFileMissing is returned when a track's file is not on disk.
var errFileMissing = notFound("the audio file is missing from the library folder")

// mediaTrackPath returns the absolute path of a track's file (built with util.SafeJoin).
func (a *API) mediaTrackPath(ctx context.Context, t *model.Track) (string, error) {
	lib, err := a.app.Store.GetLibrary(ctx, t.LibraryID)
	if err != nil {
		return "", fmt.Errorf("library of track %s: %w", t.ID, err)
	}
	return util.SafeJoin(lib.Path, t.Path)
}

// mediaTrack loads the track of the {trackId} URL parameter and its file path.
func (a *API) mediaTrack(r *http.Request) (*model.Track, string, error) {
	ctx := r.Context()
	t, err := a.app.Store.GetTrack(ctx, chi.URLParam(r, "trackId"), userFrom(r).ID)
	if err != nil {
		return nil, "", err
	}
	abs, err := a.mediaTrackPath(ctx, t)
	if err != nil {
		return nil, "", err
	}
	return t, abs, nil
}

// fileETag is a strong validator for a file version (size + modification time).
func fileETag(fi fs.FileInfo) string {
	return fmt.Sprintf(`"%x-%x"`, fi.Size(), fi.ModTime().UnixNano())
}

// serveFile serves a regular file with Range / conditional request support. A non-empty
// attachment name adds Content-Disposition: attachment.
func serveFile(w http.ResponseWriter, r *http.Request, abs, contentType, attachment string) {
	f, err := os.Open(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = errFileMissing
		}
		writeErr(w, r, err)
		return
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if !fi.Mode().IsRegular() {
		writeErr(w, r, errFileMissing)
		return
	}
	h := w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", "private, no-cache")
	h.Set("ETag", fileETag(fi))
	h.Set("X-Content-Type-Options", "nosniff")
	if attachment != "" {
		h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": attachment}))
	}
	http.ServeContent(w, r, "", fi.ModTime(), f)
}

// ---- stream

// streamOffset parses ?offset (seconds, ≥ 0).
func streamOffset(r *http.Request) float64 {
	v, err := strconv.ParseFloat(r.URL.Query().Get("offset"), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0
	}
	return v
}

// mediaStream serves audio: the original file (Range support) or an ffmpeg transcode
// (?format=raw|mp3|opus|aac, ?bitrate=kbps, ?offset=seconds).
func (a *API) mediaStream(w http.ResponseWriter, r *http.Request) {
	t, abs, err := a.mediaTrack(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	settings := a.app.Settings(r.Context())
	requested := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	maxBitRate := queryInt(r, "bitrate", 0)
	offset := streamOffset(r)
	format, bitRate, doTranscode := transcode.Decide(t, requested, maxBitRate,
		settings.TranscodeFormat, settings.TranscodeBitrate)
	if !doTranscode && offset > 0 && transcode.IsFormat(requested) {
		// The file already is in the requested format, but only a transcode can start at an
		// offset (clients seek live transcodes by re-requesting with ?offset).
		br := maxBitRate
		if br <= 0 {
			br = max(t.Bitrate, settings.TranscodeBitrate)
		}
		format, bitRate, doTranscode = requested, transcode.ClampBitRate(requested, br), true
	}
	if doTranscode && !a.app.Transcoder.Available() {
		slog.Warn("stream: transcoding requested but ffmpeg is unavailable; serving the original file",
			"track", t.ID, "ffmpeg", a.app.Transcoder.Path())
		doTranscode = false
	}
	if !doTranscode {
		serveFile(w, r, abs, util.MimeType(t.Suffix), "")
		return
	}
	a.streamTranscoded(w, r, t, abs, transcode.Options{Format: format, BitRate: bitRate, Offset: offset})
}

// countingWriter records whether anything was written (after which errors can no longer
// be reported as JSON).
type countingWriter struct {
	w http.ResponseWriter
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func (a *API) streamTranscoded(w http.ResponseWriter, r *http.Request, t *model.Track, abs string, o transcode.Options) {
	if fi, err := os.Stat(abs); err != nil || !fi.Mode().IsRegular() {
		if err == nil || errors.Is(err, fs.ErrNotExist) {
			err = errFileMissing
		}
		writeErr(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", transcode.ContentType(o.Format))
	h.Set("Cache-Control", "no-store")
	h.Set("Accept-Ranges", "none")
	h.Set("X-Content-Type-Options", "nosniff")
	if t.Duration > 0 {
		h.Set("X-Content-Duration", strconv.FormatFloat(max(t.Duration-o.Offset, 0), 'f', 2, 64))
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	cw := &countingWriter{w: w}
	err := a.app.Transcoder.Stream(r.Context(), cw, abs, o)
	switch {
	case err == nil:
	case cw.n == 0 && r.Context().Err() == nil:
		for _, k := range []string{"Content-Type", "Accept-Ranges", "X-Content-Duration"} {
			h.Del(k)
		}
		if errors.Is(err, transcode.ErrUnavailable) {
			err = newError(http.StatusServiceUnavailable, CodeUnavailable, "transcoding is unavailable (ffmpeg not found)")
		}
		writeErr(w, r, err)
	case r.Context().Err() == nil:
		slog.Warn("stream: transcode failed mid-stream", "track", t.ID, "err", err)
	}
}

// ---- download

// checkDownload enforces the user's download permission and the server setting.
func (a *API) checkDownload(r *http.Request) error {
	u := userFrom(r)
	if !u.CanDownload && !u.IsAdmin {
		return forbidden("downloads are not allowed for your account")
	}
	if !a.app.Settings(r.Context()).EnableDownloads {
		return forbidden("downloads are disabled on this server")
	}
	return nil
}

func (a *API) mediaDownload(w http.ResponseWriter, r *http.Request) {
	if err := a.checkDownload(r); err != nil {
		writeErr(w, r, err)
		return
	}
	t, abs, err := a.mediaTrack(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	name := t.Filename
	if name == "" {
		name = path.Base(t.Path)
	}
	serveFile(w, r, abs, util.MimeType(t.Suffix), sanitizeFileName(name))
}

// ---- cover

// snapCoverSize maps a requested size to the smallest standard size that is at least as
// large (0 = original; capped at the largest standard size).
func snapCoverSize(size int) int {
	if size <= 0 {
		return 0
	}
	for _, s := range coverSizes {
		if size <= s {
			return s
		}
	}
	return coverSizes[len(coverSizes)-1]
}

func (a *API) mediaCover(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "coverArtId")
	img, err := a.app.Artwork.Get(r.Context(), id, snapCoverSize(queryInt(r, "size", 0)))
	if err != nil {
		writeErr(w, r, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", img.ContentType)
	if _, _, version := util.SplitCoverArtID(id); version != "" {
		h.Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		h.Set("Cache-Control", "private, no-cache")
	}
	sum := fnv.New64a()
	_, _ = sum.Write(img.Data)
	h.Set("ETag", fmt.Sprintf(`"%x"`, sum.Sum64()))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	// A zero ModTime simply omits Last-Modified; the ETag still allows revalidation.
	http.ServeContent(w, r, "", img.ModTime, bytes.NewReader(img.Data))
}

// ---- lyrics

func (a *API) mediaLyrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	t, err := a.app.Store.GetTrack(ctx, chi.URLParam(r, "trackId"), userFrom(r).ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	abs, err := a.mediaTrackPath(ctx, t)
	if err != nil {
		// Without a usable library path only embedded lyrics can be served.
		slog.Warn("lyrics: resolving track path", "track", t.ID, "err", err)
		abs = ""
	}
	l, err := lyrics.Load(t, abs)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}
