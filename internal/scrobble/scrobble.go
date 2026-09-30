// Package scrobble sends plays, "now playing" updates and loved tracks to the users' linked
// Last.fm and ListenBrainz accounts (docs/architecture/contract.md §5.16).
//
// Both services are off until an administrator turns them on (Last.fm also needs the
// administrator's API key and shared secret). Plays are queued in the database and sent in
// the background with retries, so an outage or a restart loses nothing; plays older than
// MaxAge are dropped because the services no longer accept them. Credentials (Last.fm
// session keys, ListenBrainz tokens, the Last.fm shared secret) are encrypted with
// secret.key, never returned by the API and never logged.
package scrobble

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"rainy/internal/model"
	"rainy/internal/scanner"
	"rainy/internal/store"
	"rainy/internal/util"
)

// Services lists the scrobbling services in display order.
var Services = []string{model.ScrobbleLastfm, model.ScrobbleListenBrainz}

// ValidService reports whether id is a known service.
func ValidService(id string) bool {
	return id == model.ScrobbleLastfm || id == model.ScrobbleListenBrainz
}

const (
	// MaxAge is how long a play may wait in the queue (Last.fm refuses older scrobbles).
	MaxAge = 14 * 24 * time.Hour
	// MinDuration: Last.fm only accepts tracks longer than 30 seconds.
	MinDuration   = 30
	flushInterval = time.Minute
	maxBackoff    = 6 * time.Hour
	liveTimeout   = 10 * time.Second
	maxLive       = 4 // concurrent now-playing / love requests
	maxLoveTracks = 50
)

// Cipher encrypts credentials (*auth.Crypto).
type Cipher interface {
	Encrypt(plain string) (string, error)
	Decrypt(enc string) (string, error)
}

// Options configure the service. The URLs default to the real services (tests point them
// at local servers).
type Options struct {
	Store           *store.Store
	Cipher          Cipher
	Settings        func(context.Context) model.Settings
	Client          *http.Client // nil → 15 s timeout, HTTPS_PROXY / HTTP_PROXY honoured
	UserAgent       string
	LastfmAPI       string
	LastfmAuthURL   string
	ListenBrainzAPI string
	NoWorker        bool // tests drive Flush themselves
}

// Track is what the services receive about a play.
type Track struct {
	Title, Artist, Album, AlbumArtist string
	TrackNumber                       int
	Duration                          float64
	MbzTrackID                        string
	PlayedAt                          int64 // unix ms; 0 for now playing
}

// Service sends plays to the linked accounts.
type Service struct {
	st       *store.Store
	cipher   Cipher
	settings func(context.Context) model.Settings
	lastfm   *lastfm
	lb       *listenBrainz
	authURL  string
	now      func() time.Time

	ctx    context.Context
	cancel context.CancelFunc
	wake   chan struct{}
	done   chan struct{}
	live   chan struct{} // semaphore for now-playing and love requests
	liveWG sync.WaitGroup
	flush  sync.Mutex

	mu      sync.Mutex
	pending map[string]pendingAuth // user id → Last.fm web authentication in progress
}

// New creates the service and starts the background sender (unless o.NoWorker).
func New(o Options) *Service {
	client := o.Client
	if client == nil {
		client = newHTTPClient()
	}
	ua := o.UserAgent
	if ua == "" {
		ua = "Rainy"
	}
	s := &Service{
		st:       o.Store,
		cipher:   o.Cipher,
		settings: o.Settings,
		lastfm:   &lastfm{http: client, base: util.FirstNonEmpty(o.LastfmAPI, lastfmAPI), ua: ua},
		lb:       &listenBrainz{http: client, base: util.FirstNonEmpty(o.ListenBrainzAPI, listenBrainzAPI), ua: ua},
		authURL:  util.FirstNonEmpty(o.LastfmAuthURL, lastfmAuthURL),
		now:      time.Now,
		wake:     make(chan struct{}, 1),
		done:     make(chan struct{}),
		live:     make(chan struct{}, maxLive),
		pending:  map[string]pendingAuth{},
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	if o.NoWorker {
		close(s.done)
	} else {
		go s.loop()
	}
	return s
}

// Close stops the sender and waits for requests in flight.
func (s *Service) Close() {
	s.cancel()
	<-s.done
	s.liveWG.Wait()
}

func (s *Service) nowMs() int64 { return s.now().UnixMilli() }

// active returns the services the administrator enabled (and, for Last.fm, configured),
// with the Last.fm credentials.
func (s *Service) active(ctx context.Context) (services []string, creds lastfmCreds) {
	set := s.settings(ctx)
	if set.LastfmEnabled {
		if c, err := s.lastfmCreds(ctx); err == nil && c.key != "" && c.secret != "" {
			services, creds = append(services, model.ScrobbleLastfm), c
		}
	}
	if set.ListenBrainzEnabled {
		services = append(services, model.ScrobbleListenBrainz)
	}
	return services, creds
}

func (s *Service) lastfmCreds(ctx context.Context) (lastfmCreds, error) {
	key, err := s.st.GetValue(ctx, store.ValueLastfmAppKey)
	if err != nil {
		return lastfmCreds{}, err
	}
	enc, err := s.st.GetValue(ctx, store.ValueLastfmSigningEnc)
	if err != nil || enc == "" {
		return lastfmCreds{key: key}, err
	}
	secret, err := s.cipher.Decrypt(enc)
	if err != nil {
		slog.Warn("the stored Last.fm shared secret cannot be decrypted; set it again in Admin → Settings")
		return lastfmCreds{key: key}, nil
	}
	return lastfmCreds{key: key, secret: secret}, nil
}

func trackOf(t *model.Track, playedAt int64) Track {
	out := Track{Title: t.Title, Artist: t.Artist, Album: t.Album, AlbumArtist: t.AlbumArtist,
		TrackNumber: t.TrackNumber, Duration: t.Duration, MbzTrackID: t.MbzTrackID, PlayedAt: playedAt}
	if out.Album == scanner.UnknownAlbum {
		out.Album = ""
	}
	if out.AlbumArtist == scanner.UnknownArtist {
		out.AlbumArtist = ""
	}
	return out
}

// scrobbable reports whether the services can make sense of a track: it needs a real
// artist and title (Last.fm additionally refuses tracks of 30 seconds or less).
func scrobbable(t Track, service string) bool {
	if t.Title == "" || t.Artist == "" || t.Artist == scanner.UnknownArtist {
		return false
	}
	return service != model.ScrobbleLastfm || t.Duration == 0 || t.Duration > MinDuration
}

func (s *Service) track(ctx context.Context, trackID string) (*model.Track, error) {
	ts, err := s.st.GetTracks(ctx, []string{trackID}, "")
	if err != nil {
		return nil, err
	}
	if len(ts) == 0 {
		return nil, store.ErrNotFound
	}
	return &ts[0], nil
}

// Played records a completed play (store.RecordPlay) and queues it for the user's enabled
// accounts. Queueing problems are logged, never returned: the play itself is recorded.
func (s *Service) Played(ctx context.Context, userID, trackID string, at int64, client string) error {
	if at <= 0 {
		at = s.nowMs()
	}
	if err := s.st.RecordPlay(ctx, userID, trackID, at, client); err != nil {
		return err
	}
	services, _ := s.active(ctx)
	if len(services) == 0 {
		return nil
	}
	t, err := s.track(ctx, trackID)
	if err != nil {
		slog.Warn("queueing scrobble", "err", err)
		return nil
	}
	tr := trackOf(t, at)
	var want []string
	for _, svc := range services {
		if scrobbable(tr, svc) {
			want = append(want, svc)
		}
	}
	e := model.QueuedScrobble{UserID: userID, TrackID: trackID, Title: tr.Title, Artist: tr.Artist, Album: tr.Album,
		AlbumArtist: tr.AlbumArtist, TrackNumber: tr.TrackNumber, Duration: tr.Duration, MbzTrackID: tr.MbzTrackID, PlayedAt: at}
	queued, err := s.st.EnqueueScrobble(ctx, e, want)
	if err != nil {
		slog.Warn("queueing scrobble", "err", err)
		return nil
	}
	if len(queued) > 0 {
		s.Wake()
	}
	return nil
}

// Wake asks the sender to flush the queue now.
func (s *Service) Wake() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// goLive runs fn in the background when a slot is free (dropped otherwise: now playing and
// loves are best effort).
func (s *Service) goLive(fn func(ctx context.Context)) {
	select {
	case s.live <- struct{}{}:
	default:
		return
	}
	s.liveWG.Add(1)
	go func() {
		defer func() { <-s.live; s.liveWG.Done() }()
		ctx, cancel := context.WithTimeout(s.ctx, liveTimeout)
		defer cancel()
		fn(ctx)
	}()
}

// credential returns the user's decrypted credential for an enabled, linked account.
func (s *Service) credential(ctx context.Context, userID, service string) (string, bool) {
	a, err := s.st.GetScrobbleAccount(ctx, userID, service)
	if err != nil || !a.Enabled || a.CredentialEnc == "" {
		return "", false
	}
	cred, err := s.cipher.Decrypt(a.CredentialEnc)
	if err != nil {
		slog.Warn("a stored scrobbling credential cannot be decrypted; the user must link the account again", "service", service)
		return "", false
	}
	return cred, true
}

// NowPlaying tells the user's enabled accounts what is playing (best effort, background).
func (s *Service) NowPlaying(userID, trackID string) {
	s.goLive(func(ctx context.Context) {
		services, creds := s.active(ctx)
		if len(services) == 0 {
			return
		}
		t, err := s.track(ctx, trackID)
		if err != nil {
			return
		}
		tr := trackOf(t, 0)
		for _, svc := range services {
			if !scrobbable(tr, svc) {
				continue
			}
			cred, ok := s.credential(ctx, userID, svc)
			if !ok {
				continue
			}
			if svc == model.ScrobbleLastfm {
				err = s.lastfm.nowPlaying(ctx, creds, cred, tr)
			} else {
				err = s.lb.nowPlaying(ctx, cred, tr)
			}
			s.noteLiveError(ctx, userID, svc, err)
		}
	})
}

// Loved mirrors starring or unstarring tracks to the user's Last.fm loved tracks (best
// effort, background, at most maxLoveTracks per call).
func (s *Service) Loved(userID string, trackIDs []string, loved bool) {
	if len(trackIDs) == 0 {
		return
	}
	ids := append([]string(nil), trackIDs[:min(len(trackIDs), maxLoveTracks)]...)
	s.goLive(func(ctx context.Context) {
		services, creds := s.active(ctx)
		if !slices.Contains(services, model.ScrobbleLastfm) {
			return
		}
		cred, ok := s.credential(ctx, userID, model.ScrobbleLastfm)
		if !ok {
			return
		}
		ts, err := s.st.GetTracks(ctx, ids, "")
		if err != nil {
			return
		}
		for i := range ts {
			tr := trackOf(&ts[i], 0)
			if !scrobbable(tr, "") {
				continue
			}
			err := s.lastfm.love(ctx, creds, cred, tr, loved)
			s.noteLiveError(ctx, userID, model.ScrobbleLastfm, err)
			if err != nil {
				return
			}
		}
	})
}

// noteLiveError records what a now-playing or love request says about the account: a
// revoked credential or a configuration problem. Transient failures are only logged.
func (s *Service) noteLiveError(ctx context.Context, userID, service string, err error) {
	if err == nil {
		return
	}
	switch kindOf(err) {
	case kindAuth:
		_ = s.st.SetScrobbleAccountError(ctx, userID, service, err.Error(), true)
	case kindConfig:
		_ = s.st.SetScrobbleAccountError(ctx, userID, service, err.Error(), false)
	}
	slog.Debug("scrobbling request failed", "service", service, "err", err)
}

// ---- the sender

func (s *Service) loop() {
	defer close(s.done)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
		case <-s.wake:
		}
		s.Flush(s.ctx)
	}
}

// Flush sends every due queued play (called by the sender; exported for tests).
func (s *Service) Flush(ctx context.Context) {
	s.flush.Lock()
	defer s.flush.Unlock()
	// Old plays are dropped for every service, also one the administrator turned off since.
	for _, svc := range Services {
		if n, err := s.st.PurgeScrobblesBefore(ctx, svc, s.nowMs()-MaxAge.Milliseconds()); err != nil {
			slog.Warn("dropping old scrobbles", "service", svc, "err", err)
		} else if n > 0 {
			slog.Info("dropped scrobbles that waited too long", "service", svc, "count", n)
		}
	}
	services, creds := s.active(ctx)
	for _, svc := range services {
		users, err := s.st.ScrobbleDueUsers(ctx, svc, s.nowMs())
		if err != nil {
			slog.Warn("reading the scrobble queue", "service", svc, "err", err)
			continue
		}
		for _, userID := range users {
			if ctx.Err() != nil {
				return
			}
			s.flushUser(ctx, svc, userID, creds)
		}
	}
}

func (s *Service) flushUser(ctx context.Context, service, userID string, creds lastfmCreds) {
	cred, ok := s.credential(ctx, userID, service)
	if !ok {
		return // paused, revoked or unlinked: the plays wait (or age out)
	}
	batch := lastfmBatch
	if service == model.ScrobbleListenBrainz {
		batch = listenBrainzBatch
	}
	for round := 0; round < 20; round++ {
		due, err := s.st.DueScrobbles(ctx, userID, service, s.nowMs(), batch)
		if err != nil || len(due) == 0 {
			return
		}
		ids := make([]int64, 0, len(due))
		plays := make([]Track, 0, len(due))
		attempts := 0
		for _, q := range due {
			ids = append(ids, q.ID)
			attempts = max(attempts, q.Attempts)
			plays = append(plays, Track{Title: q.Title, Artist: q.Artist, Album: q.Album, AlbumArtist: q.AlbumArtist,
				TrackNumber: q.TrackNumber, Duration: q.Duration, MbzTrackID: q.MbzTrackID, PlayedAt: q.PlayedAt})
		}
		ignored := 0
		if service == model.ScrobbleLastfm {
			ignored, err = s.lastfm.scrobble(ctx, creds, cred, plays)
		} else {
			err = s.lb.submit(ctx, cred, plays)
		}
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			switch kindOf(err) {
			case kindRejected:
				// The service refuses these plays as such; retrying would never help.
				slog.Info("a scrobbling service refused plays", "service", service, "count", len(ids), "err", err)
				_ = s.st.DeleteScrobbles(ctx, ids)
				_ = s.st.SetScrobbleAccountError(ctx, userID, service, err.Error(), false)
				continue
			case kindAuth:
				_ = s.st.SetScrobbleAccountError(ctx, userID, service, err.Error(), true)
			default:
				_ = s.st.DeferScrobbles(ctx, ids, s.nowMs()+backoff(attempts+1).Milliseconds(), err.Error())
				_ = s.st.SetScrobbleAccountError(ctx, userID, service, err.Error(), false)
			}
			slog.Debug("scrobbling failed", "service", service, "count", len(ids), "err", err)
			return
		}
		if err := s.st.DeleteScrobbles(ctx, ids); err != nil {
			slog.Warn("removing sent scrobbles", "service", service, "err", err)
			return
		}
		_ = s.st.MarkScrobbleAccountSent(ctx, userID, service, s.nowMs())
		if ignored > 0 {
			slog.Debug("Last.fm ignored scrobbles", "count", ignored)
		}
		if len(due) < batch {
			return
		}
	}
}

// backoff is the wait before the n-th retry: 1, 2, 4, … minutes, at most maxBackoff.
func backoff(n int) time.Duration {
	d := time.Minute
	for i := 1; i < n && d < maxBackoff; i++ {
		d *= 2
	}
	return min(d, maxBackoff)
}

// Errors of the account and administration methods.
var (
	ErrInvalid       = errors.New("invalid scrobbling request")
	ErrDisabled      = errors.New("scrobbling to this service is turned off")
	ErrNotConfigured = errors.New("an API key and shared secret are needed for Last.fm")
	ErrNotLinked     = errors.New("no linked account")
	ErrExpired       = errors.New("the Last.fm sign-in expired; start again")
)
