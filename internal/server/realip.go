package server

import (
	"net"
	"net/http"
	"strings"
)

// realIP replaces r.RemoteAddr with the client address reported by a trusted
// reverse proxy. Unlike chi's middleware.RealIP it ignores True-Client-IP
// (nginx and Caddy forward it from the client untouched, so it can be forged)
// and, for X-Forwarded-For, uses the last hop — the one appended by the proxy
// directly in front of Rainy — rather than the client-controlled first entry.
func realIP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ip := forwardedIP(r.Header); ip != "" {
			r.RemoteAddr = ip
		}
		next.ServeHTTP(w, r)
	})
}

func forwardedIP(h http.Header) string {
	if ip := strings.TrimSpace(h.Get("X-Real-IP")); net.ParseIP(ip) != nil {
		return ip
	}
	xff := h.Values("X-Forwarded-For")
	if len(xff) == 0 {
		return ""
	}
	hops := strings.Split(xff[len(xff)-1], ",")
	if ip := strings.TrimSpace(hops[len(hops)-1]); net.ParseIP(ip) != nil {
		return ip
	}
	return ""
}
