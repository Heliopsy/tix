// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

func TestRenewedMovesTheExpiryWithinTheLimit(t *testing.T) {
	signedIn := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	base := StoredSession{CreatedAt: signedIn, ExpiresAt: signedIn.Add(12 * time.Hour),
		Window: 12 * time.Hour, Limit: 30 * 24 * time.Hour}
	tests := []struct {
		name string
		edit func(*StoredSession)
		at   time.Time
		want time.Time
	}{
		{"a use pushes the expiry a window past it", nil, signedIn.Add(2 * time.Hour), signedIn.Add(14 * time.Hour)},
		{"the limit caps it", func(s *StoredSession) { s.ExpiresAt = signedIn.Add(30*24*time.Hour - time.Hour) },
			signedIn.Add(30*24*time.Hour - 2*time.Hour), signedIn.Add(30 * 24 * time.Hour)},
		{"nothing past the limit", func(s *StoredSession) { s.ExpiresAt = signedIn.Add(30 * 24 * time.Hour) },
			signedIn.Add(30*24*time.Hour - time.Minute), time.Time{}},
		{"a zero limit renews nothing", func(s *StoredSession) { s.Limit = 0 }, signedIn.Add(time.Hour), time.Time{}},
		{"a limit below the window renews nothing", func(s *StoredSession) { s.Limit = time.Hour },
			signedIn.Add(30 * time.Minute), time.Time{}},
		{"never earlier than it was", func(s *StoredSession) { s.ExpiresAt = signedIn.Add(20 * time.Hour) },
			signedIn.Add(time.Hour), time.Time{}},
	}
	for _, tt := range tests {
		s := base
		if tt.edit != nil {
			tt.edit(&s)
		}
		if got := s.Renewed(tt.at); !got.Equal(tt.want) {
			t.Errorf("%s: Renewed = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func renewingVerifier(t *testing.T, window, limit time.Duration, remembered bool,
) (*clock.Fake, *touchingSessionStore, *MintedSession, *SessionVerifier) {
	t.Helper()
	clk := clock.NewFakeAt()
	st := &touchingSessionStore{fakeSessionStore: newSessionStore()}
	minted := mintSessionInto(t, st.fakeSessionStore, clk)
	stored := st.byHash[minted.Stored.TokenHash]
	stored.LastSeenAt, stored.Window, stored.Limit, stored.Remembered = clk.Now(), window, limit, remembered
	return clk, st, minted, NewSessionVerifier(st, clk)
}

func TestASessionInUseOutlivesItsFirstExpiry(t *testing.T) {
	clk, st, minted, v := renewingVerifier(t, time.Hour, 3*time.Hour, false)
	ctx := context.Background()
	for range 3 {
		clk.Advance(50 * time.Minute)
		if _, err := v.Verify(ctx, minted.Session.Token); err != nil {
			t.Fatalf("a session used every 50 minutes with a 1h window was refused at %v: %v",
				clk.Now().Sub(minted.Stored.CreatedAt), err)
		}
	}
	if want := minted.Stored.CreatedAt.Add(3 * time.Hour); !st.byHash[minted.Stored.TokenHash].ExpiresAt.Equal(want) {
		t.Errorf("expiry after 150 minutes of use = %v, want the 3h limit %v",
			st.byHash[minted.Stored.TokenHash].ExpiresAt, want)
	}
	clk.Advance(time.Hour)
	if _, err := v.Verify(ctx, minted.Session.Token); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("a session used past its maximum lifetime: err = %v, want unauthenticated", err)
	}
}

func TestRenewalIsWrittenAtMostOnceAMinute(t *testing.T) {
	clk, st, minted, v := renewingVerifier(t, time.Hour, 24*time.Hour, false)
	ctx := context.Background()
	clk.Advance(2 * time.Minute)
	actor, err := v.Verify(ctx, minted.Session.Token)
	if err != nil {
		t.Fatal(err)
	}
	first := st.byHash[minted.Stored.TokenHash].ExpiresAt
	if !first.Equal(clk.Now().Add(time.Hour)) || actor.Term == nil || !actor.Term.Renewed ||
		!actor.Term.ExpiresAt.Equal(first) {
		t.Fatalf("first renewal: stored %v, term %+v; want both at now+1h", first, actor.Term)
	}
	clk.Advance(30 * time.Second)
	actor, err = v.Verify(ctx, minted.Session.Token)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.touches) != 1 || !st.byHash[minted.Stored.TokenHash].ExpiresAt.Equal(first) {
		t.Errorf("touches = %d, expiry %v; want one write and the expiry unchanged within the minute",
			len(st.touches), st.byHash[minted.Stored.TokenHash].ExpiresAt)
	}
	if actor.Term.Renewed || !actor.Term.ExpiresAt.Equal(first) {
		t.Errorf("term within the minute = %+v; want not renewed, the stored expiry", actor.Term)
	}
}

func TestAGoneSessionIsNotRenewed(t *testing.T) {
	clk, st, minted, v := renewingVerifier(t, time.Hour, 24*time.Hour, false)
	st.gone = true
	clk.Advance(5 * time.Minute)
	if _, err := v.Verify(context.Background(), minted.Session.Token); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("a session deleted before its use was recorded: err = %v, want unauthenticated", err)
	}
}

func TestRenewedCookieFollowsOnlyARememberedSession(t *testing.T) {
	now := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	term := &core.SessionTerm{ExpiresAt: now.Add(30 * 24 * time.Hour), Remembered: true, Renewed: true, CheckedAt: now}
	c := RenewedSessionCookie("tok", term, true)
	if c == nil || c.Value != "tok" || c.MaxAge != 30*24*3600 || !c.Expires.Equal(term.ExpiresAt) ||
		!c.HttpOnly || !c.Secure {
		t.Fatalf("renewed remembered cookie = %+v; want Max-Age to the new expiry, HttpOnly and Secure", c)
	}
	for name, other := range map[string]*core.SessionTerm{
		"not renewed":    {ExpiresAt: term.ExpiresAt, Remembered: true, CheckedAt: now},
		"not remembered": {ExpiresAt: term.ExpiresAt, Renewed: true, CheckedAt: now},
		"no session":     nil,
	} {
		if c := RenewedSessionCookie("tok", other, true); c != nil {
			t.Errorf("%s: cookie %+v, want none", name, c)
		}
	}
}

func TestASwitchedSessionKeepsItsSignInLimit(t *testing.T) {
	clk := clock.NewFakeAt()
	signedIn := clk.Now().Add(-29 * 24 * time.Hour)
	minted, err := MintSession(clk, SessionInput{ActorID: "a", TenantID: "t", TTL: 12 * time.Hour,
		SignedInAt: signedIn, Limit: 29*24*time.Hour + time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if want := clk.Now().Add(time.Hour); !minted.Stored.ExpiresAt.Equal(want) || !minted.Stored.CreatedAt.Equal(signedIn) {
		t.Errorf("switched session: created %v expires %v; want the original sign-in and %v",
			minted.Stored.CreatedAt, minted.Stored.ExpiresAt, want)
	}
	if !minted.Session.CreatedAt.Equal(clk.Now()) {
		t.Errorf("the session handed back was minted at %v, want now so its cookie lasts to the expiry",
			minted.Session.CreatedAt)
	}
	fresh, err := MintSession(clk, SessionInput{ActorID: "a", TenantID: "t", TTL: 12 * time.Hour, Limit: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	if want := clk.Now().Add(12 * time.Hour); !fresh.Stored.ExpiresAt.Equal(want) {
		t.Errorf("a new sign-in with a limit below its lifetime expires %v, want the full %v", fresh.Stored.ExpiresAt, want)
	}
}
