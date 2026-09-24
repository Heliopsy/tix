// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// auditCLI seeds a store with a handful of recorded changes of two kinds.
func auditCLI(t *testing.T) *cli {
	t.Helper()
	c := newCLI(t)
	c.mustRun("task", "add", "rotate the certificates")
	c.mustRun("task", "add", "replace the router")
	c.mustRun("project", "create", "infra", "Infrastructure")
	return c
}

func TestAuditFilterSelectsByKind(t *testing.T) {
	c := auditCLI(t)
	all := c.mustRun("audit", "ls", "-o", "ndjson")
	if !strings.Contains(all.out, `"subject_type":"project"`) {
		t.Fatalf("the unfiltered log holds no project entry:\n%s", all.out)
	}
	got := c.mustRun("audit", "ls", "--filter", "kind:project", "-o", "ndjson")
	if strings.Contains(got.out, `"subject_type":"task"`) {
		t.Fatalf("kind:project kept a task entry:\n%s", got.out)
	}
	if !strings.Contains(got.out, `"subject_type":"project"`) {
		t.Fatalf("kind:project kept nothing:\n%s", got.out)
	}
}

func TestAuditFilterSearchesTheSnapshots(t *testing.T) {
	c := auditCLI(t)
	got := c.mustRun("audit", "ls", "--filter", "certificates", "-o", "ndjson")
	if !strings.Contains(got.out, "certificates") {
		t.Fatalf("a free-text search found nothing:\n%s", got.out)
	}
	if strings.Contains(got.out, "router") {
		t.Fatalf("a free-text search kept an entry it does not describe:\n%s", got.out)
	}
	if !strings.Contains(got.err, "filter kept") {
		t.Fatalf("stderr = %q, want it to say how much the filter discarded", got.err)
	}
}

func TestAuditFilterSelectsBySource(t *testing.T) {
	c := auditCLI(t)
	if got := c.mustRun("audit", "ls", "--filter", "source:cli", "-o", "ndjson"); got.out == "" {
		t.Fatal("source:cli kept nothing, though every entry was written by the cli")
	}
	got := c.mustRun("audit", "ls", "--filter", "source:web", "-o", "ndjson")
	if strings.TrimSpace(got.out) != "" {
		t.Fatalf("source:web kept entries no browser wrote:\n%s", got.out)
	}
}

func TestAuditFilterNegatesATerm(t *testing.T) {
	c := auditCLI(t)
	got := c.mustRun("audit", "ls", "--filter", "-kind:task", "-o", "ndjson")
	if strings.Contains(got.out, `"subject_type":"task"`) {
		t.Fatalf("-kind:task kept a task entry:\n%s", got.out)
	}
}

func TestAuditFilterAndFlagsBothApply(t *testing.T) {
	c := auditCLI(t)
	got := c.mustRun("audit", "ls", "--subject-type", "task", "--filter", "kind:project", "-o", "ndjson")
	if strings.TrimSpace(got.out) != "" {
		t.Fatalf("a filter widened what the flag selected:\n%s", got.out)
	}
}

func TestAuditFilterIsRefusedBeforeAnythingIsRead(t *testing.T) {
	c := auditCLI(t)
	for _, tc := range []struct{ expr, want string }{
		{"status:todo", "unknown activity filter key"},
		{"source:carrier-pigeon", "unknown source"},
	} {
		got := c.run("audit", "ls", "--filter", tc.expr)
		if got.code != core.ExitUsage {
			t.Fatalf("tix audit ls --filter %q exited %d, want %d", tc.expr, got.code, core.ExitUsage)
		}
		if !strings.Contains(got.err, tc.want) {
			t.Errorf("stderr = %q, want it to mention %q", got.err, tc.want)
		}
	}
}

func TestAuditScanDoneReadsOnePageWithoutAFilter(t *testing.T) {
	t.Parallel()
	if !auditScanDone(false, 0, 50, "more") {
		t.Error("an unfiltered listing asked for a second page")
	}
	if auditScanDone(true, 3, 50, "more") {
		t.Error("a filtered listing stopped before it had a screenful")
	}
	if !auditScanDone(true, 50, 50, "more") {
		t.Error("a filtered listing kept reading past its limit")
	}
	if !auditScanDone(true, 3, 50, "") {
		t.Error("a filtered listing kept reading past the end of the log")
	}
	if auditScanDone(true, 3, 0, "more") {
		t.Error("a filtered listing with no limit stopped early")
	}
}

func TestTenantDialerOpensTheNamedTenant(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(c.home, "switch.db")
	c.mustRun("--db", db, "--tenant", "default", "task", "add", "seed")
	c.mustRun("--db", db, "tenant", "create", "acme", "Acme Corp")

	g := &globals{db: db, tenant: "default", environ: c.environ(), dir: c.home}
	probe := &cobra.Command{}
	probe.SetContext(context.Background())
	conn, err := g.tenantDialer(probe)(context.Background(), "acme")
	if err != nil {
		t.Fatalf("dialling acme: %v", err)
	}
	defer func() { _ = conn.Close() }()
	actor, err := conn.Service.WhoAmI(conn.Context)
	if err != nil {
		t.Fatalf("WhoAmI in acme: %v", err)
	}
	if actor == nil {
		t.Fatal("the target tenant named no actor")
	}
	page, err := conn.Service.ListTasks(conn.Context, core.TaskFilter{})
	if err != nil {
		t.Fatalf("listing tasks in acme: %v", err)
	}
	for _, task := range page.Tasks {
		if task.Title == "seed" {
			t.Fatalf("the dialled connection is reading the tenant it came from: %+v", task)
		}
	}
	if conn.Close == nil {
		t.Fatal("the dialled connection cannot be closed")
	}
}
