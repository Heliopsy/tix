// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/migrations"
)

// raw is a connection past the store, for arranging a counter the store would
// never leave behind and for reading it back without NextSeq moving it.
func (f fixture) raw() *sql.DB { return f.store.db }

// seqCounter reads a project's counter directly.
func seqCounter(t *testing.T, db *sql.DB, projectID string) int64 {
	t.Helper()
	var n int64
	if err := db.QueryRowContext(context.Background(),
		rebindTest("SELECT seq_counter FROM projects WHERE id = ?"), projectID).Scan(&n); err != nil {
		t.Fatalf("reading the counter of project %q: %v", projectID, err)
	}
	return n
}

// setSeqCounter winds a project's counter to n directly.
func setSeqCounter(t *testing.T, db *sql.DB, projectID string, n int64) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(),
		rebindTest("UPDATE projects SET seq_counter = ? WHERE id = ?"), n, projectID); err != nil {
		t.Fatalf("setting the counter of project %q: %v", projectID, err)
	}
}

// numberedTask writes a task under a number of its own, as an import does.
func (f fixture) numberedTask(t *testing.T, projectID string, seq int64, trashed bool) core.Task {
	t.Helper()
	ctx := context.Background()
	task := core.Task{ProjectID: projectID, Seq: seq, Title: "imported", Status: "todo", CreatorActorID: f.actor.ID}
	if trashed {
		at := f.clock.Now()
		task.DeletedAt = &at
	}
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateTask(ctx, &task)
	}); err != nil {
		t.Fatalf("creating task numbered %d: %v", seq, err)
	}
	return task
}

// migrationNamed returns the version of the named migration.
func migrationNamed(t *testing.T, name string) int {
	t.Helper()
	all, err := migrations.All()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range all {
		if m.Name == name {
			return m.Version
		}
	}
	t.Fatalf("no %s migration", name)
	return 0
}

// An import writes tasks under their original numbers. The counter has to
// follow, trashed tasks included, and never move back for a lower number.
func TestAnExplicitlyNumberedTaskRaisesTheCounter(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	f.numberedTask(t, f.project.ID, 95, true)
	if got := seqCounter(t, f.raw(), f.project.ID); got != 95 {
		t.Fatalf("counter after importing a trashed task numbered 95 = %d, want 95", got)
	}
	f.numberedTask(t, f.project.ID, 3, false)
	if got := seqCounter(t, f.raw(), f.project.ID); got != 95 {
		t.Fatalf("counter after importing task 3 = %d, want it left at 95", got)
	}
	if got := f.newTask(t, "after the import", core.PriorityNormal).Seq; got != 96 {
		t.Fatalf("task created after the import is numbered %d, want 96", got)
	}
}

// A counter already behind its tasks, as imports before the fix left it, does
// not make task creation reuse a number: the next one steps past the highest.
func TestNextSeqStepsPastATaskHeldAboveTheCounter(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	f.numberedTask(t, f.project.ID, 868, true)
	setSeqCounter(t, f.raw(), f.project.ID, 0)
	if got := nextSeq(t, f, f.project.ID); got != 869 {
		t.Fatalf("next number with the counter at 0 and task 868 held = %d, want 869", got)
	}
	if got := nextSeq(t, f, f.project.ID); got != 870 {
		t.Fatalf("number after that = %d, want 870", got)
	}
}

// Doctor reads this list through the status report.
func TestNumberingBehindNamesOnlyTheProjectsBehind(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	other := core.Project{Key: "beta", Name: "Beta", WorkflowID: f.workflow.ID}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tx.CreateProject(ctx, &other) }); err != nil {
		t.Fatalf("creating the second project: %v", err)
	}
	f.numberedTask(t, f.project.ID, 95, true)
	f.numberedTask(t, other.ID, 4, false)
	setSeqCounter(t, f.raw(), f.project.ID, 0)

	var got []core.NumberingLag
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		var err error
		got, err = tx.NumberingBehind(ctx)
		return err
	}); err != nil {
		t.Fatalf("NumberingBehind: %v", err)
	}
	want := core.NumberingLag{Project: f.project.Key, Counter: 0, Highest: 95}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("NumberingBehind = %+v, want only %+v", got, want)
	}
}

// The repair migration raises a counter behind its tasks, leaves one already
// above them, which a hard delete explains, and leaves a project with no tasks.
func TestRepairMigrationRaisesCountersBehindTheirTasks(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	projects := map[string]core.Project{}
	for _, key := range []string{"behind", "ahead", "empty"} {
		p := core.Project{Key: key, Name: key, WorkflowID: f.workflow.ID}
		if err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tx.CreateProject(ctx, &p) }); err != nil {
			t.Fatalf("creating project %s: %v", key, err)
		}
		projects[key] = p
	}
	f.numberedTask(t, projects["behind"].ID, 7, false)
	f.numberedTask(t, projects["behind"].ID, 95, true)
	f.numberedTask(t, projects["ahead"].ID, 2, false)

	rewindTo(t, s, migrationNamed(t, "repair_task_number_counters")-1)
	setSeqCounter(t, f.raw(), projects["behind"].ID, 0)
	setSeqCounter(t, f.raw(), projects["ahead"].ID, 50)
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	for key, want := range map[string]int64{"behind": 95, "ahead": 50, "empty": 0} {
		if got := seqCounter(t, f.raw(), projects[key].ID); got != want {
			t.Errorf("counter of %s after the repair = %d, want %d", key, got, want)
		}
	}
}

// rebindTest numbers the placeholders for PostgreSQL.
func rebindTest(q string) string {
	var b strings.Builder
	n := 0
	for _, r := range q {
		if r == '?' {
			n++
			b.WriteString("$" + strconv.Itoa(n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
