// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// laneSeed is the set every grouping test draws: alice's high task tagged
// api and ops and long overdue, lena's low task tagged ops, and an unassigned
// normal task with no tag and no deadline.
type laneSeed struct {
	f                   *fixture
	b                   *browser
	lena                core.Actor
	both, opsOnly, bare string
}

func seedLanes(t *testing.T) laneSeed {
	t.Helper()
	f := newFixture(t)
	lena := seedActor(t, f.store, f.tenantA.ID, "lena", core.RoleMember)
	past := time.Date(2001, 1, 2, 0, 0, 0, 0, time.UTC)
	create := func(in core.CreateTaskInput) string {
		in.ProjectRef = "infra"
		task, err := f.svc.CreateTask(f.ctx(), in)
		if err != nil {
			t.Fatalf("creating %q: %v", in.Title, err)
		}
		return task.Ref
	}
	return laneSeed{
		f: f, b: f.as("alice"), lena: lena,
		both: create(core.CreateTaskInput{Title: "both tags", Priority: core.PriorityHigh,
			Tags: []string{"ops", "api"}, AssigneeActorID: f.actorA.ID, DueAt: &past}),
		opsOnly: create(core.CreateTaskInput{Title: "ops only", Priority: core.PriorityLow,
			Tags: []string{"ops"}, AssigneeActorID: lena.ID}),
		bare: create(core.CreateTaskInput{Title: "bare", Priority: core.PriorityNormal}),
	}
}

// lanesOf is every lane on a page and the cards in it, in drawing order.
func lanesOf(t *testing.T, page string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, key := range laneKeys(page) {
		refs := laneRefs(laneBlock(t, page, key))
		slices.Sort(refs)
		out[key] = refs
	}
	return out
}

func sorted(refs ...string) []string {
	out := append([]string(nil), refs...)
	slices.Sort(out)
	return out
}

// Each grouping draws its own lanes over the same page, every card exactly
// once, in the order the grouping defines.
func TestLanesGroupByEachGrouping(t *testing.T) {
	t.Parallel()
	s := seedLanes(t)
	cases := []struct {
		by    string
		order []string
		hold  map[string][]string
	}{
		{"project", []string{"infra"}, map[string][]string{"infra": sorted(s.both, s.opsOnly, s.bare)}},
		{"assignee", []string{"alice", "lena", core.GroupNone},
			map[string][]string{"alice": {s.both}, "lena": {s.opsOnly}, core.GroupNone: {s.bare}}},
		{"priority", []string{"high", "normal", "low"},
			map[string][]string{"high": {s.both}, "normal": {s.bare}, "low": {s.opsOnly}}},
		{"tag", []string{"api", "ops", core.GroupNone},
			map[string][]string{"api": {s.both}, "ops": {s.opsOnly}, core.GroupNone: {s.bare}}},
		{"due", []string{core.DueBucketOverdue, core.DueBucketNone},
			map[string][]string{core.DueBucketOverdue: {s.both}, core.DueBucketNone: sorted(s.opsOnly, s.bare)}},
	}
	for _, tc := range cases {
		t.Run(tc.by, func(t *testing.T) {
			page := s.b.page("/tasks?view=lanes&lanes=" + tc.by)
			if got := laneKeys(page); !slices.Equal(got, tc.order) {
				t.Fatalf("lanes = %v, want %v", got, tc.order)
			}
			for key, want := range tc.hold {
				if got := lanesOf(t, page)[key]; !slices.Equal(got, want) {
					t.Errorf("lane %s holds %v, want %v", key, got, want)
				}
			}
			if got := strings.Count(page, `data-lane-task="`+s.both+`"`); got != 1 {
				t.Errorf("%s is drawn %d times, want once", s.both, got)
			}
			note := strings.Contains(page, `class="meta lanes-note"`)
			if note != (tc.by == "tag") {
				t.Errorf("the tag rule is said = %v on lanes by %s", note, tc.by)
			}
		})
	}
}

// A person, priority or tag the filter names gets a lane even when empty,
// and a tag the filter names takes the cards that carry it.
func TestLanesNamedByTheFilterAreDrawnEmpty(t *testing.T) {
	t.Parallel()
	s := seedLanes(t)
	cases := []struct {
		by, q string
		want  map[string][]string
	}{
		{"assignee", "assignee:lena priority:highest", map[string][]string{"lena": {}}},
		{"priority", "priority:highest", map[string][]string{"highest": {}}},
		{"tag", "tag:ops", map[string][]string{"ops": sorted(s.both, s.opsOnly)}},
	}
	for _, tc := range cases {
		t.Run(tc.by, func(t *testing.T) {
			page := s.b.page("/tasks?view=lanes&lanes=" + tc.by + "&q=" + url.QueryEscape(tc.q))
			got := lanesOf(t, page)
			if len(got) != len(tc.want) {
				t.Fatalf("lanes = %v, want %v", got, tc.want)
			}
			for key, want := range tc.want {
				if !slices.Equal(got[key], want) && len(got[key])+len(want) > 0 {
					t.Errorf("lane %s holds %v, want %v", key, got[key], want)
				}
				if len(want) == 0 && !strings.Contains(laneBlock(t, page, key), "No tasks.") {
					t.Errorf("the empty lane %s does not say so", key)
				}
			}
		})
	}
}

var lanesByPattern = regexp.MustCompile(`(?s)<form method="get" action="([^"]+)" class="lanesby" data-lanesby>(.*?)</form>`)

// lanesByForm is the Group lanes by form and nothing else, or "" when the
// page has none.
func lanesByForm(page string) (action, form string) {
	m := lanesByPattern.FindStringSubmatch(page)
	if m == nil {
		return "", ""
	}
	return m[1], m[2]
}

var lanesOptionPattern = regexp.MustCompile(`<option value="([^"]+)"( selected)?>`)

// lanesByChoices is what the control offers and which one it marks.
func lanesByChoices(form string) (offered []string, current string) {
	for _, m := range lanesOptionPattern.FindAllStringSubmatch(form, -1) {
		offered = append(offered, m[1])
		if m[2] != "" {
			current = m[1]
		}
	}
	return offered, current
}

// The task list opens on project lanes and remembers the last grouping; a
// project page opens on assignee lanes, never offers Project, and neither
// reads nor writes what the task list remembered.
func TestLanesGroupingDefaultsPerScreen(t *testing.T) {
	t.Parallel()
	s := seedLanes(t)
	b := s.b

	if _, form := lanesByForm(b.page("/tasks?view=list")); form != "" {
		t.Error("the list carries the Group lanes by control")
	}
	page := b.page("/tasks?view=lanes")
	if got := laneKeys(page); !slices.Equal(got, []string{"infra"}) {
		t.Errorf("the task list's lanes = %v, want the project", got)
	}
	_, form := lanesByForm(page)
	if offered, cur := lanesByChoices(form); !slices.Equal(offered, core.LaneGroupings) || cur != core.LanesByProject {
		t.Errorf("the task list offers %v with %q chosen", offered, cur)
	}

	project := b.page("/projects/infra?view=lanes")
	if got := laneKeys(project); !slices.Equal(got, []string{"alice", "lena", core.GroupNone}) {
		t.Errorf("the project page's lanes = %v, want one per person", got)
	}
	action, form := lanesByForm(project)
	offered, cur := lanesByChoices(form)
	if action != "/projects/infra" || slices.Contains(offered, core.LanesByProject) || cur != core.LanesByAssignee {
		t.Errorf("the project page's control posts to %q and offers %v with %q chosen", action, offered, cur)
	}
	if got := laneKeys(b.page("/projects/infra?view=lanes&lanes=project")); slices.Contains(got, "infra") {
		t.Errorf("lanes=project drew a project lane on its own page: %v", got)
	}

	b.page("/projects/infra?view=lanes&lanes=tag")
	if got := b.cookie(web.LanesGroupCookie); got != "" {
		t.Errorf("a project page remembered %q", got)
	}
	b.page("/tasks?view=lanes&lanes=priority")
	if got := b.cookie(web.LanesGroupCookie); got != core.LanesByPriority {
		t.Fatalf("the task list remembered %q, want priority", got)
	}
	if got := laneKeys(b.page("/tasks?view=lanes")); !slices.Equal(got, []string{"high", "normal", "low"}) {
		t.Errorf("the remembered grouping was not drawn: %v", got)
	}
	if got := laneKeys(b.page("/projects/infra?view=lanes")); !slices.Equal(got, []string{"alice", "lena", core.GroupNone}) {
		t.Errorf("the task list's remembered grouping reached a project page: %v", got)
	}
	b.page("/tasks?view=lanes&lanes=project")
	if got := b.cookie(web.LanesGroupCookie); got != "" {
		t.Errorf("choosing the default left %q behind", got)
	}
}

var switchHrefPattern = regexp.MustCompile(`class="segment[^"]*" href="([^"]+)"`)

// The grouping is part of the address: every way the screen writes back to
// itself keeps it, and the filter form carries it like the view.
func TestTheLanesGroupingStaysInTheAddress(t *testing.T) {
	t.Parallel()
	s := seedLanes(t)
	for _, route := range []string{"/tasks", "/projects/infra"} {
		page := s.b.page(route + "?view=lanes&lanes=priority&limit=2&q=" + url.QueryEscape("-tag:nope"))
		links := switchHrefPattern.FindAllStringSubmatch(viewSwitch(t, page), -1)
		if len(links) != len(web.TaskViews) {
			t.Errorf("%s: read %d view switch links, want %d", route, len(links), len(web.TaskViews))
		}
		for _, m := range links {
			if !strings.Contains(html.UnescapeString(m[1]), "lanes=priority") {
				t.Errorf("%s: the view switch link %s drops the grouping", route, m[1])
			}
		}
		filter := regexp.MustCompile(`(?s)class="filterbar">(.*?)</form>`).FindString(page)
		if !strings.Contains(filter, `<input type="hidden" name="lanes" value="priority">`) {
			t.Errorf("%s: the filter form does not carry the grouping:\n%s", route, filter)
		}
		clear := regexp.MustCompile(`<a class="secondary clear" href="([^"]+)"`).FindStringSubmatch(page)
		if clear == nil || !strings.Contains(html.UnescapeString(clear[1]), "lanes=priority") {
			t.Errorf("%s: Clear drops the grouping: %v", route, clear)
		}
		next := regexp.MustCompile(`href="([^"]+)" rel="next"`).FindStringSubmatch(page)
		if next == nil || !strings.Contains(html.UnescapeString(next[1]), "lanes=priority") {
			t.Errorf("%s: the pager drops the grouping: %v", route, next)
		}
		live := regexp.MustCompile(`data-live="([^"]+)"`).FindStringSubmatch(page)
		if live == nil || !strings.Contains(html.UnescapeString(live[1]), "lanes=priority") {
			t.Errorf("%s: the live refresh drops the grouping: %v", route, live)
		}
		if got := laneKeys(s.b.page(html.UnescapeString(live[1]))); len(got) == 0 || got[0] != "high" {
			t.Errorf("%s: the live fragment drew %v, not lanes by priority", route, got)
		}
	}
}

var hiddenFieldPattern = regexp.MustCompile(`<input type="hidden" name="([^"]+)" value="([^"]*)">`)

// Without scripting the control is a GET form: submitting it with a choice
// draws that grouping and keeps the filter the screen had.
func TestTheLanesGroupingWorksWithoutScripting(t *testing.T) {
	t.Parallel()
	s := seedLanes(t)
	page := s.b.page("/projects/infra?view=lanes&q=" + url.QueryEscape("tag:ops"))
	action, form := lanesByForm(page)
	if action == "" || !strings.Contains(form, `<select id="lanesby-pick" name="lanes">`) ||
		!strings.Contains(form, `<button type="submit"`) {
		t.Fatalf("no plain form with a select and a submit button:\n%s", form)
	}
	q := url.Values{}
	for _, m := range hiddenFieldPattern.FindAllStringSubmatch(form, -1) {
		q.Add(html.UnescapeString(m[1]), html.UnescapeString(m[2]))
	}
	if q.Get("q") != "tag:ops" || q.Get("view") != web.TaskViewLanes {
		t.Fatalf("the form keeps %v, want the filter and the lanes view", q)
	}
	q.Set(web.LanesParam, core.LanesByPriority)
	got := lanesOf(t, s.b.page(action+"?"+q.Encode()))
	want := map[string][]string{"high": {s.both}, "low": {s.opsOnly}}
	if len(got) != len(want) || !slices.Equal(got["high"], want["high"]) || !slices.Equal(got["low"], want["low"]) {
		t.Errorf("submitting the form drew %v, want %v", got, want)
	}
}

// Each grouping keeps its own arrangement: hiding a person's lane hides
// nothing among the projects, and a starred tag lane takes the cards that
// carry its tag.
func TestLanesAreArrangedPerGrouping(t *testing.T) {
	t.Parallel()
	s := seedLanes(t)
	b := s.b
	page := b.page("/tasks?view=lanes&lanes=assignee")
	if menu := laneBlock(t, page, "lena"); !strings.Contains(menu, `name="view" value="lanes:assignee"`) ||
		strings.Contains(menu, `name="view" value="lanes"`) {
		t.Fatalf("a person's lane menu does not post its own view:\n%s", menu)
	}
	set := func(view, group, what string) {
		t.Helper()
		resp := b.post("/view/group", url.Values{"view": {view}, "group": {group}, "set": {what}, "value": {"1"}, "next": {"/tasks"}})
		defer func() { _ = resp.Body.Close() }()
		wantStatus(t, resp, http.StatusSeeOther)
	}
	set("lanes:assignee", "lena", "hidden")
	if got := laneKeys(b.page("/tasks?view=lanes&lanes=assignee")); slices.Contains(got, "lena") {
		t.Errorf("lena's hidden lane is drawn: %v", got)
	}
	if got := laneKeys(b.page("/projects/infra?view=lanes")); slices.Contains(got, "lena") {
		t.Errorf("lena's lane is hidden on the task list only: %v", got)
	}
	if got := laneKeys(b.page("/tasks?view=lanes&lanes=project")); !slices.Equal(got, []string{"infra"}) {
		t.Errorf("hiding a person changed the project lanes: %v", got)
	}

	set("lanes:tag", "ops", "starred")
	got := lanesOf(t, b.page("/tasks?view=lanes&lanes=tag"))
	if !slices.Equal(got["ops"], sorted(s.both, s.opsOnly)) {
		t.Errorf("the starred ops lane holds %v, want both tasks tagged ops", got["ops"])
	}
	if _, ok := got["api"]; ok {
		t.Errorf("the api lane is drawn with nothing in it: %v", got)
	}
}

// A lane heading's name is cut, never split: whatever the grouping, the
// name sits in .lane-name, and that rule clips a word too long for the lane
// with an ellipsis rather than letting it wrap mid-word, which is what the
// screenshot check reports as a word split.
func TestALaneNameIsCutNeverSplit(t *testing.T) {
	t.Parallel()
	s := seedLanes(t)
	for _, by := range core.LaneGroupings {
		page := s.b.page("/tasks?view=lanes&lanes=" + by)
		for _, key := range laneKeys(page) {
			block := laneBlock(t, page, key)
			head := block[:strings.Index(block, "</h2>")]
			if !regexp.MustCompile(`class="lane-name"[^>]*title="[^"]+"`).MatchString(head) {
				t.Errorf("lanes by %s: the %s heading has no .lane-name with a title:\n%s", by, key, head)
			}
		}
	}
	resp := s.b.get("/assets/app.css")
	sheet := body(t, resp)
	var rule string
	for _, r := range cssRules(sheet) {
		if r.media == "" && strings.TrimSpace(r.selector) == ".lane > .grouphead > h2 > .lane-name" {
			rule = r.body
		}
	}
	if rule == "" {
		t.Fatal("no rule for the lane name")
	}
	for _, want := range [][2]string{{"overflow", "hidden"}, {"text-overflow", "ellipsis"}, {"min-width", "0"},
		{"overflow-wrap", "normal"}, {"word-break", "normal"}, {"-webkit-line-clamp", "1"}} {
		if !declared(rule, want[0], want[1]) {
			t.Errorf("the lane name does not declare %s: %s", want[0], want[1])
		}
	}
	if strings.Contains(rule, "anywhere") || strings.Contains(rule, "break-all") {
		t.Errorf("the lane name may still break inside a word: %s", rule)
	}
}
