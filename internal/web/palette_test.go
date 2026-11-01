// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
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

func TestPaletteLeavesOutADeletedTask(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	gone := b.createTask("infra", "Retired widget")
	resp := b.post("/tasks/"+gone+"/delete", url.Values{})
	_ = resp.Body.Close()
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
	for _, path := range []string{web.RoutePaletteTasks + "?q=x", web.RoutePaletteProjects} {
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
	resp := f.as("alice").get(web.RoutePaletteProjects)
	var got []struct{ Key, Name, Href string }
	if err := json.Unmarshal([]byte(body(t, resp)), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Key != "infra" || got[0].Href != "/projects/infra" || got[0].Name == "" {
		t.Fatalf("palette projects = %+v, want infra with its name and board", got)
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
