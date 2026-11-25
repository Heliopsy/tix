// SPDX-License-Identifier: AGPL-3.0-or-later

package egress

import (
	"context"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// lookupNetIP resolves a host name; a test replaces it.
var lookupNetIP = net.DefaultResolver.LookupNetIP

// VetURL checks an address a tenant chose before anything is requested from
// it, and answers the URL to request, rebuilt from the parts it checked
// rather than the text it was given. It requires https, or http to a
// loopback host the policy permits; refuses user information and fragments;
// and resolves the host, refusing it when any address it resolves to is one
// the policy does not permit, unless the host is on the allow list. The
// dialer still checks the address it connects to, which is what holds
// against a name that resolves differently a moment later.
func (p Policy) VetURL(ctx context.Context, raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.User != nil || u.Fragment != "" || u.Opaque != "" {
		return nil, ErrNotAllowed
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return nil, ErrNotAllowed
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !isLoopbackName(host) {
			return nil, ErrNotAllowed
		}
	default:
		return nil, ErrNotAllowed
	}
	if err := p.vetHost(ctx, host); err != nil {
		return nil, err
	}
	out := &url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path, RawPath: u.RawPath, RawQuery: u.RawQuery}
	return out, nil
}

// vetHost refuses a host that names, or resolves to, an address the policy
// does not permit.
func (p Policy) vetHost(ctx context.Context, host string) error {
	if p.AllowPrivate || p.AllowsHost(host) {
		return nil
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		host = "127.0.0.1"
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if !p.AllowsAddr(ip) {
			return ErrNotAllowed
		}
		return nil
	}
	addrs, err := lookupNetIP(ctx, "ip", host)
	if err != nil || len(addrs) == 0 {
		return ErrNotAllowed
	}
	for _, ip := range addrs {
		if !p.AllowsAddr(ip) {
			return ErrNotAllowed
		}
	}
	return nil
}

// isLoopbackName reports whether a host names this machine.
func isLoopbackName(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.Unmap().IsLoopback()
}
