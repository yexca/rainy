package scrobble

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	requestTimeout = 15 * time.Second
	maxResponse    = 1 << 20 // 1 MiB
)

// errKind says what a failed request means for the queue and the account.
type errKind int

const (
	kindRetry    errKind = iota // network trouble, rate limits, outages: try again later
	kindAuth                    // the stored credential was revoked: the user must link again
	kindConfig                  // the administrator's API key or secret is wrong: keep the queue
	kindRejected                // the service refused the request itself: drop it
)

// Error is a failed request to a scrobbling service. Message is the service's own text
// (untrusted; shown to the user as plain text).
type Error struct {
	Service string
	Code    int
	Message string
	kind    errKind
}

func (e *Error) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("%s: %s (%d)", serviceName(e.Service), e.Message, e.Code)
	}
	return fmt.Sprintf("%s: %s", serviceName(e.Service), e.Message)
}

// Retryable reports whether the request may succeed later (network trouble, rate limits,
// outages) rather than being refused.
func (e *Error) Retryable() bool { return e.kind == kindRetry }

func kindOf(err error) errKind {
	var e *Error
	if errors.As(err, &e) {
		return e.kind
	}
	return kindRetry
}

func serviceName(service string) string {
	switch service {
	case "lastfm":
		return "Last.fm"
	case "listenbrainz":
		return "ListenBrainz"
	}
	return service
}

// newHTTPClient returns the client for the scrobbling APIs: bounded time, HTTPS_PROXY /
// HTTP_PROXY honoured, no redirects (the APIs never redirect).
func newHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = requestTimeout
	return &http.Client{
		Timeout:   requestTimeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// do sends req and returns the status and at most maxResponse bytes of the body. Transport
// failures are retryable Errors that never include the request (it may carry credentials).
func do(c *http.Client, req *http.Request, service string) (int, []byte, error) {
	resp, err := c.Do(req)
	if err != nil {
		msg := "the service could not be reached"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			msg = "the request timed out"
		}
		return 0, nil, &Error{Service: service, Message: msg, kind: kindRetry}
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return 0, nil, &Error{Service: service, Message: "reading the response failed", kind: kindRetry}
	}
	if len(body) > maxResponse {
		return 0, nil, &Error{Service: service, Message: "the response is too large", kind: kindRetry}
	}
	return resp.StatusCode, body, nil
}

// clip bounds untrusted text from a service before it is stored or logged.
func clip(s string, n int) string {
	s = strings.TrimSpace(strings.ToValidUTF8(s, ""))
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n]) + "…"
}
