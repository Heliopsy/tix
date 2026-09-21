package postgres

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

func taskCursor(task core.Task, dir core.SortDirection) string {
	return core.Cursor{
		SortValue: sqlb.TimeText(task.CreatedAt),
		ID:        task.ID,
		Sort:      core.SortCreatedAt,
		Direction: dir,
	}.Encode()
}

// urgencyCursor renders the compound cursor a client would carry from one
// page of the urgency ordering to the next.
func urgencyCursor(task core.Task, dir core.SortDirection) string {
	due := sqlb.NoDueSentinel
	if task.DueAt != nil {
		due = sqlb.TimeText(*task.DueAt)
	}
	return core.Cursor{
		SortValue:  strconv.Itoa(int(task.Priority)),
		SortValue2: due,
		ID:         task.ID,
		Sort:       core.SortUrgency,
		Direction:  dir,
	}.Encode()
}

func TestKeysetPaginationCoversEveryRowExactlyOnce(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	const total = 25
	want := make([]string, 0, total)
	for i := 0; i < total; i++ {
		task := f.newTask(t, fmt.Sprintf("task-%02d", i), core.PriorityNormal)
		want = append(want, task.ID)
		clk.Advance(time.Second)
	}

	for _, dir := range []core.SortDirection{core.Ascending, core.Descending} {
		dir := dir
		t.Run(string(dir), func(t *testing.T) {
			var (
				seen   []string
				unique = map[string]bool{}
				cursor string
			)
			for page := 0; page < total; page++ {
				var batch []core.Task
				err := s.View(ctx, f.scope, func(tx store.Tx) error {
					var err error
					batch, err = tx.ListTasks(ctx, core.TaskFilter{Page: core.Page{
						Limit: 7, Cursor: cursor, Sort: core.SortCreatedAt, Direction: dir,
					}})
					return err
				})
				if err != nil {
					t.Fatalf("listing page %d: %v", page, err)
				}
				if len(batch) == 0 {
					break
				}
				for _, task := range batch {
					if unique[task.ID] {
						t.Fatalf("task %q appeared on more than one page", task.ID)
					}
					unique[task.ID] = true
					seen = append(seen, task.ID)
				}
				cursor = taskCursor(batch[len(batch)-1], dir)
			}

			if len(seen) != total {
				t.Fatalf("saw %d rows across pages, want %d", len(seen), total)
			}
			expected := append([]string{}, want...)
			if dir == core.Descending {
				for i, j := 0, len(expected)-1; i < j; i, j = i+1, j-1 {
					expected[i], expected[j] = expected[j], expected[i]
				}
			}
			for i := range expected {
				if seen[i] != expected[i] {
					t.Fatalf("row %d = %q, want %q", i, seen[i], expected[i])
				}
			}
		})
	}
}

func TestListingsNeverEmitOffset(t *testing.T) {
	scope := core.TenantScope{TenantID: "tenant-1"}
	spec, err := resolvePage(core.Page{Limit: 10, Sort: core.SortCreatedAt}, core.SortCreatedAt, taskSortColumns)
	if err != nil {
		t.Fatalf("resolving page: %v", err)
	}
	b := spec.apply(sqlb.MustNew(dialect, scope, "tasks").Select(taskColumns...), "tasks.id")
	q, _ := b.SelectQuery()
	if strings.Contains(strings.ToUpper(q), "OFFSET") {
		t.Fatalf("listing query emitted OFFSET: %s", q)
	}
	if !strings.Contains(q, "tasks.tenant_id = $1") {
		t.Fatalf("listing query lost its tenant predicate: %s", q)
	}
}

func TestPageRejectsUnknownSortField(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	err := s.View(ctx, f.scope, func(tx store.Tx) error {
		_, err := tx.ListProjects(ctx, core.ProjectFilter{Page: core.Page{Sort: "nonsense"}})
		return err
	})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("unknown sort field = %v, want invalid", err)
	}

	err = s.View(ctx, f.scope, func(tx store.Tx) error {
		_, err := tx.ListProjects(ctx, core.ProjectFilter{Page: core.Page{Cursor: "!!not-base64!!"}})
		return err
	})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("malformed cursor = %v, want invalid", err)
	}
}

func TestAuditPaginatesOnItsSequence(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	const total = 9
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		for i := 0; i < total; i++ {
			e := core.AuditEntry{Action: "task.update", SubjectType: "task",
				SubjectID: fmt.Sprintf("t-%d", i), Source: core.SourceCLI}
			if err := tx.AppendAudit(ctx, &e); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("appending audit entries: %v", err)
	}

	var (
		seen   []int64
		unique = map[int64]bool{}
		cursor string
	)
	for page := 0; page < total; page++ {
		var batch []core.AuditEntry
		if err := s.View(ctx, f.scope, func(tx store.Tx) error {
			var err error
			batch, err = tx.ListAudit(ctx, core.AuditFilter{Page: core.Page{
				Limit: 4, Cursor: cursor, Sort: "seq",
			}})
			return err
		}); err != nil {
			t.Fatalf("listing audit page %d: %v", page, err)
		}
		if len(batch) == 0 {
			break
		}
		for _, e := range batch {
			if unique[e.Seq] {
				t.Fatalf("audit entry %d appeared twice", e.Seq)
			}
			unique[e.Seq] = true
			seen = append(seen, e.Seq)
		}
		last := batch[len(batch)-1]
		cursor = core.Cursor{SortValue: fmt.Sprint(last.Seq), ID: fmt.Sprint(last.Seq),
			Sort: "seq", Direction: core.Ascending}.Encode()
	}
	if len(seen) != total {
		t.Fatalf("audit entries seen = %d, want %d", len(seen), total)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i-1] >= seen[i] {
			t.Fatalf("audit entries out of order: %d then %d", seen[i-1], seen[i])
		}
	}
}

func TestTenantListingPaginates(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	for i := 0; i < 5; i++ {
		seed(t, s, clk, fmt.Sprintf("tenant-%d", i))
		clk.Advance(time.Second)
	}

	var seen []string
	cursor := ""
	for page := 0; page < 5; page++ {
		var batch []core.Tenant
		err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			var err error
			batch, err = u.ListTenants(ctx, core.Page{Limit: 2, Cursor: cursor, Sort: "key"})
			return err
		})
		if err != nil {
			t.Fatalf("listing tenants: %v", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, tenant := range batch {
			seen = append(seen, tenant.Key)
		}
		last := batch[len(batch)-1]
		cursor = core.Cursor{SortValue: last.Key, ID: last.ID, Sort: "key", Direction: core.Ascending}.Encode()
	}
	if len(seen) != 5 {
		t.Fatalf("tenants seen = %d, want 5", len(seen))
	}
	for i := 1; i < len(seen); i++ {
		if seen[i-1] >= seen[i] {
			t.Fatalf("tenants out of order: %q then %q", seen[i-1], seen[i])
		}
	}
}

// TestUrgencyKeysetPaginationCoversEveryRowExactlyOnce mirrors the SQLite
// test of the same name: both engines must order the compound urgency sort
// identically, NULL due dates included.
func TestUrgencyKeysetPaginationCoversEveryRowExactlyOnce(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	type spec struct {
		priority core.Priority
		due      *time.Time
	}
	dueOn := func(day int) *time.Time {
		d := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, day)
		return &d
	}

	var specs []spec
	for p := core.PriorityHighest; p <= core.PriorityLowest; p++ {
		specs = append(specs,
			spec{priority: p, due: dueOn(2)},
			spec{priority: p, due: dueOn(0)},
			spec{priority: p, due: dueOn(1)},
			spec{priority: p, due: nil},
			spec{priority: p, due: nil},
		)
	}

	created := make([]core.Task, 0, len(specs))
	for i, sp := range specs {
		task := core.Task{
			ProjectID:      f.project.ID,
			Title:          fmt.Sprintf("task-%02d", i),
			Status:         "todo",
			Priority:       sp.priority,
			DueAt:          sp.due,
			CreatorActorID: f.actor.ID,
		}
		if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
			return tx.CreateTask(ctx, &task)
		}); err != nil {
			t.Fatalf("creating task %d: %v", i, err)
		}
		created = append(created, task)
	}

	want := append([]core.Task{}, created...)
	sort.SliceStable(want, func(i, j int) bool {
		a, b := want[i], want[j]
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		ad, bd := sqlb.NoDueSentinel, sqlb.NoDueSentinel
		if a.DueAt != nil {
			ad = sqlb.TimeText(*a.DueAt)
		}
		if b.DueAt != nil {
			bd = sqlb.TimeText(*b.DueAt)
		}
		if ad != bd {
			return ad < bd
		}
		return a.ID < b.ID
	})

	var (
		seen   []string
		unique = map[string]bool{}
		cursor string
	)
	for page := 0; page < len(want); page++ {
		var batch []core.Task
		err := s.View(ctx, f.scope, func(tx store.Tx) error {
			var err error
			batch, err = tx.ListTasks(ctx, core.TaskFilter{Page: core.Page{
				Limit: 4, Cursor: cursor, Sort: core.SortUrgency, Direction: core.Ascending,
			}})
			return err
		})
		if err != nil {
			t.Fatalf("listing page %d: %v", page, err)
		}
		if len(batch) == 0 {
			break
		}
		for _, task := range batch {
			if unique[task.ID] {
				t.Fatalf("task %q appeared on more than one page", task.ID)
			}
			unique[task.ID] = true
			seen = append(seen, task.ID)
		}
		cursor = urgencyCursor(batch[len(batch)-1], core.Ascending)
	}

	if len(seen) != len(want) {
		t.Fatalf("saw %d rows across pages, want %d", len(seen), len(want))
	}
	for i := range want {
		if seen[i] != want[i].ID {
			t.Fatalf("row %d = %q, want %q (priority=%d due=%v)", i, seen[i], want[i].ID, want[i].Priority, want[i].DueAt)
		}
	}
}
