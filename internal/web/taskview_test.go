// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/web"
)

// parkedWorkflow is the builtin machine plus a state nothing transitions into,
// so a board drawn from it has a column no card can reach. That is what makes
// "the control is absent" an assertion about this card's own workflow rather
// than about the board happening to be short.
func parkedWorkflow() core.WorkflowDefinition {
	def := service.BuiltinWorkflow()
	def.States = append(def.States, core.State{Key: "parked", Label: "Parked"})
	def.Transitions = append(def.Transitions, core.Transition{From: "parked", To: "todo"})
	return def
}

// seedWorkflowProject puts a project in the fixture's first tenant running a
// workflow of its own row, so two projects can run the same machine without
// sharing the record it is stored in.
func seedWorkflowProject(t *testing.T, f *fixture, projectKey, workflowKey, workflowName string,
	def core.WorkflowDefinition) core.Project {
	t.Helper()
	ctx := context.Background()
	wf := core.Workflow{Key: workflowKey, Name: workflowName, Definition: def}
	project := core.Project{Key: projectKey, Name: strings.ToUpper(projectKey)}
	if err := f.store.Update(ctx, core.TenantScope{TenantID: f.tenantA.ID}, func(tx store.Tx) error {
		if err := tx.PutWorkflow(ctx, &wf); err != nil {
			return err
		}
		project.WorkflowID = wf.ID
		return tx.CreateProject(ctx, &project)
	}); err != nil {
		t.Fatalf("seeding project %q on workflow %q: %v", projectKey, workflowKey, err)
	}
	return project
}

// chooseBoard presses the view switch and returns where it sent the browser.
func (b *browser) chooseBoard(next string) string {
	b.t.Helper()
	resp := b.post("/taskview", url.Values{"view": {"board"}, "next": {next}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(b.t, resp, http.StatusSeeOther)
	return resp.Header.Get("Location")
}

// columnStates names the board's columns, in the order the page draws them,
// and nothing else on the page: the attribute belongs to a board column and
// to no other element.
var columnPattern = regexp.MustCompile(`<section class="column" data-state="([^"]+)"`)

func columnStates(page string) []string {
	out := []string{}
	for _, m := range columnPattern.FindAllStringSubmatch(page, -1) {
		out = append(out, m[1])
	}
	return out
}

// cardBlock is one board card and nothing else, so an assertion about what a
// card offers cannot pass on markup belonging to another card or to the list
// underneath. It fails rather than returning the page when the card is absent.
func cardBlock(t *testing.T, page, ref string) string {
	t.Helper()
	start := strings.Index(page, `data-task="`+ref+`"`)
	if start < 0 {
		t.Fatalf("no board card for %s:\n%s", ref, page)
	}
	open := strings.LastIndex(page[:start], "<article")
	end := strings.Index(page[start:], "</article>")
	if open < 0 || end < 0 {
		t.Fatalf("the card for %s is not a closed article", ref)
	}
	return page[open : start+end]
}

// moveOptions are the route values one card's Move control offers.
var optionPattern = regexp.MustCompile(`<option value="([^"]*)"`)

func moveOptions(t *testing.T, page, ref string) []string {
	t.Helper()
	card := cardBlock(t, page, ref)
	from := strings.Index(card, `<select id="to-`+ref+`"`)
	if from < 0 {
		return nil
	}
	block := card[from:]
	if end := strings.Index(block, "</select>"); end >= 0 {
		block = block[:end]
	}
	out := []string{}
	for _, m := range optionPattern.FindAllStringSubmatch(block, -1) {
		// The route separator is escaped in the attribute, so what the option
		// actually submits is the unescaped value.
		out = append(out, html.UnescapeString(m[1]))
	}
	return out
}

// The switch is a preference, so it survives the request it was made on.
func TestTaskViewSwitchPersistsAcrossRequests(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "infra work")

	if states := columnStates(b.page("/tasks")); len(states) != 0 {
		t.Fatalf("an untouched task screen drew a board: %v", states)
	}
	b.chooseBoard("/tasks")

	if states := columnStates(b.page("/tasks")); len(states) == 0 {
		t.Fatalf("the switch drew no board:\n%s", b.page("/tasks"))
	}
	if states := columnStates(b.page("/tasks")); len(states) == 0 {
		t.Error("the choice did not survive the next request")
	}
	if b.cookie(web.TaskViewCookie) != web.TaskViewBoard {
		t.Errorf("the choice is not stored: %q", b.cookie(web.TaskViewCookie))
	}
	if other := f.as("alice"); len(columnStates(other.page("/tasks"))) != 0 {
		t.Error("the choice leaked into another browser")
	}

	back := b.post("/taskview", url.Values{"view": {"list"}, "next": {"/tasks"}})
	_ = back.Body.Close()
	if states := columnStates(b.page("/tasks")); len(states) != 0 {
		t.Errorf("switching back kept the board: %v", states)
	}
	if b.cookie(web.TaskViewCookie) != "" {
		t.Errorf("the default was stored rather than cleared: %q", b.cookie(web.TaskViewCookie))
	}
}

// Switching the view must not change which tasks are on screen, so the filter
// the reader was reading under survives the press and still applies.
func TestSwitchingToTheBoardKeepsTheFilter(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	kept := b.createTask("infra", "tagged work")
	dropped := b.createTask("infra", "untagged work")
	tagged := b.post("/tasks/"+kept+"/tags", url.Values{"tag": {"ops"}})
	_ = tagged.Body.Close()

	filtered := "/tasks?q=" + url.QueryEscape("tag:ops")
	if list := b.page(filtered); !strings.Contains(list, kept) || strings.Contains(list, dropped) {
		t.Fatalf("the filtered list is not what this test assumes:\n%s", list)
	}

	where := b.chooseBoard(filtered)
	if where != filtered {
		t.Errorf("the switch sent the reader to %q, losing the filter from %q", where, filtered)
	}
	page := b.page(where)
	if len(columnStates(page)) == 0 {
		t.Fatalf("the switch drew no board:\n%s", page)
	}
	if !strings.Contains(page, `data-task="`+kept+`"`) {
		t.Errorf("the board dropped the task the filter selected:\n%s", page)
	}
	if strings.Contains(page, `data-task="`+dropped+`"`) {
		t.Errorf("the board shows a task the filter excluded:\n%s", page)
	}
	if !strings.Contains(page, `value="tag:ops"`) {
		t.Error("the board lost the expression out of the filter box")
	}
}

// The visibility choice is part of which tasks are selected, so it applies to
// the board exactly as it applies to the list.
func TestSwitchingToTheBoardKeepsHiddenProjects(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedWorkflowProject(t, f, "ops", "ops-flow", "Ops flow", service.BuiltinWorkflow())
	shown := b.createTask("infra", "infra work")
	away := b.createTask("ops", "ops work")

	hide := b.post("/visibility", url.Values{"project": {"infra"}, "next": {"/tasks"}})
	_ = hide.Body.Close()
	b.chooseBoard("/tasks")

	page := b.page("/tasks")
	if len(columnStates(page)) == 0 {
		t.Fatalf("the board was not drawn for the one visible project:\n%s", page)
	}
	if !strings.Contains(page, `data-task="`+shown+`"`) {
		t.Errorf("the board dropped the visible project's task:\n%s", page)
	}
	if strings.Contains(page, `data-task="`+away+`"`) {
		t.Errorf("the board shows a task of a project that was put away:\n%s", page)
	}
}

// Two projects running the same machine out of two stored workflows draw one
// set of columns. A name is not an identity, and neither is a record.
func TestIdenticalWorkflowsMergeIntoOneBoard(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedWorkflowProject(t, f, "ops", "ops-flow", "Ops flow", service.BuiltinWorkflow())
	first := b.createTask("infra", "infra work")
	second := b.createTask("ops", "ops work")
	b.chooseBoard("/tasks")

	page := b.page("/tasks")
	want := []string{}
	for _, s := range service.BuiltinWorkflow().States {
		want = append(want, s.Key)
	}
	if got := columnStates(page); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the merged board drew columns %v, want one per shared state %v", got, want)
	}
	for _, ref := range []string{first, second} {
		if !strings.Contains(page, `data-task="`+ref+`"`) {
			t.Errorf("the merged board omits %s:\n%s", ref, page)
		}
	}
	if strings.Contains(page, "do not share a workflow") {
		t.Error("two projects running the same machine were refused a board")
	}
	// A merged card says which list it came from, because one column now
	// holds two projects' work.
	if !strings.Contains(cardBlock(t, page, second), `>ops</a>`) {
		t.Errorf("a card on a merged board does not name its project:\n%s", cardBlock(t, page, second))
	}
}

// Workflows that disagree are not merged, the reader is told which projects
// disagree and about what, and nothing is dropped from the screen.
func TestWorkflowsThatDisagreeRefuseTheBoardAndNameThem(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedWorkflowProject(t, f, "ops", "ops-flow", "Ops flow", service.BuiltinWorkflow())
	// Same name as another project's workflow, and a different machine: the
	// case merging on the name would get wrong.
	odd := service.BuiltinWorkflow()
	odd.States = append(odd.States, core.State{Key: "review", Label: "Review"})
	odd.Transitions = append(odd.Transitions, core.Transition{From: "doing", To: "review"})
	seedWorkflowProject(t, f, "web", "web-flow", "Default", odd)

	first := b.createTask("infra", "infra work")
	second := b.createTask("ops", "ops work")
	third := b.createTask("web", "web work")
	b.chooseBoard("/tasks")

	page := b.page("/tasks")
	if states := columnStates(page); len(states) != 0 {
		t.Errorf("a board was drawn over disagreeing workflows: %v", states)
	}
	if !strings.Contains(page, "do not share a workflow") {
		t.Errorf("the refusal is not explained:\n%s", page)
	}
	fault := page[strings.Index(page, `class="boardfault"`):]
	if end := strings.Index(fault, "</div>"); end >= 0 {
		fault = fault[:end]
	}
	for _, want := range []string{"infra", "ops", "web", "Default"} {
		if !strings.Contains(fault, want) {
			t.Errorf("the refusal does not name %q:\n%s", want, fault)
		}
	}
	if !strings.Contains(page, `<ul class="tasklist">`) {
		t.Errorf("the refusal did not fall back to the list:\n%s", page)
	}
	for _, ref := range []string{first, second, third} {
		if !strings.Contains(page, ref) {
			t.Errorf("the refused board dropped %s from the screen entirely", ref)
		}
	}
	// The preference is untouched, so narrowing to one group is one click and
	// draws the board without pressing the switch again.
	narrowed := b.page("/tasks?q=" + url.QueryEscape("project:infra project:ops"))
	if len(columnStates(narrowed)) == 0 {
		t.Errorf("narrowing to the agreeing projects drew no board:\n%s", narrowed)
	}
	if !strings.Contains(narrowed, `data-task="`+second+`"`) {
		t.Error("the narrowed board lost one of the agreeing projects")
	}
}

// The guard that matters: the control a card offers is its own workflow's, so
// a state nothing can reach has a column and no option, and the service also
// refuses the move. Both halves, because a board that offers an impossible
// move is the defect even when the service then says no.
func TestABoardCardIsNotOfferedAnUnreachableState(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedWorkflowProject(t, f, "parka", "park-a", "Parked flow", parkedWorkflow())
	seedWorkflowProject(t, f, "parkb", "park-b", "Parked flow", parkedWorkflow())

	ref := b.createTask("parka", "parked work")
	// Named explicitly, so the two projects on the board are the two running
	// this machine and the fixture's own project is out of the selection.
	where := "/tasks?q=" + url.QueryEscape("project:parka project:parkb")
	b.chooseBoard(where)

	page := b.page(where)
	states := columnStates(page)
	if !strings.Contains(strings.Join(states, ","), "parked") {
		t.Fatalf("the board has no parked column to be offered: %v", states)
	}
	options := moveOptions(t, page, ref)
	if len(options) == 0 {
		t.Fatalf("the card offers no moves at all:\n%s", cardBlock(t, page, ref))
	}
	for _, option := range options {
		for _, step := range strings.Split(option, core.RouteSep) {
			if step == "parked" {
				t.Errorf("the card offers %q, a move its own workflow cannot make; options %v",
					option, options)
			}
		}
	}
	// The same answer from its own workflow, so the assertion above is about
	// what this card may do and not about an empty control.
	want := []string{}
	for _, r := range core.Routes(parkedWorkflow(), "todo") {
		want = append(want, r.Value())
	}
	if strings.Join(options, ",") != strings.Join(want, ",") {
		t.Errorf("the card offers %v, its own workflow allows %v", options, want)
	}
	// The fallback, which is not the protection: asking for it anyway is
	// refused rather than performed.
	refused := b.post("/tasks/"+ref+"/transition", url.Values{"route": {"parked"},
		"next": {"/tasks"}})
	defer func() { _ = refused.Body.Close() }()
	if refused.StatusCode == http.StatusSeeOther {
		t.Fatalf("the service performed a move the workflow forbids")
	}
	if page := b.page(where); !strings.Contains(page, `data-task="`+ref+`"`) {
		t.Error("the refused move moved the card off the board")
	}
}
