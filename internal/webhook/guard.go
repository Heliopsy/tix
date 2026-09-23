// SPDX-License-Identifier: AGPL-3.0-or-later

package webhook

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// ErrBlockedTarget is returned when a delivery would reach an address the
// guard refuses. Its text names no address, so a tenant reading a delivery
// failure learns nothing about the operator's network.
var ErrBlockedTarget = errors.New("target address is not permitted")

// resolveBudget bounds the advisory lookup a registration does, so a slow or
// blackholed resolver cannot stall an administrative request.
const resolveBudget = 750 * time.Millisecond

// Guard decides which network destinations a delivery may reach. The zero
// value refuses every private destination, which is the shipped policy.
type Guard struct {
	// AllowPrivate permits loopback, link-local, unique-local and RFC1918
	// destinations. It is operator configuration. A tenant can never set it,
	// because nothing on a tenant-facing path writes it.
	AllowPrivate bool

	// allowLoopback additionally permits loopback and nothing else. It is set
	// only in a binary built by `go test`, so the suite can deliver to an
	// httptest server, and is false in a shipped binary.
	allowLoopback bool
}

// NewGuard builds the guard an operator configured.
func NewGuard(allowPrivate bool) Guard {
	return Guard{AllowPrivate: allowPrivate, allowLoopback: testing.Testing()}
}

// permits reports whether one resolved address may be reached.
func (g Guard) permits(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() {
		return false
	}
	if g.AllowPrivate {
		return true
	}
	if ip.IsLoopback() {
		return g.allowLoopback
	}
	return !isBlocked(ip)
}

// isBlocked reports whether an address belongs to a range a tenant-supplied
// target must never reach. Link-local covers the cloud metadata endpoint at
// 169.254.169.254 and its IPv6 form.
func isBlocked(ip netip.Addr) bool {
	switch {
	case ip.IsLoopback(), ip.IsUnspecified(), ip.IsPrivate(),
		ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast(),
		ip.IsInterfaceLocalMulticast(), ip.IsMulticast():
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// blockedPrefixes are the ranges net/netip has no predicate for: the shared
// carrier-grade range, IETF protocol assignments, the benchmarking range, the
// IPv4 broadcast address, and the NAT64 well-known prefix that would otherwise
// smuggle an IPv4 destination past an IPv6 check.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("255.255.255.255/32"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
}

// CheckURL applies the policy everything about a target that can be known
// without connecting to it. It refuses a scheme the dispatcher cannot sign
// for, a credential embedded in the url, a literal address inside a blocked
// range, and a hostname that resolves into one.
//
// Resolution here is advisory only: it answers the common case with an
// immediate, honest error. It is not the control, because a name that resolves
// publicly now may resolve to 127.0.0.1 when the delivery is attempted. The
// control is Dialer, which inspects the address actually being connected to.
func (g Guard) CheckURL(raw string) error {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return core.Invalid("webhook url %q must be an absolute http or https url", raw)
	}
	if u.User != nil {
		return core.Invalid("webhook url must not embed a username or password; " +
			"send a credential in a header your receiver checks instead")
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return core.Invalid("webhook url %q must name a host", raw)
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		if g.AllowPrivate || g.allowLoopback {
			return nil
		}
		return g.refuse()
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if !g.permits(ip) {
			return g.refuse()
		}
		return nil
	}
	return g.checkResolved(host)
}

// checkResolved refuses a hostname that already resolves into a blocked range.
// A lookup that fails is not a rejection: the dial-time control still applies.
func (g Guard) checkResolved(host string) error {
	if g.AllowPrivate {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), resolveBudget)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil
	}
	for _, ip := range addrs {
		if !g.permits(ip) {
			return g.refuse()
		}
	}
	return nil
}

// refuse states the policy without naming the address that tripped it.
func (g Guard) refuse() error {
	return core.Invalid("webhook url must not point at a loopback, link-local, " +
		"unique-local or otherwise private address; ask the operator to allow " +
		"internal delivery targets if that is intended")
}

// CheckAddr applies the policy to a host:port the dialer is about to connect
// to. The host is always a literal address at this point, so this is the
// decision that cannot be rebound underneath.
func (g Guard) CheckAddr(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return ErrBlockedTarget
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !g.permits(ip) {
		return ErrBlockedTarget
	}
	return nil
}

// control is the dial-time hook. It runs after resolution and immediately
// before connect, which is what closes the DNS-rebinding window: a name that
// answered publicly during validation and privately at delivery is caught
// here, on the address the socket is actually about to use.
func (g Guard) control(_, address string, _ syscall.RawConn) error {
	return g.CheckAddr(address)
}

// Dialer returns a dialer that refuses a connection to a blocked address.
func (g Guard) Dialer() *net.Dialer {
	return &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second, Control: g.control}
}

// Transport returns a transport whose every connection passes the guard.
// Proxying is deliberately off: a proxied request dials the proxy, so the
// guard would inspect the proxy's address rather than the target's.
func (g Guard) Transport() *http.Transport {
	return &http.Transport{
		Proxy:                 nil,
		DialContext:           g.Dialer().DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

// Client returns the delivery client: guarded at dial time and following no
// redirect at all.
func (g Guard) Client(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		Transport:     g.Transport(),
		CheckRedirect: refuseRedirect,
	}
}

// refuseRedirect stops at the redirect rather than following it. A webhook
// receiver redirecting is unusual, and following one would carry the signature
// headers to a host that never passed validation, which is exactly how an
// endpoint that looks public reaches the metadata service. Returning
// ErrUseLastResponse rather than an error keeps the 3xx visible, so the
// operator sees "endpoint responded 302" instead of a transport failure.
func refuseRedirect(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
