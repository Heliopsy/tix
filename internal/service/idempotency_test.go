// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// idemHarness drives BeginIdempotent and FinishIdempotent for one actor.
type idemHarness struct {
	t     *testing.T
	l     *Local
	ctx   context.Context
	actor *core.Actor
}

func newIdemHarness(t *testing.T, opts ...Option) (*idemHarness, func(time.Duration)) {
	t.Helper()
	l, clk, _, actor := newLocalWith(t, opts...)
	return &idemHarness{t: t, l: l, ctx: core.WithActor(context.Background(), actor), actor: actor}, clk.Advance
}

func (h *idemHarness) begin(key, fp string) *IdempotentStart {
	h.t.Helper()
	start, err := h.l.BeginIdempotent(h.ctx, IdempotentRequest{Key: key, Fingerprint: fp, Lease: time.Minute})
	if err != nil {
		h.t.Fatalf("begin: %v", err)
	}
	return start
}

// commit makes one write as the request holding the claim.
func (h *idemHarness) commit(c *IdempotencyClaim) {
	h.t.Helper()
	ctx := WithIdempotencyClaim(h.ctx, c)
	if err := h.l.write(ctx, h.actor, func(m *mutation) error {
		return m.Audit("test.write", "tenant", h.actor.TenantID, nil, nil)
	}); err != nil {
		h.t.Fatalf("write: %v", err)
	}
}

func (h *idemHarness) finish(c *IdempotencyClaim, resp IdempotentResponse) {
	h.t.Helper()
	if err := h.l.FinishIdempotent(h.ctx, c, resp); err != nil {
		h.t.Fatalf("finish: %v", err)
	}
}

func TestAnIdempotentRequestRunsOnceThenReplaysOrRefuses(t *testing.T) {
	h, _ := newIdemHarness(t)
	first := h.begin("k", "fp")
	if first.Outcome != IdempotentRun || first.Claim == nil {
		t.Fatalf("first attempt = %+v, want run with a claim", first)
	}
	h.commit(first.Claim)
	h.finish(first.Claim, IdempotentResponse{Status: http.StatusCreated, ContentType: "application/json", Body: []byte(`{"ref":"INFRA-1"}`)})

	again := h.begin("k", "fp")
	if again.Outcome != IdempotentReplay || again.Response.Status != http.StatusCreated ||
		string(again.Response.Body) != `{"ref":"INFRA-1"}` || again.Response.ContentType != "application/json" {
		t.Fatalf("repeat = %+v, want the kept 201", again)
	}
	if other := h.begin("k", "another"); other.Outcome != IdempotentMismatch {
		t.Fatalf("a different request with the same key = %v, want mismatch", other.Outcome)
	}
}

func TestARunningRequestIsInFlightAndAnAppliedOneIsNeverRunAgain(t *testing.T) {
	h, advance := newIdemHarness(t)
	first := h.begin("k", "fp")
	h.commit(first.Claim)

	during := h.begin("k", "fp")
	if during.Outcome != IdempotentInFlight || during.RetryAfter != time.Minute {
		t.Fatalf("during the lease = %+v, want in flight for a minute", during)
	}
	advance(2 * time.Minute)
	if after := h.begin("k", "fp"); after.Outcome != IdempotentLost {
		t.Fatalf("an applied request past its lease = %v, want lost rather than run", after.Outcome)
	}
}

func TestAReservationThatCommittedNothingIsTakenOverAfterItsLease(t *testing.T) {
	h, advance := newIdemHarness(t)
	first := h.begin("k", "fp")
	if during := h.begin("k", "fp"); during.Outcome != IdempotentInFlight {
		t.Fatalf("during the lease = %v, want in flight", during.Outcome)
	}
	advance(2 * time.Minute)
	after := h.begin("k", "fp")
	if after.Outcome != IdempotentRun || after.Claim.ID == first.Claim.ID {
		t.Fatalf("past the lease = %+v, want a fresh run", after)
	}
	h.finish(first.Claim, IdempotentResponse{Status: http.StatusOK})
	if replay := h.begin("k", "fp"); replay.Outcome != IdempotentInFlight {
		t.Fatalf("the stale claim settled the new reservation: %v", replay.Outcome)
	}
}

func TestFinishKeepsWhatIsWorthRepeating(t *testing.T) {
	big := []byte(strings.Repeat("x", 10))
	for _, tc := range []struct {
		name      string
		committed bool
		resp      IdempotentResponse
		want      IdempotentOutcome
	}{
		{"success without a write is kept", false, IdempotentResponse{Status: http.StatusOK}, IdempotentReplay},
		{"client error without a write is released", false, IdempotentResponse{Status: http.StatusBadRequest}, IdempotentRun},
		{"conflict without a write is released", false, IdempotentResponse{Status: http.StatusConflict}, IdempotentRun},
		{"server error without a write is released", false, IdempotentResponse{Status: http.StatusInternalServerError}, IdempotentRun},
		{"server error after a write is kept", true, IdempotentResponse{Status: http.StatusInternalServerError}, IdempotentReplay},
		{"client error after a write is kept", true, IdempotentResponse{Status: http.StatusUnprocessableEntity}, IdempotentReplay},
		{"too large without a write is released", false, IdempotentResponse{Status: http.StatusOK, Body: big, TooLarge: true}, IdempotentRun},
		{"too large after a write is applied without an answer", true, IdempotentResponse{Status: http.StatusOK, Body: big, TooLarge: true}, IdempotentLost},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, advance := newIdemHarness(t)
			first := h.begin("k", "fp")
			if tc.committed {
				h.commit(first.Claim)
			}
			h.finish(first.Claim, tc.resp)
			advance(2 * time.Minute)
			got := h.begin("k", "fp")
			if got.Outcome != tc.want {
				t.Fatalf("retry = %v, want %v", got.Outcome, tc.want)
			}
			if tc.want == IdempotentReplay && got.Response.Status != tc.resp.Status {
				t.Fatalf("replayed %d, want %d", got.Response.Status, tc.resp.Status)
			}
		})
	}
}

func TestAKeyAnswersForItsWindowOnly(t *testing.T) {
	h, advance := newIdemHarness(t, WithIdempotencyWindow(time.Hour))
	first := h.begin("k", "fp")
	h.finish(first.Claim, IdempotentResponse{Status: http.StatusOK})
	advance(59 * time.Minute)
	if got := h.begin("k", "fp"); got.Outcome != IdempotentReplay {
		t.Fatalf("inside the window = %v, want replay", got.Outcome)
	}
	advance(2 * time.Minute)
	if got := h.begin("k", "fp"); got.Outcome != IdempotentRun {
		t.Fatalf("past the window = %v, want a new run", got.Outcome)
	}
}

func TestPruneRemovesExpiredIdempotencyKeys(t *testing.T) {
	h, advance := newIdemHarness(t)
	first := h.begin("old", "fp")
	h.finish(first.Claim, IdempotentResponse{Status: http.StatusOK})
	advance(23 * time.Hour)
	recent := h.begin("recent", "fp")
	h.finish(recent.Claim, IdempotentResponse{Status: http.StatusOK})
	advance(2 * time.Hour)

	dry, err := h.l.Prune(h.ctx, core.PruneInput{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	res, err := h.l.Prune(h.ctx, core.PruneInput{})
	if err != nil {
		t.Fatal(err)
	}
	if dry.IdempotencyKeys != 1 || res.IdempotencyKeys != 1 {
		t.Fatalf("dry run counted %d and prune removed %d, want 1 and 1", dry.IdempotencyKeys, res.IdempotencyKeys)
	}
	if got := h.begin("recent", "fp"); got.Outcome != IdempotentReplay {
		t.Fatalf("a key inside its window = %v after pruning, want replay", got.Outcome)
	}
}

func TestBeginRefusesAMalformedKey(t *testing.T) {
	h, _ := newIdemHarness(t)
	for _, key := range []string{"", strings.Repeat("k", MaxIdempotencyKeyLen+1), "café", "tab\there"} {
		_, err := h.l.BeginIdempotent(h.ctx, IdempotentRequest{Key: key, Fingerprint: "fp"})
		if !core.IsKind(err, core.KindInvalid) {
			t.Errorf("key %q: %v, want invalid", key, err)
		}
	}
	if err := ValidateIdempotencyKey(strings.Repeat("k", MaxIdempotencyKeyLen)); err != nil {
		t.Errorf("a key of the largest length: %v", err)
	}
}

func TestAWriteMarksOnlyTheClaimOfItsOwnTenant(t *testing.T) {
	h, advance := newIdemHarness(t)
	first := h.begin("k", "fp")
	foreign := *first.Claim
	foreign.TenantID = "another-tenant"
	h.commit(&foreign)
	advance(2 * time.Minute)
	if got := h.begin("k", "fp"); got.Outcome != IdempotentRun {
		t.Fatalf("a write in another tenant marked the key: %v", got.Outcome)
	}
}
