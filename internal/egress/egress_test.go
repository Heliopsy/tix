// SPDX-License-Identifier: AGPL-3.0-or-later

package egress

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestBlockedClassifiesAddresses(t *testing.T) {
	cases := []struct {
		addr    string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"127.255.255.254", true},
		{"0.0.0.0", true},
		{"0.1.2.3", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true},
		{"100.64.0.1", true},
		{"100.127.255.255", true},
		{"192.0.0.8", true},
		{"192.88.99.1", true},
		{"198.18.0.1", true},
		{"224.0.0.1", true},
		{"239.255.255.250", true},
		{"240.0.0.1", true},
		{"255.255.255.255", true},
		{"::", true},
		{"::1", true},
		{"fe80::1", true},
		{"fe80::1%eth0", true},
		{"fec0::1", true},
		{"fc00::1", true},
		{"fd12:3456::1", true},
		{"ff02::1", true},
		{"ff05::2", true},
		{"::ffff:127.0.0.1", true},
		{"::ffff:169.254.169.254", true},
		{"::ffff:10.1.2.3", true},
		{"::ffff:100.64.0.1", true},
		{"::ffff:198.18.0.1", true},
		{"::127.0.0.1", true},
		{"::ffff:0:a00:1", true},
		{"64:ff9b::a9fe:a9fe", true},
		{"64:ff9b:1::1", true},
		{"2002:7f00:1::1", true},
		{"2002:a9fe:a9fe::1", true},
		{"2001:0:4136:e378:8000:63bf:3fff:fdd2", true},
		{"100::1", true},
		{"8.8.8.8", false},
		{"203.0.113.9", false},
		{"2001:db8::1", false},
		{"1.1.1.1", false},
		{"172.32.0.1", false},
		{"100.128.0.1", false},
		{"::ffff:8.8.8.8", false},
		{"2606:4700:4700::1111", false},
		{"2a00:1450:4001::200e", false},
	}
	for _, c := range cases {
		t.Run(c.addr, func(t *testing.T) {
			ip := netip.MustParseAddr(c.addr)
			if got := Blocked(ip); got != c.blocked {
				t.Fatalf("Blocked(%s) = %v, want %v", c.addr, got, c.blocked)
			}
			if got := (Policy{}).AllowsAddr(ip); got == c.blocked {
				t.Fatalf("strict AllowsAddr(%s) = %v, want %v", c.addr, got, !c.blocked)
			}
		})
	}
}

func TestBlockedRefusesTheZeroAddr(t *testing.T) {
	if !Blocked(netip.Addr{}) {
		t.Fatal("an invalid address is not blocked")
	}
	if Permissive().AllowsAddr(netip.Addr{}) {
		t.Fatal("a permissive policy allows an invalid address")
	}
}

func TestAllowListOpensOnlyWhatItNames(t *testing.T) {
	p, err := New(false, []string{"10.20.0.0/16", "192.168.5.5", "::ffff:172.16.1.1", "jira.internal", ".corp.example"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for addr, want := range map[string]bool{
		"10.20.3.4":            true,
		"::ffff:10.20.3.4":     true,
		"10.21.0.1":            false,
		"192.168.5.5":          true,
		"192.168.5.6":          false,
		"172.16.1.1":           true,
		"127.0.0.1":            false,
		"169.254.169.254":      false,
		"8.8.8.8":              true,
		"fd00::1":              false,
		"2606:4700:4700::1111": true,
	} {
		if got := p.AllowsAddr(netip.MustParseAddr(addr)); got != want {
			t.Errorf("AllowsAddr(%s) = %v, want %v", addr, got, want)
		}
	}
	for host, want := range map[string]bool{
		"jira.internal":        true,
		"JIRA.internal.":       true,
		"x.jira.internal":      false,
		"tracker.corp.example": true,
		"corp.example":         false,
		"evilcorp.example":     false,
		"":                     false,
	} {
		if got := p.AllowsHost(host); got != want {
			t.Errorf("AllowsHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestNewRefusesAnEntryItCannotRead(t *testing.T) {
	for _, entry := range []string{"10.0.0.0/33", "http://jira", "a..b", ".", "user@host", "host:80"} {
		if _, err := New(false, []string{entry}); err == nil {
			t.Errorf("New accepted %q", entry)
		}
	}
	if _, err := New(false, []string{"", "  "}); err != nil {
		t.Errorf("New refused blank entries: %v", err)
	}
}

func TestCheckURL(t *testing.T) {
	allow, _ := New(false, []string{"localhost", "10.0.0.0/8"})
	cases := []struct {
		name string
		p    Policy
		url  string
		ok   bool
	}{
		{"public https", Policy{}, "https://tracker.example.com/api", true},
		{"public http", Policy{}, "http://tracker.example.com/api", true},
		{"ftp", Policy{}, "ftp://tracker.example.com/", false},
		{"file", Policy{}, "file:///etc/passwd", false},
		{"gopher", Permissive(), "gopher://tracker.example.com/", false},
		{"no host", Policy{}, "http:///path", false},
		{"relative", Policy{}, "/path", false},
		{"loopback literal", Policy{}, "http://127.0.0.1:8080/", false},
		{"metadata", Policy{}, "http://169.254.169.254/latest/", false},
		{"mapped v6", Policy{}, "http://[::ffff:7f00:1]/", false},
		{"v6 loopback", Policy{}, "http://[::1]/", false},
		{"localhost", Policy{}, "http://localhost/", false},
		{"localhost subdomain", Policy{}, "http://a.localhost./", false},
		{"permissive loopback", Permissive(), "http://127.0.0.1/", true},
		{"allowed host", allow, "http://localhost:9000/", true},
		{"allowed range", allow, "https://10.1.2.3/", true},
		{"localhost under an allowed loopback range", Policy{Allow: []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}}, "http://localhost:9000/", true},
		{"outside allowed range", allow, "https://192.168.0.1/", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.p.CheckURL(c.url)
			if (err == nil) != c.ok {
				t.Fatalf("CheckURL(%q) = %v, want ok=%v", c.url, err, c.ok)
			}
			if err != nil && !errors.Is(err, ErrNotAllowed) {
				t.Fatalf("CheckURL(%q) = %v, want ErrNotAllowed", c.url, err)
			}
		})
	}
}

// counting is a loopback server that records whether anything reached it.
func counting(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestClientRefusesALoopbackServerUnlessAllowed(t *testing.T) {
	srv, hits := counting(t)
	port := strings.TrimPrefix(srv.URL, "http://127.0.0.1:")
	byName := "http://localhost:" + port + "/"
	cidr, _ := New(false, []string{"127.0.0.0/8"})
	host, _ := New(false, []string{"localhost"})
	cases := []struct {
		name   string
		p      Policy
		target string
		ok     bool
	}{
		{"strict by address", Policy{}, srv.URL, false},
		{"strict by name", Policy{}, byName, false},
		{"allow private", Permissive(), srv.URL, true},
		{"allow range", cidr, srv.URL, true},
		{"allow host by name", host, byName, true},
		{"allow host does not open the address", host, srv.URL, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := hits.Load()
			resp, err := c.p.Client(5*time.Second, nil).Get(c.target)
			if resp != nil {
				_ = resp.Body.Close()
			}
			reached := hits.Load() > before
			if c.ok {
				if err != nil || !reached {
					t.Fatalf("Get = %v, reached = %v; want it delivered", err, reached)
				}
				return
			}
			if err == nil || reached {
				t.Fatalf("Get = %v, reached = %v; want it refused", err, reached)
			}
			if !errors.Is(err, ErrNotAllowed) {
				t.Fatalf("err = %v, want ErrNotAllowed", err)
			}
			if strings.Contains(err.Error(), "127.0.0.1") && !strings.Contains(c.target, "127.0.0.1") {
				t.Fatalf("the refusal names the resolved address: %v", err)
			}
		})
	}
}

func TestClientRefusesARedirectToAPrivateAddress(t *testing.T) {
	inner, hits := counting(t)
	outer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, inner.URL+"/meta", http.StatusFound)
	}))
	t.Cleanup(outer.Close)
	p, _ := New(false, []string{"localhost"})
	target := "http://localhost:" + strings.TrimPrefix(outer.URL, "http://127.0.0.1:") + "/"
	resp, err := p.Client(5*time.Second, nil).Get(target)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil || !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("Get = %v, want the redirect refused", err)
	}
	if hits.Load() != 0 {
		t.Fatal("the redirect target was reached")
	}
}

func TestDialRefusesARedirectTheURLCheckCannotSee(t *testing.T) {
	inner, hits := counting(t)
	innerByName := "http://localhost:" + strings.TrimPrefix(inner.URL, "http://127.0.0.1:") + "/"
	outer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, innerByName, http.StatusFound)
	}))
	t.Cleanup(outer.Close)
	p := Policy{AllowHosts: []string{"127.0.0.1"}}
	pass := func(*http.Request, []*http.Request) error { return nil }
	c := &http.Client{Timeout: 5 * time.Second, Transport: p.Transport(), CheckRedirect: pass}
	resp, err := c.Get(outer.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if err == nil || !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("Get = %v, want the dial refused", err)
	}
	if hits.Load() != 0 {
		t.Fatal("the redirect target was reached")
	}
}

func TestCheckRedirectAppliesTheCallersRuleAfterThePolicy(t *testing.T) {
	stop := errors.New("caller says no")
	called := false
	check := Permissive().CheckRedirect(func(*http.Request, []*http.Request) error { called = true; return stop })
	req := &http.Request{URL: &url.URL{Scheme: "https", Host: "tracker.example.com"}}
	if err := check(req, nil); !errors.Is(err, stop) || !called {
		t.Fatalf("check = %v, called = %v; want the caller's rule", err, called)
	}
	called = false
	req.URL.Scheme = "ftp"
	if err := check(req, nil); !errors.Is(err, ErrNotAllowed) || called {
		t.Fatalf("check = %v, called = %v; want the policy to refuse first", err, called)
	}
	def := Policy{}.CheckRedirect(nil)
	via := make([]*http.Request, 10)
	req.URL.Scheme = "https"
	if err := def(req, via); err == nil {
		t.Fatal("the default rule followed an eleventh redirect")
	}
	if err := def(req, via[:1]); err != nil {
		t.Fatalf("the default rule refused a public hop: %v", err)
	}
}

func TestCheckAddrRefusesWhatIsNotAnAddress(t *testing.T) {
	for _, addr := range []string{"no-port", "localhost:80", "127.0.0.1:80", "[::1]:443"} {
		if err := (Policy{}).CheckAddr(addr); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("CheckAddr(%q) = %v, want ErrNotAllowed", addr, err)
		}
	}
	if err := (Policy{}).CheckAddr("8.8.8.8:443"); err != nil {
		t.Errorf("CheckAddr refused a public address: %v", err)
	}
}

func TestStrict(t *testing.T) {
	if !(Policy{}).Strict() || Permissive().Strict() {
		t.Fatal("Strict misreports the policy")
	}
}
