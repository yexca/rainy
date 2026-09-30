package scrobble

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"rainy/internal/model"
	"rainy/internal/store"
	"rainy/internal/util"
)

// pendingAuth is a Last.fm web authentication a user started: the browser comes back with
// the same state, which ties the returned token to this user.
type pendingAuth struct {
	state   string
	expires time.Time
}

const authTimeout = 15 * time.Minute

var (
	lastfmKeyPattern = regexp.MustCompile(`^[0-9a-fA-F]{32}$`) // API keys and shared secrets
	lastfmTokPattern = regexp.MustCompile(`^[0-9A-Za-z_-]{1,64}$`)
	lbTokenPattern   = regexp.MustCompile(`^[0-9A-Za-z-]{8,64}$`)
)

// AccountStatus is what a user sees about one service.
type AccountStatus struct {
	Service     string `json:"service"`     // lastfm | listenbrainz
	Available   bool   `json:"available"`   // the administrator turned the service on (and configured Last.fm)
	Linked      bool   `json:"linked"`      // an account is linked
	NeedsRelink bool   `json:"needsRelink"` // the service revoked the credential: link again
	Username    string `json:"username"`
	Enabled     bool   `json:"enabled"` // scrobbling is not paused
	Queued      int    `json:"queued"`  // plays waiting to be sent
	LastError   string `json:"lastError"`
	LastErrorAt int64  `json:"lastErrorAt"`
	LastSentAt  int64  `json:"lastSentAt"`
	LinkedAt    int64  `json:"linkedAt"`
}

// Status returns the user's status for every service, in Services order.
func (s *Service) Status(ctx context.Context, userID string) ([]AccountStatus, error) {
	services, _ := s.active(ctx)
	accounts, err := s.st.ListScrobbleAccounts(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]AccountStatus, 0, len(Services))
	for _, svc := range Services {
		st := AccountStatus{Service: svc}
		for _, a := range services {
			st.Available = st.Available || a == svc
		}
		for _, a := range accounts {
			if a.Service != svc {
				continue
			}
			st.Linked, st.NeedsRelink = true, a.CredentialEnc == ""
			st.Username, st.Enabled = a.Username, a.Enabled
			st.LastError, st.LastErrorAt, st.LastSentAt, st.LinkedAt = a.LastError, a.LastErrorAt, a.LastSentAt, a.CreatedAt
			if st.Queued, err = s.st.CountQueuedScrobbles(ctx, userID, svc); err != nil {
				return nil, err
			}
		}
		out = append(out, st)
	}
	return out, nil
}

func (s *Service) requireActive(ctx context.Context, service string) (lastfmCreds, error) {
	set := s.settings(ctx)
	switch service {
	case model.ScrobbleLastfm:
		if !set.LastfmEnabled {
			return lastfmCreds{}, ErrDisabled
		}
		creds, err := s.lastfmCreds(ctx)
		if err != nil {
			return creds, err
		}
		if creds.key == "" || creds.secret == "" {
			return creds, ErrNotConfigured
		}
		return creds, nil
	case model.ScrobbleListenBrainz:
		if !set.ListenBrainzEnabled {
			return lastfmCreds{}, ErrDisabled
		}
		return lastfmCreds{}, nil
	}
	return lastfmCreds{}, fmt.Errorf("%w: unknown service %q", ErrInvalid, service)
}

// LastfmAuthURL starts linking a Last.fm account: it returns the Last.fm page the browser
// opens. After the user allows access there, Last.fm sends the browser to callback with
// `state` and `token` added; the web app passes both to LinkLastfm.
func (s *Service) LastfmAuthURL(ctx context.Context, userID, callback string) (string, error) {
	creds, err := s.requireActive(ctx, model.ScrobbleLastfm)
	if err != nil {
		return "", err
	}
	cb, err := url.Parse(strings.TrimSpace(callback))
	if err != nil || len(callback) > 1024 || (cb.Scheme != "https" && cb.Scheme != "http") || cb.Host == "" || cb.User != nil || cb.Fragment != "" {
		return "", fmt.Errorf("%w: the callback must be an http(s) link to this server", ErrInvalid)
	}
	state := util.NewID()
	q := cb.Query()
	q.Set("state", state)
	q.Del("token")
	cb.RawQuery = q.Encode()
	s.mu.Lock()
	now := s.now()
	for id, p := range s.pending {
		if now.After(p.expires) {
			delete(s.pending, id)
		}
	}
	s.pending[userID] = pendingAuth{state: state, expires: now.Add(authTimeout)}
	s.mu.Unlock()
	return s.authURL + "?" + url.Values{"api_key": {creds.key}, "cb": {cb.String()}}.Encode(), nil
}

// LinkLastfm finishes linking: the state must be the one LastfmAuthURL issued to this user,
// and the token is exchanged for a session key.
func (s *Service) LinkLastfm(ctx context.Context, userID, token, state string) (*AccountStatus, error) {
	creds, err := s.requireActive(ctx, model.ScrobbleLastfm)
	if err != nil {
		return nil, err
	}
	if !lastfmTokPattern.MatchString(token) {
		return nil, fmt.Errorf("%w: missing or malformed token", ErrInvalid)
	}
	s.mu.Lock()
	p, ok := s.pending[userID]
	if ok && p.state == state {
		delete(s.pending, userID)
	}
	s.mu.Unlock()
	if !ok || state == "" || p.state != state || s.now().After(p.expires) {
		return nil, ErrExpired
	}
	name, key, err := s.lastfm.session(ctx, creds, token)
	if err != nil {
		return nil, err
	}
	return s.saveAccount(ctx, userID, model.ScrobbleLastfm, name, key)
}

// LinkListenBrainz links a ListenBrainz account by its user token (checked with the
// service first).
func (s *Service) LinkListenBrainz(ctx context.Context, userID, token string) (*AccountStatus, error) {
	if _, err := s.requireActive(ctx, model.ScrobbleListenBrainz); err != nil {
		return nil, err
	}
	token = strings.TrimSpace(token)
	if !lbTokenPattern.MatchString(token) {
		return nil, fmt.Errorf("%w: that does not look like a ListenBrainz user token", ErrInvalid)
	}
	name, err := s.lb.validate(ctx, token)
	if err != nil {
		return nil, err
	}
	return s.saveAccount(ctx, userID, model.ScrobbleListenBrainz, name, token)
}

func (s *Service) saveAccount(ctx context.Context, userID, service, name, credential string) (*AccountStatus, error) {
	enc, err := s.cipher.Encrypt(credential)
	if err != nil {
		return nil, err
	}
	a := &model.ScrobbleAccount{UserID: userID, Service: service, Username: name, CredentialEnc: enc}
	if old, err := s.st.GetScrobbleAccount(ctx, userID, service); err == nil {
		a.CreatedAt = old.CreatedAt
	}
	if err := s.st.SaveScrobbleAccount(ctx, a); err != nil {
		return nil, err
	}
	s.Wake()
	return s.accountStatus(ctx, userID, service)
}

func (s *Service) accountStatus(ctx context.Context, userID, service string) (*AccountStatus, error) {
	all, err := s.Status(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].Service == service {
			return &all[i], nil
		}
	}
	return nil, ErrNotLinked
}

// SetEnabled pauses or resumes scrobbling for a linked account. Pausing drops the plays
// still waiting.
func (s *Service) SetEnabled(ctx context.Context, userID, service string, enabled bool) (*AccountStatus, error) {
	if !ValidService(service) {
		return nil, fmt.Errorf("%w: unknown service %q", ErrInvalid, service)
	}
	if err := s.st.SetScrobbleAccountEnabled(ctx, userID, service, enabled); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrNotLinked
		}
		return nil, err
	}
	if enabled {
		s.Wake()
	}
	return s.accountStatus(ctx, userID, service)
}

// Unlink forgets the account and its queued plays. It does not revoke Rainy's access at
// the service; the user can do that in their account settings there.
func (s *Service) Unlink(ctx context.Context, userID, service string) error {
	if !ValidService(service) {
		return fmt.Errorf("%w: unknown service %q", ErrInvalid, service)
	}
	return s.st.DeleteScrobbleAccount(ctx, userID, service)
}

// ---- administration

// AdminInfo is the administrator's view of scrobbling.
type AdminInfo struct {
	Lastfm       LastfmAdmin       `json:"lastfm"`
	ListenBrainz ListenBrainzAdmin `json:"listenBrainz"`
}

// LastfmAdmin shows the Last.fm configuration; the shared secret is never returned.
type LastfmAdmin struct {
	Enabled    bool   `json:"enabled"`
	APIKey     string `json:"apiKey"`
	HasSecret  bool   `json:"hasSecret"`
	Configured bool   `json:"configured"`
	Users      int    `json:"users"` // users with a linked account
}

// ListenBrainzAdmin shows the ListenBrainz status.
type ListenBrainzAdmin struct {
	Enabled bool `json:"enabled"`
	Users   int  `json:"users"`
}

// AdminInfo returns the administrator's view.
func (s *Service) AdminInfo(ctx context.Context) (*AdminInfo, error) {
	set := s.settings(ctx)
	creds, err := s.lastfmCreds(ctx)
	if err != nil {
		return nil, err
	}
	lf, err := s.st.CountScrobbleAccounts(ctx, model.ScrobbleLastfm)
	if err != nil {
		return nil, err
	}
	lb, err := s.st.CountScrobbleAccounts(ctx, model.ScrobbleListenBrainz)
	if err != nil {
		return nil, err
	}
	return &AdminInfo{
		Lastfm: LastfmAdmin{Enabled: set.LastfmEnabled, APIKey: creds.key, HasSecret: creds.secret != "",
			Configured: creds.key != "" && creds.secret != "", Users: lf},
		ListenBrainz: ListenBrainzAdmin{Enabled: set.ListenBrainzEnabled, Users: lb},
	}, nil
}

// SetLastfmCredentials stores the Last.fm API key and/or shared secret (nil = unchanged,
// "" = remove). Both are the 32-character hex strings Last.fm shows for an API account.
func (s *Service) SetLastfmCredentials(ctx context.Context, apiKey, secret *string) error {
	if apiKey != nil {
		k := strings.TrimSpace(*apiKey)
		if k != "" && !lastfmKeyPattern.MatchString(k) {
			return fmt.Errorf("%w: the API key must be 32 hexadecimal characters", ErrInvalid)
		}
		if err := s.st.SetValue(ctx, store.ValueLastfmAppKey, k); err != nil {
			return err
		}
	}
	if secret != nil {
		v := strings.TrimSpace(*secret)
		if v != "" && !lastfmKeyPattern.MatchString(v) {
			return fmt.Errorf("%w: the shared secret must be 32 hexadecimal characters", ErrInvalid)
		}
		enc := ""
		if v != "" {
			var err error
			if enc, err = s.cipher.Encrypt(v); err != nil {
				return err
			}
		}
		if err := s.st.SetValue(ctx, store.ValueLastfmSigningEnc, enc); err != nil {
			return err
		}
	}
	return nil
}
