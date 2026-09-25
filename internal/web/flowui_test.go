// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// TestTheStatusMenuIsNotClippedByTheListItSitsIn pins the fix for a menu
// nobody could read.
//
// The panel used to be a position: absolute child of the row. ul.tasklist
// clips to its rounded corners with overflow: hidden and body clips the
// horizontal axis, and no positioned element escapes a clipping ancestor, so
// the menu was cut off at the row's edge however it was styled. It is a
// popover now, painted in the top layer, which is outside every clipping
// ancestor; popovertarget opens it from plain HTML, so it needs no script,
// and the auto state dismisses on Escape and on a click outside and closes
// any other one that is open.
func TestTheStatusMenuIsNotClippedByTheListItSitsIn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "clipping check")
	page := b.page("/tasks")

	for _, want := range []string{
		`id="flow-` + ref + `"`,
		`class="flowmenu" popover`,
		`popovertarget="flow-` + ref + `"`,
		`popovertargetaction="hide"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the task list is missing %q, so the menu is not a dismissable popover", want)
		}
	}
	// The clipping ancestor is still there. That is the whole reason the
	// panel may not be positioned inside it.
	sheet := body(t, f.as("alice").get("/assets/app.css"))
	list := ruleBlock(t, sheet, "ul.tasklist {")
	if !strings.Contains(list, "overflow: hidden") {
		t.Fatal("ul.tasklist no longer clips, so this guard is measuring nothing")
	}
	menu := ruleBlock(t, sheet, ".flowmenu {")
	if strings.Contains(menu, "position: absolute") {
		t.Error("the menu is absolutely positioned again, which puts it back inside\n" +
			"ul.tasklist's overflow: hidden and clips it at the row's edge")
	}
	if strings.Contains(page, `<details class="statusmenu"`) {
		t.Error("the menu is a disclosure again; a disclosure's panel cannot leave the row")
	}
}

// TestAMultiHopMoveWritesOneAuditEntryPerHop pins that a route through other
// states is applied as the transitions it says it is, not as a jump.
func TestAMultiHopMoveWritesOneAuditEntryPerHop(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	tests := []struct {
		name  string
		route string
		want  []string
	}{
		{name: "one hop", route: "doing", want: []string{"todo>doing"}},
		{name: "through one state", route: "doing>done", want: []string{"todo>doing", "doing>done"}},
		{name: "through two states", route: "doing>done>todo",
			want: []string{"todo>doing", "doing>done", "done>todo"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ref := b.createTask("infra", "route "+tc.name)
			resp := b.post("/tasks/"+ref+"/transition",
				url.Values{"route": {tc.route}, "next": {"/tasks"}})
			defer func() { _ = resp.Body.Close() }()
			wantStatus(t, resp, http.StatusSeeOther)

			got := transitionHops(t, f, ref)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Fatalf("route %q wrote %v, want one entry per hop: %v", tc.route, got, tc.want)
			}
		})
	}
}

// TestAMultiHopMoveThatStopsPartWaySaysSo pins that a route broken in the
// middle reports where it got to, rather than reporting the whole move.
func TestAMultiHopMoveThatStopsPartWaySaysSo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "broken route")

	// done cannot be left for anything but todo, so the third hop is refused
	// by the workflow after the first two have already been written.
	resp := b.post("/tasks/"+ref+"/transition",
		url.Values{"route": {"doing>done>blocked"}, "next": {"/tasks"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "then+stopped") {
		t.Errorf("a route that stopped part way reported %q, which does not say it stopped", location)
	}
	if got := transitionHops(t, f, ref); strings.Join(got, ",") != "todo>doing,doing>done" {
		t.Errorf("hops written = %v, want the two that succeeded", got)
	}
}

// transitionHops lists a task's state changes, oldest first, as "from>to".
func transitionHops(t *testing.T, f *fixture, ref string) []string {
	t.Helper()
	parsed, err := core.ParseTaskRef(ref)
	if err != nil {
		t.Fatalf("parsing %q: %v", ref, err)
	}
	task, err := f.svc.GetTask(f.ctx(), parsed)
	if err != nil {
		t.Fatalf("loading %q: %v", ref, err)
	}
	entries, _, err := f.svc.ListAudit(f.ctx(), core.AuditFilter{
		SubjectType: "task", SubjectID: task.ID, Actions: []string{"task.transition"}})
	if err != nil {
		t.Fatalf("listing the trail: %v", err)
	}
	// Sorted rather than trusted: the trail's own order is the listing's
	// concern, and this assertion is about the sequence of writes.
	sort.Slice(entries, func(i, j int) bool { return entries[i].Seq < entries[j].Seq })
	var out []string
	for _, e := range entries {
		out = append(out, statusOf(t, e.Before)+">"+statusOf(t, e.After))
	}
	return out
}

// statusOf reads the status out of an audit entry's image of a task.
func statusOf(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	if len(raw) == 0 {
		return ""
	}
	var image struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &image); err != nil {
		t.Fatalf("reading an audit image: %v", err)
	}
	return image.Status
}

// ctx is a request context speaking for the fixture's first actor, so a test
// can read the trail the browser just wrote.
func (f *fixture) ctx() context.Context {
	ctx := core.WithTenant(context.Background(), core.TenantScope{TenantID: f.tenantA.ID})
	return core.WithActor(ctx, copyActor(f.actorA, []core.Scope{core.ScopeAll}, core.RoleAdmin))
}

// ruleBlock returns the declarations of the rule opened by the given selector.
func ruleBlock(t *testing.T, sheet, selector string) string {
	t.Helper()
	at := strings.Index(sheet, selector)
	if at < 0 {
		t.Fatalf("the stylesheet has no %q rule", selector)
	}
	rest := sheet[at:]
	end := strings.Index(rest, "}")
	if end < 0 {
		t.Fatalf("the %q rule is never closed", selector)
	}
	return rest[:end]
}
