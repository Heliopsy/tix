// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
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

// hkPanel is the board's Housekeeping panel, cut before the panel after it.
func hkPanel(t *testing.T, page string) string {
	t.Helper()
	panel := between(t, page, `<details class="panel hkpanel">`, `<details class="panel danger">`)
	if panel == "" {
		t.Fatal("the board has no Housekeeping panel")
	}
	return panel
}

// inTrash reports whether a task has been moved to the trash or is gone.
func (f *fixture) inTrash(t *testing.T, ref string) bool {
	t.Helper()
	parsed, err := core.ParseTaskRef(ref)
	if err != nil {
		t.Fatalf("parsing %q: %v", ref, err)
	}
	task, err := f.svc.GetTask(f.ctx(), parsed)
	if core.IsKind(err, core.KindNotFound) {
		return true
	}
	if err != nil {
		t.Fatalf("reading %s: %v", ref, err)
	}
	return task.Deleted()
}

// finishedIn makes a done task in each project and turns on a one-day done
// rule for the tenant, two days before now.
func finishedIn(t *testing.T, f *fixture, b *browser, projects ...string) []string {
	t.Helper()
	var refs []string
	for _, p := range projects {
		ref := b.createTask(p, "shipped in "+p)
		moved := b.post("/projects/"+p+"/move", url.Values{"ref": {ref}, "route": {"doing>done"}})
		_ = moved.Body.Close()
		refs = append(refs, ref)
	}
	saved := b.post("/admin/tenant/housekeeping", url.Values{
		"done_mode": {"days"}, "done_days": {"1"}, "cancelled_mode": {"off"}, "trash_mode": {"off"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)
	f.clock.Advance(48 * time.Hour)
	return refs
}

func TestTheBoardCleansUpItsOwnProjectNow(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "ops")
	b := f.as("alice")
	refs := finishedIn(t, f, b, "infra", "ops")

	panel := hkPanel(t, b.page("/projects/infra"))
	form := formAt(t, panel, "/projects/infra/housekeeping/run")
	for _, want := range []string{`name="confirm" value="1"`, "Cleanup acts on this project only: 1 to the trash, 0 deleted for good."} {
		if !strings.Contains(form, want) {
			t.Errorf("the run form lacks %q:\n%s", want, form)
		}
	}
	if !strings.Contains(between(t, panel, `<details class="hkrun">`, "</details>"), "<summary>Clean up now</summary>") {
		t.Errorf("Clean up now does not open a confirmation first:\n%s", panel)
	}
	if strings.Contains(b.page("/admin/tenant"), "/projects/infra/housekeeping/run") {
		t.Error("the Tenant screen carries the project's run form")
	}

	unconfirmed := b.post("/projects/infra/housekeeping/run", url.Values{})
	_ = unconfirmed.Body.Close()
	if unconfirmed.StatusCode == http.StatusSeeOther || f.inTrash(t, refs[0]) {
		t.Fatal("cleanup ran without the confirmation")
	}

	ran := b.post("/projects/infra/housekeeping/run", url.Values{"confirm": {"1"}})
	_ = ran.Body.Close()
	wantStatus(t, ran, http.StatusSeeOther)
	const want = "/projects/infra?flash=cleanup+ran%3A+1+moved+to+the+trash%2C+0+deleted+for+good#housekeeping"
	if got := ran.Header.Get("Location"); got != want {
		t.Errorf("the run went to %q, want %q", got, want)
	}
	if !f.inTrash(t, refs[0]) {
		t.Error("the project's done task was not moved to the trash")
	}
	if f.inTrash(t, refs[1]) {
		t.Error("cleaning up infra moved a task of ops")
	}
}

func TestCleanUpNowIsOffWhenNothingIsDueOrTheTrashIsOff(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, reason string
		setup        func(t *testing.T, f *fixture, b *browser)
	}{
		{"nothing due", "Nothing to clean up now.", func(*testing.T, *fixture, *browser) {}},
		{"trash off", "Paused while the trash is off.", func(t *testing.T, f *fixture, b *browser) {
			finishedIn(t, f, b, "infra")
			turnTrashOff(t, b)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t)
			b := f.as("alice")
			tc.setup(t, f, b)
			panel := hkPanel(t, b.page("/projects/infra"))
			if strings.Contains(panel, "/housekeeping/run") {
				t.Errorf("the panel offers a run:\n%s", panel)
			}
			if !strings.Contains(panel, `<button class="secondary" type="button" disabled>Clean up now</button>`) ||
				!strings.Contains(panel, tc.reason) {
				t.Errorf("the panel does not show Clean up now disabled with %q:\n%s", tc.reason, panel)
			}
		})
	}
}

func TestOnlyAnAdminCleansUpAProject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	refs := finishedIn(t, f, f.as("alice"), "infra")
	member := f.as("member:m1")
	if page := member.page("/projects/infra"); strings.Contains(page, "/housekeeping/run") {
		t.Error("a member's board carries the run form")
	}
	refused := member.post("/projects/infra/housekeeping/run", url.Values{"confirm": {"1"}})
	_ = refused.Body.Close()
	wantStatus(t, refused, http.StatusForbidden)
	if f.inTrash(t, refs[0]) {
		t.Error("a member's run moved the task")
	}
}
