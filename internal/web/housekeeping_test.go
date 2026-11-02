// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// hkRule is one rule's block of a housekeeping form, cut at the next rule.
func hkRule(t *testing.T, form, rule string) string {
	t.Helper()
	open := `<div class="cfield hkrule" data-rule="` + rule + `">`
	start := strings.Index(form, open)
	if start < 0 {
		t.Fatalf("the form has no %s rule", rule)
	}
	block := form[start+len(open):]
	if end := strings.Index(block, `class="cfield hkrule"`); end >= 0 {
		block = block[:end]
	}
	return block
}

func TestTheTenantScreenSetsHousekeepingAndCountsWhatIsDue(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "shipped")
	moved := b.post("/projects/infra/move", url.Values{"ref": {ref}, "route": {"doing>done"}})
	_ = moved.Body.Close()

	form := formAt(t, b.page("/admin/tenant"), "/admin/tenant/housekeeping")
	if done := hkRule(t, form, "done"); !strings.Contains(done, `<option value="off" selected>`) ||
		strings.Contains(done, "inherit") || strings.Contains(done, "would be") {
		t.Fatalf("a fresh tenant's done rule is not plain off:\n%s", done)
	}

	saved := b.post("/admin/tenant/housekeeping", url.Values{
		"done_mode": {"days"}, "done_days": {"1"}, "cancelled_mode": {"off"}, "trash_mode": {"days"}, "trash_days": {"30"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)
	f.clock.Advance(48 * time.Hour)

	form = formAt(t, b.page("/admin/tenant"), "/admin/tenant/housekeeping")
	done := hkRule(t, form, "done")
	for _, want := range []string{`<option value="days" selected>`, `value="1"`, "1 task would be moved to the trash now."} {
		if !strings.Contains(done, want) {
			t.Errorf("the done rule lacks %q:\n%s", want, done)
		}
	}
	if trash := hkRule(t, form, "trash"); !strings.Contains(trash, "0 tasks would be deleted for good now.") {
		t.Errorf("the trash rule does not count:\n%s", trash)
	}
	if cancelled := hkRule(t, form, "cancelled"); strings.Contains(cancelled, "would be") {
		t.Errorf("an off rule counts:\n%s", cancelled)
	}

	missing := b.post("/admin/tenant/housekeeping", url.Values{
		"done_mode": {"days"}, "done_days": {""}, "cancelled_mode": {"off"}, "trash_mode": {"off"}})
	_ = missing.Body.Close()
	if missing.StatusCode == http.StatusSeeOther {
		t.Error("a rule set to days with no number was saved")
	}
}

func TestTheBoardOverridesHousekeepingForItsProject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	saved := b.post("/admin/tenant/housekeeping", url.Values{"done_mode": {"days"}, "done_days": {"30"}})
	_ = saved.Body.Close()

	form := formAt(t, b.page("/projects/infra"), "/projects/infra/housekeeping")
	if done := hkRule(t, form, "done"); !strings.Contains(done, `<option value="inherit" selected>Inherit: 30 days</option>`) {
		t.Fatalf("the project does not inherit the tenant's 30 days:\n%s", done)
	}
	override := b.post("/projects/infra/housekeeping", url.Values{
		"done_mode": {"off"}, "cancelled_mode": {"inherit"}, "trash_mode": {"inherit"}})
	_ = override.Body.Close()
	wantStatus(t, override, http.StatusSeeOther)
	form = formAt(t, b.page("/projects/infra"), "/projects/infra/housekeeping")
	if done := hkRule(t, form, "done"); !strings.Contains(done, `<option value="off" selected>`) {
		t.Errorf("the override is not shown off:\n%s", done)
	}

	if page := f.as("viewer").page("/projects/infra"); strings.Contains(page, "/housekeeping") {
		t.Error("a viewer's board carries the housekeeping form")
	}
}

func TestWithTheTrashOffHousekeepingSaysItIsPaused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	turnTrashOff(t, b)
	page := b.page("/admin/tenant")
	form := formAt(t, page, "/admin/tenant/housekeeping")
	if !strings.Contains(form, `<p class="note hkpaused">`) {
		t.Errorf("the housekeeping form does not say it is paused:\n%s", form)
	}
	refused := b.post("/admin/tenant/housekeeping", url.Values{"done_mode": {"days"}, "done_days": {"30"}})
	_ = refused.Body.Close()
	if refused.StatusCode == http.StatusSeeOther {
		t.Error("turning on the done rule with the trash off was saved")
	}
}
