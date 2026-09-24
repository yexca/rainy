// Package manage implements library management: tag editing, covers, lyrics, rename /
// organize, upload, trash, the library doctor and the edit log (docs/architecture/contract.md §5.11,
// §7.6).
//
// Every file mutation follows the same protocol:
//
//  1. resolve the absolute path with util.SafeJoin and util.EnsureWithinRoot (symlink
//     escapes are rejected);
//  2. hold the scanner's library lock (scanner.LockLibrary) while touching files,
//     updating path-dependent rows and re-reading the affected files with
//     scanner.RescanFiles (which never takes the lock itself), so scans never observe
//     half-finished operations;
//  3. write one edit_log row per affected file and publish a "library" event.
//
// Once files have been touched, the remaining bookkeeping runs on a context detached
// from the request (context.WithoutCancel) so a client disconnect cannot leave the
// database out of sync with the disk.
//
// Read-only mounts and permission problems surface as *ReadonlyError (HTTP 409
// "readonly") with a message explaining how to fix the docker volume / PUID / PGID.
package manage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"rainy/internal/artwork"
	"rainy/internal/config"
	"rainy/internal/events"
	"rainy/internal/model"
	"rainy/internal/scanner"
	"rainy/internal/store"
	"rainy/internal/util"
)

// fileScanner is the part of *scanner.Scanner the service uses (an interface so tests can
// substitute a lightweight fake).
type fileScanner interface {
	LockLibrary() func()
	RescanFiles(ctx context.Context, libraryID int64, relPaths []string) ([]model.Track, error)
	RescanDir(ctx context.Context, libraryID int64, relDir string) error
}

// lockTimeout bounds how long an operation waits for the library lock (held by a
// running scan) before giving up with a conflict error.
var lockTimeout = 90 * time.Second

// Service performs library file operations.
type Service struct {
	st  *store.Store
	sc  fileScanner
	art *artwork.Service
	bus *events.Bus
	cfg *config.Config

	encMu    sync.Mutex
	encCache map[string]encCacheEntry // abs path → raw-tag mojibake analysis
}

// New creates the manage service.
func New(st *store.Store, sc *scanner.Scanner, art *artwork.Service, bus *events.Bus, cfg *config.Config) *Service {
	return newService(st, sc, art, bus, cfg)
}

func newService(st *store.Store, sc fileScanner, art *artwork.Service, bus *events.Bus, cfg *config.Config) *Service {
	return &Service{st: st, sc: sc, art: art, bus: bus, cfg: cfg, encCache: map[string]encCacheEntry{}}
}

// ---- shared JSON shapes (docs/architecture/contract.md §8, "manage")

// TagMap maps upper-case TagLib property names to their values.
type TagMap = map[string][]string

// ItemError reports a failed item of a batch operation.
type ItemError struct {
	TrackID string `json:"trackId"`
	Path    string `json:"path"`
	Error   string `json:"error"`
}

// BatchResult is the response of batch operations: the tracks as they are now, and the
// items that failed.
type BatchResult struct {
	Updated []model.Track `json:"updated"`
	Errors  []ItemError   `json:"errors"`

	errs []error // the errors behind Errors (for readonly detection)
}

func newBatch() *BatchResult { return &BatchResult{Updated: []model.Track{}, Errors: []ItemError{}} }

func (b *BatchResult) fail(trackID, path string, err error) {
	b.Errors = append(b.Errors, ItemError{TrackID: trackID, Path: path, Error: errorText(err)})
	b.errs = append(b.errs, err)
}

// failure returns a *ReadonlyError when nothing succeeded and every item failed because
// the library is read-only (the API then answers 409 "readonly"); nil otherwise.
func (b *BatchResult) failure() error {
	if len(b.Updated) > 0 || len(b.errs) == 0 {
		return nil
	}
	for _, err := range b.errs {
		if !IsReadonly(err) {
			return nil
		}
	}
	var re *ReadonlyError
	if errors.As(b.errs[0], &re) {
		return re
	}
	return &ReadonlyError{Err: b.errs[0]}
}

// Change is an old → new value pair in edit-log details and encoding fixes.
type Change struct {
	Old []string `json:"old"`
	New []string `json:"new"`
}

// ---- small helpers shared by the operations

// settings returns the runtime settings (defaults on error).
func (s *Service) settings(ctx context.Context) model.Settings {
	def := model.DefaultSettings(s.cfg.ScanInterval)
	set, err := s.st.GetSettings(ctx, def)
	if err != nil {
		slog.Error("manage: reading settings, using defaults", "err", err)
		return def
	}
	return set
}

// libraries caches library rows for the duration of one operation.
type libraries struct {
	st *store.Store
	m  map[int64]*model.Library
}

func (s *Service) libs() *libraries { return &libraries{st: s.st, m: map[int64]*model.Library{}} }

func (l *libraries) get(ctx context.Context, id int64) (*model.Library, error) {
	if lib, ok := l.m[id]; ok {
		return lib, nil
	}
	lib, err := l.st.GetLibrary(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("%w: library %d does not exist", store.ErrInvalid, id)
	}
	if err != nil {
		return nil, err
	}
	l.m[id] = lib
	return lib, nil
}

// libPath resolves a library-relative path to an absolute path, rejecting traversal
// and symlink escapes.
func libPath(root, rel string) (string, error) {
	abs, err := util.SafeJoin(root, rel)
	if err != nil {
		return "", err
	}
	if err := util.EnsureWithinRoot(root, abs); err != nil {
		if errors.Is(err, util.ErrUnsafePath) {
			return "", err
		}
		// Typically the library root itself is gone (unmounted volume, deleted folder).
		return "", fmt.Errorf("%w: the library folder %s is not accessible (%v)", store.ErrConflict, root, err)
	}
	return abs, nil
}

// lock takes the library lock, waiting at most lockTimeout (or until ctx is done).
func (s *Service) lock(ctx context.Context) (func(), error) {
	ch := make(chan func(), 1)
	go func() { ch <- s.sc.LockLibrary() }()
	timer := time.NewTimer(lockTimeout)
	defer timer.Stop()
	abandon := func() {
		go func() { (<-ch)() }() // release as soon as we would have acquired it
	}
	select {
	case unlock := <-ch:
		return unlock, nil
	case <-ctx.Done():
		abandon()
		return nil, ctx.Err()
	case <-timer.C:
		abandon()
		return nil, fmt.Errorf("%w: the library is busy (a scan is running); try again when it has finished", store.ErrConflict)
	}
}

// rescan re-reads files after a mutation. Failures are logged, not returned: the files
// have already been changed and the next scan will reconcile the database anyway.
func (s *Service) rescan(ctx context.Context, libraryID int64, paths []string) {
	if len(paths) == 0 {
		return
	}
	if _, err := s.sc.RescanFiles(ctx, libraryID, paths); err != nil {
		slog.Warn("manage: rescanning changed files", "library", libraryID, "files", len(paths), "err", err)
	}
}

// tracksByID returns the current rows of ids (order kept, unknown ids skipped) with the
// user's annotations.
func (s *Service) tracksByID(ctx context.Context, ids []string, userID string) []model.Track {
	if len(ids) == 0 {
		return []model.Track{}
	}
	ts, err := s.st.GetTracks(ctx, ids, userID)
	if err != nil {
		slog.Warn("manage: loading tracks", "err", err)
		return []model.Track{}
	}
	return ts
}

// logEdit appends an edit_log row (errors are logged).
func (s *Service) logEdit(ctx context.Context, u *model.User, action, trackID, path string, details any) {
	e := &model.EditLogEntry{Action: action, TrackID: trackID, Path: path}
	if u != nil {
		e.UserID, e.Username = u.ID, u.Username
	}
	if details != nil {
		b, err := json.Marshal(details)
		if err == nil {
			e.Details = b
		}
	}
	if err := s.st.AddEditLog(ctx, e); err != nil {
		slog.Error("manage: writing edit log", "action", action, "err", err)
	}
}

// publish notifies UI clients that the library changed.
func (s *Service) publish(reason string) {
	if s.bus != nil {
		s.bus.Publish(events.Library(reason))
	}
}

// refreshAggregates recomputes albums/artists/genres after rows changed without a rescan
// (trash / purge).
func (s *Service) refreshAggregates(ctx context.Context, albumIDs, artistIDs []string) {
	if len(albumIDs) > 0 {
		if err := s.st.RefreshAlbums(ctx, albumIDs); err != nil {
			slog.Warn("manage: refreshing albums", "err", err)
		}
	}
	if len(artistIDs) > 0 {
		if err := s.st.RefreshArtists(ctx, artistIDs); err != nil {
			slog.Warn("manage: refreshing artists", "err", err)
		}
	}
	if err := s.st.RefreshGenres(ctx); err != nil {
		slog.Warn("manage: refreshing genres", "err", err)
	}
}

// dedupe returns the distinct non-empty strings of v, order preserved.
func dedupe(v []string) []string {
	seen := make(map[string]bool, len(v))
	out := make([]string, 0, len(v))
	for _, s := range v {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// maxBatch bounds the number of items of one batch request.
const maxBatch = 5000

func checkBatch(n int, what string) error {
	if n == 0 {
		return fmt.Errorf("%w: no %s given", store.ErrInvalid, what)
	}
	if n > maxBatch {
		return fmt.Errorf("%w: too many %s (max %d per request)", store.ErrInvalid, what, maxBatch)
	}
	return nil
}
