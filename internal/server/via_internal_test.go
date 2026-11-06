// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"context"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
)

// background is a local service on a fake clock, with a task the jobs below
// act on, created through the CLI's own connection.
type background struct {
	clk  *clock.Fake
	conn *connect.Conn
	ctx  context.Context
	task *core.Task
}

func newBackground(t *testing.T) *background {
	t.Helper()
	clk := clock.NewFakeAt()
	conn, err := connect.Dial(context.Background(), &config.Resolved{},
		connect.Overrides{DB: filepath.Join(t.TempDir(), "tix.db"), Clock: clk})
	if err != nil {
		t.Fatalf("dialling: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ctx := conn.Context(context.Background())
	if _, err := conn.Service.CreateProject(ctx, core.CreateProjectInput{Key: "ops", Name: "Ops"}); err != nil {
		t.Fatalf("creating the project: %v", err)
	}
	task, err := conn.Service.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "ops", Title: "rotate the keys"})
	if err != nil {
		t.Fatalf("creating the task: %v", err)
	}
	return &background{clk: clk, conn: conn, ctx: ctx, task: task}
}

// entries returns the audit entries for an action on the system channel.
func (b *background) systemEntries(t *testing.T, action string) []core.AuditEntry {
	t.Helper()
	entries, _, err := b.conn.Service.ListAudit(b.ctx, core.AuditFilter{
		Actions: []string{action}, Vias: []core.ViaKind{core.ViaSystem},
	})
	if err != nil {
		t.Fatalf("listing audit: %v", err)
	}
	return entries
}

// The lease sweeper is a real worker on a real service: an expiry it writes
// is the system's, and names the job that wrote it.
func TestALeaseExpiryRecordsTheSweeperJob(t *testing.T) {
	b := newBackground(t)
	ref := core.TaskRef{ID: b.task.ID}
	if _, err := b.conn.Service.ClaimTask(b.ctx, ref, core.ClaimInput{TTL: core.Duration(time.Minute)}); err != nil {
		t.Fatalf("claiming: %v", err)
	}
	opts := Options{Service: b.conn.Service, Clock: b.clk, SweepInterval: time.Minute, Logger: slog.Default(),
		TenantID: b.conn.Info.TenantID, DisablePrune: true, DisableDispatch: true}
	var sweeper Worker
	for _, w := range workersFor(opts) {
		if w.Name() == "lease-sweeper" {
			sweeper = w
		}
	}
	if sweeper == nil {
		t.Fatal("no lease sweeper")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- sweeper.Run(ctx) }()
	defer func() { cancel(); <-done }()
	for b.clk.Tickers() == 0 {
		time.Sleep(time.Millisecond)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		b.clk.Advance(2 * time.Minute)
		if got := b.systemEntries(t, "task.lease_expire"); len(got) == 1 {
			if v := got[0].Via; v.Kind != core.ViaSystem || v.Job != JobLeaseSweep {
				t.Fatalf("the expiry recorded %+v, want the system channel and the lease-sweep job", v)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the sweeper never expired the lease")
}

// Housekeeping run by the server is the system's; the same run asked for from
// the command line keeps the command line's channel.
func TestHousekeepingRecordsItsJobAndAManualRunItsCaller(t *testing.T) {
	b := newBackground(t)
	if _, err := b.conn.Service.PutHousekeeping(b.ctx, core.HousekeepingInput{Trash: "1"}); err != nil {
		t.Fatalf("setting the policy: %v", err)
	}
	if err := b.conn.Service.DeleteTask(b.ctx, core.TaskRef{ID: b.task.ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("trashing: %v", err)
	}
	b.clk.Advance(72 * time.Hour)

	runner := systemRunner{service: b.conn.Service, tenantID: b.conn.Info.TenantID}
	if _, err := runner.RunHousekeeping(context.Background(), core.HousekeepingRunInput{}); err != nil {
		t.Fatalf("running housekeeping: %v", err)
	}
	got := b.systemEntries(t, "task.delete")
	if len(got) != 1 || got[0].Via.Job != JobHousekeeping {
		t.Fatalf("housekeeping recorded %+v, want one system entry naming the housekeeping job", got)
	}

	manual, _, err := b.conn.Service.ListAudit(b.ctx, core.AuditFilter{
		Actions: []string{"task.delete"}, Vias: []core.ViaKind{core.ViaCLILocal},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(manual) != 1 {
		t.Errorf("the trashing from the command line recorded %d cli-local entries, want 1", len(manual))
	}
}
