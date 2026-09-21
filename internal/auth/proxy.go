package auth

import (
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// Forwarded headers a trusted proxy may speak for its client with.
const (
	HeaderForwardedProto = "X-Forwarded-Proto"
	HeaderForwardedFor   = "X-Forwarded-For"
)

// Schemes an effective request scheme may take.
const (
	SchemeHTTP  = "http"
	SchemeHTTPS = "https"
)

// ProxyPolicy decides whether the immediate peer may speak for its client.
//
// Any client can send X-Forwarded-Proto and X-Forwarded-For, so they are read
// only when the peer that opened the connection is a configured proxy. A
// policy with no trusted proxies honours neither header, which is the default.
type ProxyPolicy struct {
	trusted []netip.Prefix
}

// NewProxyPolicy compiles trusted proxy addresses, each an IP or a CIDR block.
func NewProxyPolicy(specs []string) (*ProxyPolicy, error) {
	p := &ProxyPolicy{}
	for _, raw := range specs {
		spec := strings.TrimSpace(raw)
		if spec == "" {
			continue
		}
		prefix, err := parsePrefix(spec)
		if err != nil {
			return nil, err
		}
		p.trusted = append(p.trusted, prefix)
	}
	return p, nil
}

// parsePrefix accepts either a bare address or a CIDR block.
func parsePrefix(spec string) (netip.Prefix, error) {
	if strings.Contains(spec, "/") {
		prefix, err := netip.ParsePrefix(spec)
		if err != nil {
			return netip.Prefix{}, core.Invalid("trusted proxy %q is not an ip or cidr block", spec)
		}
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(spec)
	if err != nil {
		return netip.Prefix{}, core.Invalid("trusted proxy %q is not an ip or cidr block", spec)
	}
	return netip.PrefixFrom(addr.Unmap(), addr.Unmap().BitLen()), nil
}

// Empty reports whether the policy trusts nothing.
func (p *ProxyPolicy) Empty() bool { return p == nil || len(p.trusted) == 0 }

// Trusts reports whether addr is one of the configured proxies.
func (p *ProxyPolicy) Trusts(addr netip.Addr) bool {
	if p == nil || !addr.IsValid() {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range p.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// PeerIP returns the address of the peer that opened the connection.
func PeerIP(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(strings.Trim(strings.TrimSpace(host), "[]"))
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}

// ClientIP returns the address the request is attributed to: the peer itself,
// or, when the peer is a trusted proxy, the rightmost X-Forwarded-For entry
// that is not itself a trusted proxy. An invalid address is returned when the
// peer address cannot be parsed, as it cannot on a unix socket.
//
// Rate limiting and abuse accounting need this one answer, so it lives here
// rather than inline at a single call site.
func (p *ProxyPolicy) ClientIP(r *http.Request) netip.Addr {
	peer := PeerIP(r)
	if !p.Trusts(peer) {
		return peer
	}
	hops := forwardedFor(r)
	for i := len(hops) - 1; i >= 0; i-- {
		addr, err := netip.ParseAddr(hops[i])
		if err != nil {
			return peer
		}
		addr = addr.Unmap()
		if !p.Trusts(addr) {
			return addr
		}
	}
	return peer
}

// Scheme returns the scheme the client used: the connection's own, or the one
// a trusted proxy reports in X-Forwarded-Proto.
func (p *ProxyPolicy) Scheme(r *http.Request) string {
	direct := SchemeHTTP
	if r.TLS != nil {
		direct = SchemeHTTPS
	}
	if !p.Trusts(PeerIP(r)) {
		return direct
	}
	switch proto := firstProto(r.Header.Get(HeaderForwardedProto)); proto {
	case SchemeHTTP, SchemeHTTPS:
		return proto
	default:
		return direct
	}
}

// forwardedFor splits the hop list every proxy appends to.
func forwardedFor(r *http.Request) []string {
	var out []string
	for _, header := range r.Header.Values(HeaderForwardedFor) {
		for _, hop := range strings.Split(header, ",") {
			if hop = strings.Trim(strings.TrimSpace(hop), "[]"); hop != "" {
				out = append(out, hop)
			}
		}
	}
	return out
}

// firstProto reads the scheme a proxy chain reports, which is the first entry.
func firstProto(header string) string {
	proto, _, _ := strings.Cut(header, ",")
	return strings.ToLower(strings.TrimSpace(proto))
}
