// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

type paletteTask struct {
	Ref    string `json:"ref"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Href   string `json:"href"`
}

// paletteSearch runs one palette task search and decodes its answer.
func paletteSearch(t *testing.T, b *browser, q string) []paletteTask {
	t.Helper()
	resp := b.get(web.RoutePaletteTasks + "?q=" + url.QueryEscape(q))
	raw := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET palette tasks %q = %d: %s", q, resp.StatusCode, raw)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("palette tasks answered %q, want JSON", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("palette tasks Cache-Control = %q, want no-store", cc)
	}
	var out []paletteTask
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("palette tasks %q is not a JSON list: %v\n%s", q, err, raw)
	}
	return out
}

func refsOf(tasks []paletteTask) []string {
	out := make([]string, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, task.Ref)
	}
	return out
}

func TestPaletteFindsATaskByTitleAndByRef(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	deploy := b.createTask("infra", "Deploy the billing api")
	other := b.createTask("infra", "Rotate the certificates")

	got := paletteSearch(t, b, "billing")
	if len(got) != 1 || got[0].Ref != deploy {
		t.Fatalf("search billing = %v, want only %s", refsOf(got), deploy)
	}
	if got[0].Title != "Deploy the billing api" || got[0].Href != "/tasks/"+deploy || got[0].Status == "" {
		t.Errorf("result = %+v, want its title, status and address", got[0])
	}

	byRef := paletteSearch(t, b, strings.ToUpper(other))
	if len(byRef) == 0 || byRef[0].Ref != other {
		t.Fatalf("search %s = %v, want %s first", strings.ToUpper(other), refsOf(byRef), other)
	}
}

func TestPaletteSearchesTheFilterLanguage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	starred := b.createTask("infra", "Patch alpha")
	b.createTask("infra", "Patch beta")
	resp := b.post("/tasks/"+starred+"/star", url.Values{"next": {"/tasks"}})
	_ = resp.Body.Close()

	got := paletteSearch(t, b, "is:starred patch")
	if len(got) != 1 || got[0].Ref != starred {
		t.Fatalf("search is:starred patch = %v, want only %s", refsOf(got), starred)
	}
	if half := paletteSearch(t, b, "status:"); half == nil {
		t.Fatal("a half-typed term was refused instead of searched as words")
	}
}

func TestPaletteAnswersNothingForNothingTyped(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "Anything at all")
	for _, q := range []string{"", "   "} {
		if got := paletteSearch(t, b, q); len(got) != 0 {
			t.Errorf("search %q = %v, want nothing", q, refsOf(got))
		}
	}
}

func TestPaletteSearchStopsAtItsLimit(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for i := range web.PaletteTaskLimit + 3 {
		b.createTask("infra", fmt.Sprintf("Bulk chore %d", i))
	}
	if got := paletteSearch(t, b, "bulk"); len(got) != web.PaletteTaskLimit {
		t.Fatalf("search bulk answered %d tasks, want %d", len(got), web.PaletteTaskLimit)
	}
}

// A ref lookup and a page of matches together could come to one more than
// the limit; the answer still stops at it.
func TestPaletteSearchStopsAtItsLimitWithARefFirst(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	first := b.createTask("infra", "Kick-off")
	for i := range web.PaletteTaskLimit + 1 {
		b.createTask("infra", fmt.Sprintf("%s follow-up %d", first, i))
	}
	got := paletteSearch(t, b, first)
	if len(got) != web.PaletteTaskLimit {
		t.Fatalf("search %s answered %d tasks, want %d", first, len(got), web.PaletteTaskLimit)
	}
	if got[0].Ref != first {
		t.Errorf("search %s = %v, want the task it names first", first, refsOf(got))
	}
}

func TestPaletteLeavesOutADeletedTask(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	gone := b.createTask("infra", "Retired widget")
	resp := b.post("/tasks/"+gone+"/delete", url.Values{})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	for _, q := range []string{"widget", gone} {
		if got := paletteSearch(t, b, q); len(got) != 0 {
			t.Errorf("search %q = %v, want the deleted task left out", q, refsOf(got))
		}
	}
}

func TestPaletteStaysInTheSessionTenant(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ctx := core.WithTenant(context.Background(), core.TenantScope{TenantID: f.tenantB.ID})
	bob := f.actorFor("bob")
	ctx = core.WithActor(ctx, bob)
	theirs, err := f.svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "other", Title: "Secret merger plan"})
	if err != nil {
		t.Fatalf("creating the other tenant's task: %v", err)
	}

	b := f.as("alice")
	for _, q := range []string{"merger", theirs.Ref} {
		if got := paletteSearch(t, b, q); len(got) != 0 {
			t.Errorf("search %q = %v, which is another tenant's", q, refsOf(got))
		}
	}
	projects := body(t, b.get(web.RoutePaletteProjects))
	if !strings.Contains(projects, `"key":"infra"`) || strings.Contains(projects, `"key":"other"`) {
		t.Errorf("palette projects = %s, want infra and not the other tenant's", projects)
	}
}

func TestPaletteNeedsASignedInReader(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, path := range []string{web.RoutePaletteTasks + "?q=x", web.RoutePaletteProjects, web.RoutePaletteTemplates} {
		resp := f.as("").get(path)
		raw := body(t, resp)
		if resp.StatusCode == http.StatusOK || strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
			t.Errorf("%s answered a signed-out browser with %d %q: %s", path, resp.StatusCode, resp.Header.Get("Content-Type"), raw)
		}
	}
}

func TestPaletteProjectsNameEachProjectAndItsBoard(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	created := b.post("/projects", url.Values{"key": {"attic"}, "name": {"Attic"}})
	_ = created.Body.Close()
	archived := b.post("/projects/attic/archive", url.Values{})
	_ = archived.Body.Close()
	wantStatus(t, archived, http.StatusSeeOther)
	resp := b.get(web.RoutePaletteProjects)
	var got []struct{ Key, Name, Href string }
	if err := json.Unmarshal([]byte(body(t, resp)), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Key != "infra" || got[0].Href != "/projects/infra" || got[0].Name == "" {
		t.Fatalf("palette projects = %+v, want infra with its name and board and no archived project", got)
	}
}

// paletteBlock is the palette dialog alone, so an assertion about it cannot
// be met by the same word elsewhere on the page.
func paletteBlock(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, `<dialog id="palette"`)
	if start < 0 {
		t.Fatalf("the page carries no palette dialog:\n%s", page)
	}
	end := strings.Index(page[start:], "</dialog>")
	if end < 0 {
		t.Fatal("the palette dialog is never closed")
	}
	return page[start : start+end]
}

func TestEveryScreenCarriesThePaletteAndItsOpener(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, path := range []string{"/tasks", "/projects", "/activity", "/settings"} {
		page := b.page(path)
		dlg := paletteBlock(t, page)
		for _, want := range []string{`role="combobox"`, `aria-controls="palette-list"`, `role="listbox"`, `data-palette-close`} {
			if !strings.Contains(dlg, want) {
				t.Errorf("%s: the palette dialog lacks %s:\n%s", path, want, dlg)
			}
		}
		for _, want := range []string{`<script id="tix-palette-data" type="application/json">`,
			`<script src="/assets/palette.js" defer></script>`, `data-palette-open aria-keyshortcuts="Control+K Meta+K"`} {
			if !strings.Contains(page, want) {
				t.Errorf("%s lacks %s", path, want)
			}
		}
	}
	if page := f.as("").page("/login"); strings.Contains(page, `id="palette"`) || strings.Contains(page, "tix-palette-data") {
		t.Error("the sign-in screen carries the palette")
	}
}

func TestThePaletteIsInTheReadersLanguage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.lang = "bg"
	page := b.page("/tasks")
	dlg := paletteBlock(t, page)
	if !strings.Contains(dlg, "Търси задачи, проекти, екрани и действия") {
		t.Errorf("the palette dialog is not in Bulgarian:\n%s", dlg)
	}
	if !strings.Contains(page, `"label":"Покажи задачите като табло"`) {
		t.Error("the palette's actions are not in Bulgarian")
	}
}

func TestThePaletteShortcutIsBoundInEveryScheme(t *testing.T) {
	t.Parallel()
	for _, scheme := range web.KeySchemes() {
		keys := web.KeyBindingsFor(scheme)["palette"]
		if !equalKeys(keys, []string{"ctrl+k", "meta+k"}) {
			t.Errorf("scheme %q binds the palette to %v, want ctrl+k and meta+k", scheme, keys)
		}
	}
}

// paletteEmbedded is the palette data a page embeds.
type paletteEmbedded struct {
	Themes []struct{ ID, Label, Value string }
	Task   []struct {
		ID, Label, Terms, Target string
		Do                       string `json:"do"`
	}
}

func paletteDataOf(t *testing.T, page string) paletteEmbedded {
	t.Helper()
	raw := between(t, page, `<script id="tix-palette-data" type="application/json">`, "</script>")
	var out paletteEmbedded
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("the embedded palette data is not JSON: %v\n%s", err, raw)
	}
	return out
}

var (
	startTag   = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9-]*)((?:\s+[^\s=>/]+(?:="[^"]*")?)*)\s*/?>`)
	tagAttr    = regexp.MustCompile(`([^\s=>/]+)(?:="([^"]*)")?`)
	simplePart = regexp.MustCompile(`^([a-z]*)((?:#[\w-]+)?)((?:\.[\w-]+)*)((?:\[[\w-]+="[^"]*"\])*)$`)
	attrPart   = regexp.MustCompile(`\[([\w-]+)="([^"]*)"\]`)
)

// matchesOne reports whether one element of the markup is what a compound
// selector (tag, #id, .class and [attr="value"], no combinators) names. It
// reads each start tag on its own, so the parts must all hold on one element.
func matchesOne(t *testing.T, markup, selector string) bool {
	t.Helper()
	m := simplePart.FindStringSubmatch(selector)
	if m == nil {
		t.Fatalf("selector %q is not one compound selector", selector)
	}
	for _, tag := range startTag.FindAllStringSubmatch(markup, -1) {
		attrs := map[string]string{}
		for _, a := range tagAttr.FindAllStringSubmatch(tag[2], -1) {
			attrs[a[1]] = html.UnescapeString(a[2])
		}
		classes := " " + attrs["class"] + " "
		ok := m[1] == "" || strings.EqualFold(tag[1], m[1])
		ok = ok && (m[2] == "" || attrs["id"] == m[2][1:])
		for _, c := range strings.Split(m[3], ".")[1:] {
			ok = ok && strings.Contains(classes, " "+c+" ")
		}
		for _, a := range attrPart.FindAllStringSubmatch(m[4], -1) {
			v, has := attrs[a[1]]
			ok = ok && has && v == a[2]
		}
		if ok {
			return true
		}
	}
	return false
}

// offeredOn is the task actions assets/palette.js would offer on a task
// screen: those whose filled-in target names an element drawn there.
func offeredOn(t *testing.T, data paletteEmbedded, markup, prefix, ref string) []string {
	t.Helper()
	var out []string
	for _, e := range data.Task {
		target := strings.NewReplacer("{p}", prefix, "{ref}", ref).Replace(e.Target)
		if matchesOne(t, markup, target) {
			out = append(out, e.ID)
		}
	}
	return out
}

// Each task action drives a control the task screen draws, on the task's own
// page and in the task dialog, where every id carries the dlg- prefix the
// palette fills in.
func TestPaletteTaskActionsDriveTheTaskScreensOwnControls(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "Patch the kernel")
	want := []string{"task-move", "task-priority", "task-assign", "task-due", "task-tag", "task-comment",
		"task-star", "task-snooze", "task-copy-ref", "task-handoff", "task-rename", "task-describe",
		"task-copy-to", "task-move-to", "task-save-template"}

	page := b.page("/tasks/" + ref)
	data := paletteDataOf(t, page)
	if len(data.Task) == 0 {
		t.Fatal("the page embeds no task actions")
	}
	if got := offeredOn(t, data, page, "", ref); !slices.Equal(got, want) {
		t.Errorf("on the task's page the palette offers %v, want %v", got, want)
	}
	if !matchesOne(t, page, `header.taskhead[data-recent-task="`+ref+`"]`) {
		t.Error("the task screen's header does not name its task, which is how the palette knows one is on screen")
	}

	resp := b.getDialog("/tasks/" + ref)
	dialog := body(t, resp)
	if got := offeredOn(t, data, dialog, "dlg-", ref); !slices.Equal(got, want) {
		t.Errorf("in the task dialog the palette offers %v, want %v", got, want)
	}

	starred := b.post("/tasks/"+ref+"/star", url.Values{"next": {"/tasks/" + ref}})
	_ = starred.Body.Close()
	got := offeredOn(t, data, b.page("/tasks/"+ref), "", ref)
	if !slices.Contains(got, "task-unstar") || slices.Contains(got, "task-star") {
		t.Errorf("on a starred task the palette offers %v, want Unstar and not Star", got)
	}
}

// A reader who may not change the task is offered nothing that would, and
// a screen without a task offers no task action at all.
func TestPaletteTaskActionsFollowWhatTheReaderMayDo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "Read only")
	viewer := f.as("viewer")
	page := viewer.page("/tasks/" + ref)
	got := offeredOn(t, paletteDataOf(t, page), page, "", ref)
	for _, id := range got {
		if id != "task-copy-ref" && id != "task-handoff" && id != "task-star" {
			t.Errorf("a viewer is offered %s", id)
		}
	}
	if !slices.Contains(got, "task-copy-ref") {
		t.Errorf("a viewer is offered %v, want Copy reference at least", got)
	}
	if listing := viewer.page("/tasks"); matchesOne(t, listing, "header.taskhead") {
		t.Error("the task listing draws a task screen header, so the palette would offer task actions there")
	}
}

// Every colour scheme the sidebar switch cycles is offered by name, in its
// order, and the switch's form carries the field the palette fills in.
func TestPaletteOffersEachColourScheme(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/tasks")
	data := paletteDataOf(t, page)
	var got []string
	for _, th := range data.Themes {
		got = append(got, th.Value+"="+th.Label)
	}
	want := []string{"=System", "light=Light", "dark=Dark", "oled=OLED", "dim=Low contrast", "system-oled=System, OLED when dark"}
	if !slices.Equal(got, want) {
		t.Errorf("colour schemes = %v, want %v", got, want)
	}
	themer := between(t, page, `<form method="post" action="/theme"`, "</form>")
	if !matchesOne(t, page, "form.themer") || !matchesOne(t, themer, `input[name="theme"]`) {
		t.Errorf("the sidebar switch lacks the form or field the palette sends:\n%s", themer)
	}
}

func TestPaletteTemplatesNameEachTemplateAndItsShape(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "Weekly report")
	for _, form := range []url.Values{
		{"name": {"Report"}},
		{"name": {"Everywhere"}, "scope": {"tenant"}},
	} {
		resp := b.post("/tasks/"+ref+"/template", form)
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
	}
	resp := b.get(web.RoutePaletteTemplates)
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("palette templates Cache-Control = %q, want no-store", cc)
	}
	var got []struct {
		ID, Name, Title, Project string
		TenantWide               bool `json:"tenant_wide"`
	}
	if err := json.Unmarshal([]byte(body(t, resp)), &got); err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for i, tpl := range got {
		byName[tpl.Name] = i
	}
	report, everywhere := got[byName["Report"]], got[byName["Everywhere"]]
	if len(got) != 2 || report.ID == "" || report.Title != "Weekly report" || report.Project != "infra" || report.TenantWide {
		t.Fatalf("palette templates = %+v, want Report in infra titled as the task", got)
	}
	if !everywhere.TenantWide || everywhere.Project != "" {
		t.Errorf("Everywhere = %+v, want tenant-wide", everywhere)
	}

	use := b.post(web.RouteTemplateNew, url.Values{"template": {everywhere.ID}, "project_ref": {"infra"}, "next": {"/tasks"}})
	_ = use.Body.Close()
	wantStatus(t, use, http.StatusSeeOther)
	if loc := use.Header.Get("Location"); !strings.HasPrefix(loc, "/tasks?") || !strings.Contains(loc, "flashref=infra-") {
		t.Errorf("using a template the way the palette does went to %q, want back to the listing with the new task", loc)
	}
}

// The palette offers a switch to each tenant the header's switcher has a
// Switch button for, by reading that dialog: so the dialog carries a form
// naming the tenant for each one this host serves, and none for a tenant
// served elsewhere.
func TestPaletteTenantsAreTheSwitchersOwnForms(t *testing.T) {
	w := newSwitchWorld(t)
	pat := w.visitor()
	pat.signIn("pat@example.test")
	_, page := pat.do(http.MethodGet, web.RouteProjects, nil)
	dialog := between(t, page, `<dialog id="tenants"`, "</dialog>")
	test := switcherRowOf(t, page, "test")
	if !matchesOne(t, test, "form.tenantswitch-form") || !matchesOne(t, test, `input[name="to"][value="test"]`) ||
		!matchesOne(t, test, "span.tenantswitch-name") {
		t.Errorf("test's row = %s, want the Switch form and name the palette reads", test)
	}
	for _, key := range []string{"home", "away"} {
		if matchesOne(t, dialog, `input[name="to"][value="`+key+`"]`) {
			t.Errorf("the switcher has a Switch form for %s, which the palette would offer", key)
		}
	}
}
