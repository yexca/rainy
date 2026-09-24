// Package auth implements password storage, web sessions, API keys, login rate limiting
// and the HTTP middleware that puts the current user into the request context.
package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"rainy/internal/model"
	"rainy/internal/store"
)

// CookieName is the web session cookie.
const CookieName = "rainy_session"

// MinPasswordLength is the minimum password length in characters.
const MinPasswordLength = 4

// Login rate limiting: 10 failures within 10 minutes (per IP and per username) lock the
// key for 5 minutes.
const (
	maxFailures   = 10
	failureWindow = 10 * time.Minute
	lockoutPeriod = 5 * time.Minute
)

// sessionTouchInterval bounds how often a session's sliding expiry is written.
const sessionTouchInterval = time.Minute

var (
	// ErrInvalidCredentials is returned for a wrong username/password, unknown/expired
	// session or API key.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrRateLimited is returned when too many failed logins were attempted.
	ErrRateLimited = errors.New("too many failed login attempts, try again later")
	// ErrValidation is wrapped by input validation errors (bad username, short password).
	ErrValidation = errors.New("validation failed")
	// ErrInvalidUsername is returned for usernames that are empty, too long or contain
	// characters other than letters, digits and . _ - @ +.
	ErrInvalidUsername = fmt.Errorf("%w: username must be 1-64 characters: letters, digits or . _ - @ +", ErrValidation)
	// ErrWeakPassword is returned for passwords shorter than MinPasswordLength.
	ErrWeakPassword = fmt.Errorf("%w: password must be at least %d characters", ErrValidation, MinPasswordLength)
)

// Service handles users' credentials and sessions.
type Service struct {
	st      *store.Store
	crypto  *Crypto
	ttl     time.Duration
	limiter *limiter
	now     func() time.Time
	// dummyEnc is compared against for unknown usernames so login timing does not reveal
	// whether a username exists.
	dummyEnc string
}

// NewService creates the auth service. sessionTTL is the sliding session lifetime.
func NewService(st *store.Store, c *Crypto, sessionTTL time.Duration) *Service {
	if sessionTTL <= 0 {
		sessionTTL = 720 * time.Hour
	}
	s := &Service{st: st, crypto: c, ttl: sessionTTL, now: time.Now}
	s.limiter = newLimiter(maxFailures, failureWindow, lockoutPeriod, func() time.Time { return s.now() })
	s.dummyEnc, _ = c.Encrypt("rainy-dummy-password")
	return s
}

// Crypto returns the password cipher (Subsonic needs plain passwords for token auth).
func (s *Service) Crypto() *Crypto { return s.crypto }

// SessionTTL returns the configured session lifetime (for cookie Max-Age).
func (s *Service) SessionTTL() time.Duration { return s.ttl }

// ValidateUsername checks the username rules.
func ValidateUsername(username string) error {
	n := utf8.RuneCountInString(username)
	if n < 1 || n > 64 {
		return ErrInvalidUsername
	}
	for _, r := range username {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("._-@+", r) {
			continue
		}
		return ErrInvalidUsername
	}
	return nil
}

// ValidatePassword checks the password rules.
func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return ErrWeakPassword
	}
	return nil
}

// CreateUser validates the username and password, encrypts the password and inserts u.
// A duplicate username yields store.ErrConflict.
func (s *Service) CreateUser(ctx context.Context, u *model.User, password string) error {
	u.Username = strings.TrimSpace(u.Username)
	if err := ValidateUsername(u.Username); err != nil {
		return err
	}
	if err := ValidatePassword(password); err != nil {
		return err
	}
	enc, err := s.crypto.Encrypt(password)
	if err != nil {
		return err
	}
	u.PasswordEnc = enc
	return s.st.CreateUser(ctx, u)
}

// Password returns the user's plain password (for Subsonic token auth).
func (s *Service) Password(u *model.User) (string, error) {
	return s.crypto.Decrypt(u.PasswordEnc)
}

// CheckPassword reports whether password matches the user's stored password
// (constant-time comparison).
func (s *Service) CheckPassword(u *model.User, password string) bool {
	if u == nil {
		return false
	}
	plain, err := s.crypto.Decrypt(u.PasswordEnc)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(plain), []byte(password)) == 1
}

// ChangePassword validates and stores a new password. Existing sessions are kept.
func (s *Service) ChangePassword(ctx context.Context, userID, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	enc, err := s.crypto.Encrypt(password)
	if err != nil {
		return err
	}
	return s.st.SetUserPassword(ctx, userID, enc)
}

// Login checks credentials and creates a session. It returns ErrRateLimited when the IP
// or username is locked out and ErrInvalidCredentials for bad credentials.
func (s *Service) Login(ctx context.Context, username, password, userAgent, ip string) (string, *model.User, error) {
	username = strings.TrimSpace(username)
	userKey, ipKey := "user:"+strings.ToLower(username), "ip:"+ip
	if s.limiter.locked(ipKey) || s.limiter.locked(userKey) {
		return "", nil, ErrRateLimited
	}
	u, err := s.st.GetUserByUsername(ctx, username)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return "", nil, err
	}
	if u == nil {
		// Spend comparable time so response timing does not reveal unknown usernames.
		s.CheckPassword(&model.User{PasswordEnc: s.dummyEnc}, password)
	}
	if u == nil || !s.CheckPassword(u, password) {
		s.limiter.fail(ipKey)
		s.limiter.fail(userKey)
		return "", nil, ErrInvalidCredentials
	}
	s.limiter.reset(userKey)
	s.limiter.reset(ipKey)

	token, err := s.CreateSession(ctx, u.ID, userAgent, ip)
	if err != nil {
		return "", nil, err
	}
	if err := s.st.TouchUserLogin(ctx, u.ID); err != nil {
		return "", nil, err
	}
	_ = s.st.PurgeExpiredSessions(ctx)
	if fresh, err := s.st.GetUser(ctx, u.ID); err == nil {
		u = fresh
	}
	return token, u, nil
}

// CreateSession creates a new session for a user (used by login and first-run setup) and
// returns the plain token.
func (s *Service) CreateSession(ctx context.Context, userID, userAgent, ip string) (string, error) {
	token := NewToken()
	now := s.now()
	if len(userAgent) > 512 {
		userAgent = userAgent[:512]
	}
	err := s.st.CreateSession(ctx, &model.Session{
		TokenHash: HashToken(token), UserID: userID, UserAgent: userAgent, IP: ip,
		CreatedAt: now.UnixMilli(), LastSeenAt: now.UnixMilli(), ExpiresAt: now.Add(s.ttl).UnixMilli(),
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// Logout deletes the session of token.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.st.DeleteSession(ctx, HashToken(token))
}

// SessionUser returns the user of a valid session, extending its expiry (written at most
// once per minute). Unknown or expired sessions yield ErrInvalidCredentials.
func (s *Service) SessionUser(ctx context.Context, token string) (*model.User, error) {
	u, _, err := s.sessionUser(ctx, token)
	return u, err
}

// sessionUser is SessionUser that also reports whether the expiry was extended.
func (s *Service) sessionUser(ctx context.Context, token string) (*model.User, bool, error) {
	if token == "" {
		return nil, false, ErrInvalidCredentials
	}
	hash := HashToken(token)
	sess, err := s.st.GetSession(ctx, hash)
	if errors.Is(err, store.ErrNotFound) {
		return nil, false, ErrInvalidCredentials
	}
	if err != nil {
		return nil, false, err
	}
	now := s.now()
	if now.UnixMilli() >= sess.ExpiresAt {
		_ = s.st.DeleteSession(ctx, hash)
		return nil, false, ErrInvalidCredentials
	}
	u, err := s.st.GetUser(ctx, sess.UserID)
	if errors.Is(err, store.ErrNotFound) {
		_ = s.st.DeleteSession(ctx, hash)
		return nil, false, ErrInvalidCredentials
	}
	if err != nil {
		return nil, false, err
	}
	touched := false
	if now.Sub(time.UnixMilli(sess.LastSeenAt)) >= sessionTouchInterval {
		if err := s.st.TouchSessionAt(ctx, hash, now.UnixMilli(), now.Add(s.ttl).UnixMilli()); err == nil {
			touched = true
			_ = s.st.TouchUserSeen(ctx, u.ID)
		}
	}
	return u, touched, nil
}

// GenerateAPIKey creates a new API key for the user (replacing any previous one) and
// returns it; only its hash is stored, so it can be shown once.
func (s *Service) GenerateAPIKey(ctx context.Context, userID string) (string, error) {
	key := NewToken()
	hash := HashToken(key)
	if err := s.st.SetUserAPIKeyHash(ctx, userID, &hash); err != nil {
		return "", err
	}
	return key, nil
}

// RevokeAPIKey removes the user's API key.
func (s *Service) RevokeAPIKey(ctx context.Context, userID string) error {
	return s.st.SetUserAPIKeyHash(ctx, userID, nil)
}

// UserByAPIKey returns the owner of an API key (ErrInvalidCredentials if unknown).
func (s *Service) UserByAPIKey(ctx context.Context, key string) (*model.User, error) {
	if key == "" {
		return nil, ErrInvalidCredentials
	}
	u, err := s.st.GetUserByAPIKeyHash(ctx, HashToken(key))
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	return u, err
}
