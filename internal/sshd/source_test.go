// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"context"
	"log/slog"
	"net"
	"path/filepath"
	"sort"
	"testing"
	"time"

	gossh "golang.org/x/crypto/ssh"
)

func TestSourceOfReducesAnAddressToTheHostTheLimitCountsAgainst(t *testing.T) {
	tests := []struct {
		name string
		addr net.Addr
		want string
	}{
		{
			name: "a reconnect from a fresh port is the same source",
			addr: &net.TCPAddr{IP: net.ParseIP("203.0.113.9"), Port: 54321},
			want: "203.0.113.9",
		},
		{
			name: "an IPv6 address keeps its host and loses its port",
			addr: &net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 2222},
			want: "2001:db8::1",
		},
		{
			name: "an address carrying no port is its own source",
			addr: &net.UnixAddr{Name: "/run/tix.sock", Net: "unix"},
			want: "/run/tix.sock",
		},
		{
			name: "no address at all counts against nobody",
			addr: nil,
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := sourceOf(tc.addr); got != tc.want {
				t.Fatalf("sourceOf(%v) = %q, want %q", tc.addr, got, tc.want)
			}
		})
	}
}

// TestTheListenerRateLimitsTheSourceItExtracted asserts both halves of the
// rate limit at once, because either half alone passes over a listener that is
// not limiting anything.
//
// The limiter's own arithmetic is covered in ratelimit_test.go; what is
// covered here is that the server hands it a real source. A sourceOf returning
// the empty string is admitted unconditionally, and a sourceOf keeping the
// ephemeral port gives every reconnection a fresh allowance: in both cases the
// limiter is present, correct and doing nothing at all. So the refusal is
// asserted, and so is the key the allowance was counted under.
func TestTheListenerRateLimitsTheSourceItExtracted(t *testing.T) {
	p, _, st := newProvisioner(t)
	srv, err := New(Options{
		Service: p.service,
		Store:   st,
		// A fake clock never refills a bucket, so the burst is the whole
		// allowance and the count below is exact.
		Clock:        p.clk,
		Demo:         true,
		Addr:         "127.0.0.1:0",
		HostKeyPath:  filepath.Join(t.TempDir(), "host_key"),
		RatePerHour:  1,
		RateBurst:    3,
		IdleTimeout:  time.Hour,
		ReapInterval: time.Hour,
		Logger:       slog.New(slog.DiscardHandler),
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

	const attempts = 12
	refusedAt := 0
	for i := 1; i <= attempts && refusedAt == 0; i++ {
		if err := handshake(t, srv.Addr()); err != nil {
			refusedAt = i
		}
	}
	if refusedAt == 0 {
		t.Errorf("%d connections from one address were all admitted on an allowance of 3, "+
			"so the listener is not counting them against anything", attempts)
	}

	// And the allowance they were counted against is the host, with no port:
	// a limiter keyed by host:port refuses nobody, because every reconnection
	// arrives on a port it has never seen.
	srv.limiter.mu.Lock()
	var sources []string
	for source := range srv.limiter.buckets {
		sources = append(sources, source)
	}
	srv.limiter.mu.Unlock()
	sort.Strings(sources)
	if len(sources) != 1 || sources[0] != "127.0.0.1" {
		t.Fatalf("the limiter counted %q, want exactly one allowance under %q",
			sources, "127.0.0.1")
	}
}

// handshake authenticates once with a fresh key and hangs up, which is the
// smallest thing the rate limit sees.
func handshake(t *testing.T, addr string) error {
	t.Helper()
	signer, _ := newSigner(t)
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            "visitor",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
		Timeout:         30 * time.Second,
	})
	if err != nil {
		return err
	}
	return client.Close()
}
