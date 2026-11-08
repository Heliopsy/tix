// SPDX-License-Identifier: AGPL-3.0-or-later

// Package egress decides which network destinations the server may reach on a
// tenant's behalf: webhook and inbox channel deliveries, and external sync
// sources. The decision is made on the address a socket is about to connect
// to, so a name that resolves differently later, or a redirect, cannot get
// past it.
package egress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// ErrNotAllowed is the refusal for a destination the policy does not permit.
// Its text names neither the host nor the address it resolved to.
var ErrNotAllowed = errors.New("address not allowed")

// Policy is the operator's egress setting. The zero value is strict: it
// refuses every destination Blocked reports and permits the rest.
type Policy struct {
	// AllowPrivate permits every destination, internal ones included.
	AllowPrivate bool
	// Allow lists address ranges permitted even though Blocked reports them.
	Allow []netip.Prefix
	// AllowHosts lists host names permitted whatever they resolve to. An
	// entry starting with a dot matches every name under it.
	AllowHosts []string
}

// Permissive is the policy of a process the operator runs on their own
// behalf, which may reach whatever the operator's network lets it.
func Permissive() Policy { return Policy{AllowPrivate: true} }

// New builds a policy from the operator's configuration. Each allow entry is
// an address, a CIDR range, a host name or a .suffix.
func New(allowPrivate bool, allow []string) (Policy, error) {
	p := Policy{AllowPrivate: allowPrivate}
	for _, raw := range allow {
		entry := strings.ToLower(strings.TrimSpace(raw))
		if entry == "" {
			continue
		}
		if pfx, err := netip.ParsePrefix(entry); err == nil {
			p.Allow = append(p.Allow, pfx.Masked())
			continue
		}
		if ip, err := netip.ParseAddr(entry); err == nil {
			p.Allow = append(p.Allow, netip.PrefixFrom(ip.Unmap(), ip.Unmap().BitLen()))
			continue
		}
		if !validHostEntry(entry) {
			return Policy{}, fmt.Errorf("%q is not an address, a cidr range or a host name", raw)
		}
		p.AllowHosts = append(p.AllowHosts, strings.TrimSuffix(entry, "."))
	}
	return p, nil
}

// validHostEntry reports whether an allow entry reads as a host name.
func validHostEntry(entry string) bool {
	name := strings.TrimPrefix(entry, ".")
	if name == "" || strings.ContainsAny(name, "/:@ ") {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(name, "."), ".") {
		if label == "" {
			return false
		}
	}
	return true
}

// Strict reports whether the policy refuses anything at all.
func (p Policy) Strict() bool { return !p.AllowPrivate }

// AllowsAddr reports whether one resolved address may be reached.
func (p Policy) AllowsAddr(ip netip.Addr) bool {
	if !ip.IsValid() {
		return false
	}
	if p.AllowPrivate {
		return true
	}
	ip = ip.Unmap()
	for _, pfx := range p.Allow {
		if pfx.Contains(ip) {
			return true
		}
	}
	return !Blocked(ip)
}

// AllowsHost reports whether a host name is on the operator's allow list.
func (p Policy) AllowsHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" {
		return false
	}
	for _, entry := range p.AllowHosts {
		if host == entry || (strings.HasPrefix(entry, ".") && strings.HasSuffix(host, entry)) {
			return true
		}
	}
	return false
}

// CheckURL refuses what can be refused before connecting: a scheme other than
// http or https, a missing host, and a literal address the policy does not
// permit, a localhost name counting as 127.0.0.1. It does not resolve names;
// the dialer decides those.
func (p Policy) CheckURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ErrNotAllowed
	}
	return p.checkURL(u)
}

func (p Policy) checkURL(u *url.URL) error {
	if u == nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ErrNotAllowed
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return ErrNotAllowed
	}
	if p.AllowPrivate || p.AllowsHost(host) {
		return nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		host = "127.0.0.1"
	}
	if ip, err := netip.ParseAddr(host); err == nil && !p.AllowsAddr(ip) {
		return ErrNotAllowed
	}
	return nil
}

// CheckAddr applies the policy to the literal host:port a socket is about to
// connect to.
func (p Policy) CheckAddr(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return ErrNotAllowed
	}
	ip, err := netip.ParseAddr(host)
	if err != nil || !p.AllowsAddr(ip) {
		return ErrNotAllowed
	}
	return nil
}

// control is the dialer hook. It runs after resolution, immediately before
// connect, on the address the socket will actually use, which is what closes
// the DNS rebinding window.
func (p Policy) control(_, address string, _ syscall.RawConn) error {
	return p.CheckAddr(address)
}

// Dialer returns a dialer that refuses to connect to an address the policy
// does not permit.
func (p Policy) Dialer() *net.Dialer {
	return &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second, Control: p.control}
}

// DialContext dials a host:port, held to the policy unless the host is on the
// allow list by name.
func (p Policy) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	d := p.Dialer()
	if host, _, err := net.SplitHostPort(address); err == nil && p.AllowsHost(host) {
		d.Control = nil
	}
	conn, err := d.DialContext(ctx, network, address)
	if errors.Is(err, ErrNotAllowed) {
		return nil, ErrNotAllowed
	}
	return conn, err
}

// Transport returns a transport whose every connection passes the policy.
// Proxying is off: a proxied request dials the proxy, so the policy would see
// the proxy's address rather than the destination's.
func (p Policy) Transport() *http.Transport {
	return &http.Transport{
		Proxy:                 nil,
		DialContext:           p.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          16,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

// CheckRedirect returns a redirect policy that refuses a hop the policy does
// not permit before then applying next, which may be nil.
func (p Policy) CheckRedirect(next func(*http.Request, []*http.Request) error) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if err := p.checkURL(req.URL); err != nil {
			return err
		}
		if next != nil {
			return next(req, via)
		}
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		return nil
	}
}

// Client returns a client held to the policy on every connection and every
// redirect hop. next is the caller's own redirect rule, nil for the default.
func (p Policy) Client(timeout time.Duration, next func(*http.Request, []*http.Request) error) *http.Client {
	return &http.Client{Timeout: timeout, Transport: p.Transport(), CheckRedirect: p.CheckRedirect(next)}
}
