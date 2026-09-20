package retention

import (
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/core"
)

func TestWindowPerClass(t *testing.T) {
	p := core.RetentionPolicy{
		TenantID:          "t1",
		Events:            core.Duration(time.Hour),
		AuditEntries:      core.Duration(2 * time.Hour),
		WebhookDeliveries: core.Duration(3 * time.Hour),
	}
	tests := []struct {
		class Class
		want  time.Duration
		known bool
	}{
		{ClassEvents, time.Hour, true},
		{ClassAudit, 2 * time.Hour, true},
		{ClassDeliveries, 3 * time.Hour, true},
		{Class("tasks"), 0, false},
	}
	for _, tc := range tests {
		got, ok := Window(p, tc.class)
		if ok != tc.known || got != tc.want {
			t.Errorf("Window(%q) = %s, %v; want %s, %v", tc.class, got, ok, tc.want, tc.known)
		}
	}
}

func TestCutoffSubtractsTheWindow(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	p := core.RetentionPolicy{Events: core.Duration(24 * time.Hour)}

	got, ok := Cutoff(p, ClassEvents, now)
	if !ok || !got.Equal(now.Add(-24*time.Hour)) {
		t.Errorf("Cutoff = %s, %v", got, ok)
	}
	if _, ok := Cutoff(p, Class("nope"), now); ok {
		t.Error("an unknown class reported a cutoff")
	}
}

// Each class is governed independently, so one policy yields three distinct
// cutoffs and a shorter event window never shortens the audit record.
func TestCutoffsAreIndependentPerClass(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	p := core.RetentionPolicy{
		Events:            core.Duration(time.Hour),
		AuditEntries:      core.Duration(100 * time.Hour),
		WebhookDeliveries: core.Duration(10 * time.Hour),
	}
	got := Cutoffs(p, now)
	if len(got) != len(Classes()) {
		t.Fatalf("Cutoffs returned %d classes, want %d", len(got), len(Classes()))
	}
	if !got[ClassAudit].Before(got[ClassDeliveries]) || !got[ClassDeliveries].Before(got[ClassEvents]) {
		t.Errorf("cutoffs are not ordered by their windows: %+v", got)
	}
}

func TestValidateRejectsNegativeWindows(t *testing.T) {
	tests := []struct {
		name string
		p    core.RetentionPolicy
		ok   bool
	}{
		{"zero means default", core.RetentionPolicy{}, true},
		{"positive", core.DefaultRetention("t1"), true},
		{"negative events", core.RetentionPolicy{Events: core.Duration(-time.Second)}, false},
		{"negative audit", core.RetentionPolicy{AuditEntries: core.Duration(-time.Second)}, false},
		{"negative deliveries", core.RetentionPolicy{WebhookDeliveries: core.Duration(-time.Second)}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.p)
			if tc.ok && err != nil {
				t.Errorf("Validate = %v, want nil", err)
			}
			if !tc.ok && !core.IsKind(err, core.KindInvalid) {
				t.Errorf("Validate = %v, want invalid", err)
			}
		})
	}
}

func TestEffectiveFillsUnsetWindows(t *testing.T) {
	def := core.DefaultRetention("t1")
	got := Effective(core.RetentionPolicy{TenantID: "t1", Events: core.Duration(time.Hour)})

	if got.Events != core.Duration(time.Hour) {
		t.Errorf("set window was overwritten: %s", got.Events)
	}
	if got.AuditEntries != def.AuditEntries || got.WebhookDeliveries != def.WebhookDeliveries {
		t.Errorf("unset windows = %+v, want the shipped defaults", got)
	}
	if def.AuditEntries <= def.Events {
		t.Error("the shipped default must keep audit entries longer than events")
	}
}

func TestIsDefault(t *testing.T) {
	if !IsDefault(core.RetentionPolicy{TenantID: "t1"}) {
		t.Error("an unconfigured policy is not reported as the default")
	}
	if IsDefault(core.RetentionPolicy{TenantID: "t1", Events: core.Duration(time.Hour)}) {
		t.Error("a configured policy is reported as the default")
	}
}

func TestMergeLeavesUnsetWindowsAlone(t *testing.T) {
	current := core.RetentionPolicy{
		TenantID:          "t1",
		Events:            core.Duration(time.Hour),
		AuditEntries:      core.Duration(2 * time.Hour),
		WebhookDeliveries: core.Duration(3 * time.Hour),
	}
	got := Merge(current, core.RetentionPolicy{Events: core.Duration(30 * time.Minute)})

	if got.Events != core.Duration(30*time.Minute) {
		t.Errorf("event window = %s, want 30m", got.Events)
	}
	if got.AuditEntries != current.AuditEntries || got.WebhookDeliveries != current.WebhookDeliveries {
		t.Errorf("merge disturbed another class: %+v", got)
	}
	if got.TenantID != "t1" {
		t.Errorf("tenant = %q, want t1", got.TenantID)
	}
	if next := Merge(current, core.RetentionPolicy{TenantID: "t2"}); next.TenantID != "t2" {
		t.Errorf("tenant = %q, want t2", next.TenantID)
	}
}
