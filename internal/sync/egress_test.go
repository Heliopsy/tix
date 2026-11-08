// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
)

// loopbackAllowed is a source config under the operator's escape hatch opened
// for loopback only, so a test can reach an httptest server.
var loopbackAllowed = SourceConfig{Egress: egress.Policy{Allow: []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}}}

// jsonCounter is a loopback source that counts the requests reaching it.
func jsonCounter(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// fetch builds a client for base under cfg and issues one request.
func fetch(base string, cfg SourceConfig) error {
	c, err := NewHTTPClient(HTTPOptions{System: "test", BaseURL: base, Config: cfg,
		Sleep: func(time.Duration) {}, MaxRetries: 1, BaseDelay: time.Millisecond})
	if err != nil {
		return err
	}
	var out map[string]any
	return c.GetJSON(context.Background(), "/issues", nil, &out)
}

func TestHTTPClientReachesALoopbackSourceOnlyWithTheEscapeHatch(t *testing.T) {
	srv, hits := jsonCounter(t)
	byName := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)
	cases := []struct {
		name string
		base string
		cfg  SourceConfig
		ok   bool
	}{
		{"strict by address", srv.URL, SourceConfig{}, false},
		{"strict by name, refused at dial", byName, SourceConfig{Egress: egress.Policy{}}, false},
		{"allowed range", srv.URL, loopbackAllowed, true},
		{"allow private", srv.URL, SourceConfig{Egress: egress.Permissive()}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := hits.Load()
			err := fetch(c.base, c.cfg)
			reached := hits.Load() > before
			if c.ok {
				if err != nil || !reached {
					t.Fatalf("fetch = %v, reached = %v; want the source read", err, reached)
				}
				return
			}
			if err == nil || reached {
				t.Fatalf("fetch = %v, reached = %v; want it refused", err, reached)
			}
			if !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "address not allowed") {
				t.Fatalf("err = %v, want the address refusal", err)
			}
			if hits.Load() != before {
				t.Fatal("a refused source was retried into the network")
			}
		})
	}
}

// The name a refusal is about is the tenant's own; what it resolved to is the
// operator's network and stays out of the message.
func TestHTTPClientRefusalNamesNoResolvedAddress(t *testing.T) {
	srv, _ := jsonCounter(t)
	err := fetch(strings.Replace(srv.URL, "127.0.0.1", "localhost", 1), SourceConfig{})
	if err == nil {
		t.Fatal("a name resolving to loopback was reached")
	}
	for _, leak := range []string{"127.0.0.1", "::1", strings.TrimPrefix(srv.URL, "http://127.0.0.1")} {
		if strings.Contains(err.Error(), leak) {
			t.Fatalf("the refusal leaks %q: %v", leak, err)
		}
	}
}

// A source the operator allowed by name redirects to a loopback address it did
// not allow; the hop is refused and the inner address never reached.
func TestHTTPClientRefusesARedirectIntoTheNetwork(t *testing.T) {
	inner, hits := jsonCounter(t)
	outer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, inner.URL+"/latest/meta-data/", http.StatusFound)
	}))
	t.Cleanup(outer.Close)
	cfg := SourceConfig{Egress: egress.Policy{AllowHosts: []string{"localhost"}}}
	err := fetch(strings.Replace(outer.URL, "127.0.0.1", "localhost", 1), cfg)
	if err == nil {
		t.Fatal("a redirect into the network was followed")
	}
	if hits.Load() != 0 {
		t.Fatal("the redirect target was reached")
	}
}

// The egress check on a redirect runs before the origin rule, so it holds even
// for a hop the origin rule would accept.
func TestRedirectPolicyChecksTheAddressFirst(t *testing.T) {
	check := egress.Policy{}.CheckRedirect(sameOriginRedirect)
	first, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/issues", nil)
	next, _ := http.NewRequest(http.MethodGet, "http://127.0.0.1:8080/moved", nil)
	if err := check(next, []*http.Request{first}); err == nil {
		t.Fatal("a same-origin hop to a loopback address was allowed under a strict policy")
	}
}

// The url check settles a literal address or localhost before any request; a
// name that resolves to loopback only at run time is the dialer's to refuse.
// This dials the client's own transport, as such a request would.
func TestHTTPClientTransportRefusesALoopbackAddressAtDialTime(t *testing.T) {
	srv, hits := jsonCounter(t)
	addr := strings.TrimPrefix(srv.URL, "http://")
	for name, tc := range map[string]struct {
		cfg SourceConfig
		ok  bool
	}{
		"strict":       {SourceConfig{}, false},
		"escape hatch": {loopbackAllowed, true},
	} {
		t.Run(name, func(t *testing.T) {
			c, err := NewHTTPClient(HTTPOptions{System: "test", BaseURL: "https://tracker.example.com", Config: tc.cfg})
			if err != nil {
				t.Fatalf("NewHTTPClient: %v", err)
			}
			transport := c.client.(*http.Client).Transport.(*http.Transport)
			conn, err := transport.DialContext(context.Background(), "tcp", addr)
			if conn != nil {
				_ = conn.Close()
			}
			if tc.ok != (err == nil) {
				t.Fatalf("dial = %v, want ok=%v", err, tc.ok)
			}
			if err != nil && (!errors.Is(err, egress.ErrNotAllowed) || strings.Contains(err.Error(), "127.0.0.1")) {
				t.Fatalf("dial = %v, want the bare refusal", err)
			}
		})
	}
	if hits.Load() != 0 {
		t.Fatal("a dial test sent a request")
	}
}
