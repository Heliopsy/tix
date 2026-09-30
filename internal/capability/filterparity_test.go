// SPDX-License-Identifier: AGPL-3.0-or-later

package capability_test

import (
	"context"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/client"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/query"
	"github.com/heliopsy/tix/internal/tui"
	"github.com/heliopsy/tix/internal/web"
)

// The registry asserts that task.list is BOUND on every surface. It does not
// assert that the surfaces accept the same filters, which is why DueBefore and
// DueAfter reached the HTTP API and the command line could not ask for them:
// four transports, one operation, four vocabularies, and nothing comparing
// them. What follows compares the vocabularies.
//
// core.TaskFilter is the contract all four narrow a listing with, so it is the
// list reflected over rather than any surface's own. A field added to it fails
// this file until it is either reachable everywhere or exempted by name with a
// reason, which is the same shape as capability.Exemption and for the same
// reason: an absence somebody decided on reads differently from one nobody
// noticed.

// filterReach is how one core.TaskFilter field is reached from each surface.
//
// Expr is the filter expression that sets it, which covers the terminal
// interface and the browser filter bar as well as `tix task ls --filter`,
// since all three run internal/query and the shim is asserted below. CLI is
// the flag on `tix task ls` that sets it without an expression. HTTP is the
// query parameter, checked by a round trip rather than by name.
type filterReach struct {
	Expr string
	CLI  string
}

// filterFields maps each field of core.TaskFilter to how a reader reaches it.
var filterFields = map[string]filterReach{
	"ProjectKeys":    {Expr: "project:infra", CLI: "--project"},
	"Statuses":       {Expr: "status:todo", CLI: "--status"},
	"Tags":           {Expr: "tag:ops", CLI: "--tag"},
	"AssigneeIDs":    {Expr: "assignee:alice", CLI: "--assignee"},
	"Priorities":     {Expr: "priority:high", CLI: ""},
	"DueBefore":      {Expr: "due:overdue", CLI: "--overdue"},
	"DueAfter":       {Expr: "due:>2026-01-01", CLI: "--due-after"},
	"Claimed":        {Expr: "is:claimed", CLI: "--claimed"},
	"Blocked":        {Expr: "is:blocked", CLI: "--blocked"},
	"Query":          {Expr: "deploy", CLI: "--query"},
	"CreatorIDs":     {Expr: "creator:alice", CLI: ""},
	"ClaimedBy":      {Expr: "claimed-by:scout", CLI: ""},
	"ParentID":       {Expr: "parent:t-1", CLI: ""},
	"Text":           {Expr: "title~api", CLI: ""},
	"Exclude":        {Expr: "-tag:ops", CLI: ""},
	"CustomFields":   {Expr: "field.severity:high", CLI: ""},
	"ParentIsNull":   {Expr: "is:root", CLI: ""},
	"IncludeDeleted": {Expr: "is:deleted", CLI: "--include-deleted"},
}

// filterExempt names the fields no filter expression reaches, with the reason.
// An entry here is a decision; a field in neither map is an oversight, and the
// difference between the two is the whole point of writing them down.
var filterExempt = map[string]string{
	"ProjectIDs": "an internal identifier, not a name anybody types. Every surface " +
		"selects a project by key, which ProjectKeys carries; the HTTP API accepts " +
		"project_id as well because a machine caller already holds the identifier.",
	"Page": "the shape of the listing rather than a set of tasks. Sort, limit, cursor " +
		"and direction each have their own control on every surface, and the expression " +
		"spells the first two as sort: and limit:.",
}

// cliFlagExempt names the fields reachable only through --filter, with the
// reason no flag of their own is offered.
var cliFlagExempt = map[string]string{
	"Priorities": "the expression is the only spelling that can also exclude, and " +
		"--priority beside -priority:low would be one vocabulary in two grammars",
	"CreatorIDs":   "asked for far less often than assignee; --filter 'creator:alice' is the whole of it",
	"ClaimedBy":    "the same, and --claimed already answers the question people actually ask",
	"ParentID":     "`tix task tree` is the command for a parent's subtasks",
	"ParentIsNull": "the same: a flag for the roots would be a second way to say `tix task tree`",
	"Text":         "the weak and negated forms need the operators, which is a grammar rather than a flag",
	"Exclude":      "every exclusion is the negation of an inclusion, so a flag per excluded field would double the flag list",
	"CustomFields": "the name is part of the term, so a flag would be --field.severity=high, which pflag cannot register",
}

// taskFilterFields is every field of the contract a listing is narrowed by.
func taskFilterFields() []string {
	t := reflect.TypeOf(core.TaskFilter{})
	out := make([]string, 0, t.NumField())
	for i := range t.NumField() {
		out = append(out, t.Field(i).Name)
	}
	return out
}

// TestEveryTaskFilterFieldIsAccountedFor is the structural half. A field added
// to the contract reaches this file before it reaches a surface, so a filter
// that lands on one transport and not the others cannot ship quietly.
func TestEveryTaskFilterFieldIsAccountedFor(t *testing.T) {
	t.Parallel()
	known := map[string]bool{}
	for _, name := range taskFilterFields() {
		known[name] = true
		_, reachable := filterFields[name]
		reason, exempt := filterExempt[name]
		switch {
		case reachable && exempt:
			t.Errorf("core.TaskFilter.%s is both reachable and exempted", name)
		case reachable:
		case !exempt:
			t.Errorf("core.TaskFilter.%s is reachable from no surface and exempted by none; "+
				"give it a filter term, or name it in filterExempt with the reason", name)
		case strings.TrimSpace(reason) == "":
			t.Errorf("core.TaskFilter.%s is exempted without a reason", name)
		}
	}
	for name := range filterFields {
		if !known[name] {
			t.Errorf("filterFields names %q, which core.TaskFilter does not declare", name)
		}
	}
	for name := range filterExempt {
		if !known[name] {
			t.Errorf("filterExempt names %q, which core.TaskFilter does not declare", name)
		}
	}
	for name := range cliFlagExempt {
		if _, ok := filterFields[name]; !ok {
			t.Errorf("cliFlagExempt names %q, which is not a reachable field", name)
		}
	}
}

// TestEveryFilterTermActuallySetsItsField is the guard on the guard above. A
// table of expressions nobody parses would pass the structural check while
// every term in it was a typo.
func TestEveryFilterTermActuallySetsItsField(t *testing.T) {
	t.Parallel()
	for name, reach := range filterFields {
		f, err := query.Parse(reach.Expr)
		if err != nil {
			t.Errorf("%s: the filter language refuses %q: %v", name, reach.Expr, err)
			continue
		}
		if isZero(f, name) {
			t.Errorf("%s: %q parses and leaves the field unset, so the term is not the one that sets it",
				name, reach.Expr)
		}
	}
}

// TestTheTerminalAndTheBrowserRunTheOneParser is what makes the expression
// column above cover three surfaces rather than one. Both are one-line shims
// today, and both were not: the browser carried two hundred lines of its own
// parser that swallowed "title~api" as free text and answered with an empty
// board and no error.
func TestTheTerminalAndTheBrowserRunTheOneParser(t *testing.T) {
	t.Parallel()
	for name, reach := range filterFields {
		want, err := query.Parse(reach.Expr)
		if err != nil {
			continue
		}
		fromTUI, tuiErr := tui.ParseFilter(reach.Expr)
		fromWeb, webErr := web.ParseFilter(reach.Expr)
		if tuiErr != nil || webErr != nil {
			t.Errorf("%s: %q is refused by a surface: tui %v, web %v", name, reach.Expr, tuiErr, webErr)
			continue
		}
		// The due terms are answered against the moment of the call, so the
		// bounds differ by a few microseconds between three parses of the same
		// expression. Everything else compares whole.
		if name == "DueBefore" || name == "DueAfter" {
			if isZero(fromTUI, name) || isZero(fromWeb, name) {
				t.Errorf("%s: %q sets nothing on one of the surfaces", name, reach.Expr)
			}
			continue
		}
		if !reflect.DeepEqual(want, fromTUI) {
			t.Errorf("%s: the terminal answers %q differently from the shared parser", name, reach.Expr)
		}
		if !reflect.DeepEqual(want, fromWeb) {
			t.Errorf("%s: the browser answers %q differently from the shared parser", name, reach.Expr)
		}
	}
	for _, key := range query.Keys {
		if !strings.Contains(strings.Join(tui.FilterKeys, " "), key) {
			t.Errorf("the terminal's key list omits %q", key)
		}
		if !strings.Contains(strings.Join(web.FilterKeys, " "), key) {
			t.Errorf("the browser's key list omits %q", key)
		}
	}
}

// registeredFlag matches the long name in one line of pflag's usage listing.
var registeredFlag = regexp.MustCompile(`(?m)^\s+(?:-\w, )?(--[a-z0-9-]+)`)

// registeredFlags returns the flags a command actually registers, read out of
// the "Flags:" block of its help and nothing else.
//
// Searching the whole help for the flag's name is what this guard did first,
// and it passed with the flag deleted: the long description names --overdue in
// a sentence, so the assertion was reading prose rather than the flag list. It
// is the failure AGENTS.md describes -- an assertion reading something wider
// than the thing it names -- and it is why the block is cut out first.
func registeredFlags(t *testing.T, path string) map[string]bool {
	t.Helper()
	_, help := runCLI(t, path)
	_, block, ok := strings.Cut(help, "\nFlags:\n")
	if !ok {
		t.Fatalf("the help for %q has no Flags: block, so this guard reads nothing:\n%s", path, help)
	}
	if end := strings.Index(block, "\n\n"); end >= 0 {
		block = block[:end]
	}
	out := map[string]bool{}
	for _, m := range registeredFlag.FindAllStringSubmatch(block, -1) {
		out[m[1]] = true
	}
	return out
}

// TestTheCommandLineOffersAFlagForEveryFilterItShould is the check that would
// have caught the reported gap. --filter reaches every field by construction,
// so a guard written against it passes whatever the flag list holds; this one
// reads the flags `tix task ls` actually registers.
func TestTheCommandLineOffersAFlagForEveryFilterItShould(t *testing.T) {
	t.Parallel()
	flags := registeredFlags(t, "tix task ls")
	// The block has to hold flags this guard is not about, or it is reading
	// the wrong thing: a parse that found nothing would excuse every field.
	for _, known := range []string{"--filter", "--limit", "--sort"} {
		if !flags[known] {
			t.Fatalf("the parsed flag list does not hold %s, so it is not the flag list: %v", known, flags)
		}
	}
	if flags["--no-such-flag"] {
		t.Fatal("the parsed flag list holds a flag nothing registers")
	}
	for name, reach := range filterFields {
		reason, exempt := cliFlagExempt[name]
		switch {
		case reach.CLI != "" && exempt:
			t.Errorf("%s: a flag is declared and the absence of one is excused", name)
		case reach.CLI != "":
			if !flags[reach.CLI] {
				t.Errorf("%s: `tix task ls` registers no %s, so the command line cannot ask for it "+
					"without an expression", name, reach.CLI)
			}
		case !exempt:
			t.Errorf("%s: no flag on `tix task ls` and no reason recorded; add one, or name it "+
				"in cliFlagExempt with why the expression is enough", name)
		case strings.TrimSpace(reason) == "":
			t.Errorf("%s: the absent flag is excused without a reason", name)
		}
	}
}

// recordingService captures the filter a listing was asked for and answers an
// empty page, so a round trip reports what survived the wire and nothing else.
type recordingService struct {
	core.Service
	got core.TaskFilter
}

func (s *recordingService) ListTasks(_ context.Context, f core.TaskFilter) (core.TaskPage, error) {
	s.got = f
	return core.TaskPage{}, nil
}

// TestEveryTaskFilterFieldSurvivesTheWire is the HTTP half, and it names no
// parameters: a filter with every field set is sent by the real client to the
// real router, and what the service is handed is compared with what was sent.
// A field the client does not encode, or the handler does not read, fails
// whatever either of them is spelled.
func TestEveryTaskFilterFieldSurvivesTheWire(t *testing.T) {
	t.Parallel()
	before := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	after := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sent := core.TaskFilter{
		ProjectIDs:  []string{"p-1"},
		ProjectKeys: []string{"infra"},
		Statuses:    []string{"todo"},
		Tags:        []string{"ops"},
		AssigneeIDs: []string{"u-1"},
		CreatorIDs:  []string{"u-2"},
		ClaimedBy:   []string{"u-3"},
		Priorities:  []core.Priority{core.PriorityHigh},
		DueBefore:   &before,
		DueAfter:    &after,
		ParentID:    "t-1",
		Claimed:     core.Yes,
		Blocked:     core.No,
		Query:       "deploy",
		Text: []core.TextTerm{
			{Field: core.TextTitle, Mode: core.MatchContains, Value: "api"},
		},
		Exclude: core.TaskExclude{
			ProjectKeys: []string{"other"},
			Statuses:    []string{"done"},
			Tags:        []string{"noise"},
			AssigneeIDs: []string{"u-4"},
			CreatorIDs:  []string{"u-5"},
			ClaimedBy:   []string{"u-6"},
			Priorities:  []core.Priority{core.PriorityLowest},
		},
		CustomFields:   map[string]any{"severity": "high"},
		IncludeDeleted: true,
	}

	svc := &recordingService{}
	router, err := httpapi.New(httpapi.Config{
		Service: svc,
		Authenticator: auth.NewStaticAuthenticator(&core.Actor{
			ID: "u1", TenantID: "t1", Kind: core.ActorUser, Scopes: core.AllScopes,
		}),
		Resolver:        stubResolver{},
		DefaultTenantID: "t1",
		EventHandler:    okHandler(),
	})
	if err != nil {
		t.Fatalf("building router: %v", err)
	}
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	c, err := client.New(server.URL, "token")
	if err != nil {
		t.Fatalf("building client: %v", err)
	}
	if _, err := c.ListTasks(context.Background(), sent); err != nil {
		t.Fatalf("listing: %v", err)
	}

	// ParentIsNull is not sent beside ParentID: the contract refuses the two
	// together, so the round trip carries it in a second call of its own.
	got := svc.got
	for _, name := range taskFilterFields() {
		if name == "Page" || name == "ParentIsNull" {
			continue
		}
		want := reflect.ValueOf(sent).FieldByName(name).Interface()
		have := reflect.ValueOf(got).FieldByName(name).Interface()
		if !reflect.DeepEqual(want, have) {
			t.Errorf("core.TaskFilter.%s did not survive the wire: sent %v, the service was handed %v",
				name, want, have)
		}
	}

	if _, err := c.ListTasks(context.Background(), core.TaskFilter{ParentIsNull: true}); err != nil {
		t.Fatalf("listing the roots: %v", err)
	}
	if !svc.got.ParentIsNull {
		t.Error("core.TaskFilter.ParentIsNull did not survive the wire")
	}
}

// isZero reports whether a named field of a filter is still at its zero value.
func isZero(f core.TaskFilter, name string) bool {
	return reflect.ValueOf(f).FieldByName(name).IsZero()
}
