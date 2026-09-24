package subsonic

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"rainy/internal/artwork"
	"rainy/internal/lyrics"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/transcode"
	"rainy/internal/util"
)

// maxCoverSize bounds getCoverArt's size parameter.
const maxCoverSize = 3000

// trackPath returns the absolute path of a track's file (via util.SafeJoin).
func (a *API) trackPath(q *request, t *model.Track) (string, error) {
	lib, err := a.app.Store.GetLibrary(q.ctx, t.LibraryID)
	if err != nil {
		return "", notFoundAs(err, "Library")
	}
	abs, err := util.SafeJoin(lib.Path, t.Path)
	if err != nil {
		slog.Warn("subsonic: unsafe track path", "track", t.ID, "path", t.Path, "err", err)
		return "", errNotFound("Song file")
	}
	return abs, nil
}

// playableTrack loads a non-missing track and its file path from the "id" parameter.
func (a *API) playableTrack(q *request) (*model.Track, string, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, "", err
	}
	t, err := a.app.Store.GetTrack(q.ctx, id, q.user.ID)
	if err != nil {
		return nil, "", notFoundAs(err, "Song")
	}
	if t.Missing {
		return nil, "", errNotFound("Song file")
	}
	abs, err := a.trackPath(q, t)
	if err != nil {
		return nil, "", err
	}
	return t, abs, nil
}

// stream serves a song: the original file (Range requests supported) or an ffmpeg
// transcode (format, maxBitRate, timeOffset in seconds, estimateContentLength).
func (a *API) stream(q *request) (*Response, error) {
	t, abs, err := a.playableTrack(q)
	if err != nil {
		return nil, err
	}
	maxBitRate, err := q.intParam("maxBitRate", 0)
	if err != nil {
		return nil, err
	}
	offset, err := q.floatParam("timeOffset", 0)
	if err != nil {
		return nil, err
	}
	estimate, err := q.boolParam("estimateContentLength", false)
	if err != nil {
		return nil, err
	}
	requested := strings.ToLower(q.str("format"))
	set := q.appSettings()
	format, bitRate, doTranscode := transcode.Decide(t, requested, maxBitRate, set.TranscodeFormat, set.TranscodeBitrate)
	if !doTranscode && requested != transcode.FormatRaw && !transcode.IsFormat(requested) && maxBitRate <= 0 {
		if f := q.defaultTranscode(t); f != "" {
			format, bitRate, doTranscode = f, transcode.ClampBitRate(f, set.TranscodeBitrate), true
		}
	}
	if doTranscode && !a.app.Transcoder.Available() {
		slog.Warn("subsonic: ffmpeg unavailable, streaming original file", "track", t.ID)
		doTranscode = false
	}
	if !doTranscode {
		return nil, serveFile(q, abs, util.MimeType(t.Suffix), "")
	}
	bitRate = encoderBitRate(format, bitRate)
	return nil, a.streamTranscoded(q, t, abs, transcode.Options{Format: format, BitRate: bitRate, Offset: max(offset, 0)}, estimate)
}

// opusMaxBitRate is libopus' bit-rate ceiling for a mono source (kbps): ffmpeg refuses to
// open the encoder above it ("choose a value between 500 and 256000"). Stereo allows more,
// but 256 kbps Opus is already transparent and channel counts in tags can be wrong.
const opusMaxBitRate = 256

// encoderBitRate bounds kbps to what the encoder accepts for every source.
func encoderBitRate(format string, kbps int) int {
	if format == transcode.FormatOpus {
		return min(kbps, opusMaxBitRate)
	}
	return kbps
}

// serveFile serves a file with Range / conditional request support. A non-empty
// attachment name adds Content-Disposition: attachment.
func serveFile(q *request, abs, contentType, attachment string) error {
	f, err := os.Open(abs)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errNotFound("Song file")
		}
		return fmt.Errorf("opening %s: %w", abs, err)
	}
	defer func() { _ = f.Close() }()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return errNotFound("Song file")
	}
	h := q.w.Header()
	h.Set("Content-Type", contentType)
	h.Set("Cache-Control", "private, max-age=0")
	if attachment != "" {
		h.Set("Content-Disposition", contentDisposition(attachment))
	}
	http.ServeContent(q.w, q.r, "", fi.ModTime(), f)
	return nil
}

// countingWriter records whether anything was written (to know if an error envelope can
// still be sent).
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// estimateSlack is added to estimated transcode lengths (headers, frame rounding).
const estimateSlack = 16 << 10

// errLimitReached stops a transcode once the declared Content-Length has been sent.
var errLimitReached = errors.New("declared content length reached")

// cappedWriter forwards at most limit bytes. When the encoder overshoots an estimated
// Content-Length, the excess is dropped (the tail of the song is cut) instead of making
// net/http reject the write and leaving the client with a short, broken body.
type cappedWriter struct {
	w     io.Writer
	limit int64
	n     int64
}

func (c *cappedWriter) Write(p []byte) (int, error) {
	room := c.limit - c.n
	if room <= 0 {
		return 0, errLimitReached
	}
	short := int64(len(p)) > room
	if short {
		p = p[:room]
	}
	n, err := c.w.Write(p)
	c.n += int64(n)
	if err == nil && short {
		err = errLimitReached
	}
	return n, err
}

// zeroReader yields zero bytes (padding for estimated content lengths).
type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

// streamTranscoded pipes ffmpeg's output to the client.
func (a *API) streamTranscoded(q *request, t *model.Track, abs string, o transcode.Options, estimate bool) error {
	remaining := max(t.Duration-o.Offset, 0)
	h := q.w.Header()
	h.Set("Content-Type", transcode.ContentType(o.Format))
	h.Set("Cache-Control", "no-store")
	h.Set("Accept-Ranges", "none")
	h.Set("X-Content-Duration", strconv.FormatFloat(remaining, 'f', 2, 64))
	var declared int64
	if estimate && remaining > 0 {
		// bit rate × duration plus headroom for container/encoder overhead and VBR
		// overshoot; the output is zero-padded to this length below so the declared
		// length is honoured (decoders ignore trailing zeros).
		declared = int64(remaining*float64(o.BitRate)*1000/8*1.02) + estimateSlack
		h.Set("Content-Length", strconv.FormatInt(declared, 10))
	}
	if q.r.Method == http.MethodHead {
		q.w.WriteHeader(http.StatusOK)
		return nil
	}
	var out io.Writer = q.w
	if declared > 0 {
		out = &cappedWriter{w: q.w, limit: declared}
	}
	cw := &countingWriter{w: out}
	err := a.app.Transcoder.Stream(q.ctx, cw, abs, o)
	if q.ctx.Err() != nil {
		return nil
	}
	if errors.Is(err, errLimitReached) {
		slog.Debug("subsonic: transcode exceeded the estimated length; output truncated", "track", t.ID)
		return nil
	}
	if err == nil {
		if pad := declared - cw.n; declared > 0 && pad > 0 {
			_, _ = io.CopyN(q.w, zeroReader{}, pad)
		}
		return nil
	}
	if cw.n == 0 {
		for _, k := range []string{"Content-Length", "X-Content-Duration", "Accept-Ranges"} {
			h.Del(k)
		}
		return fmt.Errorf("transcoding %s: %w", t.ID, err)
	}
	slog.Warn("subsonic: transcode interrupted", "track", t.ID, "err", err)
	return nil
}

// canDownload applies the download permission and the enableDownloads setting.
func (q *request) canDownload() bool {
	return q.user.IsAdmin || (q.user.CanDownload && q.appSettings().EnableDownloads)
}

// download sends the original file of a song, or a zip of an album's or playlist's songs.
func (a *API) download(q *request) (*Response, error) {
	if !q.canDownload() {
		return nil, errForbidden()
	}
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	if p, err := a.app.Store.GetPlaylist(q.ctx, id); err == nil {
		if !canReadPlaylist(p, q.user) {
			return nil, errNotFound("Playlist")
		}
		tracks, err := a.app.Store.PlaylistTracks(q.ctx, id, q.user.ID)
		if err != nil {
			return nil, err
		}
		return nil, a.sendZip(q, p.Name, tracks, true)
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	kind, err := a.itemKind(q.ctx, id)
	if err != nil {
		return nil, errNotFound("Item")
	}
	switch kind {
	case kindTrack:
		t, abs, err := a.playableTrack(q)
		if err != nil {
			return nil, err
		}
		return nil, serveFile(q, abs, util.MimeType(t.Suffix), path.Base(t.Path))
	case kindAlbum:
		al, err := a.app.Store.GetAlbum(q.ctx, id, q.user.ID)
		if err != nil {
			return nil, err
		}
		tracks, _, err := a.app.Store.ListTracks(q.ctx, store.TrackQuery{UserID: q.user.ID, AlbumID: id, Sort: "track"})
		if err != nil {
			return nil, err
		}
		return nil, a.sendZip(q, al.Name, tracks, false)
	}
	return nil, errNotFound("Item")
}

// contentDisposition renders an attachment header (RFC 2231 encoding for non-ASCII names).
func contentDisposition(filename string) string {
	if v := mime.FormatMediaType("attachment", map[string]string{"filename": filename}); v != "" {
		return v
	}
	return "attachment"
}

// zipName makes a safe archive/file name.
func zipName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)
	s = strings.Trim(s, " .")
	if s == "" {
		return "download"
	}
	return s
}

// sendZip streams an uncompressed zip of the tracks' files.
func (a *API) sendZip(q *request, name string, tracks []model.Track, numbered bool) error {
	if len(tracks) == 0 {
		return errNotFound("Songs")
	}
	h := q.w.Header()
	h.Set("Content-Type", "application/zip")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Disposition", contentDisposition(zipName(name)+".zip"))
	if q.r.Method == http.MethodHead {
		q.w.WriteHeader(http.StatusOK)
		return nil
	}
	maxDisc := 0
	for _, t := range tracks {
		maxDisc = max(maxDisc, t.DiscNumber)
	}
	zw := zip.NewWriter(q.w)
	used := map[string]int{}
	for i := range tracks {
		t := &tracks[i]
		abs, err := a.trackPath(q, t)
		if err != nil {
			continue
		}
		entry := zipName(path.Base(t.Path))
		if numbered {
			entry = fmt.Sprintf("%03d - %s", i+1, entry)
		} else if maxDisc > 1 && t.DiscNumber > 0 {
			entry = fmt.Sprintf("Disc %d/%s", t.DiscNumber, entry)
		}
		key := strings.ToLower(entry)
		if n := used[key]; n > 0 {
			ext := path.Ext(entry)
			entry = fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(entry, ext), n+1, ext)
		}
		file, err := os.Open(abs)
		if err != nil {
			slog.Warn("subsonic: zip entry skipped", "track", t.ID, "err", err)
			continue
		}
		err = addZipFile(zw, file, entry)
		_ = file.Close()
		if err != nil {
			if q.ctx.Err() == nil {
				slog.Warn("subsonic: zip stream aborted", "track", t.ID, "err", err)
			}
			return nil // headers are sent; abort the stream
		}
		used[key]++
	}
	if err := zw.Close(); err != nil {
		slog.Warn("subsonic: finishing zip", "err", err)
	}
	return nil
}

func addZipFile(zw *zip.Writer, f *os.File, name string) error {
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store, Modified: fi.ModTime()})
	if err != nil {
		return err
	}
	_, err = io.Copy(w, f)
	return err
}

// getCoverArt serves cover art for a cover-art id (or a bare album/track/artist/playlist
// id), falling back to a placeholder image so clients always get a picture.
func (a *API) getCoverArt(q *request) (*Response, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	size, err := q.intParam("size", 0)
	if err != nil {
		return nil, err
	}
	size = min(max(size, 0), maxCoverSize)
	img, err := a.app.Artwork.Get(q.ctx, id, size)
	cache := "public, max-age=86400"
	switch {
	case err == nil:
		if _, _, version := util.SplitCoverArtID(id); version != "" {
			cache = "public, max-age=31536000, immutable"
		}
	case errors.Is(err, artwork.ErrNotFound), errors.Is(err, store.ErrNotFound), errors.Is(err, errors.ErrUnsupported):
		img = a.app.Artwork.Placeholder(size)
		cache = "public, max-age=3600"
	case q.ctx.Err() != nil:
		return nil, q.ctx.Err()
	default:
		// An unreadable or corrupt image must not leave clients without a picture.
		slog.Warn("subsonic: loading cover art", "id", id, "err", err)
		img = a.app.Artwork.Placeholder(size)
		cache = "no-cache"
	}
	if img == nil {
		return nil, errNotFound("Cover art")
	}
	h := q.w.Header()
	h.Set("Content-Type", img.ContentType)
	h.Set("Cache-Control", cache)
	http.ServeContent(q.w, q.r, "", img.ModTime, bytes.NewReader(img.Data))
	return nil, nil
}

// getAvatar: Rainy has no user avatars; known users get error 70 so clients fall back
// to their own default picture.
func (a *API) getAvatar(q *request) (*Response, error) {
	name, err := q.requiredStr("username")
	if err != nil {
		return nil, err
	}
	if _, err := a.app.Store.GetUserByUsername(q.ctx, name); err != nil {
		return nil, notFoundAs(err, "User")
	}
	return nil, errNotFound("Avatar")
}

// loadLyrics loads a track's lyrics (sidecar .lrc, then embedded); errors are logged and
// treated as "no lyrics".
func (a *API) loadLyrics(q *request, t *model.Track) *lyrics.Lyrics {
	abs, err := a.trackPath(q, t)
	if err != nil {
		abs = ""
	}
	l, err := lyrics.Load(t, abs)
	if err != nil || l == nil {
		if err != nil && !errors.Is(err, errors.ErrUnsupported) {
			slog.Warn("subsonic: loading lyrics", "track", t.ID, "err", err)
		}
		return &lyrics.Lyrics{Lines: []lyrics.Line{}, Source: "none"}
	}
	return l
}

// getLyrics (legacy) finds a song by artist and title and returns its lyrics as text.
func (a *API) getLyrics(q *request) (*Response, error) {
	artist, title := q.str("artist"), q.str("title")
	resp := newResponse()
	resp.Lyrics = &Lyrics{}
	if title == "" {
		return resp, nil
	}
	candidates, _, err := a.app.Store.ListTracks(q.ctx, store.TrackQuery{Q: artist + " " + title, Sort: "title", Limit: 50})
	if err != nil {
		return nil, err
	}
	var best *model.Track
	for i := range candidates {
		c := &candidates[i]
		if !strings.EqualFold(c.Title, title) {
			continue
		}
		if artist == "" || strings.EqualFold(c.Artist, artist) || strings.EqualFold(c.AlbumArtist, artist) {
			if c.HasLyrics {
				best = c
				break
			}
			if best == nil {
				best = c
			}
		}
	}
	if best == nil {
		return resp, nil
	}
	t, err := a.app.Store.GetTrack(q.ctx, best.ID, "")
	if err != nil {
		return nil, err
	}
	l := a.loadLyrics(q, t)
	if len(l.Lines) == 0 {
		return resp, nil
	}
	texts := make([]string, 0, len(l.Lines))
	for _, line := range l.Lines {
		texts = append(texts, line.Text)
	}
	resp.Lyrics = &Lyrics{Artist: t.Artist, Title: t.Title, Value: strings.Join(texts, "\n")}
	return resp, nil
}

// getLyricsBySongId returns structured (synced when available) lyrics of a song
// (OpenSubsonic songLyrics). Line starts already include the LRC [offset:].
func (a *API) getLyricsBySongID(q *request) (*Response, error) {
	id, err := q.requiredStr("id")
	if err != nil {
		return nil, err
	}
	t, err := a.app.Store.GetTrack(q.ctx, id, q.user.ID)
	if err != nil {
		return nil, notFoundAs(err, "Song")
	}
	resp := newResponse()
	resp.LyricsList = &LyricsList{StructuredLyrics: []StructuredLyrics{}}
	l := a.loadLyrics(q, t)
	if len(l.Lines) == 0 {
		return resp, nil
	}
	lang := l.Lang
	if lang == "" {
		lang = "xxx"
	}
	sl := StructuredLyrics{DisplayArtist: t.Artist, DisplayTitle: t.Title, Lang: lang, Synced: l.Synced,
		Lines: make([]LyricLine, 0, len(l.Lines))}
	for _, line := range l.Lines {
		ll := LyricLine{Value: line.Text}
		if l.Synced && line.Start >= 0 {
			start := line.Start
			ll.Start = &start
		}
		sl.Lines = append(sl.Lines, ll)
	}
	resp.LyricsList.StructuredLyrics = append(resp.LyricsList.StructuredLyrics, sl)
	return resp, nil
}
