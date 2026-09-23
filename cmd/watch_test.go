// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// watchOne follows the stream from the beginning and stops at the first event,
// which is what keeps the command bounded in a test.
func watchOne(t *testing.T, c *cli, extra ...string) core.Event {
	t.Helper()
	args := append([]string{"watch", "--since", "1", "--limit", "1", "-o", "ndjson"}, extra...)
	out := c.mustRun(args...).out
	var event core.Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &event); err != nil {
		t.Fatalf("watch is not ndjson: %v\n%s", err, out)
	}
	return event
}

// Resuming after a sequence number replays what the stream already holds, and
// --limit ends the command without an interrupt.
func TestWatchResumesFromASequenceNumberAndStopsAtTheLimit(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "buy milk")

	event := watchOne(t, c)
	if event.Seq <= 1 {
		t.Fatalf("seq = %d, want an event after the one resumed from", event.Seq)
	}
	if event.Type == "" || event.TenantID == "" {
		t.Fatalf("event = %+v, want a complete record", event)
	}
}

// A type filter selects the events an agent asked for and skips the rest.
func TestWatchFiltersByEventType(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "buy milk")

	for _, pattern := range []string{"task.created", "task.*"} {
		t.Run(pattern, func(t *testing.T) {
			event := watchOne(t, c, "--type", pattern)
			if event.Type != core.EventTaskCreated {
				t.Fatalf("type = %q, want task.created", event.Type)
			}
		})
	}
}

// Every format renders a followed event. The default, table, prints a
// readable line rather than the raw event type; the machine formats still
// carry the type verbatim.
func TestWatchRendersInEveryFormat(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "buy milk")

	tests := []struct {
		format string
		want   string
	}{
		{"table", "created"},
		{"json", "task.created"},
		{"yaml", "task.created"},
		{"ndjson", "task.created"},
	}
	for _, tc := range tests {
		t.Run(tc.format, func(t *testing.T) {
			got := c.mustRun("watch", "--since", "1", "--limit", "1",
				"--type", "task.created", "-o", tc.format)
			if !strings.Contains(strings.ToLower(got.out), tc.want) {
				t.Fatalf("%s output = %q, want it to contain %q", tc.format, got.out, tc.want)
			}
		})
	}
}

// The default format streams a human-readable line per event: a clock time,
// the actor, the verb and the task ref, rather than buffering into a table
// that a live tail would never finish sizing.
func TestWatchDefaultFormatIsAReadableLinePerEvent(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "buy milk")

	got := c.mustRun("watch", "--since", "1", "--limit", "1", "--type", "task.created")
	line := strings.TrimSpace(got.out)
	if strings.Count(got.out, "\n") != 1 {
		t.Fatalf("watch output = %q, want exactly one line", got.out)
	}
	for _, want := range []string{"created", "buy milk"} {
		if !strings.Contains(line, want) {
			t.Fatalf("line = %q, missing %q", line, want)
		}
	}
}

// An actor filter selects the events that actor caused. core.EventFilter has
// no actor field, so this is an end-to-end check that the flag reaches a real
// stream: every event a zero-config CLI produces carries the same local
// actor, so filtering on that actor still returns the event.
func TestWatchFiltersByActor(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "buy milk")
	first := watchOne(t, c, "--type", "task.created")
	if first.ActorID == "" {
		t.Fatal("event carries no actor to filter by")
	}

	c.mustRun("task", "add", "buy bread")
	event := watchOne(t, c, "--since", strconv.FormatInt(first.Seq, 10),
		"--type", "task.created", "--actor", first.ActorID)
	if event.ActorID != first.ActorID {
		t.Fatalf("actor = %q, want %q", event.ActorID, first.ActorID)
	}
}

// filterByActor is the client-side filter watch applies because
// core.EventFilter carries no actor field. It is unit tested against a
// synthetic channel rather than a live subscription, because a channel that
// never yields a match would block the test forever.
func TestFilterByActorKeepsOnlyMatchingActors(t *testing.T) {
	in := make(chan core.Event, 3)
	in <- core.Event{ActorID: "alice"}
	in <- core.Event{ActorID: "bob"}
	in <- core.Event{ActorID: "alice"}
	close(in)

	var got []string
	for e := range filterByActor(in, []string{"alice"}) {
		got = append(got, e.ActorID)
	}
	if len(got) != 2 || got[0] != "alice" || got[1] != "alice" {
		t.Fatalf("got = %v, want two events from alice", got)
	}
}

// No actor flag means no filtering: the channel passes through unchanged.
func TestFilterByActorPassesEverythingWhenNoneWereRequested(t *testing.T) {
	in := make(chan core.Event, 1)
	in <- core.Event{ActorID: "alice"}
	close(in)
	if _, ok := <-filterByActor(in, nil); !ok {
		t.Fatal("the event did not pass through unfiltered")
	}
}

// Establishing the stream prints a banner to standard error, not standard
// output, so a pipeline reading ndjson off stdout never sees it.
func TestWatchPrintsAStartupBannerToStderr(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "buy milk")

	got := c.mustRun("watch", "--since", "1", "--limit", "1", "--type", "task.created")
	if !strings.Contains(got.err, "watching") {
		t.Fatalf("stderr = %q, want a startup banner", got.err)
	}
	if !strings.Contains(got.err, "ctrl-c to stop") {
		t.Fatalf("stderr = %q, want it to say how to stop", got.err)
	}
	if strings.Contains(got.out, "watching") {
		t.Fatalf("stdout = %q, the banner leaked into the event stream", got.out)
	}
}

// The banner names an active filter, so a person watching a narrow slice of
// the stream can tell that is what they asked for.
func TestWatchBannerNamesTheFilter(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "buy milk")

	got := c.mustRun("watch", "--since", "1", "--limit", "1", "--type", "task.created")
	if !strings.Contains(got.err, "type=task.created") {
		t.Fatalf("stderr = %q, want it to name the type filter", got.err)
	}
}

// --quiet suppresses the banner the same way it suppresses every other
// diagnostic, and ndjson stays exactly parseable either way.
func TestWatchBannerRespectsQuiet(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "buy milk")

	got := c.mustRun("watch", "--since", "1", "--limit", "1", "-o", "ndjson", "--quiet")
	if strings.TrimSpace(got.err) != "" {
		t.Fatalf("stderr = %q, want nothing under --quiet", got.err)
	}
	var event core.Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(got.out)), &event); err != nil {
		t.Fatalf("watch is not ndjson: %v\n%s", err, got.out)
	}
}

// The banner never appears when the command exits before the stream is
// established: a bad filter fails before dialling, and a dial failure fails
// before subscribing.
func TestWatchBannerDoesNotAppearWhenTheCommandNeverConnects(t *testing.T) {
	c := newCLI(t)
	got := c.run("watch", "--since", "-1")
	if strings.Contains(got.err, "watching") {
		t.Fatalf("stderr = %q, want no banner for a filter the command refused", got.err)
	}
}

// A filter the service or the command refuses fails before anything streams.
func TestWatchRejectsABadFilter(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "negative since", args: []string{"--since", "-1"}, wantErr: "--since"},
		{name: "negative limit", args: []string{"--limit", "-1"}, wantErr: "--limit"},
		{name: "positional argument", args: []string{"events"}, wantErr: "takes no arguments"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			got := c.run(append([]string{"watch"}, tc.args...)...)
			if got.code != core.ExitUsage {
				t.Fatalf("exit = %d, want %d\n%s", got.code, core.ExitUsage, got.err)
			}
			if !strings.Contains(got.err, tc.wantErr) {
				t.Fatalf("stderr = %q, want it to mention %q", got.err, tc.wantErr)
			}
		})
	}
}

// The machine formats are what a monitoring agent reads, so every field it
// would otherwise have to go back and query for has to be on the line: the
// resume cursor, who acted, what the subject is, and the whole payload.
func TestWatchNDJSONCarriesTheWholeEvent(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "buy milk")

	event := watchOne(t, c, "--type", "task.created")
	switch {
	case event.Seq == 0:
		t.Error("event has no sequence number, so a watcher cannot record its position")
	case event.ID == "":
		t.Error("event has no id")
	case event.TenantID == "":
		t.Error("event has no tenant")
	case event.ActorID == "":
		t.Error("event has no actor")
	case event.SubjectType != "task" || event.SubjectID == "":
		t.Errorf("subject = %q/%q, want a typed identifier", event.SubjectType, event.SubjectID)
	case event.OccurredAt.IsZero():
		t.Error("event has no timestamp")
	}
	for _, key := range []string{"ref", "title", "actor_handle"} {
		if _, ok := event.Payload[key]; !ok {
			t.Errorf("payload %v is missing %q; a watcher would have to query for it", event.Payload, key)
		}
	}
}

// The default line is what a person sees, and it has to carry the cursor or
// the resume story only works for whoever already parses ndjson.
func TestWatchDefaultLineCarriesTheResumeCursor(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "buy milk")

	out := c.mustRun("watch", "--since", "1", "--limit", "1", "--type", "task.created").out
	line := strings.TrimSpace(out)
	seq := strings.Fields(line)
	if len(seq) == 0 {
		t.Fatalf("watch printed nothing")
	}
	n, err := strconv.Atoi(seq[0])
	if err != nil || n <= 0 {
		t.Fatalf("line %q does not lead with a sequence number a watcher can resume from", line)
	}
}

// A cursor the retention sweep has already passed cannot be resumed from, and
// a watcher told nothing would wait forever believing it was caught up. Both
// the direct and the served path refuse it; this pins the CLI's half.
func TestWatchRejectsANegativeCursor(t *testing.T) {
	c := newCLI(t)
	if got := c.run("watch", "--since", "-1"); got.code == core.ExitOK {
		t.Fatalf("watch accepted a negative cursor: %s", got.out)
	}
}
