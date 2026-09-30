package ytdlp

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Supported sites.
const (
	SiteYouTube  = "youtube"
	SiteBilibili = "bilibili"
)

// Sites lists the supported site ids in display order.
var Sites = []string{SiteYouTube, SiteBilibili}

// siteInfo describes what Rainy accepts and keeps for one site.
type siteInfo struct {
	// cookieDomain is the registrable domain whose cookies are kept; cookies for any other
	// domain are dropped before they are stored.
	cookieDomain string
	// loginCookies are cookie names that only exist while signed in.
	loginCookies []string
}

var sites = map[string]siteInfo{
	SiteYouTube:  {cookieDomain: "youtube.com", loginCookies: []string{"SID", "__Secure-1PSID", "__Secure-3PSID", "SAPISID", "LOGIN_INFO"}},
	SiteBilibili: {cookieDomain: "bilibili.com", loginCookies: []string{"SESSDATA"}},
}

// ValidSite reports whether id is a supported site.
func ValidSite(id string) bool {
	_, ok := sites[id]
	return ok
}

// hostRule maps an accepted URL host to its site and to the host passed to yt-dlp ("" keeps
// it). Only exact hosts are accepted: yt-dlp runs without its generic extractor, and this
// list keeps it from being pointed at any other server (for example an internal address).
type hostRule struct{ site, host string }

var hosts = map[string]hostRule{
	"youtube.com":        {SiteYouTube, "www.youtube.com"},
	"www.youtube.com":    {SiteYouTube, ""},
	"m.youtube.com":      {SiteYouTube, "www.youtube.com"},
	"music.youtube.com":  {SiteYouTube, ""},
	"youtu.be":           {SiteYouTube, ""},
	"bilibili.com":       {SiteBilibili, "www.bilibili.com"},
	"www.bilibili.com":   {SiteBilibili, ""},
	"m.bilibili.com":     {SiteBilibili, "www.bilibili.com"}, // yt-dlp only matches www
	"space.bilibili.com": {SiteBilibili, ""},
	shortLinkHost:        {SiteBilibili, ""},
}

// shortLinkHost serves bilibili's share links, which redirect to www.bilibili.com. yt-dlp has
// no extractor for them, so Rainy resolves the redirect itself (see Service.resolveShort).
const shortLinkHost = "b23.tv"

// maxURLLength bounds the accepted link.
const maxURLLength = 2048

// Target is a validated link.
type Target struct {
	Site string // SiteYouTube or SiteBilibili
	URL  string // normalized https URL
}

// Short reports whether the link is a b23.tv share link that must be resolved first.
func (t Target) Short() bool {
	u, err := url.Parse(t.URL)
	return err == nil && u.Host == shortLinkHost
}

// urlInText finds the first http(s) URL in pasted text such as bilibili's share text
// "【Title】 https://b23.tv/AbCd".
var urlInText = regexp.MustCompile(`(?i)https?://[^\s"'<>，。、【】（）《》「」]+`)

// ParseURL validates a pasted YouTube or bilibili link. It accepts share text that contains
// the link, adds a missing "https://", and rejects every host that is not on the list above,
// credentials, ports and schemes other than http(s). Errors wrap ErrInvalid.
func ParseURL(text string) (Target, error) {
	text = strings.TrimSpace(text)
	raw := urlInText.FindString(text)
	if raw == "" {
		if text == "" || strings.ContainsAny(text, " \t\r\n") || strings.Contains(text, "://") {
			return Target{}, fmt.Errorf("%w: paste a YouTube or bilibili link", ErrInvalid)
		}
		raw = "https://" + text
	}
	if len(raw) > maxURLLength {
		return Target{}, fmt.Errorf("%w: the link is too long", ErrInvalid)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return Target{}, fmt.Errorf("%w: the link is not a valid URL", ErrInvalid)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "https" && scheme != "http" {
		return Target{}, fmt.Errorf("%w: only http and https links are supported", ErrInvalid)
	}
	if u.User != nil {
		return Target{}, fmt.Errorf("%w: links with a user name or password are not supported", ErrInvalid)
	}
	if p := u.Port(); p != "" && p != "443" && p != "80" {
		return Target{}, fmt.Errorf("%w: links with a port are not supported", ErrInvalid)
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	rule, ok := hosts[host]
	if !ok {
		return Target{}, fmt.Errorf("%w: only YouTube and bilibili links are supported", ErrInvalid)
	}
	if rule.host != "" {
		host = rule.host
	}
	u.Scheme, u.Host, u.Fragment, u.RawFragment = "https", host, "", ""
	return Target{Site: rule.site, URL: u.String()}, nil
}
