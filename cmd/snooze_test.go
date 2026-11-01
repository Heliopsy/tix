// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestTaskSnoozeHidesTheTaskUntilItWakes(t *testing.T) {
	c := newCLI(t)
	for _, title := range []string{"one", "two"} {
		c.mustRun("task", "add", title)
	}
	got := c.mustRun("task", "snooze", "default-2", "tomorrow", "-o", "json")
	var task core.Task
	if err := json.Unmarshal([]byte(got.out), &task); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	local := time.Now().Local()
	morning := time.Date(local.Year(), local.Month(), local.Day()+1, 9, 0, 0, 0, time.Local)
	if task.SnoozedUntil == nil || !task.SnoozedUntil.Equal(morning) {
		t.Fatalf("snoozed until %v, want 09:00 tomorrow local, %v", task.SnoozedUntil, morning)
	}

	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"default", nil, []string{"default-1"}},
		{"--snoozed", []string{"--snoozed"}, []string{"default-2"}},
		{"--snoozed=false", []string{"--snoozed=false"}, []string{"default-1"}},
		{"--include-snoozed", []string{"--include-snoozed"}, []string{"default-1", "default-2"}},
		{"is:snoozed", []string{"--filter", "is:snoozed"}, []string{"default-2"}},
		{"snoozed:any", []string{"--filter", "snoozed:any"}, []string{"default-1", "default-2"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if refs, _ := listedRefs(t, c, tc.args...); !slices.Equal(refs, tc.want) {
				t.Fatalf("listed %v, want %v", refs, tc.want)
			}
		})
	}

	show := c.mustRun("task", "show", "default-2")
	if !strings.Contains(show.err, "snoozed until: ") {
		t.Fatalf("task show does not say the task is snoozed:\n%s", show.err)
	}
	if got := c.run("claim", "task", "default-2"); got.code != core.ExitPrecondtion {
		t.Fatalf("claiming a snoozed task exited %d, want %d:\n%s", got.code, core.ExitPrecondtion, got.err)
	}

	c.mustRun("task", "unsnooze", "default-2")
	if refs, _ := listedRefs(t, c); !slices.Equal(refs, []string{"default-1", "default-2"}) {
		t.Fatalf("after unsnooze listed %v", refs)
	}
	if show := c.mustRun("task", "show", "default-2"); strings.Contains(show.err, "snoozed until") {
		t.Fatalf("task show still says snoozed:\n%s", show.err)
	}
	c.mustRun("task", "unsnooze", "default-2")
}

func TestTaskSnoozeRefusals(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "one")
	cases := []struct {
		name string
		args []string
		code int
	}{
		{"unreadable time", []string{"task", "snooze", "default-1", "someday"}, core.ExitUsage},
		{"a past time", []string{"task", "snooze", "default-1", "2001-01-01"}, core.ExitUsage},
		{"an unknown task", []string{"task", "snooze", "missing-1", "2h"}, core.ExitNotFound},
		{"no time", []string{"task", "snooze", "default-1"}, core.ExitUsage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.run(tc.args...); got.code != tc.code {
				t.Fatalf("exited %d, want %d:\n%s", got.code, tc.code, got.err)
			}
		})
	}
	c.mustRun("task", "snooze", "default-1", "2h", "--dry-run")
	if refs, _ := listedRefs(t, c); !slices.Equal(refs, []string{"default-1"}) {
		t.Fatalf("a dry run snoozed the task: listed %v", refs)
	}
}

func TestTaskWakeReportsWhatItWoke(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("task", "wake", "-o", "json")
	var out wakeResult
	if err := json.Unmarshal([]byte(got.out), &out); err != nil || out.Woken != 0 {
		t.Fatalf("wake = %+v, %v\n%s", out, err, got.out)
	}
}
