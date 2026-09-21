package integration

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// cliDriver is one way of pointing the CLI at a service: straight at a
// scratch database with --db, or at a running server with --server and
// --token. Both are exercised through cmd.Run, never a subprocess, but that
// is the same code path a real "tix" binary runs.
type cliDriver struct {
	name string
	// run executes one invocation with the driver's target flags prefixed.
	run func(args ...string) cliResult
	// restrictedRun mints a token scoped to exactly the given scopes and runs
	// the invocation as that restricted actor, on whichever transport this
	// driver names.
	restrictedRun func(scopes []core.Scope, args ...string) cliResult
}

// localCLIDriver points the CLI straight at a fresh scratch database.
func localCLIDriver(t *testing.T) cliDriver {
	t.Helper()
	env := newCLIEnv(t)
	dbPath := filepath.Join(env.home, "data", "scratch.db")
	run := func(args ...string) cliResult {
		return env.run(append([]string{"--db", dbPath}, args...)...)
	}
	restrictedRun := func(scopes []core.Scope, args ...string) cliResult {
		tok := mintCLIToken(t, run, scopes)
		return env.run(append([]string{"--db", dbPath, "--token", tok}, args...)...)
	}
	return cliDriver{name: "CLI against --db", run: run, restrictedRun: restrictedRun}
}

// remoteCLIDriver points the CLI at a running server over --server/--token,
// fronting the same kind of Local a --db invocation would open directly.
func remoteCLIDriver(t *testing.T) cliDriver {
	t.Helper()
	h := newMatrixHarness(t)
	env := newCLIEnv(t)
	run := func(args ...string) cliResult {
		return env.run(append([]string{"--server", h.baseURL, "--token", h.adminToken}, args...)...)
	}
	restrictedRun := func(scopes []core.Scope, args ...string) cliResult {
		issued, err := h.local.CreateToken(h.adminCtx, core.CreateTokenInput{
			Name: "cli-scoped-" + randSuffix(), ActorID: h.adminActor.ID, Scopes: scopes,
		})
		if err != nil {
			t.Fatalf("minting a scoped token: %v", err)
		}
		return env.run(append([]string{"--server", h.baseURL, "--token", issued.Token}, args...)...)
	}
	return cliDriver{name: "CLI against --server", run: run, restrictedRun: restrictedRun}
}

// mintCLIToken creates a token scoped to exactly scopes using the driver's
// own (unrestricted, bootstrap) actor, and returns the token value.
func mintCLIToken(t *testing.T, run func(args ...string) cliResult, scopes []core.Scope) string {
	t.Helper()
	args := []string{"token", "create", "scoped-" + randSuffix()}
	for _, s := range scopes {
		args = append(args, "--scope", string(s))
	}
	args = append(args, "-o", "json")
	got := run(args...)
	if got.code != core.ExitOK {
		t.Fatalf("minting a scoped token: exit %d: %s", got.code, got.err)
	}
	var issued core.IssuedToken
	if err := json.Unmarshal([]byte(got.out), &issued); err != nil || issued.Token == "" {
		t.Fatalf("token create -o json = %q, want a decodable token: %v", got.out, err)
	}
	return issued.Token
}

// cliScenario is one row of the CLI transport-equivalence table.
type cliScenario struct {
	name   string
	exempt string
	// run drives the scenario and returns a comparable signature; the final
	// invocation's exit code is what is compared for both success and error
	// cases, since the CLI's exit code IS its error-kind mapping.
	run func(t *testing.T, d cliDriver) (sig, int)
}

var cliScenarios = []cliScenario{
	{
		name: "task add",
		run: func(t *testing.T, d cliDriver) (sig, int) {
			got := d.run("task", "add", "widget", "--priority", "high", "-o", "json")
			if got.code != core.ExitOK {
				return nil, got.code
			}
			var task core.Task
			if err := json.Unmarshal([]byte(got.out), &task); err != nil {
				t.Fatalf("[%s] task add -o json = %q: %v", d.name, got.out, err)
			}
			return sig{"title": task.Title, "status": task.Status, "priority": task.Priority}, got.code
		},
	},
	{
		name: "task show of an unknown ref",
		run: func(t *testing.T, d cliDriver) (sig, int) {
			got := d.run("task", "show", "nosuchtask00000000")
			return nil, got.code
		},
	},
	{
		name: "an illegal transition",
		run: func(t *testing.T, d cliDriver) (sig, int) {
			add := d.run("task", "add", "t", "-o", "json")
			mustCLI(t, d, add, "adding the task")
			var task core.Task
			mustJSON(t, d, add.out, &task)
			// The builtin workflow has no todo -> done edge.
			got := d.run("task", "mv", task.Ref, "done")
			return nil, got.code
		},
	},
	{
		name: "claim next then a conflicting claim",
		run: func(t *testing.T, d cliDriver) (sig, int) {
			add := d.run("task", "add", "t", "-o", "json")
			mustCLI(t, d, add, "adding the task")
			var task core.Task
			mustJSON(t, d, add.out, &task)
			first := d.run("claim", "next", "-o", "json")
			mustCLI(t, d, first, "the first claim")
			got := d.run("claim", "task", task.Ref, "--actor", "someone-else-1")
			return nil, got.code
		},
	},
	{
		name: "listing tasks by status",
		run: func(t *testing.T, d cliDriver) (sig, int) {
			mustCLI(t, d, d.run("task", "add", "stays todo"), "adding a todo task")
			moved := d.run("task", "add", "moves on", "-o", "json")
			mustCLI(t, d, moved, "adding a second task")
			var task core.Task
			mustJSON(t, d, moved.out, &task)
			mustCLI(t, d, d.run("task", "mv", task.Ref, "doing"), "transitioning the second task")

			got := d.run("task", "ls", "--status", "todo", "-o", "json")
			if got.code != core.ExitOK {
				return nil, got.code
			}
			var tasks []core.Task
			mustJSON(t, d, got.out, &tasks)
			titles := make([]string, len(tasks))
			for i, tsk := range tasks {
				titles[i] = tsk.Title
			}
			return sig{"titles": titles}, got.code
		},
	},
	{
		name: "a scope-restricted token is refused a write",
		run: func(t *testing.T, d cliDriver) (sig, int) {
			got := d.restrictedRun([]core.Scope{core.ScopeTaskRead}, "task", "add", "should be refused")
			return nil, got.code
		},
	},
	{
		name: "the terminal interface over a remote server",
		exempt: "the terminal interface has no --db/--server duality of its own to drive from " +
			"the shell; it is covered on both targets directly in tui_matrix_test.go instead of " +
			"through a CLI invocation here",
	},
}

func TestCLITransportEquivalence(t *testing.T) {
	for _, sc := range cliScenarios {
		sc := sc
		t.Run(sc.name, func(t *testing.T) {
			t.Parallel()
			if sc.exempt != "" {
				t.Skipf("exempt: %s", sc.exempt)
			}

			localSig, localCode := sc.run(t, localCLIDriver(t))
			remoteSig, remoteCode := sc.run(t, remoteCLIDriver(t))

			if localCode != remoteCode {
				t.Fatalf("exit code diverges between targets:\n  --db:     exit %d\n  --server: exit %d",
					localCode, remoteCode)
			}
			if localCode != core.ExitOK {
				return
			}
			if !reflect.DeepEqual(localSig, remoteSig) {
				t.Fatalf("result diverges between targets:\n  --db:     %#v\n  --server: %#v", localSig, remoteSig)
			}
		})
	}
}

func mustCLI(t *testing.T, d cliDriver, got cliResult, what string) {
	t.Helper()
	if got.code != core.ExitOK {
		t.Fatalf("[%s] %s: exit %d: %s", d.name, what, got.code, got.err)
	}
}

func mustJSON(t *testing.T, d cliDriver, out string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(out), v); err != nil {
		t.Fatalf("[%s] decoding %q: %v", d.name, out, err)
	}
}
