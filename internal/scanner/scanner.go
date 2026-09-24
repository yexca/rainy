// Package scanner walks the libraries, reads tags and keeps the database in sync
// (quick/full scans, scheduled scans, targeted rescans after file edits).
//
// Concurrency model:
//   - Run/Start execute one scan at a time (ErrScanInProgress otherwise) and hold the library
//     lock (LockLibrary) for their whole duration, so file mutations by the manage service
//     never interleave with a scan.
//   - RescanFiles and RescanDir never take the library lock (callers that mutate files hold
//     it around the mutation and the rescan); they serialise their database work with scans
//     through an internal mutex that is always acquired after the library lock, so there is
//     no lock-order inversion.
//   - Scans stop promptly when their context is cancelled; Schedule cancels a running scan
//     when its own context ends (process shutdown).
//
// Safety: a library whose root is missing, unreadable or empty while the database still has
// tracks for it is skipped with an error instead of marking everything missing (NAS share
// offline). Sub-directories that cannot be read are excluded from missing-marking too.
//
// docs/architecture/contract.md §5.8.
package scanner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"rainy/internal/config"
	"rainy/internal/events"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

// ErrScanInProgress is returned by Start/Run while another scan runs.
var ErrScanInProgress = errors.New("scan already in progress")

// ErrLibraryUnavailable is returned (wrapped) when a library root is missing, unreadable or
// unexpectedly empty.
var ErrLibraryUnavailable = errors.New("library unavailable")

// ErrClosed is returned by Start/Run after the scheduler has shut down.
var ErrClosed = errors.New("scanner stopped")

// Phases reported in Status.Phase.
const (
	PhaseIdle       = "idle"
	PhaseWalking    = "walking"
	PhaseReading    = "reading"
	PhaseRefreshing = "refreshing"
	PhaseDone       = "done"
	PhaseError      = "error"
)

const (
	// batchSize is the maximum number of tracks per UpsertTracks transaction.
	batchSize = 200
	// publishInterval throttles "scan" events (≤ 2/s).
	publishInterval = 500 * time.Millisecond
)

// Options selects what to scan.
type Options struct {
	Full      bool  // re-read every file (otherwise only new/changed files)
	LibraryID int64 // 0 = all libraries
}

// Status is the scanner progress (the "scan" event payload and GET /api/admin/scan).
type Status struct {
	Scanning   bool   `json:"scanning"`
	Full       bool   `json:"full"`
	LibraryID  int64  `json:"libraryId"`
	Phase      string `json:"phase"` // idle|walking|reading|refreshing|done|error
	FilesSeen  int    `json:"filesSeen"`
	Added      int    `json:"added"`
	Updated    int    `json:"updated"`
	Removed    int    `json:"removed"`
	Moved      int    `json:"moved"`
	Errors     int    `json:"errors"`
	StartedAt  int64  `json:"startedAt"`
	FinishedAt int64  `json:"finishedAt"`
	LastError  string `json:"lastError"`
}

// Scanner scans libraries.
type Scanner struct {
	st  *store.Store
	bus *events.Bus
	cfg *config.Config

	libLock chan struct{} // library lock (semaphore of 1): file mutations (manage) and scans
	dbMu    sync.Mutex    // serialises the DB work of scans and targeted rescans

	running atomic.Bool
	workers int

	mu          sync.Mutex
	status      Status
	lastPublish time.Time
	cancelRun   context.CancelFunc // cancels the running scan
	closed      bool
	refreshed   bool // RefreshAll ran at least once since start
	done        chan struct{}
}

// New creates a scanner.
func New(st *store.Store, bus *events.Bus, cfg *config.Config) *Scanner {
	return &Scanner{
		st:      st,
		bus:     bus,
		cfg:     cfg,
		libLock: make(chan struct{}, 1),
		workers: max(1, runtime.NumCPU()),
		status:  Status{Phase: PhaseIdle},
	}
}

// settings returns the runtime settings (defaults on error).
func (s *Scanner) settings(ctx context.Context) model.Settings {
	var interval time.Duration
	if s.cfg != nil {
		interval = s.cfg.ScanInterval
	}
	def := model.DefaultSettings(interval)
	set, err := s.st.GetSettings(ctx, def)
	if err != nil {
		slog.Warn("scanner: reading settings, using defaults", "err", err)
		return def
	}
	return set
}

// Start begins a scan in the background (detached from ctx's cancellation; it is stopped by
// the shutdown of Schedule) and returns immediately; ErrScanInProgress if one is running.
func (s *Scanner) Start(ctx context.Context, opt Options) error {
	runCtx, err := s.begin(context.WithoutCancel(ctx), opt)
	if err != nil {
		return err
	}
	go func() {
		if err := s.run(runCtx, opt); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error("scan failed", "err", err)
		}
	}()
	return nil
}

// Run scans synchronously.
func (s *Scanner) Run(ctx context.Context, opt Options) error {
	runCtx, err := s.begin(ctx, opt)
	if err != nil {
		return err
	}
	return s.run(runCtx, opt)
}

// begin claims the scanner and resets the status; the returned context is cancelled by
// shutdown.
func (s *Scanner) begin(ctx context.Context, opt Options) (context.Context, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrClosed
	}
	if !s.running.CompareAndSwap(false, true) {
		return nil, ErrScanInProgress
	}
	runCtx, cancel := context.WithCancel(ctx)
	s.cancelRun = cancel
	s.done = make(chan struct{})
	s.status = Status{Scanning: true, Full: opt.Full, LibraryID: opt.LibraryID, Phase: PhaseWalking, StartedAt: util.NowMs()}
	s.publishLocked(true)
	return runCtx, nil
}

// Status returns a snapshot of the current/last scan.
func (s *Scanner) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// update mutates the status and publishes it (throttled unless force).
func (s *Scanner) update(force bool, fn func(*Status)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.status)
	s.publishLocked(force)
}

func (s *Scanner) publishLocked(force bool) {
	if s.bus == nil {
		return
	}
	now := time.Now()
	if !force && now.Sub(s.lastPublish) < publishInterval {
		return
	}
	s.lastPublish = now
	s.bus.Publish(events.Event{Type: events.TypeScan, Data: s.status})
}

// run performs a claimed scan.
func (s *Scanner) run(ctx context.Context, opt Options) (err error) {
	defer func() {
		s.mu.Lock()
		s.cancelRun()
		s.cancelRun = nil
		close(s.done)
		s.running.Store(false)
		s.mu.Unlock()
	}()

	unlock, err := s.lockLibraryCtx(ctx)
	if err != nil {
		s.finish(err)
		return err
	}
	defer unlock()
	s.dbMu.Lock()
	defer s.dbMu.Unlock()

	start := time.Now()
	err = s.scanAll(ctx, opt)
	s.finish(err)
	st := s.Status()
	attrs := []any{"full", opt.Full, "files", st.FilesSeen, "added", st.Added, "updated", st.Updated,
		"removed", st.Removed, "moved", st.Moved, "errors", st.Errors, "elapsed", time.Since(start).Round(time.Millisecond)}
	switch {
	case errors.Is(err, context.Canceled):
		slog.Info("scan cancelled", attrs...)
	case err != nil:
		slog.Error("scan finished with errors", append(attrs, "err", err)...)
	default:
		slog.Info("scan finished", attrs...)
	}
	return err
}

// finish records the final status and publishes the "scan" and "library" events.
func (s *Scanner) finish(err error) {
	s.update(true, func(st *Status) {
		st.Scanning = false
		st.FinishedAt = util.NowMs()
		st.Phase = PhaseDone
		if err != nil {
			st.Phase = PhaseError
			st.LastError = err.Error()
		}
	})
	if s.bus != nil && !errors.Is(err, context.Canceled) {
		s.bus.Publish(events.Library("scan"))
	}
}

// scanAll scans the selected libraries, refreshes aggregates and covers.
func (s *Scanner) scanAll(ctx context.Context, opt Options) error {
	libs, err := s.st.ListLibraries(ctx)
	if err != nil {
		return fmt.Errorf("listing libraries: %w", err)
	}
	if opt.LibraryID != 0 {
		var sel []model.Library
		for _, l := range libs {
			if l.ID == opt.LibraryID {
				sel = append(sel, l)
			}
		}
		if len(sel) == 0 {
			return fmt.Errorf("library %d: %w", opt.LibraryID, store.ErrNotFound)
		}
		libs = sel
	}
	set := s.settings(ctx)
	cache := newDirCache()
	var errs []error
	changed := false
	var scanned []int64
	failed := map[int64]bool{}
	for _, lib := range libs {
		if err := ctx.Err(); err != nil {
			return err
		}
		ls := &libScan{s: s, lib: lib, full: opt.Full, set: set, cache: cache, progress: true}
		res, err := ls.scan(ctx, "")
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Error("scanning library", "library", lib.Name, "path", lib.Path, "err", err)
			s.update(true, func(st *Status) { st.Errors++; st.LastError = err.Error() })
			failed[lib.ID] = true
			errs = append(errs, fmt.Errorf("library %q: %w", lib.Name, err))
			continue
		}
		changed = changed || res.changed()
		scanned = append(scanned, lib.ID)
	}

	s.update(true, func(st *Status) { st.Phase = PhaseRefreshing })
	s.mu.Lock()
	needRefresh := changed || opt.Full || !s.refreshed
	s.mu.Unlock()
	if needRefresh {
		if err := s.st.RefreshAll(ctx); err != nil {
			return errors.Join(append(errs, fmt.Errorf("refreshing aggregates: %w", err))...)
		}
		s.mu.Lock()
		s.refreshed = true
		s.mu.Unlock()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.updateCovers(ctx, nil, set, cache, failed); err != nil {
		errs = append(errs, fmt.Errorf("updating covers: %w", err))
	}
	now := util.NowMs()
	for _, id := range scanned {
		if err := s.st.SetLibraryScanned(ctx, id, now); err != nil && !errors.Is(err, store.ErrNotFound) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Schedule runs periodic quick scans (interval from settings.ScanInterval, re-read every
// cycle) until ctx is done; then it cancels a running scan and refuses new ones.
func (s *Scanner) Schedule(ctx context.Context) {
	last := time.Now()
	for {
		interval := s.settings(ctx).ScanIntervalDuration()
		check := time.Minute
		if interval > 0 {
			check = min(max(interval/4, time.Second), time.Minute)
		}
		timer := time.NewTimer(check)
		select {
		case <-ctx.Done():
			timer.Stop()
			s.shutdown()
			return
		case <-timer.C:
		}
		interval = s.settings(ctx).ScanIntervalDuration()
		if interval <= 0 {
			continue
		}
		ref := last
		if fin := s.Status().FinishedAt; fin > 0 && time.UnixMilli(fin).After(ref) {
			ref = time.UnixMilli(fin)
		}
		if time.Since(ref) < interval {
			continue
		}
		last = time.Now()
		switch err := s.Run(ctx, Options{}); {
		case err == nil, errors.Is(err, ErrScanInProgress), errors.Is(err, context.Canceled):
		default:
			slog.Warn("scheduled scan", "err", err)
		}
	}
}

// shutdown cancels a running scan, waits briefly for it and refuses new scans.
func (s *Scanner) shutdown() {
	s.mu.Lock()
	s.closed = true
	cancel, done := s.cancelRun, s.done
	s.mu.Unlock()
	if cancel != nil {
		cancel()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			slog.Warn("scanner: scan did not stop within 10s")
		}
	}
}

// LockLibrary takes the exclusive library lock (held around file mutations and scans) and
// returns the unlock function (idempotent). Do not call RescanDir/RescanFiles expecting
// them to take it: they never do.
func (s *Scanner) LockLibrary() func() {
	s.libLock <- struct{}{}
	return s.unlocker()
}

// TryLockLibrary is LockLibrary that gives up when ctx is done (e.g. while a long full scan
// holds the lock); it returns ctx.Err() then.
func (s *Scanner) TryLockLibrary(ctx context.Context) (func(), error) {
	return s.lockLibraryCtx(ctx)
}

func (s *Scanner) lockLibraryCtx(ctx context.Context) (func(), error) {
	select {
	case s.libLock <- struct{}{}:
		return s.unlocker(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Scanner) unlocker() func() {
	var once sync.Once
	return func() { once.Do(func() { <-s.libLock }) }
}
