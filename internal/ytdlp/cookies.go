package ytdlp

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Cookies are credentials: they let anyone who holds them act as the signed-in user. Rainy
// therefore keeps them only in the data directory, encrypted with secret.key (like stored
// passwords), never returns them through the API and never logs them. Only the cookies of
// the site's own domain are kept. A download gets a private copy in its work directory,
// which yt-dlp sends to that site only (cookie domain matching), and which is deleted with
// the work directory.

const (
	maxCookieText = 512 << 10
	maxCookies    = 2000
	cookieHeader  = "# Netscape HTTP Cookie File"
)

// CookieInfo describes the stored cookies of one site. It never contains cookie values.
type CookieInfo struct {
	Site       string `json:"site"`
	Configured bool   `json:"configured"`
	Count      int    `json:"count"`
	SignedIn   bool   `json:"signedIn"`  // a sign-in cookie is present and not expired
	ExpiresAt  int64  `json:"expiresAt"` // ms; earliest expiry of the sign-in cookies, 0 = unknown / session
	UpdatedAt  int64  `json:"updatedAt"` // ms
}

// CookieSaveResult is the outcome of storing cookies.
type CookieSaveResult struct {
	CookieInfo
	Dropped int `json:"dropped"` // cookies of other domains that were discarded
}

type cookieLine struct {
	httpOnly                                      bool
	domain, subdomains, path, secure, name, value string
	expires                                       int64 // unix seconds, 0 = session cookie
}

func (c cookieLine) String() string {
	domain := c.domain
	if c.httpOnly {
		domain = "#HttpOnly_" + domain
	}
	return strings.Join([]string{domain, c.subdomains, c.path, c.secure, strconv.FormatInt(c.expires, 10), c.name, c.value}, "\t")
}

// parseCookies reads a Netscape cookies.txt export and keeps the cookies of site's domain.
// Error messages never quote the input.
func parseCookies(site, text string) (kept []cookieLine, dropped int, err error) {
	info, ok := sites[site]
	if !ok {
		return nil, 0, fmt.Errorf("%w: unknown site %q", ErrInvalid, site)
	}
	if len(text) > maxCookieText {
		return nil, 0, fmt.Errorf("%w: the cookie file is larger than %d KiB", ErrInvalid, maxCookieText>>10)
	}
	text = strings.TrimPrefix(text, string(rune(0xFEFF))) // byte order mark
	if t := strings.TrimSpace(text); strings.HasPrefix(t, "[") || strings.HasPrefix(t, "{") {
		return nil, 0, fmt.Errorf("%w: this looks like a JSON export; export the cookies in Netscape (cookies.txt) format", ErrInvalid)
	}
	index := map[string]int{}
	valid := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		c := cookieLine{}
		if rest, ok := strings.CutPrefix(line, "#HttpOnly_"); ok {
			c.httpOnly, line = true, rest
		} else if strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) == 6 { // some exporters drop the tab of an empty value
			f = append(f, "")
		}
		if len(f) != 7 || !validCookieFields(f) {
			continue
		}
		exp, _ := strconv.ParseFloat(f[4], 64)
		c.domain, c.subdomains, c.path, c.secure = f[0], strings.ToUpper(f[1]), f[2], strings.ToUpper(f[3])
		c.expires, c.name, c.value = int64(math.Max(0, exp)), f[5], f[6]
		valid++
		if !domainMatches(c.domain, info.cookieDomain) {
			dropped++
			continue
		}
		key := strings.ToLower(strings.TrimPrefix(c.domain, ".")) + "\t" + c.path + "\t" + c.name
		if i, ok := index[key]; ok {
			kept[i] = c // a later duplicate wins, as in a browser
			continue
		}
		if len(kept) >= maxCookies {
			return nil, 0, fmt.Errorf("%w: too many cookies (max %d)", ErrInvalid, maxCookies)
		}
		index[key] = len(kept)
		kept = append(kept, c)
	}
	if valid == 0 {
		return nil, 0, fmt.Errorf("%w: no cookies found; export cookies.txt in Netscape format with the “Get cookies.txt LOCALLY” extension", ErrInvalid)
	}
	if len(kept) == 0 {
		return nil, dropped, fmt.Errorf("%w: the file has no cookies for %s; export them while that site is open", ErrInvalid, info.cookieDomain)
	}
	return kept, dropped, nil
}

func validCookieFields(f []string) bool {
	if f[0] == "" || strings.ContainsAny(f[0], " /") || f[5] == "" {
		return false
	}
	for _, b := range []string{f[1], f[3]} {
		if !strings.EqualFold(b, "TRUE") && !strings.EqualFold(b, "FALSE") {
			return false
		}
	}
	if !strings.HasPrefix(f[2], "/") {
		return false
	}
	if exp, err := strconv.ParseFloat(f[4], 64); err != nil || math.IsNaN(exp) || math.IsInf(exp, 0) {
		return false
	}
	for _, s := range f {
		for _, r := range s {
			if r < 0x20 || r == 0x7f {
				return false
			}
		}
	}
	return true
}

// domainMatches reports whether a cookie domain belongs to want (the domain itself or a
// subdomain of it).
func domainMatches(domain, want string) bool {
	d := strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(domain), "."), ".")
	return d == want || strings.HasSuffix(d, "."+want)
}

func formatCookies(site string, kept []cookieLine) string {
	var b strings.Builder
	b.WriteString(cookieHeader + "\n# Stored by Rainy for " + site + ". These are credentials: never share them.\n")
	for _, c := range kept {
		b.WriteString(c.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func summarize(site string, kept []cookieLine, updated time.Time) CookieInfo {
	info := CookieInfo{Site: site, Configured: true, Count: len(kept), UpdatedAt: updated.UnixMilli()}
	now := time.Now().Unix()
	for _, c := range kept {
		if !isLoginCookie(site, c.name) {
			continue
		}
		if c.expires != 0 && c.expires <= now {
			continue
		}
		info.SignedIn = true
		if c.expires > 0 && (info.ExpiresAt == 0 || c.expires*1000 < info.ExpiresAt) {
			info.ExpiresAt = c.expires * 1000
		}
	}
	return info
}

func isLoginCookie(site, name string) bool {
	for _, n := range sites[site].loginCookies {
		if n == name {
			return true
		}
	}
	return false
}

func (s *Service) cookiePath(site string) string {
	return filepath.Join(s.opt.Dir, "cookies", site+".enc")
}

// SetCookies validates a cookies.txt export, keeps only the site's own cookies and stores
// them encrypted. Errors wrap ErrInvalid for bad input.
func (s *Service) SetCookies(site, text string) (CookieSaveResult, error) {
	kept, dropped, err := parseCookies(site, text)
	if err != nil {
		return CookieSaveResult{}, err
	}
	if s.opt.Cipher == nil {
		return CookieSaveResult{}, errors.New("ytdlp: no cipher configured")
	}
	enc, err := s.opt.Cipher.Encrypt(formatCookies(site, kept))
	if err != nil {
		return CookieSaveResult{}, fmt.Errorf("encrypting cookies: %w", err)
	}
	s.cookieMu.Lock()
	defer s.cookieMu.Unlock()
	p := s.cookiePath(site)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return CookieSaveResult{}, err
	}
	if err := writePrivateFile(p, []byte(enc)); err != nil {
		return CookieSaveResult{}, fmt.Errorf("saving cookies: %w", err)
	}
	return CookieSaveResult{CookieInfo: summarize(site, kept, time.Now()), Dropped: dropped}, nil
}

// DeleteCookies removes the stored cookies of site (no error when there are none).
func (s *Service) DeleteCookies(site string) error {
	if !ValidSite(site) {
		return fmt.Errorf("%w: unknown site %q", ErrInvalid, site)
	}
	s.cookieMu.Lock()
	defer s.cookieMu.Unlock()
	if err := os.Remove(s.cookiePath(site)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Cookies describes the stored cookies of every site, in Sites order.
func (s *Service) Cookies() []CookieInfo {
	out := make([]CookieInfo, 0, len(Sites))
	for _, site := range Sites {
		info := CookieInfo{Site: site}
		if kept, mod, ok := s.loadCookies(site); ok {
			info = summarize(site, kept, mod)
		}
		out = append(out, info)
	}
	return out
}

// HasCookies reports whether usable cookies are stored for site.
func (s *Service) HasCookies(site string) bool {
	_, _, ok := s.loadCookies(site)
	return ok
}

// loadCookies decrypts the stored cookies of site. A file that cannot be decrypted (the
// secret key was replaced) is ignored with a warning.
func (s *Service) loadCookies(site string) ([]cookieLine, time.Time, bool) {
	if s.opt.Cipher == nil || !ValidSite(site) {
		return nil, time.Time{}, false
	}
	s.cookieMu.Lock()
	defer s.cookieMu.Unlock()
	p := s.cookiePath(site)
	fi, err := os.Stat(p)
	if err != nil {
		return nil, time.Time{}, false
	}
	enc, err := os.ReadFile(p)
	if err != nil {
		slog.Warn("ytdlp: reading stored cookies", "site", site, "err", err)
		return nil, time.Time{}, false
	}
	plain, err := s.opt.Cipher.Decrypt(strings.TrimSpace(string(enc)))
	if err != nil {
		slog.Warn("ytdlp: stored cookies cannot be decrypted (was secret.key replaced?); add them again", "site", site)
		return nil, time.Time{}, false
	}
	kept, _, err := parseCookies(site, plain)
	if err != nil {
		return nil, time.Time{}, false
	}
	return kept, fi.ModTime(), true
}

// writeRunCookies writes a private copy of site's cookies into dir for one yt-dlp run and
// returns its path ("" when no cookies are stored). yt-dlp may rewrite the copy; the stored
// file is never handed to it.
func (s *Service) writeRunCookies(site, dir string) (string, error) {
	kept, _, ok := s.loadCookies(site)
	if !ok {
		return "", nil
	}
	p := filepath.Join(dir, "cookies.txt")
	if err := writePrivateFile(p, []byte(formatCookies(site, kept))); err != nil {
		return "", fmt.Errorf("preparing cookies: %w", err)
	}
	return p, nil
}

// writePrivateFile writes data to p atomically with owner-only permissions.
func writePrivateFile(p string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if err := f.Chmod(0o600); err != nil && !errors.Is(err, errors.ErrUnsupported) {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
