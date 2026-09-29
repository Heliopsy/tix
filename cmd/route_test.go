// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// transitionHops lists a task's recorded state changes, oldest first, as
// "from>to", read the way an operator reads them: off the audit log.
func transitionHops(t *testing.T, c *cli, ref string) []string {
	t.Helper()
	got := c.mustRun("audit", "ls", "-o", "ndjson")
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(got.out), "\n") {
		if line == "" {
			continue
		}
		var entry struct {
			Action string `json:"action"`
			Before struct {
				Ref    string `json:"ref"`
				Status string `json:"status"`
			} `json:"before"`
			After struct {
				Ref    string `json:"ref"`
				Status string `json:"status"`
			} `json:"after"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("reading the trail: %v\n%s", err, line)
		}
		if entry.Action != "task.transition" || entry.After.Ref != ref {
			continue
		}
		out = append(out, entry.Before.Status+">"+entry.After.Status)
	}
	return out
}

// forkedWorkflowDoc reaches done from todo two ways in the same number of
// steps, which is the case --hops refuses rather than decides.
const forkedWorkflowDoc = `
key: default
name: Default
definition:
  initial: todo
  states:
    - {key: todo, label: Todo}
    - {key: review, label: Review}
    - {key: qa, label: QA}
    - {key: done, label: Done, terminal: true}
  transitions:
    - {from: todo, to: review}
    - {from: todo, to: qa}
    - {from: review, to: done}
    - {from: qa, to: done}
`

// TestHopsPrintsTheRouteAndAppliesItOneHopAtATime is the command's half of the
// guarantee: the route is named before anything is written, and the trail
// afterwards holds one entry per hop rather than one jump.
func TestHopsPrintsTheRouteAndAppliesItOneHopAtATime(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "walk me")

	got := c.mustRun("task", "mv", "default-1", "done", "--hops")
	if !strings.Contains(got.err, "default-1: todo -> doing -> done (2 steps)") {
		t.Fatalf("the route was not printed before acting:\n%s", got.err)
	}
	if hops := transitionHops(t, c, "default-1"); strings.Join(hops, ",") != "todo>doing,doing>done" {
		t.Fatalf("the trail holds %v, want one entry per hop", hops)
	}
}

// TestHopsDryRunPrintsTheRouteAndWritesNothing is the other half: the route a
// dry run shows is the route a real run takes, and showing it costs nothing.
func TestHopsDryRunPrintsTheRouteAndWritesNothing(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "leave me alone")

	got := c.mustRun("task", "mv", "default-1", "done", "--hops", "--dry-run", "-o", "json")
	if !strings.Contains(got.err, "default-1: todo -> doing -> done (2 steps)") {
		t.Fatalf("a dry run printed no route:\n%s", got.err)
	}
	if !strings.Contains(got.err, "dry run, nothing written") {
		t.Fatalf("a dry run did not say it wrote nothing:\n%s", got.err)
	}
	var outcomes []routeOutcome
	if err := json.Unmarshal([]byte(got.out), &outcomes); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	if len(outcomes) != 1 || outcomes[0].Status != statusPlanned || !outcomes[0].DryRun {
		t.Fatalf("outcomes = %+v, want one planned dry run", outcomes)
	}
	if strings.Join(outcomes[0].Route, ">") != "doing>done" {
		t.Fatalf("route = %v, want the route it would take", outcomes[0].Route)
	}
	if len(outcomes[0].Applied) != 0 {
		t.Fatalf("a dry run reported %v as applied", outcomes[0].Applied)
	}

	// Nothing was written: not the task's state, and not the trail.
	shown := c.mustRun("task", "show", "default-1", "-o", "json")
	var task core.Task
	if err := json.Unmarshal([]byte(shown.out), &task); err != nil {
		t.Fatalf("not json: %v\n%s", err, shown.out)
	}
	if task.Status != "todo" {
		t.Fatalf("a dry run moved the task to %q", task.Status)
	}
	if hops := transitionHops(t, c, "default-1"); len(hops) != 0 {
		t.Fatalf("a dry run wrote %v to the trail", hops)
	}
}

// TestHopsReportsTheJourneyInJSON is what a script reads to see the route that
// was actually taken rather than inferring it from the final state.
func TestHopsReportsTheJourneyInJSON(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "scripted")

	got := c.mustRun("task", "mv", "default-1", "done", "--hops", "-o", "json")
	var outcomes []routeOutcome
	if err := json.Unmarshal([]byte(got.out), &outcomes); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	if len(outcomes) != 1 {
		t.Fatalf("outcomes = %+v", outcomes)
	}
	o := outcomes[0]
	switch {
	case o.Status != statusOK:
		t.Fatalf("status = %q", o.Status)
	case o.From != "todo":
		t.Errorf("from = %q, want todo", o.From)
	case strings.Join(o.Applied, ">") != "doing>done":
		t.Errorf("applied = %v, want the hops it made", o.Applied)
	case o.Hops != 2:
		t.Errorf("hops = %d, want 2", o.Hops)
	case o.Stopped != "":
		t.Errorf("stopped = %q, want nothing", o.Stopped)
	}
}

// TestHopsRefusesAnAmbiguousDestination holds the decision not to pick, on the
// surface where a caller can act on the refusal.
func TestHopsRefusesAnAmbiguousDestination(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "forked")
	c.runIn(forkedWorkflowDoc, "workflow", "put", "-f", "-")

	got := c.run("task", "mv", "default-1", "done", "--hops")
	if got.code == core.ExitOK {
		t.Fatalf("an ambiguous destination succeeded:\n%s\n%s", got.out, got.err)
	}
	for _, want := range []string{"review>done", "qa>done", "name the route you want"} {
		if !strings.Contains(got.out+got.err, want) {
			t.Errorf("the refusal does not name %q:\n%s\n%s", want, got.out, got.err)
		}
	}
	if hops := transitionHops(t, c, "default-1"); len(hops) != 0 {
		t.Fatalf("a refused route wrote %v", hops)
	}

	// Naming the route is what the refusal asks for, and it works.
	named := c.mustRun("task", "mv", "default-1", "qa>done", "--hops")
	if !strings.Contains(named.err, "default-1: todo -> qa -> done (2 steps)") {
		t.Fatalf("the named route was not printed:\n%s", named.err)
	}
	if hops := transitionHops(t, c, "default-1"); strings.Join(hops, ",") != "todo>qa,qa>done" {
		t.Fatalf("the trail holds %v, want the route that was named", hops)
	}
}

// TestASingleHopMoveIsUnchangedWithoutTheSwitch keeps the default behaviour
// where it was: one state, one transition, and no route found for anybody.
func TestASingleHopMoveIsUnchangedWithoutTheSwitch(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "plain")

	if got := c.run("task", "mv", "default-1", "done"); got.code != core.ExitPrecondtion {
		t.Fatalf("a move to an unreachable state exited %d, want %d", got.code, core.ExitPrecondtion)
	}
	if hops := transitionHops(t, c, "default-1"); len(hops) != 0 {
		t.Fatalf("a refused single move wrote %v", hops)
	}
	c.mustRun("task", "mv", "default-1", "doing")
	if hops := transitionHops(t, c, "default-1"); strings.Join(hops, ",") != "todo>doing" {
		t.Fatalf("the trail holds %v, want exactly one hop", hops)
	}
}
