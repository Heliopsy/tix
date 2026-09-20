package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
	"gopkg.in/yaml.v3"
)

// wellFormedToken is shaped like a personal access token but matches no record.
const wellFormedToken = "tix_pat_" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// result captures one invocation.
type result struct {
	code int
	out  string
	err  string
}

// cli runs the command tree against a throwaway home directory.
type cli struct {
	t    *testing.T
	home string
	data string
}

// newCLI prepares an isolated environment with no configuration at all.
func newCLI(t *testing.T) *cli {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".git"), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return &cli{t: t, home: home, data: filepath.Join(home, "data")}
}

func (c *cli) environ() []string {
	return []string{"HOME=" + c.home, "XDG_DATA_HOME=" + c.data, "XDG_CONFIG_HOME=" + filepath.Join(c.home, "conf"), "NO_COLOR=1"}
}

func (c *cli) run(args ...string) result { return c.runIn("", args...) }

func (c *cli) runIn(stdin string, args ...string) result {
	c.t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(stdin), &out, &errb, c.environ(), c.home)
	return result{code: code, out: out.String(), err: errb.String()}
}

// mustRun fails the test when the invocation did not succeed.
func (c *cli) mustRun(args ...string) result {
	c.t.Helper()
	got := c.run(args...)
	if got.code != core.ExitOK {
		c.t.Fatalf("tix %s exited %d\nstdout: %s\nstderr: %s",
			strings.Join(args, " "), got.code, got.out, got.err)
	}
	return got
}

func (c *cli) dbPath() string { return filepath.Join(c.data, "tix", "tix.db") }

func TestZeroConfigFirstRunCreatesTheDatabase(t *testing.T) {
	c := newCLI(t)
	if _, err := os.Stat(c.dbPath()); err == nil {
		t.Fatal("database existed before the first command")
	}
	got := c.mustRun("task", "add", "buy milk")
	if !strings.Contains(got.out, "buy milk") {
		t.Fatalf("stdout = %q, want the title", got.out)
	}
	if !strings.Contains(got.out, "default-1") {
		t.Fatalf("stdout = %q, want a reusable ref", got.out)
	}
	if _, err := os.Stat(c.dbPath()); err != nil {
		t.Fatalf("database was not created: %v", err)
	}
	if got.err != "" {
		t.Fatalf("stderr = %q, want nothing on a successful run", got.err)
	}
}

func TestEndToEndAddListClaim(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "buy milk")

	list := c.mustRun("task", "ls")
	if !strings.Contains(list.out, "default-1") || !strings.Contains(list.out, "buy milk") {
		t.Fatalf("task ls = %q", list.out)
	}

	claim := c.mustRun("claim", "next", "-o", "json")
	var payload struct {
		Task       *core.Task `json:"task"`
		LeaseToken string     `json:"lease_token"`
	}
	if err := json.Unmarshal([]byte(claim.out), &payload); err != nil {
		t.Fatalf("claim next output is not json: %v\n%s", err, claim.out)
	}
	if payload.Task == nil || payload.Task.Ref != "default-1" {
		t.Fatalf("claim next returned %+v", payload.Task)
	}
	if payload.LeaseToken == "" {
		t.Fatal("claim next returned no lease token")
	}
}

func TestTaskAddOnlyNeedsATitle(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("task", "add", "-o", "json", "write the report")
	var task core.Task
	if err := json.Unmarshal([]byte(got.out), &task); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	switch {
	case task.Ref != "default-1":
		t.Errorf("ref = %q", task.Ref)
	case task.Status != "todo":
		t.Errorf("status = %q", task.Status)
	case task.Priority != core.PriorityNormal:
		t.Errorf("priority = %d", task.Priority)
	}
}

func TestTaskListInEveryFormat(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "alpha")
	c.mustRun("task", "add", "beta")

	t.Run("table", func(t *testing.T) {
		got := c.mustRun("task", "ls")
		if !strings.Contains(got.out, "REF") || !strings.Contains(got.out, "alpha") {
			t.Fatalf("table output = %q", got.out)
		}
		if strings.Contains(got.out, "\x1b[") {
			t.Fatal("table output carries colour sequences")
		}
	})
	t.Run("json", func(t *testing.T) {
		got := c.mustRun("task", "ls", "-o", "json")
		var tasks []core.Task
		if err := json.Unmarshal([]byte(got.out), &tasks); err != nil {
			t.Fatalf("not json: %v\n%s", err, got.out)
		}
		if len(tasks) != 2 {
			t.Fatalf("got %d tasks", len(tasks))
		}
	})
	t.Run("yaml", func(t *testing.T) {
		got := c.mustRun("task", "ls", "-o", "yaml")
		decoder := yaml.NewDecoder(strings.NewReader(got.out))
		count := 0
		for {
			var task core.Task
			if err := decoder.Decode(&task); err != nil {
				break
			}
			count++
		}
		if count != 2 {
			t.Fatalf("decoded %d yaml documents\n%s", count, got.out)
		}
	})
	t.Run("ndjson", func(t *testing.T) {
		got := c.mustRun("task", "ls", "-o", "ndjson")
		lines := strings.Split(strings.TrimSpace(got.out), "\n")
		if len(lines) != 2 {
			t.Fatalf("got %d lines\n%s", len(lines), got.out)
		}
		for _, line := range lines {
			var task core.Task
			if err := json.Unmarshal([]byte(line), &task); err != nil {
				t.Fatalf("line %q is not json: %v", line, err)
			}
		}
	})
}

func TestEmptyListingIsAnEmptyCollection(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("task", "ls", "-o", "json")
	if strings.TrimSpace(got.out) != "[]" {
		t.Fatalf("empty listing = %q, want []", got.out)
	}
}

func TestExitCodes(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "alpha")

	cases := []struct {
		name  string
		args  []string
		stdin string
		code  int
	}{
		{"unknown ref", []string{"task", "show", "nope-9"}, "", core.ExitNotFound},
		{"unknown project", []string{"project", "show", "missing"}, "", core.ExitNotFound},
		{"empty queue", []string{"claim", "next", "--status", "done"}, "", core.ExitNotFound},
		{"conflict", []string{"project", "create", "default", "Default"}, "", core.ExitConflict},
		{"permission", []string{"--token", wellFormedToken, "task", "ls"}, "", core.ExitPermission},
		{"usage missing argument", []string{"task", "show"}, "", core.ExitUsage},
		{"usage unknown flag", []string{"task", "ls", "--nope"}, "", core.ExitUsage},
		{"usage unknown command", []string{"nonsense"}, "", core.ExitUsage},
		{"usage unknown format", []string{"task", "ls", "-o", "toml"}, "", core.ExitUsage},
		{"usage bad priority", []string{"task", "add", "x", "--priority", "urgent"}, "", core.ExitUsage},
		{"usage both targets", []string{"--db", "a.db", "--server", "https://x", "task", "ls"}, "", core.ExitUsage},
		{"usage bad ref", []string{"task", "show", "!!"}, "", core.ExitUsage},
		{"usage unsupported shell", []string{"completion", "tcsh"}, "", core.ExitUsage},
		{"precondition illegal transition", []string{"task", "mv", "default-1", "done"}, "", core.ExitPrecondtion},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := c.runIn(tc.stdin, tc.args...)
			if got.code != tc.code {
				t.Fatalf("exit = %d, want %d\nstdout: %s\nstderr: %s", got.code, tc.code, got.out, got.err)
			}
			if got.code != core.ExitOK && got.err == "" {
				t.Fatal("a failure wrote nothing to standard error")
			}
		})
	}
}

func TestErrorsNeverReachStandardOutput(t *testing.T) {
	c := newCLI(t)
	got := c.run("task", "show", "nope-9")
	if strings.Contains(got.out, "error") {
		t.Fatalf("stdout carried an error: %q", got.out)
	}
	if !strings.Contains(got.err, "error:") {
		t.Fatalf("stderr = %q, want an error", got.err)
	}
}

func TestQuietSuppressesDiagnosticsButNotData(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "alpha")
	loud := c.mustRun("-v", "task", "ls")
	if !strings.Contains(loud.err, "target:") {
		t.Fatalf("verbose stderr = %q", loud.err)
	}
	quiet := c.mustRun("-q", "-v", "task", "ls")
	if quiet.err != "" {
		t.Fatalf("quiet stderr = %q", quiet.err)
	}
	if !strings.Contains(quiet.out, "alpha") {
		t.Fatalf("quiet stdout = %q", quiet.out)
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	c := newCLI(t)
	before := c.mustRun("task", "ls", "-o", "json")

	add := c.mustRun("task", "add", "ghost", "--dry-run", "-o", "json")
	if !strings.Contains(add.out, statusPlanned) || !strings.Contains(add.out, "dry_run") {
		t.Fatalf("dry run output = %q", add.out)
	}
	after := c.mustRun("task", "ls", "-o", "json")
	if before.out != after.out {
		t.Fatalf("dry run changed the listing:\nbefore %s\nafter %s", before.out, after.out)
	}

	c.mustRun("task", "add", "real")
	c.mustRun("task", "rm", "default-1", "--dry-run")
	if got := c.mustRun("task", "show", "default-1", "-o", "json"); !strings.Contains(got.out, "real") {
		t.Fatalf("dry-run delete removed the task: %q", got.out)
	}
}

func TestDryRunStillValidates(t *testing.T) {
	c := newCLI(t)
	if got := c.run("task", "add", "--dry-run", "x", "--priority", "nonsense"); got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitUsage)
	}
	if got := c.run("task", "rm", "missing-1", "--dry-run"); got.code != core.ExitNotFound {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitNotFound)
	}
}

func TestRefsReadFromStandardInput(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "alpha")
	c.mustRun("task", "add", "beta")

	got := c.runIn("default-1\ndefault-2\n", "task", "mv", "-", "doing", "-o", "json")
	if got.code != core.ExitOK {
		t.Fatalf("exit = %d\n%s\n%s", got.code, got.out, got.err)
	}
	var outcomes []outcome
	if err := json.Unmarshal([]byte(got.out), &outcomes); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	if len(outcomes) != 2 {
		t.Fatalf("got %d outcomes", len(outcomes))
	}
	for _, o := range outcomes {
		if o.Status != statusOK {
			t.Fatalf("outcome %+v", o)
		}
	}
}

func TestBulkPartialFailureExitsNonZero(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "alpha")

	got := c.runIn("default-1\ndefault-99\n", "task", "mv", "-", "doing", "-o", "json")
	if got.code == core.ExitOK {
		t.Fatal("partial failure exited 0")
	}
	var outcomes []outcome
	if err := json.Unmarshal([]byte(got.out), &outcomes); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	if len(outcomes) != 2 {
		t.Fatalf("got %d outcomes", len(outcomes))
	}
	if outcomes[0].Status != statusOK || outcomes[1].Status != statusFailed {
		t.Fatalf("outcomes = %+v", outcomes)
	}
	if outcomes[1].Code != string(core.KindNotFound) {
		t.Fatalf("failed outcome code = %q", outcomes[1].Code)
	}
}

func TestBodyFromStandardInput(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "alpha")
	got := c.runIn("piped body\n", "comment", "add", "default-1", "-", "-o", "json")
	if got.code != core.ExitOK {
		t.Fatalf("exit = %d: %s", got.code, got.err)
	}
	if !strings.Contains(got.out, "piped body") {
		t.Fatalf("comment output = %q", got.out)
	}
}

func TestGlobalFlagsWorkOnEitherSideOfTheSubcommand(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "alpha")
	before := c.mustRun("-o", "json", "task", "ls")
	after := c.mustRun("task", "ls", "-o", "json")
	if before.out != after.out {
		t.Fatalf("flag position changed the result:\n%s\n%s", before.out, after.out)
	}
}

func TestTaskLifecycle(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("task", "add", "-p", "infra", "parent task")
	c.mustRun("task", "add", "-p", "infra", "child task", "--parent", "infra-1",
		"--priority", "high", "--tag", "ops", "--due", "2030-01-02", "--body", "details")

	c.mustRun("task", "edit", "infra-2", "--title", "renamed", "--priority", "low")
	if got := c.mustRun("task", "show", "infra-2", "-o", "json"); !strings.Contains(got.out, "renamed") {
		t.Fatalf("edit did not apply: %s", got.out)
	}
	c.mustRun("task", "tree", "infra-1")
	c.mustRun("dep", "add", "infra-2", "infra-1")
	c.mustRun("dep", "ls", "infra-2")
	c.mustRun("dep", "rm", "infra-2", "infra-1")
	c.mustRun("tag", "add", "infra-1", "urgent")
	c.mustRun("tag", "ls")
	c.mustRun("tag", "rm", "infra-1", "urgent")

	comment := c.mustRun("comment", "add", "infra-1", "a note", "-o", "json")
	var created core.Comment
	if err := json.Unmarshal([]byte(comment.out), &created); err != nil {
		t.Fatalf("not json: %v", err)
	}
	c.mustRun("comment", "ls", "infra-1")
	c.mustRun("comment", "edit", created.ID, "edited note")
	c.mustRun("comment", "rm", created.ID)

	c.mustRun("task", "mv", "infra-1", "doing")
	c.mustRun("task", "rm", "infra-2")
	c.mustRun("task", "restore", "infra-2")
	c.mustRun("task", "ls", "--all", "--unclaimed", "--sort", "priority", "--desc", "--limit", "1")
	c.mustRun("task", "ls", "--include-deleted", "--query", "renamed", "--assignee", "nobody")
}

func TestProjectWorkflowAndFieldCommands(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure", "--description", "core")
	c.mustRun("project", "ls", "--include-archived")
	c.mustRun("project", "show", "infra")
	c.mustRun("project", "edit", "infra", "--name", "Platform")
	c.mustRun("project", "archive", "infra")
	c.mustRun("project", "rm", "infra")

	c.mustRun("workflow", "ls")
	c.mustRun("workflow", "get", "default")
	wf := `
key: review
name: Review
definition:
  initial: open
  states:
    - key: open
      tag: Open
    - key: closed
      tag: Closed
      terminal: true
  transitions:
    - from: open
      to: closed
`
	if got := c.runIn(wf, "workflow", "put", "-f", "-"); got.code != core.ExitOK {
		t.Fatalf("workflow put exited %d: %s", got.code, got.err)
	}
	c.mustRun("workflow", "rm", "review")

	c.mustRun("project", "create", "web", "Web")
	c.mustRun("field", "put", "web", "severity", "--type", "enum", "--option", "low", "--option", "high")
	c.mustRun("field", "ls", "web")
	c.mustRun("field", "rm", "web", "severity")
}

func TestClaimCommands(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "alpha")

	claim := c.mustRun("claim", "task", "default-1", "--ttl", "5m", "-o", "json")
	var payload struct {
		LeaseToken string `json:"lease_token"`
	}
	if err := json.Unmarshal([]byte(claim.out), &payload); err != nil {
		t.Fatalf("not json: %v", err)
	}
	c.mustRun("claim", "renew", "default-1", "--token", payload.LeaseToken, "--ttl", "10m")
	c.mustRun("claim", "release", "default-1", "--token", payload.LeaseToken,
		"--status", "doing", "--result", "exit_code=0")
	c.mustRun("claim", "sweep", "--limit", "10")
	c.mustRun("claim", "sweep", "--dry-run")
}

func TestClaimExecRunsAndReleases(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "alpha")

	got := c.mustRun("claim", "exec", "--ttl", "5m", "--on-start", "doing", "-o", "json", "--", "true")
	var res execResult
	if err := json.Unmarshal([]byte(got.out), &res); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	if res.Ref != "default-1" || res.ExitCode != 0 || res.Status != "done" {
		t.Fatalf("exec result = %+v", res)
	}
	if shown := c.mustRun("task", "show", "default-1", "-o", "json"); !strings.Contains(shown.out, "\"status\": \"done\"") {
		t.Fatalf("task was not released to done: %s", shown.out)
	}
}

func TestClaimExecPropagatesTheChildExitCode(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "alpha")
	got := c.run("claim", "exec", "--ttl", "5m", "--on-start", "doing", "--on-failure", "todo", "--", "false")
	if got.code != 1 {
		t.Fatalf("exit = %d, want the child's 1\n%s\n%s", got.code, got.out, got.err)
	}
}

func TestConfigAndContextCommands(t *testing.T) {
	c := newCLI(t)
	c.mustRun("config", "show")
	sources := c.mustRun("config", "show", "--sources", "-o", "json")
	if !strings.Contains(sources.out, "\"source\": \"default\"") {
		t.Fatalf("sources output = %q", sources.out)
	}

	c.mustRun("ctx", "add", "work", "--db", filepath.Join(c.home, "work.db"), "--tenant", "acme")
	if got := c.mustRun("ctx", "list", "-o", "json"); !strings.Contains(got.out, "work") {
		t.Fatalf("ctx list = %q", got.out)
	}
	c.mustRun("ctx", "use", "work")
	if got := c.mustRun("ctx", "show", "-o", "json"); !strings.Contains(got.out, "\"current\": true") {
		t.Fatalf("ctx show = %q", got.out)
	}
	// The context names tenant acme, which the context's own database does not
	// have yet: create it through the default tenant, then work inside it.
	c.mustRun("--tenant", "default", "tenant", "create", "acme", "Acme")
	c.mustRun("project", "create", "acme", "Acme")
	c.mustRun("task", "add", "in the work context")
	if _, err := os.Stat(filepath.Join(c.home, "work.db")); err != nil {
		t.Fatalf("context database was not used: %v", err)
	}
	c.mustRun("ctx", "rm", "work")
	if got := c.run("ctx", "show", "missing"); got.code != core.ExitNotFound {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitNotFound)
	}
	if got := c.run("ctx", "add", "bad", "--db", "x", "--server", "https://y"); got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitUsage)
	}
}

func TestTargetOverrides(t *testing.T) {
	c := newCLI(t)
	other := filepath.Join(c.home, "other.db")
	c.mustRun("--db", other, "task", "add", "elsewhere")
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("--db was ignored: %v", err)
	}
	if got := c.mustRun("task", "ls", "-o", "json"); strings.Contains(got.out, "elsewhere") {
		t.Fatalf("the default database saw the override's task: %s", got.out)
	}
	if got := c.run("--server", "https://example.invalid", "task", "ls"); got.code == core.ExitOK {
		t.Fatal("remote mode reported success")
	}
}

func TestDoctorReportsTheResolvedTarget(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("doctor", "-o", "json")
	for _, want := range []string{"target", "schema", "identity", "configuration"} {
		if !strings.Contains(got.out, want) {
			t.Fatalf("doctor output is missing %q: %s", want, got.out)
		}
	}
}

func TestAdministrativeCommands(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "alpha")
	c.mustRun("audit", "ls")
	c.mustRun("audit", "ls", "--subject-type", "task", "--since", "2000-01-01", "-o", "ndjson")
	c.mustRun("prune", "--dry-run")
	c.mustRun("tenant", "ls")
	c.mustRun("tenant", "show", "default")
	c.mustRun("tenant", "edit", "default", "--name", "Renamed")
	c.mustRun("domain", "ls")
	c.mustRun("member", "ls")
	if got := c.run("member", "add", "someone", "--role", "wizard"); got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitUsage)
	}
}

// User, token and webhook management are implemented; only snapshot transfer
// and external sync are not. Listing surfaces must therefore succeed, and the
// two that remain absent must still fail rather than pretend.
func TestImplementedSurfacesWork(t *testing.T) {
	c := newCLI(t)
	for _, args := range [][]string{
		{"user", "ls"},
		{"token", "ls"},
		{"webhook", "ls"},
	} {
		if got := c.run(args...); got.code != core.ExitOK {
			t.Fatalf("tix %s exited %d: %s", strings.Join(args, " "), got.code, got.err)
		}
	}
}

// Import deliberately has no default mode, because replace is destructive, so
// invoking it with no mode is a usage error rather than a silent choice.
func TestImportRequiresAnExplicitMode(t *testing.T) {
	c := newCLI(t)
	if got := c.run("import"); got.code != core.ExitUsage {
		t.Fatalf("tix import with no mode exited %d, want %d", got.code, core.ExitUsage)
	}
}

func TestVersionCommand(t *testing.T) {
	c := newCLI(t)
	if got := c.mustRun("version"); !strings.HasPrefix(got.out, "tix ") {
		t.Fatalf("version = %q", got.out)
	}
	got := c.mustRun("version", "-o", "json")
	var info buildInfo
	if err := json.Unmarshal([]byte(got.out), &info); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	if info.Version == "" || info.GoVersion == "" {
		t.Fatalf("build info = %+v", info)
	}
}

func TestDocsEmitsTheCommandTree(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("docs")
	for _, want := range []string{"# tix", "## tix task", "### tix task add", "Examples:", "Exit codes:"} {
		if !strings.Contains(got.out, want) {
			t.Fatalf("docs output is missing %q", want)
		}
	}

	dir := filepath.Join(c.home, "docs")
	c.mustRun("docs", "--dir", dir)
	if _, err := os.Stat(filepath.Join(dir, "tix_task_add.md")); err != nil {
		t.Fatalf("per-command file missing: %v", err)
	}
}

func TestCompletionScripts(t *testing.T) {
	c := newCLI(t)
	for _, shell := range []string{"bash", "zsh", "fish"} {
		got := c.mustRun("completion", shell)
		if len(got.out) < 100 {
			t.Fatalf("%s completion is suspiciously short", shell)
		}
	}
}

func TestDynamicCompletion(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("task", "add", "-p", "infra", "alpha", "--tag", "ops")

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"task refs", []string{cobraCompletionVerb, "task", "show", ""}, "infra-1"},
		{"project keys", []string{cobraCompletionVerb, "task", "ls", "--project", ""}, "infra"},
		{"tags", []string{cobraCompletionVerb, "task", "ls", "--tag", ""}, "ops"},
		{"statuses", []string{cobraCompletionVerb, "claim", "next", "--status", ""}, "todo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := c.run(tc.args...)
			if !strings.Contains(got.out, tc.want) {
				t.Fatalf("completion output = %q, want %q", got.out, tc.want)
			}
		})
	}
}

// cobraCompletionVerb is the hidden command cobra uses to serve completions.
const cobraCompletionVerb = "__complete"

func TestHelpCarriesExamplesAndExitCodes(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("task", "add", "--help")
	if !strings.Contains(got.out, "tix task add") {
		t.Fatalf("help = %q", got.out)
	}
	if !strings.Contains(got.out, "Exit codes:") {
		t.Fatalf("help does not document exit codes: %q", got.out)
	}
	if !strings.Contains(got.out, "Examples:") {
		t.Fatalf("help has no example: %q", got.out)
	}
	for _, group := range []string{"task", "project", "workflow", "field", "claim", "comment", "dep",
		"tag", "user", "token", "ctx", "config", "doctor", "webhook", "prune", "docs", "completion", "version"} {
		if root := c.mustRun("--help"); !strings.Contains(root.out, group) {
			t.Fatalf("root help is missing the %q group", group)
		}
	}
}

func TestGroupsPrintHelpWithoutASubcommand(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("task")
	if !strings.Contains(got.out, "Available Commands") {
		t.Fatalf("group help = %q", got.out)
	}
}

func TestDryRunAcrossTheMutatingSurface(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("task", "add", "-p", "infra", "alpha")

	dry := [][]string{
		{"task", "add", "ghost", "--dry-run"},
		{"task", "edit", "infra-1", "--title", "x", "--dry-run"},
		{"task", "mv", "infra-1", "doing", "--dry-run"},
		{"task", "rm", "infra-1", "--dry-run"},
		{"task", "restore", "infra-1", "--dry-run"},
		{"dep", "add", "infra-1", "infra-1", "--dry-run"},
		{"dep", "rm", "infra-1", "infra-1", "--dry-run"},
		{"tag", "add", "infra-1", "ops", "--dry-run"},
		{"tag", "rm", "infra-1", "ops", "--dry-run"},
		{"comment", "add", "infra-1", "note", "--dry-run"},
		{"comment", "edit", "someid", "note", "--dry-run"},
		{"comment", "rm", "someid", "--dry-run"},
		{"project", "create", "web", "Web", "--dry-run"},
		{"project", "edit", "infra", "--name", "x", "--dry-run"},
		{"project", "archive", "infra", "--dry-run"},
		{"project", "rm", "infra", "--dry-run"},
		{"field", "put", "infra", "sev", "--dry-run"},
		{"field", "rm", "infra", "sev", "--dry-run"},
		{"tenant", "create", "acme", "Acme", "--dry-run"},
		{"tenant", "edit", "default", "--name", "x", "--dry-run"},
		{"tenant", "rm", "default", "--dry-run"},
		{"domain", "add", "tix.example.com", "--dry-run"},
		{"domain", "rm", "tix.example.com", "--dry-run"},
		{"member", "add", "someone", "--dry-run"},
		{"member", "rm", "someone", "--dry-run"},
		{"user", "create", "a@b.c", "--password", "hunter2hunter2", "--dry-run"},
		{"user", "edit", "someid", "--display-name", "x", "--dry-run"},
		{"user", "rm", "someid", "--dry-run"},
		{"token", "create", "agent", "--scope", "task:read", "--expires", "2030-01-01", "--dry-run"},
		{"token", "rm", "someid", "--dry-run"},
		{"webhook", "put", "https://hooks.example.com/tix", "--event", "task.*", "--dry-run"},
		{"webhook", "rm", "someid", "--dry-run"},
		{"webhook", "redeliver", "someid", "--dry-run"},
	}
	for _, args := range dry {
		t.Run(strings.Join(args[:2], " "), func(t *testing.T) {
			got := c.run(args...)
			if got.code != core.ExitOK {
				t.Fatalf("exit = %d\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
			}
			if !strings.Contains(got.out, statusPlanned) {
				t.Fatalf("output = %q, want a plan", got.out)
			}
		})
	}

	after := c.mustRun("task", "ls", "-o", "json")
	var tasks []core.Task
	if err := json.Unmarshal([]byte(after.out), &tasks); err != nil {
		t.Fatalf("not json: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Title != "alpha" {
		t.Fatalf("dry runs changed the data: %+v", tasks)
	}
	projects := c.mustRun("project", "ls", "-o", "json")
	if strings.Contains(projects.out, "\"key\": \"web\"") {
		t.Fatalf("a dry run created a project: %s", projects.out)
	}
}

func TestWebhookAndTokenListFlags(t *testing.T) {
	c := newCLI(t)
	if got := c.run("webhook", "deliveries", "--status", "failed", "--endpoint", "x"); got.code != core.ExitOK {
		t.Fatalf("deliveries exited %d: %s", got.code, got.err)
	}
	if got := c.run("token", "ls"); got.code != core.ExitOK {
		t.Fatalf("token ls exited %d: %s", got.code, got.err)
	}
	// An unknown actor is reported as missing rather than listing nothing.
	if got := c.run("token", "ls", "--actor", "someone"); got.code != core.ExitNotFound {
		t.Fatalf("token ls for an unknown actor exited %d, want %d", got.code, core.ExitNotFound)
	}
	// An unknown user id must still be reported as missing.
	if got := c.run("user", "show", "someid"); got.code != core.ExitNotFound {
		t.Fatalf("user show for an unknown id exited %d, want %d", got.code, core.ExitNotFound)
	}
}

func TestContextCompletionAndExplicitConfigFile(t *testing.T) {
	c := newCLI(t)
	path := filepath.Join(c.home, "explicit.yaml")
	body := "tenant: default\ncurrent_context: work\ncontexts:\n  work:\n    database: " +
		filepath.Join(c.home, "explicit.db") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	c.mustRun("--config", path, "task", "add", "from the explicit file")
	if _, err := os.Stat(filepath.Join(c.home, "explicit.db")); err != nil {
		t.Fatalf("explicit config was ignored: %v", err)
	}
	got := c.run("--config", path, cobraCompletionVerb, "ctx", "use", "")
	if !strings.Contains(got.out, "work") {
		t.Fatalf("context completion = %q", got.out)
	}
}

func TestPerDirectoryDiscoveryAndItsOptOut(t *testing.T) {
	c := newCLI(t)
	local := filepath.Join(c.home, ".tix.yaml")
	body := "current_context: here\ncontexts:\n  here:\n    database: " +
		filepath.Join(c.home, "here.db") + "\n"
	if err := os.WriteFile(local, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	c.mustRun("task", "add", "discovered")
	if _, err := os.Stat(filepath.Join(c.home, "here.db")); err != nil {
		t.Fatalf("discovery was ignored: %v", err)
	}
	c.mustRun("--no-discovery", "task", "add", "not discovered")
	if _, err := os.Stat(c.dbPath()); err != nil {
		t.Fatalf("--no-discovery did not fall back to the default database: %v", err)
	}
}

func TestWorkflowPutFromAFile(t *testing.T) {
	c := newCLI(t)
	path := filepath.Join(c.home, "wf.json")
	doc := `{"key":"review","name":"Review","definition":{"initial":"open",` +
		`"states":[{"key":"open"},{"key":"closed","terminal":true}],` +
		`"transitions":[{"from":"open","to":"closed"}]}}`
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	c.mustRun("workflow", "put", "-f", path)
	if got := c.mustRun("workflow", "ls", "-o", "json"); !strings.Contains(got.out, "review") {
		t.Fatalf("workflow ls = %q", got.out)
	}
	if got := c.run("workflow", "put", "-f", filepath.Join(c.home, "missing.json")); got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitUsage)
	}
	if got := c.runIn("{}", "workflow", "put", "-f", "-"); got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitUsage)
	}
}

func TestCustomFieldsRoundTrip(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("field", "put", "infra", "points", "--type", "int")
	c.mustRun("task", "add", "-p", "infra", "sized", "--field", "points=5")
	got := c.mustRun("task", "show", "infra-1", "-o", "json")
	if !strings.Contains(got.out, "points") {
		t.Fatalf("custom field missing: %s", got.out)
	}
	if bad := c.run("task", "add", "-p", "infra", "x", "--field", "novalue"); bad.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", bad.code, core.ExitUsage)
	}
	if bad := c.run("task", "add", "-p", "infra", "x", "--due", "not-a-date"); bad.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", bad.code, core.ExitUsage)
	}
	if bad := c.run("claim", "next", "--ttl", "forever"); bad.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", bad.code, core.ExitUsage)
	}
}

func TestTitleAndRefsFromStandardInput(t *testing.T) {
	c := newCLI(t)
	if got := c.runIn("piped title\n", "task", "add", "-"); got.code != core.ExitOK {
		t.Fatalf("exit = %d: %s", got.code, got.err)
	}
	if got := c.mustRun("task", "ls", "-o", "json"); !strings.Contains(got.out, "piped title") {
		t.Fatalf("listing = %q", got.out)
	}
	if got := c.runIn("", "task", "rm", "-"); got.code != core.ExitUsage {
		t.Fatalf("empty stdin exit = %d, want %d", got.code, core.ExitUsage)
	}
	if got := c.mustRun("task", "restore", "default-1", "--dry-run"); !strings.Contains(got.out, "default-1") {
		t.Fatalf("restore plan = %q", got.out)
	}
}

func TestMutuallyExclusiveListFilters(t *testing.T) {
	c := newCLI(t)
	if got := c.run("task", "ls", "--claimed", "--unclaimed"); got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitUsage)
	}
	c.mustRun("task", "add", "alpha")
	c.mustRun("task", "ls", "--claimed")
	c.mustRun("task", "ls", "--blocked")
}
