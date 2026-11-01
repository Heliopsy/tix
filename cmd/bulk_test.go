// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/csv"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func outcomes(t *testing.T, out string) []taskOutcome {
	t.Helper()
	var rows []taskOutcome
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("not json: %v\n%s", err, out)
	}
	return rows
}

func TestTaskBulk(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("project", "create", "web", "Web")
	for _, title := range []string{"one", "two", "three"} {
		c.mustRun("task", "add", "-p", "infra", title)
	}

	t.Run("priority in one request", func(t *testing.T) {
		rows := outcomes(t, c.mustRun("task", "bulk", "priority", "infra-1", "infra-2", "--priority", "high", "-o", "json").out)
		if len(rows) != 2 || rows[0].Status != statusOK || rows[1].Status != statusOK {
			t.Fatalf("outcomes = %+v", rows)
		}
		for _, ref := range []string{"infra-1", "infra-2"} {
			if got := showTask(t, c, ref); got.Priority != core.PriorityHigh {
				t.Errorf("%s priority = %d", ref, got.Priority)
			}
		}
		if got := showTask(t, c, "infra-3"); got.Priority != core.PriorityNormal {
			t.Errorf("infra-3 changed: %d", got.Priority)
		}
	})
	t.Run("a refusal is reported and fails the command", func(t *testing.T) {
		got := c.run("task", "bulk", "tag-add", "infra-1", "infra-99", "--tag", "ops", "-o", "json")
		if got.code != core.ExitNotFound {
			t.Fatalf("exit = %d, want %d\n%s", got.code, core.ExitNotFound, got.err)
		}
		rows := outcomes(t, got.out)
		if rows[0].Status != statusOK || rows[1].Status != statusFailed || rows[1].Code != string(core.KindNotFound) {
			t.Fatalf("outcomes = %+v", rows)
		}
		if !slices.Contains(showTask(t, c, "infra-1").Tags, "ops") {
			t.Fatal("the refusal of infra-99 held back infra-1")
		}
	})
	t.Run("copy names the copies", func(t *testing.T) {
		rows := outcomes(t, c.mustRun("task", "bulk", "copy", "infra-3", "--project", "web", "-o", "json").out)
		if rows[0].Copy != "web-1" {
			t.Fatalf("outcomes = %+v, want the copy named", rows)
		}
	})
	t.Run("an unknown action is a usage error", func(t *testing.T) {
		if got := c.run("task", "bulk", "explode", "infra-1"); got.code != core.ExitUsage {
			t.Fatalf("exit = %d, want usage", got.code)
		}
	})
}

func TestManyRefsWhereOneWasTaken(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("project", "create", "web", "Web")
	for _, title := range []string{"one", "two", "three"} {
		c.mustRun("task", "add", "-p", "infra", title)
	}

	rows := outcomes(t, c.mustRun("task", "edit", "infra-1", "infra-2", "--priority", "low", "-o", "json").out)
	if len(rows) != 2 || showTask(t, c, "infra-2").Priority != core.PriorityLow {
		t.Fatalf("task edit with two refs: %+v", rows)
	}
	for _, args := range [][]string{{"--title", "x"}, {"--version", "1"}} {
		got := c.run(append([]string{"task", "edit", "infra-1", "infra-2"}, args...)...)
		if got.code != core.ExitUsage {
			t.Errorf("task edit two refs %v: exit = %d, want invalid", args, got.code)
		}
	}

	c.mustRun("tag", "add", "infra-1,infra-3", "ops", "sec")
	for _, ref := range []string{"infra-1", "infra-3"} {
		tags := showTask(t, c, ref).Tags
		if !slices.Contains(tags, "ops") || !slices.Contains(tags, "sec") {
			t.Errorf("%s tags = %v after tag add with a list", ref, tags)
		}
	}
	c.mustRun("tag", "rm", "infra-1", "infra-3", "ops")
	for _, ref := range []string{"infra-1", "infra-3"} {
		if slices.Contains(showTask(t, c, ref).Tags, "ops") {
			t.Errorf("%s still tagged ops after tag rm with two refs", ref)
		}
	}

	rows = outcomes(t, c.mustRun("task", "copy", "infra-1", "infra-2", "--to", "web", "-o", "json").out)
	if len(rows) != 2 || showTask(t, c, "web-2").Title != "two" {
		t.Fatalf("task copy with two refs: %+v", rows)
	}
	if got := c.run("task", "copy", "infra-1", "infra-2", "--title", "x"); got.code != core.ExitUsage {
		t.Errorf("task copy two refs --title: exit = %d, want invalid", got.code)
	}
}

func TestTaskLsCSV(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "=SUM(A1)", "--tag", "ops", "--tag", "-x")
	c.mustRun("task", "add", `say "hi", then go`)

	got := c.mustRun("task", "ls", "-o", "csv", "--sort", "seq")
	records, err := csv.NewReader(strings.NewReader(got.out)).ReadAll()
	if err != nil {
		t.Fatalf("not csv: %v\n%s", err, got.out)
	}
	header := "ref,project,title,status,priority,assignee,tags,due,created,updated"
	if strings.Join(records[0], ",") != header {
		t.Fatalf("header = %v", records[0])
	}
	if len(records) != 3 {
		t.Fatalf("rows = %d, want a header and two tasks\n%s", len(records), got.out)
	}
	if records[1][0] != "default-1" || records[1][1] != "default" || records[1][2] != "'=SUM(A1)" {
		t.Errorf("first row = %v, want the formula title defused", records[1])
	}
	if records[1][6] != "ops -x" && records[1][6] != "'-x ops" {
		t.Errorf("tags cell = %q", records[1][6])
	}
	if records[2][2] != `say "hi", then go` {
		t.Errorf("quoted title = %q", records[2][2])
	}

	if got := c.run("project", "ls", "-o", "csv"); got.code != core.ExitUsage || !strings.Contains(got.err, "not supported") {
		t.Fatalf("project ls -o csv: exit = %d\n%s", got.code, got.err)
	}
}
