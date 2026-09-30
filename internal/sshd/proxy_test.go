// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	gossh "golang.org/x/crypto/ssh"
)

// parseBudget is how long a test listener waits for a header. It is a real
// socket deadline, which no fake clock can drive, so it is short enough that
// waiting for it twice costs nothing and long enough that a loaded machine
// still delivers a header inside it.
const parseBudget = 2 * time.Second

// v1 builds a version 1 line.
func v1(family, source, dest string, sport, dport int) []byte {
	return fmt.Appendf(nil, "PROXY %s %s %s %d %d\r\n", family, source, dest, sport, dport)
}

// v2 builds a version 2 header around an address block, deriving the declared
// length from the block so that a test cannot claim a length it did not write.
func v2(command, family byte, body []byte) []byte {
	out := []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}
	out = append(out, 0x20|command, family<<4|0x1, 0, 0)
	binary.BigEndian.PutUint16(out[14:16], uint16(len(body)))
	return append(out, body...)
}

// v2Block lays out one AF_INET or AF_INET6 address block.
func v2Block(source, dest netip.AddrPort) []byte {
	body := append([]byte(nil), source.Addr().AsSlice()...)
	body = append(body, dest.Addr().AsSlice()...)
	body = binary.BigEndian.AppendUint16(body, source.Port())
	return binary.BigEndian.AppendUint16(body, dest.Port())
}

// proxied returns a listener trusting the given proxies, and a dial function
// that opens a connection to it from loopback.
func proxied(t *testing.T, trusted ...string) (*proxyListener, func(t *testing.T) net.Conn) {
	t.Helper()
	raw, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	policy, err := auth.NewProxyPolicy(trusted)
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	ln := &proxyListener{
		Listener: raw,
		policy:   policy,
		timeout:  parseBudget,
		log:      slog.New(slog.DiscardHandler),
	}
	dial := func(t *testing.T) net.Conn {
		t.Helper()
		conn, err := net.Dial("tcp", raw.Addr().String())
		if err != nil {
			t.Fatalf("dialling: %v", err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		return conn
	}
	return ln, dial
}

// TestAnUntrustedPeerSendingAProxyHeaderIsNotParsed asserts both halves at
// once: the forged address is not believed, and the bytes that carried it are
// still on the stream. Either half alone passes over a listener that closed the
// connection or swallowed the header while reporting the transport address.
func TestAnUntrustedPeerSendingAProxyHeaderIsNotParsed(t *testing.T) {
	ln, dial := proxied(t, "192.0.2.1")
	conn := dial(t)
	forged := v1("TCP4", "203.0.113.7", "198.51.100.1", 4242, 2222)
	if _, err := conn.Write(append(append([]byte(nil), forged...), "SSH-2.0-tix\r\n"...)); err != nil {
		t.Fatalf("writing: %v", err)
	}

	served, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	t.Cleanup(func() { _ = served.Close() })

	if got, want := served.RemoteAddr().String(), conn.LocalAddr().String(); got != want {
		t.Errorf("RemoteAddr = %q, want the client's own transport address %q", got, want)
	}
	want := string(forged) + "SSH-2.0-tix\r\n"
	got := make([]byte, len(want))
	if err := served.SetReadDeadline(time.Now().Add(parseBudget)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	if _, err := io.ReadFull(served, got); err != nil {
		t.Fatalf("reading what the client sent: %v", err)
	}
	if string(got) != want {
		t.Errorf("the transport read %q, want the header delivered as ordinary input %q", got, want)
	}
}

// TestATrustedProxySpeaksForItsClient covers each shape the protocol allows,
// and asserts in every case that the header itself is off the stream: a reader
// that reported the right address but left the header in place would corrupt
// the SSH version exchange.
func TestATrustedProxySpeaksForItsClient(t *testing.T) {
	client4 := netip.MustParseAddrPort("203.0.113.7:4242")
	client6 := netip.MustParseAddrPort("[2001:db8::7]:4343")
	dest4 := netip.MustParseAddrPort("198.51.100.1:2222")
	dest6 := netip.MustParseAddrPort("[2001:db8::1]:2222")

	tests := []struct {
		name   string
		header []byte
		want   string
		// transport asks for the peer's own address instead of a fixed one,
		// because a proxy speaking for itself keeps whatever port it dialled
		// from.
		transport bool
	}{
		{
			name:   "v1 TCP4",
			header: v1("TCP4", "203.0.113.7", "198.51.100.1", 4242, 2222),
			want:   "203.0.113.7:4242",
		},
		{
			name:   "v1 TCP6",
			header: v1("TCP6", "2001:db8::7", "2001:db8::1", 4343, 2222),
			want:   "[2001:db8::7]:4343",
		},
		{
			name:      "v1 UNKNOWN keeps the transport address",
			header:    []byte("PROXY UNKNOWN\r\n"),
			transport: true,
		},
		{
			name:   "v2 AF_INET",
			header: v2(0x1, 0x1, v2Block(client4, dest4)),
			want:   "203.0.113.7:4242",
		},
		{
			name:   "v2 AF_INET6",
			header: v2(0x1, 0x2, v2Block(client6, dest6)),
			want:   "[2001:db8::7]:4343",
		},
		{
			name:      "v2 LOCAL keeps the transport address",
			header:    v2(0x0, 0x0, nil),
			transport: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ln, dial := proxied(t, "127.0.0.1")
			conn := dial(t)
			if _, err := conn.Write(append(append([]byte(nil), tc.header...), "SSH-2.0-tix\r\n"...)); err != nil {
				t.Fatalf("writing: %v", err)
			}
			served, err := ln.Accept()
			if err != nil {
				t.Fatalf("Accept: %v", err)
			}
			t.Cleanup(func() { _ = served.Close() })

			want := tc.want
			if tc.transport {
				want = conn.LocalAddr().String()
			}
			if got := served.RemoteAddr().String(); got != want {
				t.Errorf("RemoteAddr = %q, want %q", got, want)
			}
			if err := served.SetReadDeadline(time.Now().Add(parseBudget)); err != nil {
				t.Fatalf("deadline: %v", err)
			}
			rest := make([]byte, len("SSH-2.0-tix\r\n"))
			if _, err := io.ReadFull(served, rest); err != nil {
				t.Fatalf("reading after the header: %v", err)
			}
			if string(rest) != "SSH-2.0-tix\r\n" {
				t.Errorf("the transport read %q first, want the header consumed and %q left",
					rest, "SSH-2.0-tix\r\n")
			}
		})
	}
}

// TestATrustedProxyWithNoValidHeaderIsClosed pins the refusal rather than a
// fallback. Accepting the connection as the proxy's own address would hand the
// rate limiter one bucket for every client behind a misconfigured proxy, which
// is the bug this whole path exists to fix.
func TestATrustedProxyWithNoValidHeaderIsClosed(t *testing.T) {
	tests := []struct {
		name string
		send string
	}{
		{"a client that speaks ssh straight away", "SSH-2.0-OpenSSH_9.6\r\n"},
		{"a header that is not well formed", "PROXY TCP4 nonsense nonsense 1 2\r\n"},
		{"a v2 header with an unsupported version", "\x0D\x0A\x0D\x0A\x00\x0D\x0A\x51\x55\x49\x54\x0A\x31\x11\x00\x00"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ln, dial := proxied(t, "127.0.0.1")
			conn := dial(t)
			if _, err := conn.Write([]byte(tc.send)); err != nil {
				t.Fatalf("writing: %v", err)
			}
			served, err := ln.Accept()
			if err != nil {
				t.Fatalf("Accept: %v", err)
			}
			t.Cleanup(func() { _ = served.Close() })

			if err := served.SetReadDeadline(time.Now().Add(parseBudget)); err != nil &&
				!strings.Contains(err.Error(), "closed") {
				t.Fatalf("deadline: %v", err)
			}
			assertClosed(t, served, "the listener's side of the connection")
			// And the client sees it go, rather than sitting on a connection
			// the listener has quietly stopped serving.
			if err := conn.SetReadDeadline(time.Now().Add(parseBudget)); err != nil {
				t.Fatalf("client deadline: %v", err)
			}
			assertClosed(t, conn, "the client's connection")
		})
	}
}

// TestAStalledTrustedProxyIsClosedAndDelaysNobody is the reason the parse is
// not inside Accept. A peer that connects and sends nothing must cost only
// itself: the assertion is that a second connection resolves while the first is
// still stalling, well inside the deadline that will eventually close it.
func TestAStalledTrustedProxyIsClosedAndDelaysNobody(t *testing.T) {
	ln, dial := proxied(t, "127.0.0.1")

	stalled := dial(t)
	accepted := time.Now()
	stalledConn, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	t.Cleanup(func() { _ = stalledConn.Close() })
	// Accept must stay a syscall. Reading the header inside it would hold the
	// listener here for the whole deadline, and no other pending connection
	// would be accepted at all until this one gave up.
	if waited := time.Since(accepted); waited > parseBudget/2 {
		t.Errorf("Accept blocked for %v on a peer that sent nothing, want it to return at once", waited)
	}

	speaking := dial(t)
	if _, err := speaking.Write(v1("TCP4", "203.0.113.7", "198.51.100.1", 4242, 2222)); err != nil {
		t.Fatalf("writing: %v", err)
	}
	speakingConn, err := ln.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	t.Cleanup(func() { _ = speakingConn.Close() })

	start := time.Now()
	if got := speakingConn.RemoteAddr().String(); got != "203.0.113.7:4242" {
		t.Fatalf("RemoteAddr = %q, want the second client resolved", got)
	}
	if waited := time.Since(start); waited > parseBudget/2 {
		t.Errorf("the second connection waited %v for the first one's header, want none of it", waited)
	}

	// The stalled peer is closed on its own deadline, not left holding a slot.
	if err := stalled.SetReadDeadline(time.Now().Add(4 * parseBudget)); err != nil {
		t.Fatalf("client deadline: %v", err)
	}
	assertClosed(t, stalled, "the stalled connection")
	if waited := time.Since(start); waited < parseBudget/2 {
		t.Errorf("the stalled connection was closed after %v, want it to have had its deadline", waited)
	}
}

// TestNothingWrapsTheListenerWithoutATrustedProxy is the default: no policy, no
// wrapper, so no byte is read from a connection before the SSH transport sees
// it.
func TestNothingWrapsTheListenerWithoutATrustedProxy(t *testing.T) {
	tests := []struct {
		name    string
		trusted []string
		wrapped bool
	}{
		{name: "nothing configured"},
		{name: "an empty list", trusted: []string{}},
		{name: "a list of blanks", trusted: []string{"", "   "}},
		{name: "one proxy", trusted: []string{"127.0.0.1"}, wrapped: true},
		{name: "a block", trusted: []string{"10.0.0.0/8"}, wrapped: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, _, st := newProvisioner(t)
			srv, err := New(Options{
				Service:        p.service,
				Store:          st,
				Clock:          p.clk,
				Addr:           "127.0.0.1:0",
				HostKeyPath:    filepath.Join(t.TempDir(), "host_key"),
				TrustedProxies: tc.trusted,
				Logger:         slog.New(slog.DiscardHandler),
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if err := srv.Listen(); err != nil {
				t.Fatalf("Listen: %v", err)
			}
			t.Cleanup(func() { _ = srv.Close() })
			_, wrapped := srv.listener.(*proxyListener)
			if wrapped != tc.wrapped {
				t.Fatalf("the listener is wrapped = %v, want %v", wrapped, tc.wrapped)
			}
		})
	}
}

func TestAMalformedTrustedProxyIsRefusedAtConstruction(t *testing.T) {
	p, _, st := newProvisioner(t)
	_, err := New(Options{
		Service:        p.service,
		Store:          st,
		Clock:          p.clk,
		Addr:           "127.0.0.1:0",
		HostKeyPath:    filepath.Join(t.TempDir(), "host_key"),
		TrustedProxies: []string{"not-an-ip"},
		Logger:         slog.New(slog.DiscardHandler),
	})
	if err == nil {
		t.Fatal("New accepted a trusted proxy that is not an address")
	}
	if !strings.Contains(err.Error(), "not-an-ip") {
		t.Fatalf("New error = %q, want it to name the offending value", err)
	}
}

// TestTheRateLimiterBucketsByTheResolvedAddress drives the real connection
// callback, which is the one reader that runs before the SSH transport
// exchanges an application byte and the one that fails unsafely when it reads
// the proxy instead of the client.
//
// Both halves are asserted, because either alone passes over a listener that
// collapsed every client onto the proxy: one client is refused once its own
// allowance is gone, and another behind the same proxy is not.
func TestTheRateLimiterBucketsByTheResolvedAddress(t *testing.T) {
	p, _, st := newProvisioner(t)
	srv, err := New(Options{
		Service: p.service,
		Store:   st,
		// A fake clock never refills a bucket, so the burst is the whole
		// allowance and the counts below are exact.
		Clock:          p.clk,
		Demo:           true,
		Addr:           "127.0.0.1:0",
		HostKeyPath:    filepath.Join(t.TempDir(), "host_key"),
		TrustedProxies: []string{"127.0.0.1"},
		RatePerHour:    1,
		RateBurst:      2,
		IdleTimeout:    parseBudget,
		ReapInterval:   time.Hour,
		Logger:         slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = srv.Serve(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	first := v1("TCP4", "203.0.113.7", "198.51.100.1", 4242, 2222)
	second := v1("TCP4", "198.51.100.9", "198.51.100.1", 4343, 2222)
	for i := 1; i <= 2; i++ {
		if err := handshakeBehind(t, srv.Addr(), first); err != nil {
			t.Fatalf("connection %d of an allowance of 2 was refused: %v", i, err)
		}
	}
	if err := handshakeBehind(t, srv.Addr(), first); err == nil {
		t.Error("a third connection was admitted on an allowance of 2, " +
			"so the limiter is not counting the client the proxy named")
	}
	// The other client behind the same proxy still has its own allowance. If
	// the limiter were keyed on the proxy, this would be refused too.
	if err := handshakeBehind(t, srv.Addr(), second); err != nil {
		t.Errorf("a second client behind the same proxy was refused: %v", err)
	}

	srv.limiter.mu.Lock()
	var sources []string
	for source := range srv.limiter.buckets {
		sources = append(sources, source)
	}
	srv.limiter.mu.Unlock()
	sort.Strings(sources)
	want := []string{"198.51.100.9", "203.0.113.7"}
	if strings.Join(sources, ",") != strings.Join(want, ",") {
		t.Fatalf("the limiter counted %q, want one allowance per client %q", sources, want)
	}
}

// assertClosed fails unless conn has been closed by the other end. A read that
// merely hit its own deadline is not evidence of a close: the difference is the
// whole distinction between refusing a connection and leaving it open, and a
// bare "Read returned an error" passes over both.
func assertClosed(t *testing.T, conn net.Conn, what string) {
	t.Helper()
	n, err := conn.Read(make([]byte, 64))
	switch {
	case err == nil:
		t.Fatalf("%s read %d bytes, want it closed", what, n)
	case errors.Is(err, os.ErrDeadlineExceeded):
		t.Fatalf("%s timed out rather than being closed: %v", what, err)
	}
}

// handshakeBehind prepends header to a connection and then authenticates over
// it, which is what a client reaching the listener through a proxy looks like.
func handshakeBehind(t *testing.T, addr string, header []byte) error {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dialling: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.Write(header); err != nil {
		t.Fatalf("writing the header: %v", err)
	}
	signer, _ := newSigner(t)
	client, chans, reqs, err := gossh.NewClientConn(conn, addr, &gossh.ClientConfig{
		User:            "visitor",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
		Timeout:         30 * time.Second,
	})
	if err != nil {
		return err
	}
	return gossh.NewClient(client, chans, reqs).Close()
}
