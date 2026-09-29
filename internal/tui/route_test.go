// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
)

// liveBoard is a board over a real service on a temporary database, so a
// keystroke here writes the rows a keystroke in the interface writes.
type liveBoard struct {
	model Model
	local *service.Local
	scope core.TenantScope
	ctx   context.Context
	task  *core.Task
}

func newLiveBoard(t *testing.T) *liveBoard {
	t.Helper()
	ctx := context.Background()
	clk := clock.NewFakeAt()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "tix.db"), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}

	tenant := core.Tenant{Key: "acme", Name: "Acme"}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &tenant)
	}); err != nil {
		t.Fatalf("creating tenant: %v", err)
	}
	scope := core.TenantScope{TenantID: tenant.ID}
	actor := core.Actor{Kind: core.ActorUser, Handle: "alice", Role: core.RoleAdmin,
		Scopes: []core.Scope{core.ScopeAll}}
	workflow := core.Workflow{Key: "default", Name: "Default", Definition: service.BuiltinWorkflow()}
	project := core.Project{Key: "infra", Name: "infra"}
	if err := st.Update(ctx, scope, func(tx store.Tx) error {
		if err := tx.CreateActor(ctx, &actor); err != nil {
			return err
		}
		if err := tx.PutWorkflow(ctx, &workflow); err != nil {
			return err
		}
		project.WorkflowID = workflow.ID
		return tx.CreateProject(ctx, &project)
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	actor.TenantID = tenant.ID

	local := service.New(st, service.WithClock(clk), service.WithHooks(service.HookOff))
	actorCtx := core.WithActor(core.WithTenant(ctx, scope), &actor)
	task, err := local.CreateTask(actorCtx, core.CreateTaskInput{ProjectRef: "infra", Title: "walk me"})
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}

	def := workflow.Definition
	m := New(Config{Access: fullAccess(), Actor: fullActor(), Environ: []string{"NO_COLOR=1"},
		Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }})
	m.width, m.height = 120, 40
	m.svc, m.ctx = local, actorCtx
	m, _ = m.reduce(boardMsg{project: project, workflow: &def, tasks: []core.Task{*task}})
	return &liveBoard{model: m, local: local, scope: scope, ctx: actorCtx, task: task}
}

// trail lists the task's recorded state changes, oldest first, as "from>to".
func (b *liveBoard) trail(t *testing.T) []string {
	t.Helper()
	entries, _, err := b.local.ListAudit(b.ctx, core.AuditFilter{
		SubjectType: "task", SubjectID: b.task.ID, Actions: []string{"task.transition"}})
	if err != nil {
		t.Fatalf("listing the trail: %v", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Seq < entries[j].Seq })
	var out []string
	for _, e := range entries {
		out = append(out, imageStatus(t, e.Before)+">"+imageStatus(t, e.After))
	}
	return out
}

func imageStatus(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	if len(raw) == 0 {
		return ""
	}
	var image struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(raw, &image); err != nil {
		t.Fatalf("reading an audit image: %v", err)
	}
	return image.Status
}

// TestThePickerAppliesARouteOneHopAtATime drives the transition picker the way
// a reader does and then reads what it wrote. The service proves the rule; this
// proves the terminal is wired to it, which is the half that was missing
// everywhere but the browser's task list.
func TestThePickerAppliesARouteOneHopAtATime(t *testing.T) {
	b := newLiveBoard(t)

	// The shipped workflow leaves todo for doing, blocked or cancelled, and
	// reaches done only through doing, so the fourth choice is that route.
	m, _ := b.model.reduce(pressKey("t"))
	if len(m.choices) < 4 {
		t.Fatalf("the picker offers %+v, which has no route in it", m.choices)
	}
	route := m.choices[3]
	if route.Value != "doing>done" {
		t.Fatalf("the fourth choice is %q, want the route through doing", route.Value)
	}
	if !strings.Contains(route.Label, "via Doing") || !strings.Contains(route.Label, "2 steps") {
		t.Errorf("the route is offered as %q, which does not show its hops", route.Label)
	}

	_, cmd := m.reduce(pressKey("4"))
	if cmd == nil {
		t.Fatal("picking a route did nothing")
	}
	msg, ok := cmd().(actionMsg)
	if !ok || msg.err != nil {
		t.Fatalf("applying the route: %+v", msg)
	}
	if want := "moved " + b.task.Ref + " todo → doing → done, one step at a time"; msg.sentence != want {
		t.Errorf("sentence = %q, want %q", msg.sentence, want)
	}
	if got := b.trail(t); strings.Join(got, ",") != "todo>doing,doing>done" {
		t.Fatalf("the picker wrote %v, want one entry per hop", got)
	}
}

// TestThePickerKeepsASingleHopOneKeystroke is the cost the routes may not add
// to. The adjacent states come first, so the move a reader makes most often is
// still t then 1.
func TestThePickerKeepsASingleHopOneKeystroke(t *testing.T) {
	b := newLiveBoard(t)
	m, _ := b.model.reduce(pressKey("t"))
	_, cmd := m.reduce(pressKey("1"))
	if cmd == nil {
		t.Fatal("the first choice did nothing")
	}
	if msg, ok := cmd().(actionMsg); !ok || msg.err != nil {
		t.Fatalf("applying the first choice: %+v", msg)
	}
	if got := b.trail(t); strings.Join(got, ",") != "todo>doing" {
		t.Fatalf("t then 1 wrote %v, want exactly one hop", got)
	}
}
