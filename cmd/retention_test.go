// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// retentionOf reads the policy back as json.
func retentionOf(t *testing.T, c *cli) core.RetentionPolicy {
	t.Helper()
	out := c.mustRun("retention", "show", "-o", "json").out
	var policy core.RetentionPolicy
	if err := json.Unmarshal([]byte(out), &policy); err != nil {
		t.Fatalf("retention show is not json: %v\n%s", err, out)
	}
	return policy
}

// A fresh tenant reports the shipped policy, in every format.
func TestRetentionShowReportsTheEffectivePolicy(t *testing.T) {
	c := newCLI(t)
	policy := retentionOf(t, c)
	want := core.DefaultRetention(policy.TenantID)
	if policy != want {
		t.Fatalf("policy = %+v, want %+v", policy, want)
	}

	for _, format := range []string{"table", "json", "yaml"} {
		t.Run(format, func(t *testing.T) {
			got := c.mustRun("retention", "show", "-o", format)
			if !strings.Contains(strings.ToLower(got.out), "audit") {
				t.Fatalf("%s output = %q", format, got.out)
			}
		})
	}
}

// Setting one window leaves the others exactly as they were.
func TestRetentionSetChangesOnlyTheWindowsGiven(t *testing.T) {
	c := newCLI(t)
	before := retentionOf(t, c)

	c.mustRun("retention", "set", "--events", "48h")
	after := retentionOf(t, c)
	if after.Events != core.Duration(48*3600*1e9) {
		t.Fatalf("events = %s, want 48h", after.Events)
	}
	if after.AuditEntries != before.AuditEntries || after.WebhookDeliveries != before.WebhookDeliveries {
		t.Fatalf("policy = %+v, want the other windows unchanged from %+v", after, before)
	}

	c.mustRun("retention", "set", "--audit", "72h", "--webhook-deliveries", "12h")
	final := retentionOf(t, c)
	if final.Events != after.Events {
		t.Fatalf("events = %s, want it kept at %s", final.Events, after.Events)
	}
	if final.AuditEntries != core.Duration(72*3600*1e9) || final.WebhookDeliveries != core.Duration(12*3600*1e9) {
		t.Fatalf("policy = %+v", final)
	}
}

// A window the command cannot act on is refused, and the stored policy stands.
func TestRetentionSetRejectsBadWindows(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "no window at all", args: nil, wantErr: "at least one of"},
		{name: "zero window", args: []string{"--events", "0s"}, wantErr: "must be positive"},
		{name: "negative window", args: []string{"--events", "-1h"}, wantErr: "must not be negative"},
		{name: "not a duration", args: []string{"--audit", "forever"}, wantErr: "must be a duration"},
		{name: "a week, which nothing renders", args: []string{"--audit", "4w"}, wantErr: "must be a duration"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			before := retentionOf(t, c)
			got := c.run(append([]string{"retention", "set"}, tc.args...)...)
			if got.code != core.ExitUsage {
				t.Fatalf("exit = %d, want %d\n%s", got.code, core.ExitUsage, got.err)
			}
			if !strings.Contains(strings.ToLower(got.err), tc.wantErr) {
				t.Fatalf("stderr = %q, want it to mention %q", got.err, tc.wantErr)
			}
			if after := retentionOf(t, c); after != before {
				t.Fatalf("policy = %+v, want it unchanged", after)
			}
		})
	}
}

// TestRetentionSetTakesTheWindowAsItIsShown is the command-line end of the
// vocabulary fix: the retention windows are read in days on every screen, and
// "30d" had to be typed as "720h".
func TestRetentionSetTakesTheWindowAsItIsShown(t *testing.T) {
	c := newCLI(t)
	c.mustRun("retention", "set", "--events", "30d", "--audit", "365d", "--webhook-deliveries", "7d")
	policy := retentionOf(t, c)
	if policy.Events.D() != 720*time.Hour {
		t.Errorf("events = %v, want 720h", policy.Events.D())
	}
	if policy.AuditEntries.D() != 8760*time.Hour {
		t.Errorf("audit = %v, want 8760h", policy.AuditEntries.D())
	}
	if policy.WebhookDeliveries.D() != 168*time.Hour {
		t.Errorf("webhook deliveries = %v, want 168h", policy.WebhookDeliveries.D())
	}
	if got := policy.Events.Human(); got != "30d" {
		t.Errorf("Human() = %q, want the form it was typed in", got)
	}
}

// A dry run reports the windows it would write and changes nothing.
func TestRetentionSetDryRunWritesNothing(t *testing.T) {
	c := newCLI(t)
	before := retentionOf(t, c)
	got := c.mustRun("retention", "set", "--events", "1h", "--dry-run", "-o", "json")
	if !strings.Contains(got.out, "retention.put") || !strings.Contains(got.out, "1h0m0s") {
		t.Fatalf("dry run output = %q", got.out)
	}
	if after := retentionOf(t, c); after != before {
		t.Fatalf("policy = %+v, want it unchanged", after)
	}
}
