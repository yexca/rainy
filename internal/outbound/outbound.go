// Package outbound builds the HTTP clients Rainy uses to reach other servers. Every client
// has an explicit redirect policy, uses the environment's proxy settings (HTTPS_PROXY /
// HTTP_PROXY / NO_PROXY), and bounds dial and response-header time. Code outside this
// package never builds an http.Client or uses http.DefaultClient, http.DefaultTransport,
// http.Get, or http.Post (a test in this package enforces it).
//
// The package is a primitive: it imports only the standard library.
package outbound

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Defaults for TransportOptions fields left zero.
const (
	DefaultDialTimeout           = 10 * time.Second
	DefaultResponseHeaderTimeout = 30 * time.Second
	DefaultMaxResponseHeader     = 1 << 20
)

var (
	// ErrRedirect is returned (wrapped in the client's *url.Error) when a redirect breaks
	// the client's policy. Its message names the target host, never the full URL.
	ErrRedirect = errors.New("redirect refused")
	// ErrTooLarge is returned by ReadAll when the body is longer than the limit.
	ErrTooLarge = errors.New("response too large")
)

// Redirects is a client's redirect policy. The zero value follows no redirect: the client
// returns the 3xx response itself, for the caller to treat as an answer or an error.
type Redirects struct {
	// Max is the number of redirects followed; 0 follows none.
	Max int
	// HTTPSOnly refuses redirects to plain http. Otherwise http and https are accepted.
	HTTPSOnly bool
	// Allow, when set, must accept every redirect target. first is the original request's
	// URL. Use SameHost or a host allow-list; nil accepts any host (only for clients whose
	// transport checks every hop, such as the lxmusic guarded transport).
	Allow func(target, first *url.URL) bool
}

// Check is an http.Client CheckRedirect function enforcing r. Targets with user info or a
// scheme other than http(s) are always refused.
func (r Redirects) Check(req *http.Request, via []*http.Request) error {
	if r.Max <= 0 {
		return http.ErrUseLastResponse
	}
	host := req.URL.Hostname()
	if len(via) > r.Max {
		return fmt.Errorf("%w: more than %d redirects (last to %s)", ErrRedirect, r.Max, host)
	}
	switch {
	case req.URL.Scheme != "https" && (r.HTTPSOnly || req.URL.Scheme != "http"),
		req.URL.User != nil,
		host == "",
		r.Allow != nil && !r.Allow(req.URL, via[0].URL):
		return fmt.Errorf("%w: to %s", ErrRedirect, host)
	}
	return nil
}

// SameHost is a Redirects.Allow function that keeps redirects on the original host.
func SameHost(target, first *url.URL) bool {
	return strings.EqualFold(target.Hostname(), first.Hostname())
}

// TransportOptions tune NewTransport. Zero fields take the defaults above.
type TransportOptions struct {
	DialTimeout           time.Duration
	ResponseHeaderTimeout time.Duration
	// DialContext replaces the default dialer (DialTimeout is then ignored), for example to
	// check every dialled address.
	DialContext func(ctx context.Context, network, addr string) (net.Conn, error)
	// Proxy chooses the proxy per request; nil uses http.ProxyFromEnvironment.
	Proxy func(*http.Request) (*url.URL, error)
}

// NewTransport returns a transport with Go's default pooling and TLS settings and the
// given proxy, dial, and header limits. Callers may adjust the result before use.
func NewTransport(o TransportOptions) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = o.Proxy
	if t.Proxy == nil {
		t.Proxy = http.ProxyFromEnvironment
	}
	t.DialContext = o.DialContext
	if t.DialContext == nil {
		d := o.DialTimeout
		if d <= 0 {
			d = DefaultDialTimeout
		}
		t.DialContext = (&net.Dialer{Timeout: d, KeepAlive: 30 * time.Second}).DialContext
	}
	t.ResponseHeaderTimeout = o.ResponseHeaderTimeout
	if t.ResponseHeaderTimeout <= 0 {
		t.ResponseHeaderTimeout = DefaultResponseHeaderTimeout
	}
	t.MaxResponseHeaderBytes = DefaultMaxResponseHeader
	return t
}

// Options configure NewClient.
type Options struct {
	// Timeout bounds a whole exchange, including reading the body; 0 relies on the request
	// context alone (for long downloads).
	Timeout time.Duration
	// Transport sends the requests; nil uses NewTransport(TransportOptions{}). A custom
	// transport, such as one that checks every hop's address, is used as is.
	Transport http.RoundTripper
	// Redirects is the redirect policy; the zero value follows none.
	Redirects Redirects
}

// NewClient returns a client for outbound requests.
func NewClient(o Options) *http.Client {
	t := o.Transport
	if t == nil {
		t = NewTransport(TransportOptions{})
	}
	return &http.Client{Timeout: o.Timeout, Transport: t, CheckRedirect: o.Redirects.Check}
}

// ReadAll reads at most limit bytes of r. A longer body returns ErrTooLarge and no data.
func ReadAll(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrTooLarge
	}
	return data, nil
}
