// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func viewPrefs(t *testing.T, c *cli, view string) map[string]core.GroupPref {
	t.Helper()
	got := c.mustRun("view", "ls", view, "-o", "json")
	var prefs []core.GroupPref
	if err := json.Unmarshal([]byte(got.out), &prefs); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	out := map[string]core.GroupPref{}
	for _, p := range prefs {
		out[p.Group] = p
	}
	return out
}

func TestViewSetChangesOnlyTheFlagsGiven(t *testing.T) {
	c := newCLI(t)
	c.mustRun("view", "set", "board", "done", "--hidden", "--starred")
	c.mustRun("view", "set", "board", "done", "--starred=false", "--collapsed")
	got := viewPrefs(t, c, "board")["done"]
	want := core.GroupPref{View: "board", Group: "done", Hidden: true, Collapsed: true}
	if got != want {
		t.Fatalf("done = %+v, want %+v", got, want)
	}

	c.mustRun("view", "set", "lanes", "work", "--starred")
	c.mustRun("view", "order", "lanes", "homelab", "work")
	lanes := viewPrefs(t, c, "lanes")
	if lanes["homelab"].Position != 1 || lanes["work"].Position != 2 || !lanes["work"].Starred {
		t.Fatalf("lanes = %+v", lanes)
	}

	c.mustRun("view", "reset", "lanes", "homelab")
	if lanes := viewPrefs(t, c, "lanes"); len(lanes) != 1 {
		t.Fatalf("after resetting homelab: %+v", lanes)
	}
	c.mustRun("view", "reset", "lanes")
	if lanes := viewPrefs(t, c, "lanes"); len(lanes) != 0 {
		t.Fatalf("after resetting the view: %+v", lanes)
	}
}

func TestViewDryRunsAndRefusals(t *testing.T) {
	c := newCLI(t)
	c.mustRun("view", "set", "board", "done", "--hidden", "--dry-run")
	c.mustRun("view", "order", "board", "doing", "--dry-run")
	if prefs := viewPrefs(t, c, "board"); len(prefs) != 0 {
		t.Fatalf("a dry run wrote %+v", prefs)
	}
	c.mustRun("view", "set", "board", "done", "--hidden")
	c.mustRun("view", "reset", "board", "--dry-run")
	if prefs := viewPrefs(t, c, "board"); len(prefs) != 1 {
		t.Fatalf("a dry reset wrote: %+v", prefs)
	}

	cases := []struct {
		name string
		args []string
		code int
	}{
		{"set with no flag", []string{"view", "set", "board", "todo"}, core.ExitUsage},
		{"unknown view", []string{"view", "ls", "calendar"}, core.ExitUsage},
		{"unknown project board", []string{"view", "ls", "project:nope"}, core.ExitNotFound},
		{"a state no workflow has", []string{"view", "set", "board", "shipped", "--hidden"}, core.ExitUsage},
		{"a group named twice", []string{"view", "order", "board", "todo", "todo"}, core.ExitUsage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.run(tc.args...); got.code != tc.code {
				t.Fatalf("exited %d, want %d\n%s", got.code, tc.code, got.err)
			}
		})
	}
	if prefs := viewPrefs(t, c, "board"); len(prefs) != 1 || !prefs["done"].Hidden {
		t.Fatalf("a refusal wrote: %+v", prefs)
	}
}
