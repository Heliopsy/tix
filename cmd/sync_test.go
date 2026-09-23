// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	extsync "github.com/heliopsy/tix/internal/sync"
)

const cliMapping = `
version: 1
project: default
identity:
  id: id
  version: updated
  updated_at: updated
fields:
  title: title
status_field: status
statuses:
  Open: todo
default_status: todo
`

const cliRows = `id,title,status,updated
C-1,imported from a file,Open,2026-01-01T00:00:00Z
`

// configureSource registers a source and points the environment at a mapping
// and a file, returning the source identifier.
func configureSource(t *testing.T, c *cli, name string) string {
	t.Helper()
	dir := t.TempDir()
	mapping := filepath.Join(dir, "mapping.yaml")
	if err := os.WriteFile(mapping, []byte(cliMapping), 0o600); err != nil {
		t.Fatalf("writing mapping: %v", err)
	}
	rows := filepath.Join(dir, "rows.csv")
	if err := os.WriteFile(rows, []byte(cliRows), 0o600); err != nil {
		t.Fatalf("writing rows: %v", err)
	}
	t.Setenv(extsync.EnvName(name, extsync.EnvMapping), mapping)
	t.Setenv(extsync.EnvName(name, extsync.EnvFile), rows)

	got := c.mustRun("-o", "json", "sync", "source", "add", core.SystemGeneric, name)
	var source core.SyncSource
	if err := json.Unmarshal([]byte(got.out), &source); err != nil {
		t.Fatalf("decoding source: %v\n%s", err, got.out)
	}
	if source.ID == "" {
		t.Fatalf("source has no identifier: %s", got.out)
	}
	return source.ID
}

func TestSyncCommandGroupsPrintHelp(t *testing.T) {
	c := newCLI(t)
	for _, args := range [][]string{{"sync"}, {"sync", "source"}} {
		got := c.mustRun(args...)
		if !strings.Contains(got.out, "Available Commands") {
			t.Errorf("tix %s printed %q, want help", strings.Join(args, " "), got.out)
		}
	}
}

func TestSyncSourceLifecycleThroughTheCLI(t *testing.T) {
	c := newCLI(t)
	id := configureSource(t, c, "cliops")

	listed := c.mustRun("-o", "json", "sync", "source", "ls")
	if !strings.Contains(listed.out, "cliops") {
		t.Errorf("listing = %q, want the registered source", listed.out)
	}

	plan := c.mustRun("-o", "json", "sync", "source", "add", core.SystemJira, "planned", "--dry-run")
	if !strings.Contains(plan.out, "planned") {
		t.Errorf("dry run = %q", plan.out)
	}
	if after := c.mustRun("-o", "json", "sync", "source", "ls"); strings.Contains(after.out, "planned") {
		t.Errorf("a dry run registered a source: %q", after.out)
	}

	if got := c.mustRun("-o", "json", "sync", "source", "rm", id, "--dry-run"); !strings.Contains(got.out, id) {
		t.Errorf("dry run removal = %q", got.out)
	}
	c.mustRun("sync", "source", "rm", id)
	if after := c.mustRun("-o", "json", "sync", "source", "ls"); strings.Contains(after.out, "cliops") {
		t.Errorf("the source survived removal: %q", after.out)
	}
}

func TestSyncRunImportsAndSupportsDryRunAndFull(t *testing.T) {
	c := newCLI(t)
	id := configureSource(t, c, "cliops")

	dry := c.mustRun("-o", "json", "sync", "run", id, "--dry-run")
	var planned core.SyncResult
	if err := json.Unmarshal([]byte(dry.out), &planned); err != nil {
		t.Fatalf("decoding dry run: %v\n%s", err, dry.out)
	}
	if !planned.DryRun || planned.Created["task"] != 1 {
		t.Fatalf("dry run = %+v", planned)
	}
	if !strings.Contains(dry.err, "nothing was written") {
		t.Errorf("stderr = %q, want the dry run announced", dry.err)
	}
	if listed := c.mustRun("-o", "json", "task", "ls"); strings.Contains(listed.out, "imported from a file") {
		t.Error("a dry run created a task")
	}

	real := c.mustRun("-o", "json", "sync", "run", id)
	var done core.SyncResult
	if err := json.Unmarshal([]byte(real.out), &done); err != nil {
		t.Fatalf("decoding run: %v\n%s", err, real.out)
	}
	if done.Created["task"] != 1 {
		t.Fatalf("run = %+v", done)
	}
	if listed := c.mustRun("-o", "json", "task", "ls"); !strings.Contains(listed.out, "imported from a file") {
		t.Errorf("the imported task is missing: %q", listed.out)
	}

	again := c.mustRun("-o", "json", "sync", "run", id, "--full")
	var repeat core.SyncResult
	if err := json.Unmarshal([]byte(again.out), &repeat); err != nil {
		t.Fatalf("decoding second run: %v\n%s", err, again.out)
	}
	if repeat.Created["task"] != 0 || repeat.Skipped["task"] != 1 {
		t.Errorf("second run = %+v, want the record skipped and nothing duplicated", repeat)
	}
}

func TestSyncCommandsReportBadInput(t *testing.T) {
	c := newCLI(t)

	bad := c.run("sync", "source", "add", "trello", "x")
	if bad.code != core.KindInvalid.ExitCode() {
		t.Errorf("unknown system exited %d, want %d", bad.code, core.KindInvalid.ExitCode())
	}
	missing := c.run("sync", "run", "no-such-source")
	if missing.code != core.KindNotFound.ExitCode() {
		t.Errorf("unknown source exited %d, want %d", missing.code, core.KindNotFound.ExitCode())
	}
	if got := c.run("sync", "source", "add", "generic"); got.code != core.ExitUsage {
		t.Errorf("missing argument exited %d, want %d", got.code, core.ExitUsage)
	}
}
