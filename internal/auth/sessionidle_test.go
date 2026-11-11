// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// touchingSessionStore records every TouchSession call and moves the stored
// last-seen time the way the real store does.
type touchingSessionStore struct {
	*fakeSessionStore
	touches []time.Time
}

func (s *touchingSessionStore) TouchSession(_ context.Context, hash string, at time.Time) error {
	s.touches = append(s.touches, at)
	s.byHash[hash].LastSeenAt = at
	return nil
}

func TestAnIdleSessionIsRefusedAndAUsedOneIsKept(t *testing.T) {
	clk := clock.NewFakeAt()
	st := &touchingSessionStore{fakeSessionStore: newSessionStore()}
	minted := mintSessionInto(t, st.fakeSessionStore, clk)
	stored := st.byHash[minted.Stored.TokenHash]
	stored.LastSeenAt, stored.IdleTimeout = clk.Now(), 10*time.Minute
	v := NewSessionVerifier(st, clk)

	clk.Advance(9 * time.Minute)
	if _, err := v.Verify(context.Background(), minted.Session.Token); err != nil {
		t.Fatalf("a session used within the idle timeout was refused: %v", err)
	}
	if len(st.touches) != 1 {
		t.Fatalf("touches = %d, want 1 after a request a minute or more since the last", len(st.touches))
	}

	clk.Advance(30 * time.Second)
	if _, err := v.Verify(context.Background(), minted.Session.Token); err != nil {
		t.Fatal(err)
	}
	if len(st.touches) != 1 {
		t.Errorf("touches = %d, want still 1: use is recorded at most once a minute", len(st.touches))
	}

	clk.Advance(11 * time.Minute)
	if _, err := v.Verify(context.Background(), minted.Session.Token); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("a session idle past its timeout: err = %v, want unauthenticated", err)
	}
}

func TestNoIdleTimeoutLeavesOnlyTheExpiry(t *testing.T) {
	clk := clock.NewFakeAt()
	s := StoredSession{ExpiresAt: clk.Now().Add(time.Hour), LastSeenAt: clk.Now()}
	if !s.Active(clk.Now().Add(50 * time.Minute)) {
		t.Error("with no idle timeout a long-unused session before its expiry was refused")
	}
	s.IdleTimeout = 30 * time.Minute
	if s.Active(clk.Now().Add(50 * time.Minute)) {
		t.Error("a session unused past its idle timeout was accepted")
	}
}

func TestSessionCookieLastsOnlyWhenRemembered(t *testing.T) {
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	plain := SessionCookie(core.Session{Token: "t", CreatedAt: now, ExpiresAt: now.Add(12 * time.Hour)}, true)
	if plain.MaxAge != 0 || !plain.Expires.IsZero() {
		t.Errorf("an ordinary session's cookie has Max-Age %d and Expires %v, want a browser-session cookie",
			plain.MaxAge, plain.Expires)
	}
	kept := SessionCookie(core.Session{Token: "t", CreatedAt: now, ExpiresAt: now.Add(30 * 24 * time.Hour),
		Remembered: true}, true)
	if want := 30 * 24 * 3600; kept.MaxAge != want || !kept.Expires.Equal(now.Add(30*24*time.Hour)) {
		t.Errorf("a remembered session's cookie: Max-Age %d, Expires %v; want %d and the session's expiry",
			kept.MaxAge, kept.Expires, want)
	}
	if !kept.HttpOnly || !kept.Secure {
		t.Error("the remembered cookie lost HttpOnly or Secure")
	}
}
