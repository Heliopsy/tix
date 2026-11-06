// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/web"
)

// projectControl is the filter bar's project select and nothing else, so an
// assertion about it cannot pass on another select's options.
func projectControl(t *testing.T, page string) string {
	t.Helper()
	block := between(t, page, `<select name="project"`, `</select>`)
	if block == "" {
		t.Fatal("the filter bar carries no project control")
	}
	return block
}

// listing is the task screen's listing region, the part a live refresh
// replaces, so a reference found in it is a row, card or lane card.
func listing(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, `<div id="tasks-listing">`)
	if start < 0 {
		t.Fatal("the screen carries no listing")
	}
	block := page[start:]
	if end := strings.Index(block, `<div id="live-screen"`); end >= 0 {
		block = block[:end]
	}
	return block
}

// odd is the builtin workflow with a review state, so a board drawn from it
// has a column no other project's board has.
func oddWorkflow() core.WorkflowDefinition {
	def := service.BuiltinWorkflow()
	def.States = append(def.States, core.State{Key: "review", Label: "Review"})
	def.Transitions = append(def.Transitions, core.Transition{From: "doing", To: "review"})
	return def
}

func TestTheProjectControlNarrowsTheListTheBoardAndTheLanes(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	web1 := seedWorkflowProject(t, f, "web", "web-flow", "Web flow", oddWorkflow())
	mine := b.createTask("web", "web work")
	other := b.createTask("infra", "infra work")

	for _, view := range []string{web.TaskViewList, web.TaskViewBoard, web.TaskViewLanes} {
		t.Run(view, func(t *testing.T) {
			b.setCookie(web.TaskViewCookie, view)
			page := b.page("/tasks?project=web")
			rows := listing(t, page)
			if !strings.Contains(rows, `id="t-`+mine+`"`) {
				t.Errorf("the %s left out the chosen project's task", view)
			}
			if strings.Contains(rows, `id="t-`+other+`"`) {
				t.Errorf("the %s kept a task from a project the control left out", view)
			}
			if !strings.Contains(projectControl(t, page), `value="web" selected`) {
				t.Errorf("the %s's project control does not show the project it is answering", view)
			}
			next := html.UnescapeString(between(t, viewSwitch(t, page), `name="next" value="`, `"`))
			if !strings.Contains(next, "project=web") {
				t.Errorf("the view switch on the %s returns to %q, dropping the project", view, next)
			}
			if got := liveFilterOf(t, page).ProjectIDs; !slices.Equal(got, []string{web1.ID}) {
				t.Errorf("the %s subscribes to projects %v, want only %s", view, got, web1.ID)
			}
			late := b.createTask("infra", "arrives in another project")
			fragment := b.page(liveAttr(t, page, "data-live"))
			if strings.Contains(fragment, `id="t-`+late+`"`) {
				t.Errorf("a live refresh of the %s drew a task from a project the control left out", view)
			}
			if !strings.Contains(fragment, `id="t-`+mine+`"`) {
				t.Errorf("a live refresh of the %s lost the chosen project's task", view)
			}
		})
	}
}

func TestABoardOfOneProjectHasThatProjectsColumns(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedWorkflowProject(t, f, "web", "web-flow", "Web flow", oddWorkflow())
	b.createTask("web", "web work")
	b.createTask("infra", "infra work")
	b.setCookie(web.TaskViewCookie, web.TaskViewBoard)

	if states := columnStates(b.page("/tasks?project=web")); !slices.Contains(states, "review") {
		t.Errorf("the board of one project is not drawn in its own workflow: %v", states)
	}
	if states := columnStates(b.page("/tasks?project=infra")); slices.Contains(states, "review") {
		t.Errorf("the board of infra borrowed another project's column: %v", states)
	}
	// Two projects whose workflows disagree cannot share columns; the screen
	// says so and lists them rather than drawing one workflow's board.
	mixed := b.page("/tasks?project=web,infra")
	if states := columnStates(mixed); len(states) != 0 {
		t.Errorf("a board was drawn over two workflows that disagree: %v", states)
	}
	refusalBlock(t, mixed)
	if !strings.Contains(projectControl(t, mixed), `value="web,infra" selected`) {
		t.Error("the project control does not show the two projects the screen is answering")
	}
}

func TestTheProjectControlRidesAlongThePagerAndTheSavedFilter(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedProject(t, f.store, f.tenantA.ID, "web")
	b.createTask("web", "first")
	b.createTask("web", "second")

	page := b.page("/tasks?project=web&limit=1")
	next := html.UnescapeString(between(t, page, `class="pager-step pager-next" href="`, `"`))
	if !strings.Contains(next, "project=web") {
		t.Errorf("the next page %q drops the project", next)
	}

	resp := b.post("/filters", url.Values{"name": {"web only"}, "project": {"web"}, "q": {"tag:ui"}})
	_ = resp.Body.Close()
	filters, err := f.svc.ListSavedFilters(f.ctx())
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 || filters[0].Query != "project:web tag:ui" {
		t.Errorf("the saved filter is %+v, want the query project:web tag:ui", filters)
	}
}
