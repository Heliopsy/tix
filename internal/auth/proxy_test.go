// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// proxyRequest builds a request from peer carrying the forwarded headers.
func proxyRequest(peer, proto, forwardedFor string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/whoami", nil)
	r.RemoteAddr = peer
	if proto != "" {
		r.Header.Set(HeaderForwardedProto, proto)
	}
	if forwardedFor != "" {
		r.Header.Set(HeaderForwardedFor, forwardedFor)
	}
	return r
}

func TestProxyPolicyScheme(t *testing.T) {
	tests := []struct {
		name    string
		trusted []string
		peer    string
		proto   string
		tls     bool
		want    string
	}{
		{"no trust, forwarded ignored", nil, "203.0.113.9:5000", "https", false, SchemeHTTP},
		{"untrusted peer forging https", []string{"127.0.0.1"}, "203.0.113.9:5000", "https", false, SchemeHTTP},
		{"trusted peer reports https", []string{"127.0.0.1"}, "127.0.0.1:5000", "https", false, SchemeHTTPS},
		{"trusted peer reports http", []string{"127.0.0.1"}, "127.0.0.1:5000", "http", false, SchemeHTTP},
		{"trusted peer, chain", []string{"127.0.0.1"}, "127.0.0.1:5000", "https, http", false, SchemeHTTPS},
		{"trusted peer, junk header", []string{"127.0.0.1"}, "127.0.0.1:5000", "gopher", false, SchemeHTTP},
		{"trusted cidr", []string{"10.0.0.0/8"}, "10.4.2.1:5000", "https", false, SchemeHTTPS},
		{"direct tls, no header", nil, "203.0.113.9:5000", "", true, SchemeHTTPS},
		{"ipv6 trusted peer", []string{"::1"}, "[::1]:5000", "https", false, SchemeHTTPS},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewProxyPolicy(tt.trusted)
			if err != nil {
				t.Fatalf("policy: %v", err)
			}
			r := proxyRequest(tt.peer, tt.proto, "")
			if tt.tls {
				r.TLS = &tls.ConnectionState{}
			}
			if got := p.Scheme(r); got != tt.want {
				t.Fatalf("scheme = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProxyPolicyClientIP(t *testing.T) {
	tests := []struct {
		name    string
		trusted []string
		peer    string
		xff     string
		want    string
	}{
		{"no trust returns peer", nil, "203.0.113.9:5000", "198.51.100.7", "203.0.113.9"},
		{"untrusted peer forging xff", []string{"127.0.0.1"}, "203.0.113.9:5000", "198.51.100.7", "203.0.113.9"},
		{"trusted peer honours xff", []string{"127.0.0.1"}, "127.0.0.1:5000", "198.51.100.7", "198.51.100.7"},
		{"skips trusted hops", []string{"127.0.0.1", "10.0.0.0/8"}, "127.0.0.1:5000",
			"198.51.100.7, 10.1.1.1, 10.1.1.2", "198.51.100.7"},
		{"client may forge earlier hops", []string{"127.0.0.1"}, "127.0.0.1:5000",
			"1.1.1.1, 198.51.100.7", "198.51.100.7"},
		{"empty xff falls back to peer", []string{"127.0.0.1"}, "127.0.0.1:5000", "", "127.0.0.1"},
		{"junk xff falls back to peer", []string{"127.0.0.1"}, "127.0.0.1:5000", "not-an-ip", "127.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewProxyPolicy(tt.trusted)
			if err != nil {
				t.Fatalf("policy: %v", err)
			}
			if got := p.ClientIP(proxyRequest(tt.peer, "", tt.xff)); got.String() != tt.want {
				t.Fatalf("client ip = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestProxyPolicyRejectsBadSpec(t *testing.T) {
	for _, spec := range []string{"nonsense", "10.0.0.0/99", "1.2.3.4/"} {
		if _, err := NewProxyPolicy([]string{spec}); !core.IsKind(err, core.KindInvalid) {
			t.Fatalf("spec %q: expected an invalid error, got %v", spec, err)
		}
	}
	p, err := NewProxyPolicy([]string{"", "  "})
	if err != nil {
		t.Fatalf("blank entries: %v", err)
	}
	if !p.Empty() {
		t.Fatal("blank entries produced a trusted proxy")
	}
}

func TestProxyPolicyUnparseablePeer(t *testing.T) {
	p, err := NewProxyPolicy([]string{"127.0.0.1"})
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	r := proxyRequest("@", "https", "198.51.100.7")
	if got := p.ClientIP(r); got.IsValid() {
		t.Fatalf("client ip = %q, want an invalid address", got)
	}
	if got := p.Scheme(r); got != SchemeHTTP {
		t.Fatalf("scheme = %q, want %q", got, SchemeHTTP)
	}
}
