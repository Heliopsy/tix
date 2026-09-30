// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/proxyproto"
)

// proxyListener reports the client address a trusted L4 proxy names, so that
// every reader of RemoteAddr under this listener gets the client rather than
// the proxy without knowing a proxy is there.
type proxyListener struct {
	net.Listener
	policy  *auth.ProxyPolicy
	timeout time.Duration
	log     *slog.Logger
}

// Accept returns as soon as the kernel has a connection and leaves the header
// to a goroutine of that connection's own: reading it here would serialize the
// listener behind one peer's first packet.
func (l *proxyListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	// Trust is decided on the transport peer address, which the operating
	// system reports and a client cannot choose, and never on anything the
	// connection carries. An untrusted peer's stream is not touched, so a
	// header it prepends stays ordinary input to the SSH transport.
	if !l.policy.Trusts(peerAddr(conn.RemoteAddr())) {
		return conn, nil
	}
	return newProxyConn(conn, l.timeout, l.log), nil
}

// peerAddr reduces a transport address to the IP trust is decided on.
func peerAddr(addr net.Addr) netip.Addr {
	if addr == nil {
		return netip.Addr{}
	}
	if tcp, ok := addr.(*net.TCPAddr); ok {
		if parsed, ok := netip.AddrFromSlice(tcp.IP); ok {
			return parsed.Unmap()
		}
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		host = addr.String()
	}
	parsed, err := netip.ParseAddr(strings.Trim(strings.TrimSpace(host), "[]"))
	if err != nil {
		return netip.Addr{}
	}
	return parsed.Unmap()
}

// proxyConn is a connection from a trusted proxy whose header has been read, or
// is being read, before anything else sees the connection.
//
// The parse is eager rather than deferred to the first Read because the
// connection callback applies the per-source rate limit before the SSH
// transport exchanges any application byte: a lazily parsed wrapper would
// answer that caller with the proxy's address, leaving the one reader that
// fails unsafely the one reader still wrong.
type proxyConn struct {
	net.Conn
	done   chan struct{}
	remote net.Addr
}

// newProxyConn starts the parse and returns at once.
func newProxyConn(conn net.Conn, timeout time.Duration, log *slog.Logger) *proxyConn {
	p := &proxyConn{Conn: conn, done: make(chan struct{}), remote: conn.RemoteAddr()}
	if log == nil {
		log = slog.Default()
	}
	go p.resolve(timeout, log)
	return p
}

// resolve reads the header under a deadline, so a proxy that opens a
// connection and stalls holds its slot for a bounded time and nobody else's.
func (p *proxyConn) resolve(timeout time.Duration, log *slog.Logger) {
	defer close(p.done)
	if timeout <= 0 {
		timeout = DefaultIdleTimeout
	}
	_ = p.Conn.SetReadDeadline(time.Now().Add(timeout))
	header, err := proxyproto.Parse(p.Conn)
	_ = p.Conn.SetReadDeadline(time.Time{})
	if err != nil {
		// A trusted proxy that sends no valid header is refused rather than
		// accepted as itself. Falling back to the transport address would
		// reintroduce the collapsed rate-limit bucket for precisely the
		// connection that went wrong, and make a misconfigured proxy look as
		// though it were working.
		log.Warn("ssh: no valid proxy protocol header from a trusted proxy",
			"proxy", sourceOf(p.remote), "error", err)
		_ = p.Close()
		return
	}
	if header.Local {
		return
	}
	p.remote = net.TCPAddrFromAddrPort(header.Source)
	log.Debug("ssh: resolved the client address from a proxy protocol header",
		"proxy", sourceOf(p.Conn.RemoteAddr()), "client", sourceOf(p.remote), "version", header.Version)
}

// RemoteAddr reports the client address, waiting for the parse that decides it.
func (p *proxyConn) RemoteAddr() net.Addr {
	<-p.done
	return p.remote
}

// Read delivers the stream after the header, never the header itself.
func (p *proxyConn) Read(b []byte) (int, error) {
	<-p.done
	return p.Conn.Read(b)
}

// SetDeadline waits for the parse, whose own read deadline it would otherwise
// clear from under it.
func (p *proxyConn) SetDeadline(t time.Time) error {
	<-p.done
	return p.Conn.SetDeadline(t)
}

// SetReadDeadline waits for the parse, for the same reason as SetDeadline.
func (p *proxyConn) SetReadDeadline(t time.Time) error {
	<-p.done
	return p.Conn.SetReadDeadline(t)
}
