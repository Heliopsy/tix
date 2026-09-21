package sshd

import (
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
)

func TestLimiterSpendsAndRefillsPerSource(t *testing.T) {
	clk := clock.NewFakeAt()
	l := newLimiter(clk, time.Minute, 2, 16)

	tests := []struct {
		name    string
		source  string
		advance time.Duration
		want    bool
	}{
		{"first from a source", "10.0.0.1", 0, true},
		{"second from the same source", "10.0.0.1", 0, true},
		{"third exhausts the burst", "10.0.0.1", 0, false},
		{"another source is unaffected", "10.0.0.2", 0, true},
		{"a refill admits one more", "10.0.0.1", time.Minute, true},
		{"and only one", "10.0.0.1", 0, false},
		{"an unknown address is not counted", "", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clk.Advance(tc.advance)
			if got := l.allow(tc.source); got != tc.want {
				t.Fatalf("allow(%q) = %v, want %v", tc.source, got, tc.want)
			}
		})
	}
}

func TestLimiterStaysBounded(t *testing.T) {
	clk := clock.NewFakeAt()
	l := newLimiter(clk, time.Minute, 1, 4)
	for i := 0; i < 100; i++ {
		l.allow(string(rune('a' + i%26)))
	}
	if len(l.buckets) > 4 {
		t.Fatalf("the limiter holds %d sources, want at most 4", len(l.buckets))
	}
}

func TestRateIntervalIsTheGapBetweenRefills(t *testing.T) {
	tests := []struct {
		perHour int
		want    time.Duration
	}{
		{60, time.Minute},
		{3600, time.Second},
		{0, time.Hour},
		{-1, time.Hour},
	}
	for _, tc := range tests {
		if got := rateInterval(tc.perHour); got != tc.want {
			t.Errorf("rateInterval(%d) = %v, want %v", tc.perHour, got, tc.want)
		}
	}
}
