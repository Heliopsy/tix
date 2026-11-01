// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// snoozedUntil reads a task's wake time back from the service, nil when it is
// not snoozed.
func snoozedUntil(t *testing.T, f *fixture, ref string) *time.Time {
	t.Helper()
	task, err := f.svc.GetTask(f.ctx(), core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)})
	if err != nil {
		t.Fatalf("reading %s: %v", ref, err)
	}
	return task.SnoozedUntil
}

// snoozeMenu is the task screen's snooze popover and nothing else of the page.
func snoozeMenu(t *testing.T, page, ref string) string {
	t.Helper()
	return popover(t, page, "snooze-"+ref)
}

// The task screen's title bar opens a menu of the three presets and a date
// picker, every one a plain form post that returns to the screen.
func TestTheTaskScreenOffersASnoozeMenu(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "not this week")
	page := b.page("/tasks/" + ref)

	if !strings.Contains(between(t, page, `<div class="titlebar"`, "</div>"), `popovertarget="snooze-`+ref+`"`) {
		t.Fatalf("the title bar has no Snooze button opening the menu:\n%s", page)
	}
	menu := snoozeMenu(t, page, ref)
	if menu == "" {
		t.Fatalf("the task screen has no snooze menu")
	}
	form := formAt(t, menu, "/tasks/"+ref+"/snooze")
	for _, want := range []string{
		`name="until" value="later"`, `name="until" value="tomorrow"`, `name="until" value="next-week"`,
		`type="datetime-local"`, `name="at"`, `name="csrf_token"`, `name="next" value="/tasks/` + ref,
		"Later today", "Tomorrow morning", "Next week", "Pick a date",
	} {
		if !strings.Contains(form, want) {
			t.Errorf("the snooze form lacks %s:\n%s", want, form)
		}
	}
	if strings.Contains(page, "snoozemark") {
		t.Errorf("an awake task carries a snoozed badge:\n%s", page)
	}
}

// A preset snoozes the task until the moment it names in the reader's zone,
// and the task leaves the default listing.
func TestASnoozePresetSnoozesTheTaskInTheReadersZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.setCookie(web.TimezoneCookie, "Asia/Tokyo")
	ref := b.createTask("infra", "after the weekend")

	resp := b.post("/tasks/"+ref+"/snooze", url.Values{"until": {"tomorrow"}, "next": {"/tasks"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	loc := resp.Header.Get("Location")
	if !strings.HasPrefix(loc, "/tasks?flash=snoozed+"+ref+"+until+") || !strings.HasSuffix(loc, "#t-"+ref) {
		t.Errorf("redirected to %q, want back to the listing at the row with a toast", loc)
	}
	until := snoozedUntil(t, f, ref)
	if until == nil {
		t.Fatal("the preset did not snooze the task")
	}
	tokyo, _ := time.LoadLocation("Asia/Tokyo")
	if got := until.In(tokyo); got.Hour() != 9 || got.Minute() != 0 {
		t.Errorf("tomorrow woke at %s in Tokyo, want 09:00 there", got)
	}
	if strings.Contains(b.page("/tasks"), `id="t-`+ref+`"`) {
		t.Error("the snoozed task is still in the default listing")
	}
	if !strings.Contains(b.page("/tasks?q=is:snoozed"), `id="t-`+ref+`"`) {
		t.Error("is:snoozed does not list the snoozed task")
	}
}

// A date and time picked by hand is read in the reader's zone too.
func TestAPickedDateSnoozesUntilThatMoment(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.setCookie(web.TimezoneCookie, "Europe/Sofia")
	ref := b.createTask("infra", "on a date")
	sofia, _ := time.LoadLocation("Europe/Sofia")
	want := time.Now().In(sofia).Add(72 * time.Hour).Truncate(time.Minute)

	resp := b.post("/tasks/"+ref+"/snooze", url.Values{"at": {want.Format(web.SnoozeAtLayout)}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	until := snoozedUntil(t, f, ref)
	if until == nil || !until.Equal(want) {
		t.Fatalf("snoozed until %v, want %v", until, want)
	}
}

// A picked time that has passed is refused and changes nothing.
func TestASnoozeInThePastIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "too late")

	for name, form := range map[string]url.Values{
		"a past date":    {"at": {"2020-01-01T09:00"}},
		"an unread date": {"at": {"soon"}},
		"no choice":      {},
		"an unknown one": {"until": {"someday"}},
	} {
		resp := b.post("/tasks/"+ref+"/snooze", form)
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusSeeOther || resp.StatusCode < 400 {
			t.Errorf("%s: answered %d, want a refusal", name, resp.StatusCode)
		}
	}
	if until := snoozedUntil(t, f, ref); until != nil {
		t.Fatalf("a refused snooze still snoozed the task until %v", until)
	}
}

// A snoozed task's screen names its wake time and offers Unsnooze, which
// brings it back.
func TestASnoozedTaskShowsItsWakeTimeAndUnsnoozes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "sleeping")
	resp := b.post("/tasks/"+ref+"/snooze", url.Values{"until": {"next-week"}})
	_ = resp.Body.Close()

	page := b.page("/tasks/" + ref)
	pills := between(t, page, `id="task-pills"`, `</div>`)
	if !strings.Contains(pills, `class="snoozemark"`) || !strings.Contains(pills, "Snoozed until ") {
		t.Errorf("the snoozed task's pills carry no wake time:\n%s", pills)
	}
	if !strings.Contains(pills, `title="Snoozed by alice"`) {
		t.Errorf("the badge does not say who snoozed it:\n%s", pills)
	}
	if strings.Contains(page, `id="snooze-`+ref+`"`) {
		t.Error("a snoozed task still offers the snooze menu")
	}
	form := formAt(t, page, "/tasks/"+ref+"/snooze/remove")
	if !strings.Contains(form, ">Unsnooze</button>") {
		t.Errorf("the unsnooze form has no Unsnooze button:\n%s", form)
	}

	back := b.post("/tasks/"+ref+"/snooze/remove", url.Values{"next": {"/tasks/" + ref}})
	_ = back.Body.Close()
	wantStatus(t, back, http.StatusSeeOther)
	if until := snoozedUntil(t, f, ref); until != nil {
		t.Fatalf("unsnooze left the task snoozed until %v", until)
	}
	if !strings.Contains(b.page("/tasks"), `id="t-`+ref+`"`) {
		t.Error("the unsnoozed task is not back in the listing")
	}
}

// A reader who may not write tasks is offered neither control.
func TestAReaderWithoutTaskWriteCannotSnooze(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "read only")
	viewer := f.as("viewer")
	viewer.chooseLanes("/tasks")

	page := viewer.page("/tasks/" + ref)
	if strings.Contains(page, "/snooze") || strings.Contains(page, "snooze-"+ref) || strings.Contains(page, ">Snooze<") {
		t.Errorf("a viewer's task screen offers snoozing:\n%s", page)
	}
	card := laneCard(t, viewer.page("/tasks"), ref)
	if strings.Contains(card, "/snooze") {
		t.Errorf("a viewer's lane card offers snoozing:\n%s", card)
	}
}

// The lane card's more actions menu carries the presets and the picker,
// swapping the card in place, and Unsnooze on a snoozed card.
func TestALaneCardsMenuSnoozesAndUnsnoozes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "from the lane")
	b.chooseLanes("/tasks")

	menu := moreMenu(t, laneCard(t, b.page("/tasks"), ref), ref)
	form := menuForm(t, menu, "/tasks/"+ref+"/snooze")
	if form == "" {
		t.Fatalf("the more actions menu has no snooze form:\n%s", menu)
	}
	for _, want := range []string{
		`name="until" value="later"`, `name="until" value="tomorrow"`, `name="until" value="next-week"`,
		`name="at"`, `hx-target="#t-` + ref + `"`, `hx-select-oob="#toast"`, `data-lane-control="more"`,
		`name="next" value="/tasks"`,
	} {
		if !strings.Contains(form, want) {
			t.Errorf("the lane snooze form lacks %s:\n%s", want, form)
		}
	}

	resp := b.post("/tasks/"+ref+"/snooze", url.Values{"until": {"later"}, "next": {"/tasks"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if snoozedUntil(t, f, ref) == nil {
		t.Fatal("the lane's preset did not snooze the task")
	}
	if strings.Contains(b.page("/tasks"), `data-lane-task="`+ref+`"`) {
		t.Error("the snoozed card is still in the lanes")
	}

	shown := laneCard(t, b.page("/tasks?q=snoozed:any"), ref)
	if !strings.Contains(shown, `class="snoozemark"`) {
		t.Errorf("a snoozed card shown under snoozed:any carries no badge:\n%s", shown)
	}
	snoozedMenu := moreMenu(t, shown, ref)
	if menuForm(t, snoozedMenu, "/tasks/"+ref+"/snooze") != "" {
		t.Error("a snoozed card still offers the snooze presets")
	}
	if !strings.Contains(menuForm(t, snoozedMenu, "/tasks/"+ref+"/snooze/remove"), ">Unsnooze</button>") {
		t.Errorf("a snoozed card's menu offers no Unsnooze:\n%s", snoozedMenu)
	}
}

// A row drawn under is:snoozed says until when, and an awake row says nothing.
func TestASnoozedRowCarriesItsWakeTime(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	asleep := b.createTask("infra", "asleep")
	awake := b.createTask("infra", "awake")
	resp := b.post("/tasks/"+asleep+"/snooze", url.Values{"until": {"tomorrow"}})
	_ = resp.Body.Close()

	page := b.page("/tasks?q=snoozed:any")
	row := between(t, page, `<li id="t-`+asleep+`"`, "</li>")
	if !strings.Contains(row, `class="snoozemark"`) || !strings.Contains(row, "until ") {
		t.Errorf("the snoozed row carries no wake time:\n%s", row)
	}
	if strings.Contains(between(t, page, `<li id="t-`+awake+`"`, "</li>"), "snoozemark") {
		t.Error("an awake row carries a snoozed badge")
	}
}
