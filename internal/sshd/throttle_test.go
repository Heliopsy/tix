// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	gossh "golang.org/x/crypto/ssh"
)

func TestTakeSaysHowLongUntilTheNextToken(t *testing.T) {
	clk := clock.NewFakeAt()
	l := newLimiter(clk, time.Minute, 1, 4)
	if wait := l.take("a"); wait != 0 {
		t.Fatalf("the first connection was told to wait %v", wait)
	}
	if wait := l.take("a"); wait != time.Minute {
		t.Fatalf("an empty bucket said wait %v, want a full interval", wait)
	}
	clk.Advance(45 * time.Second)
	if wait := l.take("a"); wait != 15*time.Second {
		t.Fatalf("a bucket three quarters refilled said wait %v, want 15s", wait)
	}
	if wait := newLimiter(clk, 0, 1, 4).take(""); wait != 0 {
		t.Fatalf("an unknown address was told to wait %v", wait)
	}
}

func TestThrottleMessageRoundsUpToWholeSeconds(t *testing.T) {
	got := throttleMessage(1500 * time.Millisecond)
	want := "too many connections from this address; try again in 2 seconds"
	if got != want {
		t.Fatalf("throttleMessage = %q, want %q", got, want)
	}
}

// TestAThrottledConnectionIsToldWhy is the reported failure: a few quick
// logins in a row ended in "Permission denied (publickey)", which reads as an
// unknown key. A throttled connection must authenticate and then be refused
// with the reason and a nonzero exit, and one connection spends one token.
func TestAThrottledConnectionIsToldWhy(t *testing.T) {
	p, _, st := newProvisioner(t)
	srv, err := New(Options{
		Service:      p.service,
		Store:        st,
		Clock:        p.clk,
		Demo:         true,
		Addr:         "127.0.0.1:0",
		HostKeyPath:  filepath.Join(t.TempDir(), "host_key"),
		RatePerHour:  60,
		RateBurst:    1,
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
	t.Cleanup(func() { cancel(); <-done })

	dialKeys := func(n int) *gossh.Client {
		var signers []gossh.Signer
		for range n {
			s, _ := newSigner(t)
			signers = append(signers, s)
		}
		client, err := gossh.Dial("tcp", srv.Addr(), &gossh.ClientConfig{
			User:            "visitor",
			Auth:            []gossh.AuthMethod{gossh.PublicKeys(signers...)},
			HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
			Timeout:         30 * time.Second,
		})
		if err != nil {
			t.Fatalf("a throttled connection failed authentication, so the client sees only a denial: %v", err)
		}
		t.Cleanup(func() { _ = client.Close() })
		return client
	}

	if err := throttleRefusal(t, dialKeys(1)); err != nil {
		t.Fatalf("the first connection of a burst of 1 was throttled: %v", err)
	}
	if got := srv.limiter.buckets["127.0.0.1"].tokens; got != 0 {
		t.Fatalf("one connection left %v tokens, want it to spend exactly one", got)
	}

	sess, err := dialKeys(1).NewSession()
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	var stderr bytes.Buffer
	sess.Stderr = &stderr
	err = sess.Run("board")
	var exit *gossh.ExitError
	if !errors.As(err, &exit) || exit.ExitStatus() == 0 {
		t.Fatalf("a throttled session ended with %v, want a nonzero exit", err)
	}
	want := "tix: too many connections from this address; try again in 60 seconds"
	if !strings.Contains(stderr.String(), want) {
		t.Fatalf("a throttled session was told %q, want %q", stderr.String(), want)
	}
}
