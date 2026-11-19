// SPDX-License-Identifier: AGPL-3.0-or-later

package logging_test

import (
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/logging"
)

func TestDedupeLetsOneRecordAWindowThroughAndCountsTheRest(t *testing.T) {
	clk := clock.NewFakeAt()
	d := logging.NewDedupe(clk, time.Minute, 16)
	type step struct {
		advance time.Duration
		key     string
		ok      bool
		held    int
	}
	steps := []step{
		{0, "a", true, 0},
		{time.Second, "a", false, 0},
		{time.Second, "a", false, 0},
		{0, "b", true, 0},
		{58 * time.Second, "a", true, 2},
		{time.Second, "a", false, 0},
		{time.Minute, "a", true, 1},
		{time.Minute, "a", true, 0},
		{0, "b", true, 0},
	}
	for i, s := range steps {
		clk.Advance(s.advance)
		ok, held := d.Allow(s.key)
		if ok != s.ok || held != s.held {
			t.Errorf("step %d (%s): Allow = %v, %d; want %v, %d", i, s.key, ok, held, s.ok, s.held)
		}
	}
}

func TestDedupeHoldsAtMostItsKeys(t *testing.T) {
	clk := clock.NewFakeAt()
	d := logging.NewDedupe(clk, time.Minute, 2)
	for _, k := range []string{"a", "b", "c"} {
		clk.Advance(time.Second)
		if ok, _ := d.Allow(k); !ok {
			t.Fatalf("first record for %s held back", k)
		}
	}
	if ok, _ := d.Allow("a"); !ok {
		t.Error("the oldest key was kept past the bound, so its repeat was held back")
	}
	if ok, _ := d.Allow("c"); ok {
		t.Error("the newest key was dropped to make room")
	}
}
