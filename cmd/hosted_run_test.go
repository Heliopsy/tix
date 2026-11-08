// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/sshd"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/store/sqlite/sqlitetest"
)

// hostedRig is a store with one tenant, one admin actor and one project, and
// the service over it, which is what the SSH listener hands RunHosted.
type hostedRig struct {
	svc   core.Service
	actor *core.Actor
}

func newHostedRig(t *testing.T) hostedRig {
	t.Helper()
	ctx := context.Background()
	clk := clock.NewFakeAt()
	st, err := sqlite.Open(sqlitetest.Path(t), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	tenant := core.Tenant{Key: "acme", Name: "Acme"}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error { return u.CreateTenant(ctx, &tenant) }); err != nil {
		t.Fatalf("creating tenant: %v", err)
	}
	actor := core.Actor{Kind: core.ActorUser, Handle: "ada", Role: core.RoleAdmin, Scopes: []core.Scope{core.ScopeAll}}
	workflow := core.Workflow{Key: "default", Name: "Default", Definition: service.BuiltinWorkflow()}
	if err := st.Update(ctx, core.TenantScope{TenantID: tenant.ID}, func(tx store.Tx) error {
		if err := tx.CreateActor(ctx, &actor); err != nil {
			return err
		}
		return tx.PutWorkflow(ctx, &workflow)
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	svc := service.New(st, service.WithClock(clk), service.WithHooks(service.HookOff))
	return hostedRig{svc: svc, actor: &actor}
}

// run invokes one hosted command the way the listener does.
func (r hostedRig) run(t *testing.T, stdin string, args ...string) (code int, out, errOut string) {
	t.Helper()
	var o, e bytes.Buffer
	code = RunHosted(context.Background(), sshd.Invocation{
		Args: args, Stdin: strings.NewReader(stdin), Stdout: &o, Stderr: &e,
		Service: r.svc, Actor: r.actor, Via: core.Via{Kind: core.ViaSSH}, Tenant: "acme",
	})
	return code, o.String(), e.String()
}

func TestRunHostedRunsACommandAsTheSessionsActor(t *testing.T) {
	r := newHostedRig(t)
	if code, _, errOut := r.run(t, "", "project", "create", "infra", "Infra"); code != core.ExitOK {
		t.Fatalf("project create: exit %d, %s", code, errOut)
	}
	if code, _, errOut := r.run(t, "", "task", "add", "first", "-p", "infra"); code != core.ExitOK {
		t.Fatalf("task add: exit %d, %s", code, errOut)
	}
	code, out, errOut := r.run(t, "", "task", "ls", "-p", "infra", "-o", "json")
	if code != core.ExitOK {
		t.Fatalf("task ls: exit %d, %s", code, errOut)
	}
	if !strings.Contains(out, `"first"`) {
		t.Fatalf("task ls over ssh did not list the task just added:\n%s", out)
	}
	code, out, _ = r.run(t, "", "whoami", "-o", "json")
	var who struct {
		Target string `json:"target"`
		Tenant string `json:"tenant"`
		User   string `json:"user"`
	}
	if code != core.ExitOK || json.Unmarshal([]byte(out), &who) != nil ||
		who.User != "ada" || who.Tenant != "acme" || who.Target != "ssh" {
		t.Fatalf("whoami over ssh: exit %d, %s", code, out)
	}
}

func TestRunHostedRefusesWhatTheListenerDoesNotHost(t *testing.T) {
	r := newHostedRig(t)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"nothing at all", nil, "not a command this listener runs"},
		{"an interface word", []string{"board"}, "an interface the listener opens itself"},
		{"a local command", []string{"ctx", "ls"}, `"ctx" is not a command this listener runs`},
		{"the server itself", []string{"serve"}, `"serve" is not a command this listener runs`},
		{"cobra's help", []string{"help"}, `"help" is not a command this listener runs`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errOut := r.run(t, "", tc.args...)
			if code != core.ExitUsage {
				t.Errorf("exit %d, want %d (usage)", code, core.ExitUsage)
			}
			if !strings.Contains(errOut, tc.want) {
				t.Errorf("stderr %q does not say %q", errOut, tc.want)
			}
			if out != "" {
				t.Errorf("a refused command wrote to stdout: %q", out)
			}
		})
	}
}

func TestRunHostedRefusesServerSideFlagsAndPaths(t *testing.T) {
	r := newHostedRig(t)
	if code, _, errOut := r.run(t, "", "project", "create", "infra", "Infra"); code != core.ExitOK {
		t.Fatalf("project create: exit %d, %s", code, errOut)
	}
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"a database", []string{"task", "ls", "--db", "/tmp/other.db"}, "--db picks a target"},
		{"another tenant", []string{"task", "ls", "--tenant", "other"}, "--tenant picks a target"},
		{"a token", []string{"task", "ls", "--token", "x"}, "--token picks a target"},
		{"a file to write", []string{"export", "tasks", "-p", "infra", "--out", "/tmp/infra.csv"}, "names a file"},
		{"a file to read", []string{"import", "tasks", "-p", "infra", "/etc/passwd"}, "names a file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, errOut := r.run(t, "", tc.args...)
			if code == core.ExitOK {
				t.Fatalf("%v ran over ssh", tc.args)
			}
			if !strings.Contains(errOut, tc.want) {
				t.Errorf("stderr %q does not say %q", errOut, tc.want)
			}
		})
	}
}

func TestRunHostedImportsFromStandardInput(t *testing.T) {
	r := newHostedRig(t)
	if code, _, errOut := r.run(t, "", "project", "create", "infra", "Infra"); code != core.ExitOK {
		t.Fatalf("project create: exit %d, %s", code, errOut)
	}
	code, _, errOut := r.run(t, "title,status\nfrom stdin,todo\n", "import", "tasks", "-p", "infra", "-")
	if code != core.ExitOK {
		t.Fatalf("import from stdin: exit %d, %s", code, errOut)
	}
	code, out, _ := r.run(t, "", "export", "tasks", "-p", "infra")
	if code != core.ExitOK || !strings.Contains(out, "from stdin") {
		t.Fatalf("export to stdout: exit %d, %q", code, out)
	}
}

func TestMaxImportBytesFollowsTheResolvedConfig(t *testing.T) {
	g := &globals{}
	if got := g.maxImportBytes(); got != config.DefaultMaxImportBytes {
		t.Errorf("with no configuration: %d, want the default %d", got, config.DefaultMaxImportBytes)
	}
	g.resolved = config.Defaulted()
	g.resolved.Config.Server.MaxImportBytes = 1234
	if got := g.maxImportBytes(); got != 1234 {
		t.Errorf("with server.max_import_bytes 1234: %d", got)
	}
}
