package manage

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"rainy/internal/lxmusic"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
)

// Online music downloads (docs/architecture/contract.md §7.6 "online").
//
// A job asks the lx-music source scripts for a link to a search result (the sources are
// tried in the order the settings give, until one yields a file that is really audio),
// downloads it into a private work directory under <data>/tmp, writes title, artists,
// album, lyrics and cover through TagLib, and imports the file exactly like an upload
// (place): library lock, rescan, a "download" edit_log row and a library event. Existing
// files are never overwritten.

// OnlineSource is the part of *lxmusic.Service online downloads use.
type OnlineSource interface {
	Candidates(ctx context.Context, platform string, sel lxmusic.Selection) ([]lxmusic.Candidate, error)
	MusicURL(ctx context.Context, sourceID string, song lxmusic.Song, want string) (link, quality string, err error)
	Download(ctx context.Context, link string, w io.Writer, limit int64, progress func(done, total int64)) (int64, error)
	Lyrics(ctx context.Context, song lxmusic.Song) (text, translation string, err error)
	Cover(ctx context.Context, song lxmusic.Song) ([]byte, error)
}

// SetOnlineSource enables StartOnlineDownload.
func (s *Service) SetOnlineSource(o OnlineSource) { s.online = o }

const (
	onlineWorkers              = 3
	maxPendingOnline           = 100
	maxOnlineSongs             = 50 // per request
	maxOnlineFile        int64 = 1 << 30
	defaultOnlineQuality       = "flac"
	onlineLinkAttempts         = 2 // links asked from one source when the first one has expired
)

// OnlineDownloadRequest is the body of POST /api/manage/online/downloads.
type OnlineDownloadRequest struct {
	Songs     []lxmusic.Song `json:"songs"`
	Quality   string         `json:"quality"` // the best quality wanted: 128k | 320k | flac (default) | flac24bit
	LibraryID int64          `json:"libraryId"`
	Dir       string         `json:"dir"`
	Organize  bool           `json:"organize"`
	Lyrics    bool           `json:"lyrics"` // embed the catalogue's lyrics (and translation)
	Cover     bool           `json:"cover"`  // embed the catalogue's cover
}

// OnlineJob describes what an online download job fetches.
type OnlineJob struct {
	Song    lxmusic.Song `json:"song"`
	Quality string       `json:"quality"` // requested (the best wanted)
	Got     string       `json:"got"`     // quality asked from the source that answered ("" until then)
	Source  string       `json:"source"`  // name of that source
	Lyrics  bool         `json:"lyrics"`
	Cover   bool         `json:"cover"`
}

// StartOnlineDownload validates req and queues one job per song. Errors wrap
// store.ErrInvalid (bad input) or store.ErrConflict (too many jobs).
func (s *Service) StartOnlineDownload(ctx context.Context, u *model.User, req OnlineDownloadRequest) ([]DownloadJob, error) {
	if s.online == nil {
		return nil, fmt.Errorf("online downloads are not available: %w", errors.ErrUnsupported)
	}
	if len(req.Songs) == 0 {
		return nil, fmt.Errorf("%w: choose at least one song", store.ErrInvalid)
	}
	if len(req.Songs) > maxOnlineSongs {
		return nil, fmt.Errorf("%w: at most %d songs at a time", store.ErrInvalid, maxOnlineSongs)
	}
	for i := range req.Songs {
		if err := req.Songs[i].Normalize(); err != nil {
			return nil, fmt.Errorf("%w: %s", store.ErrInvalid, strings.TrimPrefix(err.Error(), lxmusic.ErrInvalid.Error()+": "))
		}
	}
	if req.Quality == "" {
		req.Quality = defaultOnlineQuality
	}
	if !lxmusic.ValidQuality(req.Quality) {
		return nil, fmt.Errorf("%w: quality must be 128k, 320k, flac or flac24bit", store.ErrInvalid)
	}
	if req.LibraryID <= 0 {
		return nil, fmt.Errorf("%w: libraryId is required", store.ErrInvalid)
	}
	lib, err := s.libs().get(ctx, req.LibraryID)
	if err != nil {
		return nil, err
	}
	baseDir := cleanDir(req.Dir)
	if err := checkDirParts(baseDir); err != nil {
		return nil, err
	}
	if _, err := libPath(lib.Path, baseDir); err != nil {
		return nil, err
	}

	s.dls.mu.Lock()
	if pending := s.pendingLocked(JobKindOnline); pending+len(req.Songs) > maxPendingOnline {
		s.dls.mu.Unlock()
		return nil, fmt.Errorf("%w: %d online downloads are already waiting; try again when some have finished", store.ErrConflict, pending)
	}
	out := make([]DownloadJob, 0, len(req.Songs))
	var started []*downloadJob
	for _, song := range req.Songs {
		jctx, cancel := context.WithTimeout(s.dls.ctx, downloadTimeout)
		j := &downloadJob{
			DownloadJob: DownloadJob{
				ID: util.NewID(), Kind: JobKindOnline, URL: song.PageURL, Title: song.Title, Status: JobQueued, ETA: -1,
				LibraryID: lib.ID, Dir: baseDir, Organize: req.Organize, TrackIDs: []string{}, Errors: []ItemError{},
				CreatedBy: u.Username, CreatedAt: util.NowMs(),
				Online: &OnlineJob{Song: song, Quality: req.Quality, Lyrics: req.Lyrics, Cover: req.Cover},
			},
			user: u, lib: lib, ctx: jctx, cancel: cancel,
		}
		s.dls.jobs = append(s.dls.jobs, j)
		snap := j.DownloadJob
		o := *snap.Online
		snap.Online = &o
		out = append(out, snap)
		started = append(started, j)
	}
	s.pruneDownloadsLocked()
	s.dls.wg.Add(len(started))
	s.dls.mu.Unlock()
	for _, j := range started {
		go s.runOnline(j)
	}
	return out, nil
}

func (s *Service) runOnline(j *downloadJob) {
	defer s.dls.wg.Done()
	defer j.cancel()
	select {
	case s.dls.onlineSlots <- struct{}{}:
		defer func() { <-s.dls.onlineSlots }()
	case <-j.ctx.Done():
		s.finishJob(j, JobCanceled, "")
		return
	}
	if j.ctx.Err() != nil {
		s.finishJob(j, JobCanceled, "")
		return
	}
	set := s.settings(j.ctx)
	if !set.LxSourcesEnabled {
		s.finishJob(j, JobError, "online music was turned off by an administrator")
		return
	}
	s.updateJob(j, func(d *DownloadJob) {
		d.Status, d.Phase, d.StartedAt, d.Progress = JobRunning, "downloading", util.NowMs(), -1
	})
	dir := filepath.Join(s.cfg.TmpDir(), "online-job-"+j.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		s.finishJob(j, JobError, err.Error())
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	s.dls.mu.Lock()
	info := *j.Online
	s.dls.mu.Unlock()
	song := info.Song

	path, err := s.fetchOnline(j, dir, song, info.Quality, lxmusic.Selection{Mode: set.LxSourceMode, SourceID: set.LxSourceID})
	if j.ctx.Err() != nil {
		msg := ""
		if errors.Is(j.ctx.Err(), context.DeadlineExceeded) {
			msg = "the download took too long and was stopped"
		}
		s.finishJob(j, JobCanceled, msg)
		return
	}
	if err != nil {
		s.finishJob(j, JobError, err.Error())
		return
	}

	s.updateJob(j, func(d *DownloadJob) { d.Phase, d.Speed, d.ETA = "processing", 0, -1 })
	s.tagOnline(j.ctx, path, info)
	if _, err := tags.Read(path, tags.ReadOptions{}); err != nil && !errors.Is(err, errors.ErrUnsupported) {
		s.finishJob(j, JobError, "the downloaded file is not readable audio")
		return
	}
	fi, err := os.Stat(path)
	if err != nil {
		s.finishJob(j, JobError, err.Error())
		return
	}
	s.updateJob(j, func(d *DownloadJob) { d.Status, d.Phase, d.Progress = JobImporting, "", 1 })

	s.dls.mu.Lock()
	info = *j.Online
	s.dls.mu.Unlock()
	res := newBatch()
	files := []staged{{name: onlineName(song, filepath.Ext(path)), tmp: path, size: fi.Size(), title: song.Title}}
	placeErr := s.place(j.ctx, j.user, j.lib, j.Dir, j.Organize, files, res, "download", func(f *staged) map[string]any {
		return map[string]any{"title": song.Title, "source": song.PageURL, "quality": info.Got, "via": info.Source, "size": f.size}
	})
	ids := make([]string, 0, len(res.Updated))
	for _, t := range res.Updated {
		ids = append(ids, t.ID)
	}
	status, msg := JobDone, ""
	if len(ids) == 0 {
		status = JobError
		switch {
		case placeErr != nil:
			msg = errorText(placeErr)
		case len(res.Errors) > 0:
			msg = res.Errors[0].Error
		default:
			msg = "the file could not be imported"
		}
	}
	s.updateJob(j, func(d *DownloadJob) { d.TrackIDs, d.Errors = ids, res.Errors })
	s.finishJob(j, status, msg)
}

// fetchOnline asks the sources in turn for a link and downloads it until one yields audio.
// It returns the path of the file (named audio.<ext> in dir).
func (s *Service) fetchOnline(j *downloadJob, dir string, song lxmusic.Song, want string, sel lxmusic.Selection) (string, error) {
	cands, err := s.online.Candidates(j.ctx, song.Platform, sel)
	if err != nil {
		return "", err
	}
	if len(cands) == 0 {
		if sel.Mode == model.LxSourceModeFixed {
			return "", errors.New("the selected music source is disabled, removed, or does not provide this platform")
		}
		return "", errors.New("no enabled music source provides this platform; an administrator can add one in Settings → Sources")
	}
	var failures []string
	for _, c := range cands {
		path, err := s.fetchFromSource(j, dir, c, song, want)
		if err == nil {
			return path, nil
		}
		if j.ctx.Err() != nil {
			return "", j.ctx.Err()
		}
		failures = append(failures, c.Name+": "+sourceError(err))
	}
	return "", errors.New(strings.Join(failures, "; "))
}

// fetchFromSource downloads the song through one source. Like lx-music, a link that has
// expired (HTTP 401, 403 or 410, or a host that does not resolve) makes it ask the source for
// a new link once before giving up on the source.
func (s *Service) fetchFromSource(j *downloadJob, dir string, c lxmusic.Candidate, song lxmusic.Song, want string) (string, error) {
	var err error
	for attempt := 0; attempt < onlineLinkAttempts; attempt++ {
		if j.ctx.Err() != nil {
			return "", j.ctx.Err()
		}
		var link, quality string
		link, quality, err = s.online.MusicURL(j.ctx, c.ID, song, want)
		if err != nil {
			return "", err
		}
		s.updateJob(j, func(d *DownloadJob) { d.Online.Got, d.Online.Source, d.Progress = quality, c.Name, -1 })
		var path string
		path, err = s.downloadAudio(j, dir, link)
		if err == nil {
			return path, nil
		}
		if !lxmusic.LinkExpired(err) {
			return "", err
		}
	}
	return "", err
}

func sourceError(err error) string {
	msg := err.Error()
	for _, e := range []error{lxmusic.ErrScript, lxmusic.ErrUpstream, lxmusic.ErrUnavailable} {
		msg = strings.TrimPrefix(msg, e.Error()+": ")
	}
	return msg
}

// downloadAudio downloads link and names the file after the audio format it contains.
func (s *Service) downloadAudio(j *downloadJob, dir, link string) (string, error) {
	tmp := filepath.Join(dir, "download.part")
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	start := util.NowMs()
	_, err = s.online.Download(j.ctx, link, f, maxOnlineFile, func(done, total int64) {
		s.updateJob(j, func(d *DownloadJob) {
			elapsed := float64(util.NowMs()-start) / 1000
			if elapsed > 0 {
				d.Speed = float64(done) / elapsed
			}
			if total > 0 {
				d.Progress = float64(done) / float64(total)
				if d.Speed > 0 {
					d.ETA = int(float64(total-done) / d.Speed)
				}
			} else {
				d.Progress = -1
			}
		})
	})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	ext, err := sniffAudio(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	path := filepath.Join(dir, "audio"+ext)
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, nil
}

// sniffAudio returns the file extension for the audio format of a file, judged by its
// content (links often end in a misleading name, and a failed link may return HTML).
func sniffAudio(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, 64)
	n, _ := io.ReadFull(bufio.NewReader(f), head)
	head = head[:n]
	// Skip an ID3v2 tag in front of FLAC or MPEG data.
	if len(head) >= 10 && bytes.HasPrefix(head, []byte("ID3")) {
		size := int64(head[6]&0x7f)<<21 | int64(head[7]&0x7f)<<14 | int64(head[8]&0x7f)<<7 | int64(head[9]&0x7f)
		if head[5]&0x10 != 0 {
			size += 10 // footer
		}
		next := make([]byte, 4)
		if _, err := f.ReadAt(next, 10+size); err == nil && string(next) == "fLaC" {
			return ".flac", nil
		}
		return ".mp3", nil
	}
	switch {
	case bytes.HasPrefix(head, []byte("fLaC")):
		return ".flac", nil
	case len(head) >= 12 && string(head[4:8]) == "ftyp":
		return ".m4a", nil
	case bytes.HasPrefix(head, []byte("OggS")):
		if bytes.Contains(head, []byte("OpusHead")) {
			return ".opus", nil
		}
		return ".ogg", nil
	case len(head) >= 12 && string(head[:4]) == "RIFF" && string(head[8:12]) == "WAVE":
		return ".wav", nil
	case bytes.HasPrefix(head, []byte("MAC ")):
		return ".ape", nil
	case bytes.HasPrefix(head, []byte("wvpk")):
		return ".wv", nil
	case len(head) >= 2 && head[0] == 0xff && head[1]&0xe0 == 0xe0 && head[1]&0x06 != 0:
		return ".mp3", nil // MPEG audio frame (an ADTS AAC header has layer bits 00)
	}
	return "", errors.New("the link did not lead to a playable audio file")
}

// tagOnline writes the catalogue's metadata, lyrics and cover into a downloaded file.
// Failures are logged: the file keeps whatever tags it came with.
func (s *Service) tagOnline(ctx context.Context, path string, info OnlineJob) {
	song := info.Song
	changes := map[string][]string{"TITLE": {song.Title}}
	if len(song.Artists) > 0 {
		changes["ARTIST"] = song.Artists
	}
	if song.Album != "" {
		changes["ALBUM"] = []string{song.Album}
	}
	if info.Lyrics {
		if text, translation, err := s.online.Lyrics(ctx, song); err == nil {
			if lyrics := pairTranslation(text, translation); strings.TrimSpace(lyrics) != "" {
				changes["LYRICS"] = []string{lyrics}
			}
		} else if !errors.Is(err, lxmusic.ErrNotFound) {
			slog.Info("manage: lyrics of an online download", "title", song.Title, "err", err)
		}
	}
	if err := tags.Write(path, changes); err != nil {
		slog.Warn("manage: writing tags of an online download", "title", song.Title, "err", err)
	}
	if info.Cover {
		data, err := s.online.Cover(ctx, song)
		if err != nil {
			if !errors.Is(err, lxmusic.ErrNotFound) {
				slog.Info("manage: cover of an online download", "title", song.Title, "err", err)
			}
			return
		}
		pc, err := prepareCover(data)
		if err != nil {
			slog.Info("manage: cover of an online download", "title", song.Title, "err", err)
			return
		}
		if err := tags.WritePicture(path, pc.data); err != nil {
			slog.Warn("manage: embedding the cover of an online download", "title", song.Title, "err", err)
		}
	}
}

// onlineName is the library-relative name of a download that is not organized by tags:
// "<title> - <artists><ext>" (lx-music's default "歌名 - 歌手").
func onlineName(song lxmusic.Song, ext string) string {
	name := song.Title
	if len(song.Artists) > 0 {
		name += " - " + strings.Join(song.Artists, "、")
	}
	base := sanitizeComponent(name, maxComponent-len(ext))
	if base == "" {
		base = "download"
	}
	return base + ext
}

var lrcStamp = regexp.MustCompile(`^((?:\[\d{1,3}:\d{1,2}(?:[.:]\d{1,3})?\])+)(.*)$`)

// pairTranslation merges a translation into LRC text as same-timestamp lines after each
// original line (the bilingual form Rainy's lyrics view understands). Translation lines
// whose time has no original line are dropped; without a timed original the text is
// returned unchanged.
func pairTranslation(text, translation string) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	if strings.TrimSpace(translation) == "" {
		return text
	}
	trans := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(translation, "\r\n", "\n"), "\n") {
		m := lrcStamp.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || strings.TrimSpace(m[2]) == "" {
			continue
		}
		for _, stamp := range splitStamps(m[1]) {
			trans[normalizeStamp(stamp)] = strings.TrimSpace(m[2])
		}
	}
	if len(trans) == 0 {
		return text
	}
	var out []string
	for _, line := range strings.Split(text, "\n") {
		out = append(out, line)
		m := lrcStamp.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || strings.TrimSpace(m[2]) == "" {
			continue
		}
		stamps := splitStamps(m[1])
		if t, ok := trans[normalizeStamp(stamps[0])]; ok && t != strings.TrimSpace(m[2]) {
			out = append(out, m[1]+t)
		}
	}
	return strings.Join(out, "\n")
}

func splitStamps(s string) []string {
	parts := strings.SplitAfter(s, "]")
	out := parts[:0]
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// normalizeStamp maps "[1:02.5]" and "[01:02.500]" to the same key (centiseconds).
func normalizeStamp(stamp string) string {
	var m, sec, frac int
	var fracText string
	inner := strings.Trim(stamp, "[]")
	mm, rest, _ := strings.Cut(inner, ":")
	ss, f, found := strings.Cut(strings.Replace(rest, ":", ".", 1), ".")
	_, _ = fmt.Sscan(mm, &m)
	_, _ = fmt.Sscan(ss, &sec)
	if found {
		fracText = (f + "000")[:3]
		_, _ = fmt.Sscan(fracText, &frac)
	}
	return fmt.Sprintf("%d.%02d", m*60+sec, frac/10)
}
