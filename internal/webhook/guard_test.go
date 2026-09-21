package webhook

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// denying is the shipped policy. The zero value is used rather than
// NewGuard(false) so the loopback relaxation a test binary gets is not in
// play: these cases assert what a released binary does.
var denying = Guard{}

func TestGuardRefusesTargetsInsideTheNetwork(t *testing.T) {
	cases := []struct {
		name string
		url  string
	}{
		{"loopback literal", "http://127.0.0.1:1/hook"},
		{"loopback elsewhere in the range", "http://127.9.9.9/hook"},
		{"loopback by name", "http://localhost:9000/hook"},
		{"loopback subdomain", "https://tix.localhost/hook"},
		{"loopback name with a trailing dot", "http://localhost./hook"},
		{"loopback v6", "http://[::1]:9000/hook"},
		{"loopback mapped into v6", "http://[::ffff:127.0.0.1]/hook"},
		{"cloud metadata endpoint", "http://169.254.169.254/latest/meta-data/"},
		{"link local v6", "http://[fe80::1]/hook"},
		{"rfc1918 ten", "https://10.0.0.5/hook"},
		{"rfc1918 172", "https://172.16.3.4/hook"},
		{"rfc1918 192.168", "https://192.168.1.1/hook"},
		{"unique local v6", "https://[fd00::1]/hook"},
		{"unspecified", "http://0.0.0.0/hook"},
		{"carrier grade nat", "https://100.64.0.1/hook"},
		{"ietf protocol assignments", "https://192.0.0.8/hook"},
		{"broadcast", "http://255.255.255.255/hook"},
		{"nat64 smuggling v4", "https://[64:ff9b::7f00:1]/hook"},
		{"multicast", "https://239.1.2.3/hook"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := denying.CheckURL(c.url)
			if err == nil {
				t.Fatalf("CheckURL(%q) allowed a target inside the network", c.url)
			}
			if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("error kind = %v, want invalid", core.KindOf(err))
			}
			if strings.Contains(err.Error(), c.url) {
				t.Errorf("the refusal quotes the target back: %v", err)
			}
		})
	}
}

func TestGuardRefusesCredentialsInTheURL(t *testing.T) {
	for _, raw := range []string{
		"https://user:secret@hooks.example.test/hook",
		"https://token@hooks.example.test/hook",
	} {
		err := denying.CheckURL(raw)
		if !core.IsKind(err, core.KindInvalid) {
			t.Errorf("CheckURL(%q) = %v, want a validation error", raw, err)
		}
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "token") {
			t.Errorf("the refusal repeats the credential: %v", err)
		}
	}
}

func TestGuardAllowsAPublicTarget(t *testing.T) {
	for _, raw := range []string{
		"https://hooks.example.test/hook",
		"https://203.0.113.10/hook",
		"https://[2001:db8::1]/hook",
	} {
		if err := denying.CheckURL(raw); err != nil {
			t.Errorf("CheckURL(%q) = %v, want it accepted", raw, err)
		}
	}
}

func TestGuardRefusesASchemeItCannotDeliverOver(t *testing.T) {
	for _, raw := range []string{"", "/hook", "ftp://example.test/h", "file:///etc/passwd", "https://"} {
		if err := denying.CheckURL(raw); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("CheckURL(%q) = %v, want a validation error", raw, err)
		}
	}
}

func TestOperatorOptInAllowsInternalTargets(t *testing.T) {
	allowing := Guard{AllowPrivate: true}
	for _, raw := range []string{
		"http://127.0.0.1:9000/hook",
		"http://localhost:9000/hook",
		"https://10.0.0.5/hook",
		"http://169.254.169.254/latest/meta-data/",
	} {
		if err := allowing.CheckURL(raw); err != nil {
			t.Errorf("CheckURL(%q) with the operator opt-in = %v, want it accepted", raw, err)
		}
	}
	// The opt-in does not make a credential in the url acceptable.
	if err := allowing.CheckURL("https://user:pw@10.0.0.5/hook"); err == nil {
		t.Error("the operator opt-in accepted a credential embedded in the url")
	}
}

// A name that answers publicly while the registration is validated and
// privately when the delivery is attempted is the rebinding case. Validation
// cannot settle it, so the decision is taken again on the address the socket
// is about to use.
func TestGuardDecidesOnTheAddressNotTheName(t *testing.T) {
	if err := denying.CheckURL("https://hooks.example.test/hook"); err != nil {
		t.Fatalf("a public name was refused during validation: %v", err)
	}
	for _, addr := range []string{"127.0.0.1:443", "169.254.169.254:80", "[::1]:443", "10.1.2.3:443"} {
		if err := denying.CheckAddr(addr); !errors.Is(err, ErrBlockedTarget) {
			t.Errorf("CheckAddr(%q) = %v, want the target to be blocked", addr, err)
		}
	}
	if err := denying.CheckAddr("203.0.113.10:443"); err != nil {
		t.Errorf("CheckAddr on a public address = %v, want it allowed", err)
	}
	if err := denying.CheckAddr("not-an-address"); !errors.Is(err, ErrBlockedTarget) {
		t.Errorf("CheckAddr on an unparsable address = %v, want it blocked", err)
	}
}

// The dial-time control is what a live request is actually held to, so it is
// exercised through a real client rather than only through CheckAddr.
func TestGuardedClientRefusesToConnectToALoopbackServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := denying.Client(DefaultTimeout).Get(srv.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("the guarded client reached a loopback server")
	}
	if !errors.Is(err, ErrBlockedTarget) {
		t.Fatalf("connecting = %v, want it blocked by the guard", err)
	}
}

func TestGuardedClientDoesNotFollowRedirects(t *testing.T) {
	var reached bool
	inner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	defer inner.Close()
	outer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, inner.URL, http.StatusFound)
	}))
	defer outer.Close()

	// Loopback is allowed here so the redirect, not the address, is what the
	// request is stopped by.
	resp, err := Guard{AllowPrivate: true}.Client(DefaultTimeout).Get(outer.URL)
	if err != nil {
		t.Fatalf("requesting the redirecting server: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want the redirect itself to be returned", resp.StatusCode)
	}
	if reached {
		t.Error("the redirect was followed to its target")
	}
}

func TestTransportErrorNamesNoAddress(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"blocked", ErrBlockedTarget, "endpoint address is not permitted"},
		{"blocked and wrapped", &url.Error{Op: "Post", URL: "http://10.0.0.5/h", Err: ErrBlockedTarget},
			"endpoint address is not permitted"},
		{"dns", &url.Error{Op: "Post", URL: "http://x/h", Err: &net.DNSError{Err: "no such host", Name: "x"}},
			"endpoint host could not be resolved"},
		{"other", &url.Error{Op: "Post", URL: "http://10.0.0.5:22/h",
			Err: errors.New("dial tcp 10.0.0.5:22: connect: connection refused")},
			"endpoint could not be reached"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := transportError(c.err)
			if got != c.want {
				t.Fatalf("transportError = %q, want %q", got, c.want)
			}
			for _, leak := range []string{"10.0.0.5", ":22", "connection refused", "no such host"} {
				if strings.Contains(got, leak) {
					t.Errorf("transportError leaks %q: %q", leak, got)
				}
			}
		})
	}
}
