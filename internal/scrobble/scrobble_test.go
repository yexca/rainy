package scrobble

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"rainy/internal/auth"
	"rainy/internal/db/dbtest"
	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

var ctx = context.Background()

// Synthetic Last.fm API credentials (32 hex characters, like real ones).
var (
	testKey    = strings.Repeat("01", 16)
	testSecret = strings.Repeat("fe", 16)
)

// fakeLastfm is a Last.fm API stand-in that checks every signature.
type fakeLastfm struct {
	t     *testing.T
	mu    sync.Mutex
	calls []url.Values
	reply func(p url.Values) (int, string) // nil → success
}

func (f *fakeLastfm) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil || r.Method != http.MethodPost {
		f.t.Errorf("bad request %s %v", r.Method, err)
	}
	p := r.PostForm
	sig := p.Get("api_sig")
	check := url.Values{}
	for k, v := range p {
		if k != "api_sig" {
			check[k] = v
		}
	}
	if sig != lastfmSign(check, testSecret) || p.Get("api_key") != testKey || p.Get("format") != "json" {
		f.t.Errorf("bad signature or key for %v", p.Get("method"))
	}
	f.mu.Lock()
	f.calls = append(f.calls, p)
	reply := f.reply
	f.mu.Unlock()
	if reply != nil {
		status, body := reply(p)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
		return
	}
	switch p.Get("method") {
	case "auth.getSession":
		_, _ = io.WriteString(w, `{"session":{"name":"rain-listener","key":"session-key-1","subscriber":0}}`)
	case "track.scrobble":
		_, _ = io.WriteString(w, `{"scrobbles":{"scrobble":{},"@attr":{"accepted":1,"ignored":"0"}}}`)
	default:
		_, _ = io.WriteString(w, `{}`)
	}
}

// setReply changes the answer (under the lock: the server runs in other goroutines).
func (f *fakeLastfm) setReply(reply func(p url.Values) (int, string)) {
	f.mu.Lock()
	f.reply = reply
	f.mu.Unlock()
}

func (f *fakeLastfm) methods() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.calls))
	for _, c := range f.calls {
		out = append(out, c.Get("method"))
	}
	return out
}

func (f *fakeLastfm) last() url.Values {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[len(f.calls)-1]
}

type fakeLB struct {
	mu     sync.Mutex
	bodies []map[string]any
	status int
	tokens []string
}

func (f *fakeLB) setStatus(status int) {
	f.mu.Lock()
	f.status = status
	f.mu.Unlock()
}

func (f *fakeLB) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens = append(f.tokens, r.Header.Get("Authorization"))
	if r.Header.Get("Authorization") != "Token 11111111-2222-3333-4444-555555555555" {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"code":401,"error":"Invalid authorization token."}`)
		return
	}
	switch r.URL.Path {
	case "/1/validate-token":
		_, _ = io.WriteString(w, `{"code":200,"message":"Token valid.","valid":true,"user_name":"lb-listener"}`)
	case "/1/submit-listens":
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.bodies = append(f.bodies, body)
		if f.status != 0 {
			w.WriteHeader(f.status)
			_, _ = io.WriteString(w, `{"error":"try later"}`)
			return
		}
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

type env struct {
	s        *Service
	st       *store.Store
	settings model.Settings
	lf       *fakeLastfm
	lb       *fakeLB
	user     *model.User
	track    model.Track
	clock    time.Time
}

func newEnv(t *testing.T) *env {
	t.Helper()
	st := store.New(dbtest.New(t))
	e := &env{st: st, lf: &fakeLastfm{t: t}, lb: &fakeLB{}, clock: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)}
	lfSrv, lbSrv := httptest.NewServer(e.lf), httptest.NewServer(e.lb)
	t.Cleanup(lfSrv.Close)
	t.Cleanup(lbSrv.Close)
	e.settings = model.DefaultSettings(0)
	e.settings.LastfmEnabled, e.settings.ListenBrainzEnabled = true, true
	e.s = New(Options{
		Store: st, Cipher: auth.NewCrypto(bytes.Repeat([]byte{3}, 32)),
		Settings:  func(context.Context) model.Settings { return e.settings },
		LastfmAPI: lfSrv.URL, LastfmAuthURL: "https://auth.example.com/api/auth/", ListenBrainzAPI: lbSrv.URL,
		NoWorker: true,
	})
	e.s.now = func() time.Time { return e.clock }
	t.Cleanup(e.s.Close)
	key, secret := testKey, testSecret
	if err := e.s.SetLastfmCredentials(ctx, &key, &secret); err != nil {
		t.Fatal(err)
	}
	lib := &model.Library{Name: "Music", Path: t.TempDir()}
	if err := st.CreateLibrary(ctx, lib); err != nil {
		t.Fatal(err)
	}
	e.track = model.Track{ID: util.NewID(), LibraryID: lib.ID, Path: "a/1.flac", Title: "Night Rain", Artist: "The Rainy Days",
		Album: "Wet Streets", AlbumArtist: "Various Artists", AlbumID: util.AlbumID("Various Artists", "Wet Streets"),
		ArtistID: util.ArtistID("The Rainy Days"), AlbumArtistID: util.ArtistID("Various Artists"), TrackNumber: 3, Duration: 215.4}
	if err := st.UpsertTracks(ctx, []model.Track{e.track}); err != nil {
		t.Fatal(err)
	}
	e.user = &model.User{Username: "u", PasswordEnc: "synthetic"}
	if err := st.CreateUser(ctx, e.user); err != nil {
		t.Fatal(err)
	}
	return e
}

// linkLastfm runs the web authentication the way the browser does.
func (e *env) linkLastfm(t *testing.T) {
	t.Helper()
	raw, err := e.s.LastfmAuthURL(ctx, e.user.ID, "https://rainy.example.com/settings/lastfm?x=1")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	cb, _ := url.Parse(u.Query().Get("cb"))
	if u.Host != "auth.example.com" || u.Query().Get("api_key") != testKey || cb.Host != "rainy.example.com" || cb.Query().Get("x") != "1" {
		t.Fatalf("auth url %s", raw)
	}
	st, err := e.s.LinkLastfm(ctx, e.user.ID, "tok123", cb.Query().Get("state"))
	if err != nil {
		t.Fatal(err)
	}
	if !st.Linked || st.Username != "rain-listener" || !st.Enabled || !st.Available {
		t.Fatalf("status %+v", st)
	}
}

func (e *env) linkLB(t *testing.T) {
	t.Helper()
	if _, err := e.s.LinkListenBrainz(ctx, e.user.ID, " 11111111-2222-3333-4444-555555555555 "); err != nil {
		t.Fatal(err)
	}
}

func TestLastfmLinkAndScrobble(t *testing.T) {
	e := newEnv(t)
	e.linkLastfm(t)
	a, _ := e.st.GetScrobbleAccount(ctx, e.user.ID, model.ScrobbleLastfm)
	if a.CredentialEnc == "" || strings.Contains(a.CredentialEnc, "session-key-1") {
		t.Fatalf("session key stored in plain text: %q", a.CredentialEnc)
	}

	at := e.clock.Add(-time.Hour).UnixMilli()
	if err := e.s.Played(ctx, e.user.ID, e.track.ID, at, "web"); err != nil {
		t.Fatal(err)
	}
	if n, _ := e.st.CountQueuedScrobbles(ctx, e.user.ID, model.ScrobbleLastfm); n != 1 {
		t.Fatalf("queued %d", n)
	}
	// The play is recorded in the history too.
	if plays, total, _ := e.st.ListPlays(ctx, e.user.ID, 0, 0, 0, 10); total != 1 || plays[0].Title != "Night Rain" {
		t.Fatalf("history %+v", plays)
	}
	e.s.Flush(ctx)
	p := e.lf.last()
	if p.Get("method") != "track.scrobble" || p.Get("sk") != "session-key-1" || p.Get("artist[0]") != "The Rainy Days" ||
		p.Get("track[0]") != "Night Rain" || p.Get("album[0]") != "Wet Streets" || p.Get("albumArtist[0]") != "Various Artists" ||
		p.Get("timestamp[0]") != strconv.FormatInt(at/1000, 10) || p.Get("duration[0]") != "215" || p.Get("trackNumber[0]") != "3" {
		t.Fatalf("scrobble params %v", p)
	}
	if n, _ := e.st.CountQueuedScrobbles(ctx, e.user.ID, model.ScrobbleLastfm); n != 0 {
		t.Fatalf("still queued: %d", n)
	}
	st, _ := e.s.Status(ctx, e.user.ID)
	if st[0].LastSentAt == 0 || st[0].LastError != "" || st[0].Queued != 0 {
		t.Fatalf("status after sending %+v", st[0])
	}
}

func TestLastfmStateMustMatch(t *testing.T) {
	e := newEnv(t)
	if _, err := e.s.LinkLastfm(ctx, e.user.ID, "tok123", "made-up"); !errors.Is(err, ErrExpired) {
		t.Fatalf("link without a started sign-in: %v", err)
	}
	raw, _ := e.s.LastfmAuthURL(ctx, e.user.ID, "https://rainy.example.com/settings/lastfm")
	u, _ := url.Parse(raw)
	cb, _ := url.Parse(u.Query().Get("cb"))
	state := cb.Query().Get("state")
	// Another user cannot finish this user's sign-in.
	other := &model.User{Username: "o", PasswordEnc: "synthetic"}
	_ = e.st.CreateUser(ctx, other)
	if _, err := e.s.LinkLastfm(ctx, other.ID, "tok123", state); !errors.Is(err, ErrExpired) {
		t.Fatalf("cross-user link: %v", err)
	}
	e.clock = e.clock.Add(authTimeout + time.Second)
	if _, err := e.s.LinkLastfm(ctx, e.user.ID, "tok123", state); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired sign-in: %v", err)
	}
	if len(e.lf.methods()) != 0 {
		t.Fatalf("Last.fm was contacted: %v", e.lf.methods())
	}
	for _, cb := range []string{"", "javascript:alert(1)", "https://" + "user:pw@rainy.example.com/" /* user info */, "/settings", "ftp://rainy.example.com/"} {
		if _, err := e.s.LastfmAuthURL(ctx, e.user.ID, cb); !errors.Is(err, ErrInvalid) {
			t.Errorf("callback %q: %v", cb, err)
		}
	}
}

func TestLastfmErrors(t *testing.T) {
	e := newEnv(t)
	e.linkLastfm(t)
	at := e.clock.Add(-time.Hour).UnixMilli()
	_ = e.s.Played(ctx, e.user.ID, e.track.ID, at, "web")

	// An outage defers the play with a backoff.
	e.lf.setReply(func(url.Values) (int, string) {
		return http.StatusServiceUnavailable, `{"error":11,"message":"Service Offline"}`
	})
	e.s.Flush(ctx)
	due, _ := e.st.DueScrobbles(ctx, e.user.ID, model.ScrobbleLastfm, e.clock.UnixMilli(), 10)
	if len(due) != 0 {
		t.Fatal("deferred play is due immediately")
	}
	due, _ = e.st.DueScrobbles(ctx, e.user.ID, model.ScrobbleLastfm, e.clock.Add(time.Minute).UnixMilli(), 10)
	if len(due) != 1 || due[0].Attempts != 1 || !strings.Contains(due[0].LastError, "Service Offline") {
		t.Fatalf("deferred %+v", due)
	}
	st, _ := e.s.Status(ctx, e.user.ID)
	if st[0].LastError == "" || st[0].NeedsRelink {
		t.Fatalf("status after outage %+v", st[0])
	}

	// A revoked session asks the user to link again and keeps the play.
	e.clock = e.clock.Add(time.Minute)
	e.lf.setReply(func(url.Values) (int, string) {
		return http.StatusForbidden, `{"error":9,"message":"Invalid session key"}`
	})
	e.s.Flush(ctx)
	st, _ = e.s.Status(ctx, e.user.ID)
	if !st[0].NeedsRelink || st[0].Queued != 1 {
		t.Fatalf("status after revocation %+v", st[0])
	}
	e.lf.setReply(nil)
	calls := len(e.lf.methods())
	e.s.Flush(ctx)
	if len(e.lf.methods()) != calls {
		t.Fatal("sent with a revoked session")
	}
	e.linkLastfm(t) // linking again sends the waiting play
	e.s.Flush(ctx)
	if n, _ := e.st.CountQueuedScrobbles(ctx, e.user.ID, model.ScrobbleLastfm); n != 0 {
		t.Fatalf("after relinking %d queued", n)
	}

	// Refused plays are dropped instead of retried forever.
	_ = e.s.Played(ctx, e.user.ID, e.track.ID, at, "web")
	e.lf.setReply(func(url.Values) (int, string) {
		return http.StatusBadRequest, `{"error":6,"message":"Invalid parameters"}`
	})
	e.s.Flush(ctx)
	if n, _ := e.st.CountQueuedScrobbles(ctx, e.user.ID, model.ScrobbleLastfm); n != 0 {
		t.Fatalf("refused play kept: %d", n)
	}

	// Plays older than MaxAge are dropped without being sent.
	e.lf.setReply(nil)
	_ = e.s.Played(ctx, e.user.ID, e.track.ID, e.clock.Add(-MaxAge-time.Hour).UnixMilli(), "web")
	calls = len(e.lf.methods())
	e.s.Flush(ctx)
	if n, _ := e.st.CountQueuedScrobbles(ctx, e.user.ID, model.ScrobbleLastfm); n != 0 || len(e.lf.methods()) != calls {
		t.Fatalf("old play: queued %d, calls %d → %d", n, calls, len(e.lf.methods()))
	}
}

func TestDisabledServicesSendNothing(t *testing.T) {
	e := newEnv(t)
	e.linkLastfm(t)
	e.linkLB(t)
	e.settings.LastfmEnabled, e.settings.ListenBrainzEnabled = false, false
	if err := e.s.Played(ctx, e.user.ID, e.track.ID, 0, "web"); err != nil {
		t.Fatal(err)
	}
	e.s.NowPlaying(e.user.ID, e.track.ID)
	e.s.Loved(e.user.ID, []string{e.track.ID}, true)
	e.s.Flush(ctx)
	e.s.liveWG.Wait()
	if n, _ := e.st.CountQueuedScrobbles(ctx, e.user.ID, model.ScrobbleLastfm); n != 0 {
		t.Fatal("queued while disabled")
	}
	if m := e.lf.methods(); len(m) != 1 { // only the auth.getSession of linking
		t.Fatalf("Last.fm calls while disabled: %v", m)
	}
	if _, err := e.s.LastfmAuthURL(ctx, e.user.ID, "https://rainy.example.com/"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("auth while disabled: %v", err)
	}
	// Enabled but without credentials.
	e.settings.LastfmEnabled = true
	empty := ""
	_ = e.s.SetLastfmCredentials(ctx, nil, &empty)
	if _, err := e.s.LastfmAuthURL(ctx, e.user.ID, "https://rainy.example.com/"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("auth without a secret: %v", err)
	}
	info, _ := e.s.AdminInfo(ctx)
	if info.Lastfm.HasSecret || info.Lastfm.Configured || info.Lastfm.APIKey != testKey || info.Lastfm.Users != 1 {
		t.Fatalf("admin info %+v", info.Lastfm)
	}
}

func TestNowPlayingAndLoves(t *testing.T) {
	e := newEnv(t)
	e.linkLastfm(t)
	e.linkLB(t)
	e.s.NowPlaying(e.user.ID, e.track.ID)
	e.s.liveWG.Wait()
	e.s.Loved(e.user.ID, []string{e.track.ID}, true)
	e.s.liveWG.Wait()
	e.s.Loved(e.user.ID, []string{e.track.ID}, false)
	e.s.liveWG.Wait()
	got := strings.Join(e.lf.methods(), ",")
	if got != "auth.getSession,track.updateNowPlaying,track.love,track.unlove" {
		t.Fatalf("Last.fm calls %s", got)
	}
	e.lb.mu.Lock()
	defer e.lb.mu.Unlock()
	if len(e.lb.bodies) != 1 || e.lb.bodies[0]["listen_type"] != "playing_now" {
		t.Fatalf("ListenBrainz bodies %v", e.lb.bodies)
	}
}

func TestListenBrainz(t *testing.T) {
	e := newEnv(t)
	if _, err := e.s.LinkListenBrainz(ctx, e.user.ID, "99999999-2222-3333-4444-555555555555"); err == nil {
		t.Fatal("linked with a token the service refused")
	}
	if _, err := e.s.LinkListenBrainz(ctx, e.user.ID, "not a token"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("malformed token: %v", err)
	}
	e.linkLB(t)
	for i := range 3 {
		_ = e.s.Played(ctx, e.user.ID, e.track.ID, e.clock.Add(-time.Duration(i+1)*time.Minute).UnixMilli(), "web")
	}
	e.lb.setStatus(http.StatusTooManyRequests)
	e.s.Flush(ctx)
	if n, _ := e.st.CountQueuedScrobbles(ctx, e.user.ID, model.ScrobbleListenBrainz); n != 3 {
		t.Fatalf("rate limited plays: %d queued", n)
	}
	e.lb.setStatus(0)
	e.clock = e.clock.Add(time.Minute)
	e.s.Flush(ctx)
	if n, _ := e.st.CountQueuedScrobbles(ctx, e.user.ID, model.ScrobbleListenBrainz); n != 0 {
		t.Fatalf("still queued: %d", n)
	}
	e.lb.mu.Lock()
	body := e.lb.bodies[len(e.lb.bodies)-1]
	e.lb.mu.Unlock()
	payload := body["payload"].([]any)
	first := payload[0].(map[string]any)
	meta := first["track_metadata"].(map[string]any)
	info := meta["additional_info"].(map[string]any)
	if body["listen_type"] != "import" || len(payload) != 3 || meta["artist_name"] != "The Rainy Days" ||
		meta["release_name"] != "Wet Streets" || info["duration_ms"] != float64(215400) || first["listened_at"] == nil {
		t.Fatalf("submission %v", body)
	}

	// Pausing drops the waiting plays; unlinking forgets the account.
	_ = e.s.Played(ctx, e.user.ID, e.track.ID, 0, "web")
	if st, err := e.s.SetEnabled(ctx, e.user.ID, model.ScrobbleListenBrainz, false); err != nil || st.Enabled || st.Queued != 0 {
		t.Fatalf("paused %+v %v", st, err)
	}
	if err := e.s.Unlink(ctx, e.user.ID, model.ScrobbleListenBrainz); err != nil {
		t.Fatal(err)
	}
	st, _ := e.s.Status(ctx, e.user.ID)
	if st[1].Service != model.ScrobbleListenBrainz || st[1].Linked {
		t.Fatalf("after unlinking %+v", st[1])
	}
	if _, err := e.s.SetEnabled(ctx, e.user.ID, model.ScrobbleListenBrainz, true); !errors.Is(err, ErrNotLinked) {
		t.Fatalf("resume unlinked: %v", err)
	}
}

func TestScrobbable(t *testing.T) {
	base := Track{Title: "T", Artist: "A", Duration: 200}
	cases := []struct {
		t       Track
		service string
		want    bool
	}{
		{base, model.ScrobbleLastfm, true},
		{Track{Title: "T", Artist: "[Unknown Artist]", Duration: 200}, model.ScrobbleLastfm, false},
		{Track{Title: "", Artist: "A"}, model.ScrobbleListenBrainz, false},
		{Track{Title: "T", Artist: "A", Duration: 25}, model.ScrobbleLastfm, false},
		{Track{Title: "T", Artist: "A", Duration: 25}, model.ScrobbleListenBrainz, true},
	}
	for _, c := range cases {
		if got := scrobbable(c.t, c.service); got != c.want {
			t.Errorf("%+v on %s: %v", c.t, c.service, got)
		}
	}
	if backoff(1) != time.Minute || backoff(3) != 4*time.Minute || backoff(40) != maxBackoff {
		t.Fatal("backoff")
	}
}

func TestSetLastfmCredentialsValidates(t *testing.T) {
	e := newEnv(t)
	bad := "not-hex"
	if err := e.s.SetLastfmCredentials(ctx, &bad, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad key: %v", err)
	}
	if err := e.s.SetLastfmCredentials(ctx, nil, &bad); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad secret: %v", err)
	}
	enc, _ := e.st.GetValue(ctx, store.ValueLastfmSigningEnc)
	if enc == "" || strings.Contains(enc, testSecret) {
		t.Fatalf("secret stored in plain text: %q", enc)
	}
}

// A redirect never drops plays: the APIs do not redirect, so it is treated as a transient
// problem (a proxy or captive portal) and retried.
func TestRedirectIsRetried(t *testing.T) {
	e := newEnv(t)
	e.linkLastfm(t)
	_ = e.s.Played(ctx, e.user.ID, e.track.ID, e.clock.Add(-time.Hour).UnixMilli(), "web")
	e.lf.setReply(func(url.Values) (int, string) { return http.StatusFound, "" })
	e.s.Flush(ctx)
	if n, _ := e.st.CountQueuedScrobbles(ctx, e.user.ID, model.ScrobbleLastfm); n != 1 {
		t.Fatalf("redirected play: %d queued", n)
	}
}
