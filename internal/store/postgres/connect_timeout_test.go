// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
)

// silentListener accepts connections and then says nothing, which is what a
// database too busy to answer looks like from here. A refused port would fail
// instantly and prove nothing about a wait.
func silentListener(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	held := make(chan net.Conn, 8)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				close(held)
				return
			}
			held <- conn
		}
	}()
	t.Cleanup(func() {
		_ = ln.Close()
		for conn := range held {
			_ = conn.Close()
		}
	})
	return ln
}

// TestOpenGivesUpAtTheConfiguredConnectTimeout proves the timeout is the
// configured one rather than the fifteen seconds it used to be hard-coded to.
func TestOpenGivesUpAtTheConfiguredConnectTimeout(t *testing.T) {
	ln := silentListener(t)
	dsn := "postgres://tix:tix@" + ln.Addr().String() + "/tix?sslmode=disable"

	start := time.Now()
	s, err := Open(dsn, clock.NewFakeAt(), WithConnectTimeout(300*time.Millisecond))
	elapsed := time.Since(start)
	if err == nil {
		_ = s.Close()
		t.Fatal("Open succeeded against a database that never answered")
	}
	if !strings.Contains(err.Error(), "unreachable within 300ms") {
		t.Fatalf("error does not name the configured wait: %v", err)
	}
	if elapsed < 250*time.Millisecond {
		t.Fatalf("gave up after %s, which is before the configured 300ms", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("waited %s, so the configured 300ms was ignored", elapsed)
	}
}

// TestWithConnectTimeoutKeepsTheDefaultForANonPositiveValue guards the one
// way a caller could accidentally ask for no wait at all.
func TestWithConnectTimeoutKeepsTheDefaultForANonPositiveValue(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Second} {
		cfg := openConfig{connectTimeout: DefaultConnectTimeout}
		WithConnectTimeout(d)(&cfg)
		if cfg.connectTimeout != DefaultConnectTimeout {
			t.Fatalf("WithConnectTimeout(%s) set %s, want the default %s", d, cfg.connectTimeout, DefaultConnectTimeout)
		}
	}
}
