// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/logging"
)

// syncBuffer is a buffer a logger and a test can share.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// records decodes every JSON record written so far.
func (b *syncBuffer) records(t *testing.T) []map[string]any {
	t.Helper()
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []map[string]any
	for line := range strings.Lines(b.buf.String()) {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("decoding %q: %v", line, err)
		}
		out = append(out, rec)
	}
	return out
}

// withMessage returns the records whose msg is msg.
func withMessage(recs []map[string]any, msg string) []map[string]any {
	var out []map[string]any
	for _, r := range recs {
		if r["msg"] == msg {
			out = append(out, r)
		}
	}
	return out
}

// refusalRouter is a Router with only what logRequest and logRefusal read.
func refusalRouter(clk clock.Clock) (*Router, *syncBuffer) {
	buf := &syncBuffer{}
	rt := &Router{cfg: Config{Logger: slog.New(slog.NewJSONHandler(buf, nil))}}
	rt.refusals = logging.NewDedupe(clk, refusalWindow, refusalMaxKeys, rt.logHeldBack)
	return rt, buf
}

// refuse sends one request from ip through logRequest to a handler that
// notes reason and answers status.
func refuse(rt *Router, ip, reason string, status int) {
	h := rt.logRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth.NoteRefusal(r.Context(), auth.Refusal{Reason: reason})
		w.WriteHeader(status)
	}))
	r := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	r = r.WithContext(context.WithValue(r.Context(), ctxKeyClientIP, netip.MustParseAddr(ip)))
	h.ServeHTTP(httptest.NewRecorder(), r)
}

func TestARefusalIsNamedOnlyWhenTheAnswerWasOne(t *testing.T) {
	cases := []struct {
		name   string
		reason string
		status int
		logged bool
	}{
		{"scope denied on a 403", auth.RefusedScopeDenied, http.StatusForbidden, true},
		{"optional scope denied on a 200 page", auth.RefusedScopeDenied, http.StatusOK, false},
		{"no credentials on a 401", auth.RefusedNoCredentials, http.StatusUnauthorized, true},
		{"no credentials on a public 200", auth.RefusedNoCredentials, http.StatusOK, false},
		{"wrong code on a 401", auth.RefusedTOTPInvalid, http.StatusUnauthorized, true},
		{"server error after a note", auth.RefusedTOTPInvalid, http.StatusInternalServerError, false},
		{"session refused on a page", auth.RefusedSessionExpired, http.StatusSeeOther, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rt, buf := refusalRouter(clock.NewFakeAt())
			refuse(rt, "192.0.2.1", tc.reason, tc.status)
			recs := buf.records(t)
			requests := withMessage(recs, "request")
			if len(requests) != 1 {
				t.Fatalf("%d request lines, want 1", len(requests))
			}
			_, named := requests[0]["auth_reason"]
			if named != tc.logged {
				t.Errorf("request line names auth_reason = %v, want %v: %v", named, tc.logged, requests[0])
			}
			if got := len(withMessage(recs, MsgAuthRefused)); got != map[bool]int{true: 1}[tc.logged] {
				t.Errorf("%d refusal records, want logged = %v", got, tc.logged)
			}
		})
	}
}

func TestOneIPv6SlashSixtyFourIsOneClientToTheRefusalLog(t *testing.T) {
	clk := clock.NewFakeAt()
	rt, buf := refusalRouter(clk)
	for _, ip := range []string{"2001:db8:1:2::1", "2001:db8:1:2::2", "2001:db8:1:2:ffff::9", "2001:db8:1:3::1",
		"192.0.2.1", "192.0.2.2"} {
		refuse(rt, ip, auth.RefusedBadPassword, http.StatusUnauthorized)
	}
	var got []any
	for _, r := range withMessage(buf.records(t), MsgAuthRefused) {
		got = append(got, r["client_ip"])
	}
	want := []any{"2001:db8:1:2::1", "2001:db8:1:3::1", "192.0.2.1", "192.0.2.2"}
	if len(got) != len(want) {
		t.Fatalf("refusal records from %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("refusal records from %v, want %v", got, want)
		}
	}
	clk.Advance(refusalWindow)
	rt.refusals.Sweep()
	held := withMessage(buf.records(t), MsgAuthRefusalsHeldBack)
	if len(held) != 1 || held[0]["client_ip"] != "2001:db8:1:2::/64" || held[0]["suppressed"] != 2.0 ||
		held[0]["reason"] != auth.RefusedBadPassword || held[0]["level"] != "WARN" {
		t.Errorf("held back records %v, want one warn for 2001:db8:1:2::/64 with suppressed 2", held)
	}
}

func TestRefusalsHeldBackAreLoggedWhenTheClientStops(t *testing.T) {
	clk := clock.NewFakeAt()
	rt, buf := refusalRouter(clk)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- rt.ReportHeldBackRefusals(ctx) }()
	for range 50 {
		refuse(rt, "192.0.2.1", auth.RefusedBadPassword, http.StatusUnauthorized)
	}
	if got := withMessage(buf.records(t), MsgAuthRefused); len(got) != 1 {
		t.Fatalf("%d refusal records, want 1", len(got))
	}
	cancel()
	<-done
	held := withMessage(buf.records(t), MsgAuthRefusalsHeldBack)
	if len(held) != 1 || held[0]["client_ip"] != "192.0.2.1" || held[0]["suppressed"] != 49.0 {
		t.Errorf("held back records %v, want one for 192.0.2.1 with suppressed 49", held)
	}
}

func TestAFullRefusalLogKeepsItsPollersAndCountsTheRestTogether(t *testing.T) {
	clk := clock.NewFakeAt()
	rt, buf := refusalRouter(clk)
	refuse(rt, "192.0.2.1", auth.RefusedTokenExpired, http.StatusUnauthorized)
	for i := range refusalMaxKeys + 100 {
		ip := netip.AddrFrom4([4]byte{10, byte(i >> 16), byte(i >> 8), byte(i)})
		refuse(rt, ip.String(), auth.RefusedUnknownAccount, http.StatusUnauthorized)
	}
	refuse(rt, "192.0.2.1", auth.RefusedTokenExpired, http.StatusUnauthorized)
	if got := len(withMessage(buf.records(t), MsgAuthRefused)); got != refusalMaxKeys+1 {
		t.Errorf("%d refusal records, want one per key and one overflow record, %d", got, refusalMaxKeys+1)
	}
	clk.Advance(time.Minute)
	rt.refusals.Sweep()
	var overflow, poller bool
	for _, r := range withMessage(buf.records(t), MsgAuthRefusalsHeldBack) {
		overflow = overflow || r["overflow"] == true && r["suppressed"] == 100.0
		poller = poller || r["client_ip"] == "192.0.2.1" && r["suppressed"] == 1.0
	}
	if !overflow || !poller {
		t.Errorf("overflow reported %v, poller kept and reported %v", overflow, poller)
	}
}
