package server

import (
	"net/http"
	"testing"
)

func TestForwardedIP(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string][]string
		want    string
	}{
		{"none", nil, ""},
		{"x-real-ip", map[string][]string{"X-Real-Ip": {"203.0.113.7"}}, "203.0.113.7"},
		{"true-client-ip ignored", map[string][]string{"True-Client-Ip": {"192.0.2.44"}}, ""},
		{"xff last hop", map[string][]string{"X-Forwarded-For": {"192.0.2.66, 203.0.113.9"}}, "203.0.113.9"},
		{"xff multiple headers", map[string][]string{"X-Forwarded-For": {"192.0.2.66", "198.51.100.2"}}, "198.51.100.2"},
		{"x-real-ip wins", map[string][]string{"X-Real-Ip": {"198.51.100.1"}, "X-Forwarded-For": {"192.0.2.66"}}, "198.51.100.1"},
		{"garbage", map[string][]string{"X-Real-Ip": {"nope"}, "X-Forwarded-For": {"also nope"}}, ""},
		{"ipv6", map[string][]string{"X-Real-Ip": {"2001:db8::1"}}, "2001:db8::1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := forwardedIP(http.Header(tt.headers)); got != tt.want {
				t.Fatalf("forwardedIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
