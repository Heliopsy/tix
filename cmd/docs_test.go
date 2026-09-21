package cmd

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestDocsTableGroupsEveryTopLevelCommand(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("docs", "--table")

	for _, want := range []string{
		"### Working with tasks",
		"### Administration",
		"### Configuration and tooling",
		"| Command | Does |",
		"| `tix task` | Create, list and change tasks |",
		"| `tix bundle` |",
		"| `tix serve` |",
	} {
		if !strings.Contains(got.out, want) {
			t.Errorf("table is missing %q\n%s", want, got.out)
		}
	}
	if strings.Contains(got.out, "`tix help`") {
		t.Error("table lists the help command")
	}
	if strings.Contains(got.out, "`tix task add`") {
		t.Error("depth 1 table lists a subcommand")
	}
}

func TestDocsTableDepthIncludesSubcommands(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("docs", "--table", "--depth", "2")

	for _, want := range []string{"`tix task add`", "`tix claim exec`", "`tix bundle export`"} {
		if !strings.Contains(got.out, want) {
			t.Errorf("depth 2 table is missing %s\n%s", want, got.out)
		}
	}
	if strings.Contains(got.out, "`tix sync source add`") {
		t.Error("depth 2 table reaches a third level")
	}
}

func TestDocsTableRejectsUnusableFlags(t *testing.T) {
	c := newCLI(t)

	if got := c.run("docs", "--table", "--dir", c.home); got.code != core.ExitUsage {
		t.Errorf("--table with --dir exited %d, want %d", got.code, core.ExitUsage)
	}
	if got := c.run("docs", "--table", "--depth", "0"); got.code != core.ExitUsage {
		t.Errorf("--depth 0 exited %d, want %d", got.code, core.ExitUsage)
	}
}

func TestDocsDefaultStillEmitsThePages(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("docs")

	for _, want := range []string{"# tix", "## tix task", "### tix task add"} {
		if !strings.Contains(got.out, want) {
			t.Errorf("page output is missing %q", want)
		}
	}
}
