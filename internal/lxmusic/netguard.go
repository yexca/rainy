package lxmusic

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"syscall"
	"time"
)

// Outbound requests on behalf of source scripts (and downloads of the links they return) may
// reach public internet addresses only. The check runs twice: on the URL before a request
// (IP literals, "localhost") and on every address actually dialled (after DNS resolution,
// so rebinding a public name to a private address does not help). Requests through an
// HTTP(S)_PROXY resolve the target first, because the proxy does the dialling.

// v4Prefix builds an IPv4 prefix (written as bytes, like the rest of the code base).
func v4Prefix(a, b, c, d byte, bits int) netip.Prefix {
	return netip.PrefixFrom(netip.AddrFrom4([4]byte{a, b, c, d}), bits)
}

// specialPrefixes are IPv4/IPv6 ranges that are not public unicast addresses and that
// netip.Addr's predicates do not cover.
var specialPrefixes = []netip.Prefix{
	v4Prefix(0, 0, 0, 0, 8),                 // "this" network
	v4Prefix(100, 64, 0, 0, 10),             // carrier-grade NAT
	v4Prefix(192, 0, 0, 0, 24),              // IETF protocol assignments
	v4Prefix(192, 0, 2, 0, 24),              // documentation (TEST-NET-1)
	v4Prefix(198, 18, 0, 0, 15),             // benchmarking
	v4Prefix(198, 51, 100, 0, 24),           // documentation (TEST-NET-2)
	v4Prefix(203, 0, 113, 0, 24),            // documentation (TEST-NET-3)
	v4Prefix(240, 0, 0, 0, 4),               // reserved, broadcast
	netip.MustParsePrefix("64:ff9b:1::/48"), // local-use NAT64
	netip.MustParsePrefix("100::/64"),       // discard
	netip.MustParsePrefix("2001:db8::/32"),  // documentation
}

// nat64 is the well-known NAT64 prefix; the embedded IPv4 address is checked instead.
var nat64 = netip.MustParsePrefix("64:ff9b::/96")

// blockedAddr reports whether a is not a public unicast address.
func blockedAddr(a netip.Addr) bool {
	a = a.Unmap()
	if a.Is6() && nat64.Contains(a) {
		b := a.As16()
		a = netip.AddrFrom4([4]byte{b[12], b[13], b[14], b[15]})
	}
	if !a.IsValid() || a.IsUnspecified() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() ||
		a.IsLinkLocalMulticast() || a.IsInterfaceLocalMulticast() || a.IsMulticast() {
		return true
	}
	for _, p := range specialPrefixes {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// checkURL validates a request URL: http(s), a host, no user info, and not a blocked IP
// literal or localhost name.
func checkURL(u *url.URL, allowPrivate bool) error {
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: only http and https URLs are allowed", ErrBlocked)
	}
	if u.User != nil {
		return fmt.Errorf("%w: URLs with user info are not allowed", ErrBlocked)
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return fmt.Errorf("%w: the URL has no host", ErrBlocked)
	}
	if allowPrivate {
		return nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("%w: %s", ErrBlocked, host)
	}
	if a, err := netip.ParseAddr(host); err == nil && blockedAddr(a) {
		return fmt.Errorf("%w: %s", ErrBlocked, a)
	}
	return nil
}

// proxyAddrs returns the host:port of the configured HTTP(S) proxies (dialled without the
// address check: an operator's proxy often lives on the local network).
func proxyAddrs() map[string]bool {
	out := map[string]bool{}
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy"} {
		v := strings.TrimSpace(os.Getenv(name))
		if v == "" {
			continue
		}
		if !strings.Contains(v, "://") {
			v = "http://" + v
		}
		u, err := url.Parse(v)
		if err != nil || u.Hostname() == "" {
			continue
		}
		port := u.Port()
		if port == "" {
			switch u.Scheme {
			case "https":
				port = "443"
			case "socks5", "socks5h":
				port = "1080"
			default:
				port = "80"
			}
		}
		out[net.JoinHostPort(u.Hostname(), port)] = true
	}
	return out
}

// guardedTransport checks every request (including each redirect hop) before sending it.
type guardedTransport struct {
	base         *http.Transport
	allowPrivate bool
}

func (g *guardedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := checkURL(req.URL, g.allowPrivate); err != nil {
		return nil, err
	}
	if !g.allowPrivate {
		if p, err := http.ProxyFromEnvironment(req); err == nil && p != nil {
			if err := resolvePublic(req.Context(), req.URL.Hostname()); err != nil {
				return nil, err
			}
		}
	}
	return g.base.RoundTrip(req)
}

// resolvePublic fails when host resolves to any blocked address.
func resolvePublic(ctx context.Context, host string) error {
	if _, err := netip.ParseAddr(host); err == nil {
		return nil // already checked by checkURL
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("%w: resolving %s: %v", ErrUpstream, host, err)
	}
	for _, a := range addrs {
		if blockedAddr(a) {
			return fmt.Errorf("%w: %s resolves to %s", ErrBlocked, host, a.Unmap())
		}
	}
	return nil
}

// newTransport returns a transport whose connections go to public addresses only (or to
// the configured proxy).
func newTransport(allowPrivate bool) *guardedTransport {
	proxies := proxyAddrs()
	plain := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	guarded := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			if allowPrivate {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("%w: %s", ErrBlocked, address)
			}
			a, err := netip.ParseAddr(host)
			if err != nil || blockedAddr(a) {
				return fmt.Errorf("%w: %s", ErrBlocked, host)
			}
			return nil
		},
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = http.ProxyFromEnvironment
	t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if proxies[addr] {
			return plain.DialContext(ctx, network, addr)
		}
		return guarded.DialContext(ctx, network, addr)
	}
	t.ResponseHeaderTimeout = 30 * time.Second
	t.MaxResponseHeaderBytes = 1 << 20
	t.MaxIdleConnsPerHost = 4
	t.IdleConnTimeout = 60 * time.Second
	return &guardedTransport{base: t, allowPrivate: allowPrivate}
}

// maxRedirects bounds redirects followed by fetch clients.
const maxRedirects = 5

// newClients returns the client for lx.request (redirects are returned to the script, like
// lx-music's needle client does) and the client for fetching script and audio URLs
// (following up to maxRedirects redirects, each one checked again).
func newClients(allowPrivate bool) (script, fetch *http.Client) {
	t := newTransport(allowPrivate)
	script = &http.Client{
		Transport:     t,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	fetch = &http.Client{
		Transport: t,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) > maxRedirects {
				return errors.New("too many redirects")
			}
			return nil
		},
	}
	return script, fetch
}

// requestError removes the *url.Error wrapper (its message repeats the full URL, which may
// carry tokens) and names the host instead.
func requestError(host string, err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	if errors.Is(err, ErrBlocked) {
		return err
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Err != nil && errors.Is(opErr.Err, ErrBlocked) {
		return opErr.Err
	}
	return fmt.Errorf("%w: %s: %w", ErrUpstream, host, err) // keeps *net.DNSError & co. inspectable
}
