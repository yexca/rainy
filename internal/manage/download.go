package manage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/tags"
	"rainy/internal/util"
	"rainy/internal/ytdlp"
)

// Downloads from YouTube and bilibili (docs/architecture/contract.md §7.6 "downloads").
//
// A job runs yt-dlp in a private work directory under <data>/tmp, writes the tags derived
// from yt-dlp's metadata (and a square cover cropped from the thumbnail) through TagLib, and
// then imports the files exactly like an upload (place): library lock, rescan, one
// "download" edit_log row per file and a library event. Jobs live in memory; a restart
// cancels them and New in package ytdlp removes their leftovers.

// Downloader runs yt-dlp (implemented by *ytdlp.Service).
type Downloader interface {
	Ready(ctx context.Context) error
	Download(ctx context.Context, req ytdlp.Request, progress func(ytdlp.Progress)) ([]ytdlp.Item, error)
}

// SetDownloader enables StartDownload.
func (s *Service) SetDownloader(d Downloader) { s.downloader = d }

const (
	downloadWorkers       = 2
	maxPendingDownloads   = 20
	keepFinishedDownloads = 200
	downloadTimeout       = 3 * time.Hour
	maxThumbnailBytes     = 20 << 20
)

// Download job statuses.
const (
	JobQueued    = "queued"
	JobRunning   = "running"
	JobImporting = "importing"
	JobDone      = "done"
	JobError     = "error"
	JobCanceled  = "canceled"
)

// DownloadRequest is the body of POST /api/manage/downloads.
type DownloadRequest struct {
	URL       string `json:"url"`
	LibraryID int64  `json:"libraryId"`
	Dir       string `json:"dir"`
	Organize  bool   `json:"organize"`
	Format    string `json:"format"`   // best (default) | m4a | mp3 | opus
	Playlist  bool   `json:"playlist"` // download the whole playlist the link belongs to
}

// Download job kinds.
const (
	JobKindLink   = "link"   // yt-dlp, from a YouTube / bilibili link
	JobKindOnline = "online" // an online music search result, through lx-music sources (online.go)
)

// DownloadJob is the state of one download (native API shape).
type DownloadJob struct {
	ID         string      `json:"id"`
	Kind       string      `json:"kind"` // link | online
	URL        string      `json:"url"`
	Site       string      `json:"site"`
	Title      string      `json:"title"`
	Status     string      `json:"status"`   // queued | running | importing | done | error | canceled
	Phase      string      `json:"phase"`    // while running: downloading | processing
	Progress   float64     `json:"progress"` // 0…1 overall, -1 = unknown
	Item       int         `json:"item"`     // current playlist entry (1-based), 0 = unknown
	Items      int         `json:"items"`    // playlist entries, 0 = unknown
	Speed      float64     `json:"speed"`    // bytes per second
	ETA        int         `json:"eta"`      // seconds, -1 = unknown
	Error      string      `json:"error"`    // failure, or what went wrong for some entries of a finished job
	LibraryID  int64       `json:"libraryId"`
	Dir        string      `json:"dir"`
	Organize   bool        `json:"organize"`
	Format     string      `json:"format"`
	Playlist   bool        `json:"playlist"`
	TrackIDs   []string    `json:"trackIds"`
	Errors     []ItemError `json:"errors"`
	CreatedBy  string      `json:"createdBy"`
	CreatedAt  int64       `json:"createdAt"`
	StartedAt  int64       `json:"startedAt"`
	FinishedAt int64       `json:"finishedAt"`
	Online     *OnlineJob  `json:"online"` // online downloads only (null for links)
}

func (j DownloadJob) active() bool {
	return j.Status == JobQueued || j.Status == JobRunning || j.Status == JobImporting
}

type downloadJob struct {
	DownloadJob // guarded by downloads.mu
	target      ytdlp.Target
	user        *model.User
	lib         *model.Library
	ctx         context.Context
	cancel      context.CancelFunc
}

// downloads is the in-memory job list.
type downloads struct {
	mu          sync.Mutex
	jobs        []*downloadJob // oldest first
	slots       chan struct{}  // running link downloads
	onlineSlots chan struct{}  // running online downloads
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func newDownloads() *downloads {
	ctx, cancel := context.WithCancel(context.Background())
	return &downloads{
		slots: make(chan struct{}, downloadWorkers), onlineSlots: make(chan struct{}, onlineWorkers),
		ctx: ctx, cancel: cancel,
	}
}

// CloseDownloads cancels every job and waits for them to stop.
func (s *Service) CloseDownloads() {
	s.dls.cancel()
	s.dls.wg.Wait()
}

// StartDownload validates req and queues a download. Errors wrap store.ErrInvalid (bad
// input) or store.ErrConflict (yt-dlp not ready, too many jobs).
func (s *Service) StartDownload(ctx context.Context, u *model.User, req DownloadRequest) (*DownloadJob, error) {
	if s.downloader == nil {
		return nil, fmt.Errorf("downloads are not available: %w", errors.ErrUnsupported)
	}
	target, err := ytdlp.ParseURL(req.URL)
	if err != nil {
		return nil, fromYtdlp(err)
	}
	if req.Format == "" {
		req.Format = ytdlp.FormatBest
	}
	if !ytdlp.ValidFormat(req.Format) {
		return nil, fmt.Errorf("%w: format must be best, m4a, mp3 or opus", store.ErrInvalid)
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
	if err := s.downloader.Ready(ctx); err != nil {
		return nil, fromYtdlp(err)
	}

	jctx, cancel := context.WithTimeout(s.dls.ctx, downloadTimeout)
	j := &downloadJob{
		DownloadJob: DownloadJob{
			ID: util.NewID(), Kind: JobKindLink, URL: target.URL, Site: target.Site, Status: JobQueued, Progress: 0, ETA: -1,
			LibraryID: lib.ID, Dir: baseDir, Organize: req.Organize, Format: req.Format, Playlist: req.Playlist,
			TrackIDs: []string{}, Errors: []ItemError{}, CreatedBy: u.Username, CreatedAt: util.NowMs(),
		},
		target: target, user: u, lib: lib, ctx: jctx, cancel: cancel,
	}
	s.dls.mu.Lock()
	pending := s.pendingLocked(JobKindLink)
	if pending >= maxPendingDownloads {
		s.dls.mu.Unlock()
		cancel()
		return nil, fmt.Errorf("%w: %d downloads are already waiting; try again when some have finished", store.ErrConflict, pending)
	}
	s.dls.jobs = append(s.dls.jobs, j)
	s.pruneDownloadsLocked()
	snap := j.DownloadJob
	s.dls.wg.Add(1)
	s.dls.mu.Unlock()
	go s.runDownload(j)
	return &snap, nil
}

// pendingLocked counts the queued and running jobs of a kind.
func (s *Service) pendingLocked(kind string) int {
	n := 0
	for _, o := range s.dls.jobs {
		if o.Kind == kind && o.active() {
			n++
		}
	}
	return n
}

// fromYtdlp maps ytdlp errors to the store errors the API understands.
func fromYtdlp(err error) error {
	msg := err.Error()
	for _, e := range []error{ytdlp.ErrInvalid, ytdlp.ErrUpstream} {
		msg = strings.TrimPrefix(msg, e.Error()+": ")
	}
	switch {
	case errors.Is(err, ytdlp.ErrInvalid):
		return fmt.Errorf("%w: %s", store.ErrInvalid, msg)
	case errors.Is(err, ytdlp.ErrNotInstalled):
		return fmt.Errorf("%w: yt-dlp is not installed yet; an administrator can install it in Settings → yt-dlp", store.ErrConflict)
	case errors.Is(err, ytdlp.ErrNoFFmpeg), strings.HasPrefix(msg, "yt-dlp cannot run"):
		return fmt.Errorf("%w: %s", store.ErrConflict, msg)
	case errors.Is(err, ytdlp.ErrUpstream):
		return errors.New(msg) // yt-dlp's own message, e.g. "[youtube] …: Video unavailable"
	}
	return err
}

// DownloadJobs returns every job, newest first.
func (s *Service) DownloadJobs() []DownloadJob {
	s.dls.mu.Lock()
	defer s.dls.mu.Unlock()
	out := make([]DownloadJob, 0, len(s.dls.jobs))
	for i := len(s.dls.jobs) - 1; i >= 0; i-- {
		d := s.dls.jobs[i].DownloadJob
		d.TrackIDs = append([]string{}, d.TrackIDs...)
		d.Errors = append([]ItemError{}, d.Errors...)
		if d.Online != nil {
			o := *d.Online
			d.Online = &o
		}
		out = append(out, d)
	}
	return out
}

// RemoveDownload cancels a queued or running job, or removes a finished one from the list.
func (s *Service) RemoveDownload(id string) error {
	s.dls.mu.Lock()
	defer s.dls.mu.Unlock()
	for i, j := range s.dls.jobs {
		if j.ID != id {
			continue
		}
		if j.active() {
			j.cancel()
		} else {
			s.dls.jobs = append(s.dls.jobs[:i], s.dls.jobs[i+1:]...)
		}
		return nil
	}
	return store.ErrNotFound
}

// pruneDownloadsLocked drops the oldest finished jobs beyond keepFinishedDownloads.
func (s *Service) pruneDownloadsLocked() {
	finished := 0
	for _, j := range s.dls.jobs {
		if !j.active() {
			finished++
		}
	}
	keep := s.dls.jobs[:0]
	for _, j := range s.dls.jobs {
		if !j.active() && finished > keepFinishedDownloads {
			finished--
			continue
		}
		keep = append(keep, j)
	}
	s.dls.jobs = keep
}

func (s *Service) updateJob(j *downloadJob, fn func(d *DownloadJob)) {
	s.dls.mu.Lock()
	defer s.dls.mu.Unlock()
	fn(&j.DownloadJob)
}

func (s *Service) finishJob(j *downloadJob, status, msg string) {
	s.updateJob(j, func(d *DownloadJob) {
		d.Status, d.Error, d.Phase, d.Speed, d.ETA = status, msg, "", 0, -1
		d.FinishedAt = util.NowMs()
		if status == JobDone {
			d.Progress = 1
		}
	})
	s.dls.mu.Lock()
	s.pruneDownloadsLocked()
	s.dls.mu.Unlock()
}

func applyProgress(d *DownloadJob, p ytdlp.Progress) {
	d.Phase, d.Item, d.Items, d.Speed, d.ETA = p.Phase, p.Item, p.Items, p.Speed, p.ETA
	if p.Title != "" && (d.Title == "" || d.Playlist) {
		d.Title = p.Title
	}
	frac := p.Fraction
	if frac < 0 {
		d.Progress = -1
		return
	}
	if p.Items > 0 && p.Item > 0 {
		frac = (float64(p.Item-1) + frac) / float64(p.Items)
	}
	d.Progress = min(max(frac, 0), 1)
}

func (s *Service) runDownload(j *downloadJob) {
	defer s.dls.wg.Done()
	defer j.cancel()
	select {
	case s.dls.slots <- struct{}{}:
		defer func() { <-s.dls.slots }()
	case <-j.ctx.Done():
		s.finishJob(j, JobCanceled, "")
		return
	}
	if j.ctx.Err() != nil {
		s.finishJob(j, JobCanceled, "")
		return
	}
	if !s.settings(j.ctx).YtdlpEnabled {
		s.finishJob(j, JobError, "downloads from links were turned off by an administrator")
		return
	}
	s.updateJob(j, func(d *DownloadJob) {
		d.Status, d.Phase, d.StartedAt = JobRunning, ytdlp.PhaseDownloading, util.NowMs()
	})
	dir := filepath.Join(s.cfg.TmpDir(), "ytdlp-job-"+j.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		s.finishJob(j, JobError, err.Error())
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	items, dlErr := s.downloader.Download(j.ctx, ytdlp.Request{Target: j.target, Format: j.Format, Playlist: j.Playlist, Dir: dir},
		func(p ytdlp.Progress) { s.updateJob(j, func(d *DownloadJob) { applyProgress(d, p) }) })
	if j.ctx.Err() != nil {
		msg := ""
		if errors.Is(j.ctx.Err(), context.DeadlineExceeded) {
			msg = "the download took too long and was stopped"
		}
		s.finishJob(j, JobCanceled, msg)
		return
	}
	if len(items) == 0 {
		if dlErr == nil {
			dlErr = errors.New("nothing was downloaded")
		}
		s.finishJob(j, JobError, errorText(fromYtdlp(dlErr)))
		return
	}
	if len(items) == 1 && !j.Playlist {
		s.updateJob(j, func(d *DownloadJob) { d.Title = items[0].Title })
	}
	s.updateJob(j, func(d *DownloadJob) { d.Status, d.Phase, d.Progress, d.Speed, d.ETA = JobImporting, "", 1, 0, -1 })

	res := newBatch()
	files := prepareDownloads(items, j.Playlist, res)
	source := j.URL
	placeErr := s.place(j.ctx, j.user, j.lib, j.Dir, j.Organize, files, res, "download", func(f *staged) map[string]any {
		d := map[string]any{"source": source, "size": f.size}
		if f.title != "" {
			d["title"] = f.title
		}
		return d
	})
	ids := make([]string, 0, len(res.Updated))
	for _, t := range res.Updated {
		ids = append(ids, t.ID)
	}
	var msgs []string
	if placeErr != nil {
		msgs = append(msgs, errorText(placeErr))
	}
	if dlErr != nil {
		msgs = append(msgs, errorText(fromYtdlp(dlErr)))
	}
	status := JobDone
	if len(ids) == 0 {
		status = JobError
		if len(msgs) == 0 && len(res.Errors) > 0 {
			msgs = append(msgs, res.Errors[0].Error)
		}
	}
	s.updateJob(j, func(d *DownloadJob) { d.TrackIDs, d.Errors = ids, res.Errors })
	s.finishJob(j, status, strings.Join(msgs, "; "))
}

// prepareDownloads writes Rainy's tags and cover into the downloaded files (through TagLib)
// and turns them into staged files for place. Files that are not readable audio become
// item errors.
func prepareDownloads(items []ytdlp.Item, playlist bool, res *BatchResult) []staged {
	files := make([]staged, 0, len(items))
	for _, it := range items {
		name := downloadName(it, playlist)
		if t := it.Tags(); len(t) > 0 {
			if err := tags.Write(it.Path, t); err != nil {
				slog.Warn("manage: writing tags of a download", "name", name, "err", err)
			}
		}
		if it.Thumbnail != "" {
			if img, err := squareCover(it.Thumbnail); err != nil {
				slog.Warn("manage: preparing the cover of a download", "name", name, "err", err)
			} else if err := tags.WritePicture(it.Path, img); err != nil {
				slog.Warn("manage: embedding the cover of a download", "name", name, "err", err)
			}
		}
		if _, err := tags.Read(it.Path, tags.ReadOptions{}); err != nil && !errors.Is(err, errors.ErrUnsupported) {
			res.fail("", name, itemError{"not a valid audio file"})
			continue
		}
		fi, err := os.Stat(it.Path)
		if err != nil {
			res.fail("", name, err)
			continue
		}
		files = append(files, staged{name: name, tmp: it.Path, size: fi.Size(), title: it.Title})
	}
	return files
}

// downloadName is the library-relative name of a download when it is not organized by
// tags: "<title>.<ext>", or "<playlist>/<NN> <title>.<ext>" for playlist entries.
func downloadName(it ytdlp.Item, playlist bool) string {
	ext := strings.ToLower(filepath.Ext(it.Path))
	title := util.FirstNonEmpty(it.Title, it.ID, "download")
	if playlist && it.PlaylistIndex > 0 {
		title = fmt.Sprintf("%02d %s", it.PlaylistIndex, title)
	}
	base := sanitizeComponent(title, maxComponent-len(ext))
	if base == "" {
		base = "download"
	}
	name := base + ext
	if playlist && it.PlaylistTitle != "" {
		if dir := sanitizeComponent(it.PlaylistTitle, maxComponent); dir != "" {
			name = dir + "/" + name
		}
	}
	return name
}

// squareCover crops a video thumbnail to its centred square (album covers are square) and
// returns it as a JPEG that fits coverTargetDim.
func squareCover(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxThumbnailBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxThumbnailBytes {
		return nil, errors.New("thumbnail is too large")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxCoverPixels {
		return nil, fmt.Errorf("unsupported thumbnail size %dx%d", cfg.Width, cfg.Height)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	side := min(b.Dx(), b.Dy())
	if side <= 0 {
		return nil, errors.New("empty thumbnail")
	}
	if si, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	}); ok && b.Dx() != b.Dy() {
		x, y := b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2
		img = si.SubImage(image.Rect(x, y, x+side, y+side))
	}
	return encodeJPEG(fit(img, coverTargetDim))
}
