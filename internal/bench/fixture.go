package bench

import (
	"context"
	"fmt"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// Tenant is one seeded tenant and everything the benchmarks address inside it.
type Tenant struct {
	Tenant   core.Tenant
	Scope    core.TenantScope
	Actor    core.Actor
	Workflow core.Workflow
	Projects []core.Project
	Tags     []core.Tag
	// Tasks is how many tasks this tenant holds.
	Tasks int
}

// Fixture is a seeded dataset over one store.
type Fixture struct {
	Store   store.Store
	Spec    Spec
	Dialect store.Dialect
	Tenants []Tenant
}

// Primary is the tenant the benchmarks read from. It holds the largest share.
func (f *Fixture) Primary() Tenant { return f.Tenants[0] }

// CursorAt returns the cursor that opens the page starting at rank depth of the
// primary tenant's unfiltered created_at listing. Creation times are one Step
// apart, so the position is arithmetic rather than a walk.
func (f *Fixture) CursorAt(depth int) string {
	if depth <= 0 {
		return ""
	}
	return core.Cursor{
		SortValue: sqlb.TimeText(Epoch.Add(time.Duration(depth) * Step)),
		Sort:      core.SortCreatedAt,
		Direction: core.Ascending,
	}.Encode()
}

// SearchTerm is a body token carried by one task in SearchSelectivity.
const SearchTerm = "chromatic"

// SearchSelectivity is how many tasks share one search token.
const SearchSelectivity = 1000

// Seed builds the dataset described by spec in st. It writes through the store
// API in batched transactions; nothing reaches around the tenant-scoped builder.
func Seed(ctx context.Context, st store.Store, spec Spec) (*Fixture, error) {
	if spec.Tenants < 1 || spec.Projects < spec.Tenants || spec.Tasks < 1 {
		return nil, fmt.Errorf("bench spec %q is not seedable: %+v", spec.Name, spec)
	}
	if spec.Batch <= 0 {
		spec.Batch = 1000
	}
	f := &Fixture{Store: st, Spec: spec, Dialect: st.Dialect()}

	shares := split(spec.Tasks, spec.Tenants)
	projectShares := split(spec.Projects, spec.Tenants)
	for i := range spec.Tenants {
		t, err := seedTenant(ctx, st, spec, i, projectShares[i], shares[i])
		if err != nil {
			return nil, err
		}
		f.Tenants = append(f.Tenants, *t)
	}
	return f, nil
}

// split distributes total over n buckets, front-loading the remainder so the
// primary tenant is the largest.
func split(total, n int) []int {
	out := make([]int, n)
	base, rem := total/n, total%n
	for i := range out {
		out[i] = base
		if i < rem {
			out[i]++
		}
	}
	return out
}

func seedTenant(ctx context.Context, st store.Store, spec Spec, index, projects, tasks int) (*Tenant, error) {
	key := fmt.Sprintf("bench-%s-t%d-%d", spec.Name, index, time.Now().UnixNano())
	out := &Tenant{Tenant: core.Tenant{Key: key, Name: key}, Tasks: tasks}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &out.Tenant)
	}); err != nil {
		return nil, fmt.Errorf("creating bench tenant %q: %w", key, err)
	}
	out.Scope = core.TenantScope{TenantID: out.Tenant.ID}

	out.Workflow = core.Workflow{Key: "bench", Name: "Bench", Definition: workflowDefinition()}
	out.Actor = core.Actor{Kind: core.ActorUser, Handle: "bench-worker", DisplayName: "Bench Worker", Scopes: []core.Scope{core.ScopeAll}}
	err := st.Update(ctx, out.Scope, func(tx store.Tx) error {
		if err := tx.PutWorkflow(ctx, &out.Workflow); err != nil {
			return err
		}
		if err := tx.CreateActor(ctx, &out.Actor); err != nil {
			return err
		}
		for p := range projects {
			project := core.Project{
				Key:        fmt.Sprintf("p%02d", p),
				Name:       fmt.Sprintf("Project %02d", p),
				WorkflowID: out.Workflow.ID,
			}
			if err := tx.CreateProject(ctx, &project); err != nil {
				return err
			}
			out.Projects = append(out.Projects, project)
		}
		for l := range spec.Tags {
			tag := core.Tag{Name: fmt.Sprintf("tag-%02d", l)}
			if err := tx.PutTag(ctx, &tag); err != nil {
				return err
			}
			out.Tags = append(out.Tags, tag)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("seeding tenant %q: %w", key, err)
	}
	if err := seedTasks(ctx, st, spec, out); err != nil {
		return nil, err
	}
	return out, nil
}

func workflowDefinition() core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "To do", Category: core.CategoryTodo},
			{Key: "doing", Label: "Doing", Category: core.CategoryInProgress},
			{Key: "review", Label: "Review", Category: core.CategoryInProgress},
			{Key: "done", Label: "Done", Terminal: true, Category: core.CategoryDone},
			{Key: "cancelled", Label: "Cancelled", Terminal: true, Category: core.CategoryDone},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "doing"},
			{From: "doing", To: "review"},
			{From: "review", To: "done"},
			{From: "todo", To: "cancelled"},
		},
	}
}

// seedTasks writes the tenant's tasks in batched transactions. Seq and Ref are
// supplied so CreateTask skips its per-row MAX(seq) probe and read-back, which
// is the difference between minutes and hours at a million rows.
func seedTasks(ctx context.Context, st store.Store, spec Spec, t *Tenant) error {
	seqs := make([]int64, len(t.Projects))
	prior := make([]string, len(t.Projects))

	for start := 0; start < t.Tasks; start += spec.Batch {
		end := min(start+spec.Batch, t.Tasks)
		err := st.Update(ctx, t.Scope, func(tx store.Tx) error {
			for i := start; i < end; i++ {
				p := i % len(t.Projects)
				seqs[p]++
				task := buildTask(t, p, i, seqs[p])
				if err := tx.CreateTask(ctx, &task); err != nil {
					return err
				}
				if spec.TagRatio > 0 && i%spec.TagRatio == 0 && len(t.Tags) > 0 {
					tag := t.Tags[i%len(t.Tags)]
					if err := tx.AttachTag(ctx, task.ID, tag.ID); err != nil {
						return err
					}
				}
				if spec.DepRatio > 0 && i%spec.DepRatio == 0 && prior[p] != "" {
					dep := core.Dependency{TaskID: task.ID, DependsOn: prior[p]}
					if err := tx.AddDependency(ctx, &dep); err != nil {
						return err
					}
				}
				prior[p] = task.ID
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("seeding tasks %d..%d of tenant %q: %w", start, end, t.Tenant.Key, err)
		}
	}
	return nil
}

func buildTask(t *Tenant, projectIndex, i int, seq int64) core.Task {
	created := Epoch.Add(time.Duration(i) * Step)
	project := t.Projects[projectIndex]
	task := core.Task{
		ProjectID:      project.ID,
		Seq:            seq,
		Ref:            fmt.Sprintf("%s-%d", project.Key, seq),
		Title:          fmt.Sprintf("task %d in %s", i, project.Key),
		Body:           bodyFor(i),
		Status:         Statuses[i%len(Statuses)],
		Priority:       core.Priority(1 + i%5),
		CreatorActorID: t.Actor.ID,
		CreatedAt:      created,
		Version:        1,
	}
	if i%3 == 0 {
		task.AssigneeActorID = t.Actor.ID
	}
	if i%4 == 0 {
		due := created.Add(72 * time.Hour)
		task.DueAt = &due
	}
	if task.Status == "done" || task.Status == "cancelled" {
		done := created.Add(time.Hour)
		task.CompletedAt = &done
	}
	return task
}

func bodyFor(i int) string {
	if i%SearchSelectivity == 0 {
		return fmt.Sprintf("body %d mentioning %s in passing", i, SearchTerm)
	}
	return fmt.Sprintf("body %d with ordinary filler text and no distinguishing token", i)
}
