package ytdlp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"rainy/internal/tags"
	"rainy/internal/util"
)

// Audio formats. FormatBest keeps the original audio stream (Opus from YouTube, AAC from
// bilibili) without re-encoding; the others convert with ffmpeg.
const (
	FormatBest = "best"
	FormatM4A  = "m4a"
	FormatMP3  = "mp3"
	FormatOpus = "opus"
)

// ValidFormat reports whether f is a supported format ("" means FormatBest).
func ValidFormat(f string) bool {
	switch f {
	case "", FormatBest, FormatM4A, FormatMP3, FormatOpus:
		return true
	}
	return false
}

const (
	// MaxPlaylistItems caps the entries of one playlist download.
	MaxPlaylistItems = 100
	// maxFileSize is passed to yt-dlp (the upload limit per file).
	maxFileSize = "2G"
	waitDelay   = 10 * time.Second
)

// Request describes one download.
type Request struct {
	Target   Target
	Format   string // FormatBest (default), FormatM4A, FormatMP3 or FormatOpus
	Playlist bool   // download every entry (up to MaxPlaylistItems) instead of just the linked one
	Dir      string // empty work directory owned by the caller; removed by the caller
}

// Item is one downloaded audio file with the metadata yt-dlp reported for it.
type Item struct {
	Path          string // absolute, inside Request.Dir
	Thumbnail     string // absolute path of the downloaded thumbnail (JPEG), "" if none
	ID            string
	Title         string
	Artists       []string
	Album         string
	AlbumArtists  []string
	TrackNumber   int
	Date          string // YYYY-MM-DD or YYYY
	WebpageURL    string
	PlaylistTitle string
	PlaylistIndex int
}

// Tags returns the TagLib properties Rainy writes into the file (only non-empty ones).
func (it Item) Tags() map[string][]string {
	out := map[string][]string{}
	set := func(k string, v ...string) {
		var keep []string
		for _, s := range v {
			if s = strings.TrimSpace(s); s != "" {
				keep = append(keep, s)
			}
		}
		if len(keep) > 0 {
			out[k] = keep
		}
	}
	set("TITLE", it.Title)
	set("ARTIST", it.Artists...)
	set("ALBUM", it.Album)
	set("ALBUMARTIST", it.AlbumArtists...)
	set("DATE", it.Date)
	if it.TrackNumber > 0 {
		set("TRACKNUMBER", strconv.Itoa(it.TrackNumber))
	}
	set("COMMENT", it.WebpageURL)
	return out
}

// Progress reports a running download.
type Progress struct {
	Phase    string  // PhaseDownloading or PhaseProcessing
	Item     int     // 1-based entry being downloaded (playlists), 0 = unknown
	Items    int     // number of entries, 0 = unknown
	Title    string  // title of the current entry
	Fraction float64 // 0…1 of the current entry, -1 = unknown
	Speed    float64 // bytes per second, 0 = unknown
	ETA      int     // seconds, -1 = unknown
}

// Download phases.
const (
	PhaseDownloading = "downloading"
	PhaseProcessing  = "processing"
)

// argsInput is everything buildArgs needs.
type argsInput struct {
	url, format, out, cache, ffmpeg, cookies string
	jsName, jsPath                           string
	playlist                                 bool
}

// Output markers printed through --print and --progress-template.
const (
	markStart    = "[rainy-start]"
	markProgress = "[rainy-progress]"
	markPost     = "[rainy-postprocess]"
	markItem     = "[rainy-item]"
)

// buildArgs returns yt-dlp's fixed argument list; the link is the last argument, after "--".
func buildArgs(in argsInput) []string {
	a := []string{
		"--ignore-config", "--no-plugin-dirs",
		"--use-extractors", "default,-generic",
		"--no-color", "--newline", "--progress", "--progress-delta", "0.5", "--no-simulate",
		"--progress-template", "download:" + markProgress + "%(progress.downloaded_bytes)s/%(progress.total_bytes)s/%(progress.total_bytes_estimate)s/%(progress.speed)s/%(progress.eta)s",
		"--progress-template", "postprocess:" + markPost + "%(progress.postprocessor)s",
		"--print", "before_dl:" + markStart + "%(.{id,title,playlist_index,n_entries,playlist_count})j",
		"--print", "after_move:" + markItem + "%(.{filepath,id,title,track,artist,artists,creator,creators,uploader,channel,album,album_artist,album_artists,track_number,release_date,release_year,upload_date,playlist_title,playlist_index,webpage_url})j",
		"--match-filters", "!is_live",
		"--cache-dir", in.cache,
		"--paths", in.out, "--output", "%(id)s.%(ext)s",
		"--no-mtime",
		"--max-filesize", maxFileSize,
		"--format", "bestaudio/best",
		"--extract-audio",
		"--write-thumbnail", "--convert-thumbnails", "jpg",
	}
	if in.format != "" && in.format != FormatBest {
		a = append(a, "--audio-format", in.format, "--audio-quality", "0")
	}
	if in.ffmpeg != "" {
		a = append(a, "--ffmpeg-location", in.ffmpeg)
	}
	if in.playlist {
		a = append(a, "--yes-playlist", "--playlist-items", "1:"+strconv.Itoa(MaxPlaylistItems))
	} else {
		a = append(a, "--no-playlist")
	}
	if in.jsName != "" {
		a = append(a, "--js-runtimes", in.jsName+":"+in.jsPath)
	}
	if in.cookies != "" {
		a = append(a, "--cookies", in.cookies)
	}
	return append(a, "--", in.url)
}

// Download runs yt-dlp for req and returns the downloaded audio files. With playlists some
// entries may fail: the items that did succeed are returned together with the error.
func (s *Service) Download(ctx context.Context, req Request, progress func(Progress)) ([]Item, error) {
	if progress == nil {
		progress = func(Progress) {}
	}
	if !ValidFormat(req.Format) {
		return nil, fmt.Errorf("%w: unknown audio format %q", ErrInvalid, req.Format)
	}
	bin, _, err := s.binary(ctx)
	if err != nil {
		return nil, err
	}
	ffmpeg, err := s.ffmpeg()
	if err != nil {
		return nil, err
	}
	target := req.Target
	if target.Short() {
		if target, err = s.resolveShort(ctx, target); err != nil {
			return nil, err
		}
	}
	out, tmp := filepath.Join(req.Dir, "out"), filepath.Join(req.Dir, "tmp")
	for _, d := range []string{out, tmp} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, err
		}
	}
	cookies, err := s.writeRunCookies(target.Site, req.Dir)
	if err != nil {
		return nil, err
	}
	cache := filepath.Join(s.opt.Dir, "cache")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		return nil, err
	}
	jsName, jsPath := jsRuntime()
	args := buildArgs(argsInput{
		url: target.URL, format: req.Format, out: out, cache: cache, ffmpeg: ffmpeg, cookies: cookies,
		jsName: jsName, jsPath: jsPath, playlist: req.Playlist,
	})

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = out
	cmd.Env = childEnv(tmp)
	cmd.WaitDelay = waitDelay
	setProcessGroup(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting yt-dlp: %w", err)
	}
	p := &parser{out: out, progress: progress}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); p.read(stdout) }()
	go func() { defer wg.Done(); p.read(stderr) }()
	wg.Wait()
	runErr := cmd.Wait()
	killProcessGroup(cmd)

	items := p.result()
	if ctx.Err() != nil {
		return items, ctx.Err()
	}
	for _, it := range items {
		if err := defragmentMP4(ctx, ffmpeg, it.Path); err != nil {
			slog.Warn("ytdlp: remuxing fragmented MP4", "id", it.ID, "err", err)
		}
	}
	if runErr != nil {
		msg := p.errorMessage()
		if msg == "" {
			msg = runErr.Error()
		}
		return items, fmt.Errorf("%w: %s", ErrUpstream, msg)
	}
	if len(items) == 0 {
		if msg := p.errorMessage(); msg != "" {
			return nil, fmt.Errorf("%w: %s", ErrUpstream, msg)
		}
		return nil, fmt.Errorf("%w: nothing was downloaded (live streams and unavailable videos are skipped)", ErrUpstream)
	}
	return items, nil
}

// resolveShort follows one redirect of a b23.tv share link and validates where it points.
func (s *Service) resolveShort(ctx context.Context, t Target) (Target, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL, nil)
	if err != nil {
		return Target{}, err
	}
	req.Header.Set("User-Agent", s.opt.UserAgent)
	client := *s.client
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return Target{}, ctx.Err()
		}
		return Target{}, fmt.Errorf("%w: cannot open the share link: %v", ErrUpstream, unwrapURLError(err))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	_ = resp.Body.Close()
	loc, err := resp.Location()
	if err != nil || resp.StatusCode < 300 || resp.StatusCode > 399 {
		return Target{}, fmt.Errorf("%w: the share link does not lead to a bilibili video", ErrInvalid)
	}
	next, err := ParseURL(loc.String())
	if err != nil || next.Site != SiteBilibili || next.Short() {
		return Target{}, fmt.Errorf("%w: the share link does not lead to a bilibili video", ErrInvalid)
	}
	return next, nil
}

// parser consumes yt-dlp's output lines.
type parser struct {
	out      string
	progress func(Progress)

	mu    sync.Mutex
	cur   Progress
	items []Item
	errs  []string // "ERROR:" lines
	tail  []string // last other lines of output
}

func (p *parser) read(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		p.line(strings.TrimRight(sc.Text(), "\r"))
	}
	_, _ = io.Copy(io.Discard, r) // an overlong line stops the scanner; keep yt-dlp from blocking
}

func (p *parser) line(l string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case strings.HasPrefix(l, markProgress):
		f := strings.Split(strings.TrimPrefix(l, markProgress), "/")
		if len(f) != 5 {
			return
		}
		done, total, est := num(f[0]), num(f[1]), num(f[2])
		if total <= 0 {
			total = est
		}
		p.cur.Phase = PhaseDownloading
		p.cur.Fraction = -1
		if done >= 0 && total > 0 {
			p.cur.Fraction = min(done/total, 1)
		}
		p.cur.Speed = max(num(f[3]), 0)
		p.cur.ETA = int(num(f[4]))
		p.progress(p.cur)
	case strings.HasPrefix(l, markPost):
		p.cur.Phase, p.cur.Fraction, p.cur.Speed, p.cur.ETA = PhaseProcessing, 1, 0, -1
		p.progress(p.cur)
	case strings.HasPrefix(l, markStart):
		var v struct {
			Title         string  `json:"title"`
			PlaylistIndex flexInt `json:"playlist_index"`
			NEntries      flexInt `json:"n_entries"`
			PlaylistCount flexInt `json:"playlist_count"`
		}
		if json.Unmarshal([]byte(strings.TrimPrefix(l, markStart)), &v) != nil {
			return
		}
		p.cur = Progress{Phase: PhaseDownloading, Item: int(v.PlaylistIndex), Items: int(v.NEntries), Title: v.Title, Fraction: 0, ETA: -1}
		if p.cur.Items == 0 {
			p.cur.Items = int(v.PlaylistCount)
		}
		p.progress(p.cur)
	case strings.HasPrefix(l, markItem):
		if it, ok := p.parseItem(strings.TrimPrefix(l, markItem)); ok {
			p.items = append(p.items, it)
		}
	case strings.HasPrefix(l, "ERROR:"):
		p.errs = append(p.errs, truncate(strings.TrimSpace(strings.TrimPrefix(l, "ERROR:")), 400))
		if len(p.errs) > 20 {
			p.errs = p.errs[1:]
		}
	default:
		if l = strings.TrimSpace(l); l != "" {
			p.tail = append(p.tail, truncate(l, 400))
			if len(p.tail) > 20 {
				p.tail = p.tail[1:]
			}
		}
	}
}

// parseItem reads an after_move line and checks that the file is an audio file inside the
// output directory.
func (p *parser) parseItem(js string) (Item, bool) {
	var v struct {
		Filepath      string   `json:"filepath"`
		ID            string   `json:"id"`
		Title         string   `json:"title"`
		Track         string   `json:"track"`
		Artist        string   `json:"artist"`
		Artists       []string `json:"artists"`
		Creator       string   `json:"creator"`
		Creators      []string `json:"creators"`
		Uploader      string   `json:"uploader"`
		Channel       string   `json:"channel"`
		Album         string   `json:"album"`
		AlbumArtist   string   `json:"album_artist"`
		AlbumArtists  []string `json:"album_artists"`
		TrackNumber   flexInt  `json:"track_number"`
		ReleaseDate   string   `json:"release_date"`
		ReleaseYear   flexInt  `json:"release_year"`
		UploadDate    string   `json:"upload_date"`
		PlaylistTitle string   `json:"playlist_title"`
		PlaylistIndex flexInt  `json:"playlist_index"`
		WebpageURL    string   `json:"webpage_url"`
	}
	if err := json.Unmarshal([]byte(js), &v); err != nil {
		return Item{}, false
	}
	path := filepath.Clean(v.Filepath)
	if !filepath.IsAbs(path) {
		path = filepath.Join(p.out, path)
	}
	if !util.IsWithin(p.out, path) || !tags.IsAudioFile(path) {
		return Item{}, false
	}
	if fi, err := os.Lstat(path); err != nil || !fi.Mode().IsRegular() {
		return Item{}, false
	}
	it := Item{
		Path: path, ID: v.ID, Title: firstNonEmpty(v.Track, v.Title, v.ID),
		Album: v.Album, TrackNumber: int(v.TrackNumber),
		PlaylistTitle: v.PlaylistTitle, PlaylistIndex: int(v.PlaylistIndex),
	}
	it.Artists = firstList(v.Artists, split(v.Artist), split(v.Creator), v.Creators, []string{cleanUploader(firstNonEmpty(v.Uploader, v.Channel))})
	it.AlbumArtists = firstList(v.AlbumArtists, split(v.AlbumArtist))
	switch {
	case len(v.ReleaseDate) == 8:
		it.Date = ytDate(v.ReleaseDate)
	case v.ReleaseYear > 0:
		it.Date = strconv.Itoa(int(v.ReleaseYear))
	case len(v.UploadDate) == 8:
		it.Date = ytDate(v.UploadDate)
	}
	if u, err := url.Parse(v.WebpageURL); err == nil && (u.Scheme == "https" || u.Scheme == "http") {
		it.WebpageURL = v.WebpageURL
	}
	stem := strings.TrimSuffix(path, filepath.Ext(path))
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp"} {
		if fi, err := os.Lstat(stem + ext); err == nil && fi.Mode().IsRegular() {
			it.Thumbnail = stem + ext
			break
		}
	}
	return it, true
}

func (p *parser) result() []Item {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]Item(nil), p.items...)
}

// errorMessage summarizes why yt-dlp failed.
func (p *parser) errorMessage() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.errs) > 0 {
		msg := p.errs[len(p.errs)-1]
		if n := len(p.errs); n > 1 {
			msg = fmt.Sprintf("%s (and %d more errors)", msg, n-1)
		}
		return msg
	}
	if len(p.tail) > 0 {
		return p.tail[len(p.tail)-1]
	}
	return ""
}

// flexInt decodes a JSON number, numeric string or null.
type flexInt int

func (f *flexInt) UnmarshalJSON(b []byte) error {
	b = bytes.Trim(b, `"`)
	if n, err := strconv.ParseFloat(string(b), 64); err == nil {
		*f = flexInt(n)
	}
	return nil
}

// num parses a progress field ("NA" → -1).
func num(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return -1
	}
	return v
}

func ytDate(d string) string {
	if _, err := strconv.Atoi(d); err != nil || len(d) != 8 {
		return ""
	}
	return d[:4] + "-" + d[4:6] + "-" + d[6:]
}

// cleanUploader turns YouTube's auto-generated "Artist - Topic" channels into the artist.
func cleanUploader(s string) string { return strings.TrimSpace(strings.TrimSuffix(s, " - Topic")) }

func split(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return []string{s}
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			return s
		}
	}
	return ""
}

func firstList(lists ...[]string) []string {
	for _, l := range lists {
		var keep []string
		for _, s := range l {
			if s = strings.TrimSpace(s); s != "" {
				keep = append(keep, s)
			}
		}
		if len(keep) > 0 {
			return keep
		}
	}
	return nil
}
