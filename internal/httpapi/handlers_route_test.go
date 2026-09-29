// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// routePath is the route endpoint for one task.
func routePath(ref string) string { return "/api/v1/tasks/" + ref + "/route" }

// hops lists the task's recorded state changes over the wire, oldest first, as
// "from>to". It reads the audit endpoint, which is what an operator reviewing
// a task actually reads.
func (f *apiFixture) hops(t *testing.T, ref string) []string {
	t.Helper()
	resp := f.call(http.MethodGet, "/api/v1/tasks/"+ref+"/audit", nil)
	mustStatus(t, resp, http.StatusOK)
	var page struct {
		Items []core.AuditEntry `json:"items"`
	}
	decodeBody(t, resp, &page)
	var out []string
	entries := page.Items
	sort.Slice(entries, func(i, j int) bool { return entries[i].Seq < entries[j].Seq })
	for _, e := range entries {
		if e.Action != "task.transition" {
			continue
		}
		out = append(out, wireStatus(t, e.Before)+">"+wireStatus(t, e.After))
	}
	return out
}

func wireStatus(t *testing.T, raw json.RawMessage) string {
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

// TestARouteOverTheAPIWritesOneEntryPerHop drives the route through the API's
// own entry point and then reads the trail back through the API. The service
// holds the rule; this holds that the endpoint is wired to it.
func TestARouteOverTheAPIWritesOneEntryPerHop(t *testing.T) {
	f := newFixture(t)

	tests := []struct {
		name string
		body core.RouteInput
		want []string
	}{
		{"one hop", core.RouteInput{Route: []string{"doing"}}, []string{"todo>doing"}},
		{"through one state", core.RouteInput{Route: []string{"doing", "done"}},
			[]string{"todo>doing", "doing>done"}},
		{"a named destination", core.RouteInput{To: "done"}, []string{"todo>doing", "doing>done"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := f.createTask("route " + tt.name)
			resp := f.call(http.MethodPost, routePath(task.Ref), tt.body)
			mustStatus(t, resp, http.StatusOK)
			var result core.RouteResult
			decodeBody(t, resp, &result)
			if result.Partial() {
				t.Fatalf("route stopped at %q: %s", result.Stopped, result.Reason)
			}
			if result.Hops() != len(tt.want) {
				t.Errorf("the response reports %d hops, want %d", result.Hops(), len(tt.want))
			}
			if got := f.hops(t, task.Ref); strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("the trail holds %v, want one entry per hop: %v", got, tt.want)
			}
		})
	}
}

// TestARouteOverTheAPIThatStopsPartWayIsNotAnError holds the shape of the
// reply: the hops that landed are real, so the caller is told where the task
// is, not merely that something failed.
func TestARouteOverTheAPIThatStopsPartWayIsNotAnError(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("broken route")

	// The shipped workflow leaves done only for todo, so the third hop is
	// refused after the first two have been written.
	resp := f.call(http.MethodPost, routePath(task.Ref),
		core.RouteInput{Route: []string{"doing", "done", "blocked"}})
	mustStatus(t, resp, http.StatusOK)
	var result core.RouteResult
	decodeBody(t, resp, &result)

	if !result.Partial() || result.Stopped != "blocked" {
		t.Fatalf("result = %+v, want a partial route stopped at blocked", result)
	}
	if strings.Join(result.Applied, ",") != "doing,done" {
		t.Errorf("applied = %v, want the two hops that happened", result.Applied)
	}
	if got := f.hops(t, task.Ref); strings.Join(got, ",") != "todo>doing,doing>done" {
		t.Fatalf("the trail holds %v, want only the hops that happened", got)
	}
}

// TestAnAmbiguousDestinationOverTheAPIIsRefused keeps the refusal on the wire,
// in the envelope every other refusal uses.
func TestAnAmbiguousDestinationOverTheAPIIsRefused(t *testing.T) {
	f := newFixture(t)
	f.putForkedWorkflow(t)
	task := f.createTask("forked")

	resp := f.call(http.MethodPost, routePath(task.Ref), core.RouteInput{To: "done"})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	decodeBody(t, resp, &env)
	if env.Error.Code != string(core.KindInvalid) {
		t.Errorf("code = %q, want %q", env.Error.Code, core.KindInvalid)
	}
	if !strings.Contains(env.Error.Message, "name the route you want") {
		t.Errorf("message = %q, which does not ask for a route", env.Error.Message)
	}
	if got := f.hops(t, task.Ref); len(got) != 0 {
		t.Errorf("the trail holds %v after a refusal, want nothing", got)
	}
}

// TestTheSingleStateTransitionEndpointIsUnchanged holds the additive promise:
// a client that posts one status keeps the behaviour it had.
func TestTheSingleStateTransitionEndpointIsUnchanged(t *testing.T) {
	f := newFixture(t)
	task := f.createTask("one state")

	resp := f.call(http.MethodPost, "/api/v1/tasks/"+task.Ref+"/transition",
		core.TransitionInput{To: "doing"})
	mustStatus(t, resp, http.StatusOK)
	var moved core.Task
	decodeBody(t, resp, &moved)
	if moved.Status != "doing" {
		t.Fatalf("status = %q, want doing", moved.Status)
	}

	// It is still a single transition, not a route: a state it cannot reach
	// directly is refused rather than walked to.
	refused := f.call(http.MethodPost, "/api/v1/tasks/"+task.Ref+"/transition",
		core.TransitionInput{To: "doing"})
	defer func() { _ = refused.Body.Close() }()
	if refused.StatusCode == http.StatusOK {
		t.Fatal("the transition endpoint accepted a move the workflow does not permit")
	}
	if got := f.hops(t, task.Ref); strings.Join(got, ",") != "todo>doing" {
		t.Fatalf("the trail holds %v, want the one transition that happened", got)
	}
}

// putForkedWorkflow points the fixture's project at a workflow that reaches
// done two ways in the same number of steps.
func (f *apiFixture) putForkedWorkflow(t *testing.T) {
	t.Helper()
	def := core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "Todo"},
			{Key: "review", Label: "Review"},
			{Key: "qa", Label: "QA"},
			{Key: "done", Label: "Done", Terminal: true},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "review"},
			{From: "todo", To: "qa"},
			{From: "review", To: "done"},
			{From: "qa", To: "done"},
		},
	}
	resp := f.call(http.MethodPut, wire.RouteWorkflows,
		core.WorkflowInput{Key: "default", Name: "Default", Definition: def})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("replacing the workflow: %d %s", resp.StatusCode, readBody(t, resp))
	}
}
