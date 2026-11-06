// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	stdsync "sync"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
	"github.com/heliopsy/tix/internal/sync/openproject"
)

// openProjectFixture is a tenant with two people, an OpenProject source using
// the shipped default mapping, and a server replaying a recorded collection.
type openProjectFixture struct {
	local  *Local
	ctx    context.Context
	source *core.SyncSource
	jane   string
	john   string

	mu    stdsync.Mutex
	pages map[string]string
}

func newOpenProjectFixture(t *testing.T, unknown string) *openProjectFixture {
	t.Helper()
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	f := &openProjectFixture{local: l, ctx: ctx, pages: map[string]string{}}

	if _, err := l.PutWorkflow(ctx, core.WorkflowInput{Key: "stock", Name: "Stock", Definition: BuiltinWorkflow()}); err != nil {
		t.Fatalf("creating workflow: %v", err)
	}
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "ops", Name: "Ops", WorkflowKey: "stock"}); err != nil {
		t.Fatalf("creating project: %v", err)
	}
	jane, err := l.CreateUser(ctx, core.CreateUserInput{Email: "jane@corp.test", Handle: "jane", DisplayName: "Jane D."})
	if err != nil {
		t.Fatalf("creating jane: %v", err)
	}
	john, err := l.CreateUser(ctx, core.CreateUserInput{Email: "john@corp.test", Handle: "john", DisplayName: "Johnny"})
	if err != nil {
		t.Fatalf("creating john: %v", err)
	}
	f.jane, f.john = jane.ID, john.ID

	for _, page := range []string{"1", "2"} {
		raw, err := os.ReadFile(filepath.Join("..", "sync", "openproject", "testdata", "work_packages_"+page+".json"))
		if err != nil {
			t.Fatalf("reading fixture: %v", err)
		}
		f.pages[page] = string(raw)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		body, ok := f.pages[r.URL.Query().Get("offset")]
		f.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	mapping := string(openproject.DefaultMapping())
	mapping = strings.Replace(mapping, "project: CHANGE-ME", "project: ops", 1)
	mapping = strings.Replace(mapping, "  # map:\n  #   OpenProject Name: tix-handle-or-email\n",
		"  map:\n    J. Smith: john\n", 1)
	mapping = strings.Replace(mapping, "unknown: unassigned", "unknown: "+unknown, 1)
	path := filepath.Join(t.TempDir(), "openproject.yaml")
	if err := os.WriteFile(path, []byte(mapping), 0o600); err != nil {
		t.Fatalf("writing mapping: %v", err)
	}
	t.Setenv(extsync.EnvName("op", extsync.EnvMapping), path)
	t.Setenv(extsync.EnvName("op", extsync.EnvURL), srv.URL)
	t.Setenv(extsync.EnvName("op", extsync.EnvPageSize), "2")

	src, err := l.PutSyncSource(ctx, core.SyncSourceInput{System: core.SystemOpenProject, Name: "op"})
	if err != nil {
		t.Fatalf("registering source: %v", err)
	}
	f.source = src
	return f
}

func (f *openProjectFixture) run(t *testing.T, dryRun bool) *core.SyncResult {
	t.Helper()
	res, err := f.local.RunSync(f.ctx, core.RunSyncInput{SourceID: f.source.ID, DryRun: dryRun, Full: true})
	if err != nil {
		t.Fatalf("RunSync: %v", err)
	}
	return res
}

// byTitle returns the imported tasks keyed by title.
func (f *openProjectFixture) byTitle(t *testing.T) map[string]core.Task {
	t.Helper()
	page, err := f.local.ListTasks(f.ctx, core.TaskFilter{Page: core.Page{Limit: 100}})
	if err != nil {
		t.Fatalf("listing tasks: %v", err)
	}
	out := map[string]core.Task{}
	for _, task := range page.Tasks {
		out[task.Title] = task
	}
	return out
}

func (f *openProjectFixture) actors(t *testing.T) map[string]core.Actor {
	t.Helper()
	list, _, err := f.local.ListActors(f.ctx, core.Page{Limit: core.MaxPageLimit})
	if err != nil {
		t.Fatalf("listing actors: %v", err)
	}
	out := map[string]core.Actor{}
	for _, a := range list {
		out[a.Handle] = a
	}
	return out
}

// bump makes the source report a later change to one work package.
func (f *openProjectFixture) bump(page, from, to string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pages[page] = strings.Replace(f.pages[page], from, to, 1)
}

func TestRunSyncOpenProjectMatchesAssignees(t *testing.T) {
	f := newOpenProjectFixture(t, "unassigned")
	res := f.run(t, false)
	if res.Created["task"] != 4 {
		t.Fatalf("created = %v, want all four work packages; warnings %v", res.Created, res.Warnings)
	}
	tasks := f.byTitle(t)

	tests := []struct {
		title, assignee, status string
	}{
		{"Set up CI", f.jane, "todo"},
		{"Write the operator guide", f.john, "doing"},
		{"Fix the sign-in loop", "", "done"},
		{"Vendor lock-in", "", "todo"},
	}
	for _, tc := range tests {
		t.Run(tc.title, func(t *testing.T) {
			task := tasks[tc.title]
			if task.AssigneeActorID != tc.assignee {
				t.Errorf("assignee = %q, want %q", task.AssigneeActorID, tc.assignee)
			}
			if task.Status != tc.status {
				t.Errorf("status = %q, want %q", task.Status, tc.status)
			}
		})
	}

	unknown := tasks["Fix the sign-in loop"]
	if unknown.CustomFields["op_assignee"] != "Nobody Known" {
		t.Errorf("custom fields = %v, want the unmatched assignee kept under the mapping prefix", unknown.CustomFields)
	}
	if unknown.ParentID == "" || unknown.ParentID != tasks["Write the operator guide"].ID {
		t.Errorf("parent = %q, want the task imported from work package 42", unknown.ParentID)
	}
	if strings.Join(res.UnmatchedUsers, "|") != "Nobody Known" {
		t.Errorf("unmatched users = %v", res.UnmatchedUsers)
	}
	want := "1 assignee has no tix actor and was left unassigned: Nobody Known"
	if !anyContains(res.Warnings, want) {
		t.Errorf("warnings = %v, want %q", res.Warnings, want)
	}
	if _, ok := f.actors(t)["ext-nobody-known"]; ok {
		t.Error("the unassigned policy created a placeholder actor")
	}
}

func TestRunSyncDryRunReportsUnmatchedUsers(t *testing.T) {
	tests := []struct {
		unknown string
		want    string
	}{
		{"unassigned", "1 assignee has no tix actor and would be left unassigned: Nobody Known"},
		{"placeholder", "1 assignee has no tix actor and would get placeholder actors: Nobody Known"},
	}
	for _, tc := range tests {
		t.Run(tc.unknown, func(t *testing.T) {
			f := newOpenProjectFixture(t, tc.unknown)
			before := len(f.actors(t))
			res := f.run(t, true)
			if res.Created["task"] != 4 || !res.DryRun {
				t.Fatalf("plan = %v dry %v", res.Created, res.DryRun)
			}
			if strings.Join(res.UnmatchedUsers, "|") != "Nobody Known" {
				t.Errorf("unmatched users = %v", res.UnmatchedUsers)
			}
			if !anyContains(res.Warnings, tc.want) {
				t.Errorf("warnings = %v, want %q", res.Warnings, tc.want)
			}
			if n := len(f.byTitle(t)); n != 0 {
				t.Errorf("a dry run wrote %d tasks", n)
			}
			if after := len(f.actors(t)); after != before {
				t.Errorf("a dry run went from %d to %d actors", before, after)
			}
		})
	}
}

func TestRunSyncPlaceholderCarriesTheAssignmentAcrossRuns(t *testing.T) {
	f := newOpenProjectFixture(t, "placeholder")
	res := f.run(t, false)
	if !anyContains(res.Warnings, "1 assignee has no tix actor and got placeholder actors: Nobody Known") {
		t.Errorf("warnings = %v", res.Warnings)
	}
	if strings.Join(res.UnmatchedUsers, "|") != "Nobody Known" {
		t.Errorf("unmatched users = %v", res.UnmatchedUsers)
	}
	actors := f.actors(t)
	placeholder, ok := actors["ext-nobody-known"]
	if !ok {
		t.Fatalf("no placeholder actor among %v", actors)
	}
	if placeholder.Kind != core.ActorUser || placeholder.DisplayName != "Nobody Known" {
		t.Errorf("placeholder = %+v", placeholder)
	}
	if _, err := f.local.GetUser(f.ctx, placeholder.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetUser(placeholder) = %v, want no account behind it", err)
	}
	members, err := f.local.ListMembers(f.ctx)
	if err != nil {
		t.Fatalf("listing members: %v", err)
	}
	for _, m := range members {
		if m.ActorID == placeholder.ID {
			t.Errorf("placeholder holds membership %+v", m)
		}
	}
	task := f.byTitle(t)["Fix the sign-in loop"]
	if task.AssigneeActorID != placeholder.ID {
		t.Fatalf("assignee = %q, want the placeholder %q", task.AssigneeActorID, placeholder.ID)
	}
	if _, kept := task.CustomFields["op_assignee"]; kept {
		t.Errorf("custom fields = %v; a carried assignment needs no fallback field", task.CustomFields)
	}

	f.bump("2", "2026-01-03T10:00:00Z", "2026-01-05T10:00:00Z")
	again := f.run(t, false)
	if again.Updated["task"] != 1 {
		t.Fatalf("second run updated %v, want the bumped work package", again.Updated)
	}
	if n, m := len(f.actors(t)), len(actors); n != m {
		t.Errorf("second run went from %d to %d actors; the placeholder must be reused", m, n)
	}
	if got := f.byTitle(t)["Fix the sign-in loop"].AssigneeActorID; got != placeholder.ID {
		t.Errorf("second run assignee = %q, want the same placeholder %q", got, placeholder.ID)
	}
	if strings.Join(again.UnmatchedUsers, "|") != "Nobody Known" {
		t.Errorf("second run unmatched users = %v", again.UnmatchedUsers)
	}
}

func TestRunSyncPlaceholderNeverTakesOverAnAccount(t *testing.T) {
	f := newOpenProjectFixture(t, "placeholder")
	holder, err := f.local.CreateUser(f.ctx, core.CreateUserInput{Email: "squat@corp.test", Handle: "ext-nobody-known"})
	if err != nil {
		t.Fatalf("creating account: %v", err)
	}
	res := f.run(t, false)
	task := f.byTitle(t)["Fix the sign-in loop"]
	if task.AssigneeActorID == holder.ID || task.AssigneeActorID != "" {
		t.Errorf("assignee = %q, want unassigned rather than the account holding the handle", task.AssigneeActorID)
	}
	if task.CustomFields["op_assignee"] != "Nobody Known" {
		t.Errorf("custom fields = %v", task.CustomFields)
	}
	if !anyContains(res.Warnings, "which a tix account holds") {
		t.Errorf("warnings = %v", res.Warnings)
	}
}

func TestAssigneeMatchStrategies(t *testing.T) {
	users := &syncUsers{
		handles:      map[string]string{"jane": "A1", "alex1": "A2", "alex2": "A3", "ext-ghost": "P1"},
		matchHandles: map[string]string{"jane": "A1", "alex1": "A2", "alex2": "A3"},
		emails:       map[string]string{"jane@corp.test": "A1"},
		names:        map[string][]string{"jane doe": {"A1"}, "alex": {"A2", "A3"}},
		answers:      map[string]string{},
		placeholders: map[string]string{"ext-ghost": "P1"},
	}
	tests := []struct {
		name, value, email, want string
		rules                    extsync.Users
	}{
		{"email in the value", "JANE@corp.test", "", "A1", extsync.Users{}},
		{"email field", "Somebody", "Jane@Corp.Test", "A1", extsync.Users{}},
		{"handle any case", "Jane", "", "A1", extsync.Users{}},
		{"unique name", "jane doe", "", "A1", extsync.Users{}},
		{"ambiguous name", "Alex", "", "", extsync.Users{}},
		{"strategy left out", "jane doe", "", "", extsync.Users{Match: []string{extsync.MatchEmail, extsync.MatchHandle}}},
		{"strategies off", "jane", "", "", extsync.Users{Match: []string{}}},
		{"explicit handle", "Boss", "", "A2", extsync.Users{Map: map[string]string{"Boss": "ALEX1"}, Match: []string{}}},
		{"explicit email", "Boss", "", "A1", extsync.Users{Map: map[string]string{"Boss": "jane@corp.test"}, Match: []string{}}},
		{"explicit placeholder", "Ghost", "", "P1", extsync.Users{Map: map[string]string{"Ghost": "ext-ghost"}}},
		{"placeholders are not matched", "ext-ghost", "", "", extsync.Users{}},
		{"explicit miss falls through", "jane", "", "A1", extsync.Users{Map: map[string]string{"jane": "nobody"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			users.answers = map[string]string{}
			var warned []string
			got := users.match(tc.rules, tc.value, tc.email, func(w string) { warned = append(warned, w) })
			if got != tc.want {
				t.Errorf("match(%q, %q) = %q, want %q", tc.value, tc.email, got, tc.want)
			}
			if tc.name == "explicit miss falls through" && !anyContains(warned, "users.map.jane names") {
				t.Errorf("warnings = %v, want the stale map entry reported", warned)
			}
		})
	}
}

func TestPlaceholderHandleAndHrefTail(t *testing.T) {
	handles := map[string]string{
		"Jane Doe":              "ext-jane-doe",
		"  O'Brien, Pat  ":      "ext-o-brien-pat",
		"Иван Петров":           "ext-иван-петров",
		"jdoe@corp.test":        "ext-jdoe-corp-test",
		strings.Repeat("a", 90): "ext-" + strings.Repeat("a", 60),
	}
	for in, want := range handles {
		if got := placeholderHandle(in); got != want {
			t.Errorf("placeholderHandle(%q) = %q, want %q", in, got, want)
		}
	}
	if got := placeholderHandle("@@@"); !strings.HasPrefix(got, "ext-") || len(got) <= len("ext-") {
		t.Errorf("placeholderHandle(@@@) = %q", got)
	}
	tails := map[string]string{
		"/api/v3/work_packages/42":                 "42",
		"https://op.test/api/v3/work_packages/42/": "42",
		"42":            "",
		"PROJ-7":        "",
		"ftp://x/y/9":   "",
		"/":             "",
		"relative/path": "",
	}
	for in, want := range tails {
		if got := hrefTail(in); got != want {
			t.Errorf("hrefTail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnmatchedWarningWording(t *testing.T) {
	many := make([]string, maxUnmatchedNamed+3)
	for i := range many {
		many[i] = "p" + string(rune('a'+i))
	}
	tests := []struct {
		names        []string
		dry, holders bool
		want         string
	}{
		{[]string{"A", "B"}, false, false, "2 assignees have no tix actor and were left unassigned: A, B"},
		{[]string{"A"}, true, false, "1 assignee has no tix actor and would be left unassigned: A"},
		{[]string{"A", "B"}, false, true, "2 assignees have no tix actor and got placeholder actors: A, B"},
		{[]string{"A"}, true, true, "1 assignee has no tix actor and would get placeholder actors: A"},
		{many, false, false, " and 3 more"},
	}
	for _, tc := range tests {
		if got := unmatchedWarning(tc.names, tc.dry, tc.holders); !strings.Contains(got, tc.want) {
			t.Errorf("unmatchedWarning = %q, want %q", got, tc.want)
		}
	}
}
