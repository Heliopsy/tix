// SPDX-License-Identifier: AGPL-3.0-or-later

package webhook

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
)

// ErrBlockedTarget is returned when a delivery would reach an address the
// guard refuses. Its text names no address, so a tenant reading a delivery
// failure learns nothing about the operator's network.
var ErrBlockedTarget = egress.ErrNotAllowed

// resolveBudget bounds the advisory lookup a registration does, so a slow or
// blackholed resolver cannot stall an administrative request.
const resolveBudget = 750 * time.Millisecond

// Guard decides which network destinations a delivery may reach. The zero
// value refuses every internal destination, which is the shipped policy.
type Guard struct {
	// Policy is the operator's egress setting. A tenant can never set it,
	// because nothing on a tenant-facing path writes it.
	Policy egress.Policy
}

// NewGuard builds the guard for an operator who allowed internal targets or
// did not.
func NewGuard(allowPrivate bool) Guard {
	return Guard{Policy: egress.Policy{AllowPrivate: allowPrivate}}
}

// GuardFor builds the guard the operator's egress policy describes.
func GuardFor(p egress.Policy) Guard { return Guard{Policy: p} }

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
	p := g.Policy
	if err := p.CheckURL(raw); err != nil {
		return g.refuse()
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return nil
	}
	return g.checkResolved(p, host)
}

// checkResolved refuses a hostname that already resolves into a blocked range.
// A lookup that fails is not a rejection: the dial-time control still applies.
func (g Guard) checkResolved(p egress.Policy, host string) error {
	if p.AllowPrivate || p.AllowsHost(host) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), resolveBudget)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil
	}
	for _, ip := range addrs {
		if !p.AllowsAddr(ip) {
			return g.refuse()
		}
	}
	return nil
}

// refuse states the policy without naming the address that tripped it.
func (g Guard) refuse() error {
	return core.Invalid("webhook url address not allowed: it must not point at a loopback, link-local, " +
		"unique-local or otherwise private address; ask the operator to allow " +
		"internal delivery targets if that is intended")
}

// CheckAddr applies the policy to a host:port the dialer is about to connect
// to. The host is always a literal address at this point, so this is the
// decision that cannot be rebound underneath.
func (g Guard) CheckAddr(address string) error {
	return g.Policy.CheckAddr(address)
}

// Transport returns a transport whose every connection passes the guard.
// Proxying is off: a proxied request dials the proxy, so the guard would
// inspect the proxy's address rather than the target's.
func (g Guard) Transport() *http.Transport {
	return g.Policy.Transport()
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
