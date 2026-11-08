// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/cmd"
	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/sshd"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/store/sqlite/sqlitetest"
	gossh "golang.org/x/crypto/ssh"
)

// hostedListener serves enrolled keys with the real command runner, the way
// tix ssh and tix serve --ssh wire it.
type hostedListener struct {
	addr string
	svc  core.Service
	st   store.Store
}

func newHostedListener(t *testing.T, tune func(*sshd.Options)) hostedListener {
	t.Helper()
	ctx := context.Background()
	clk := clock.New()
	st, err := sqlite.Open(sqlitetest.Path(t), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	svc := service.New(st, service.WithClock(clk), service.WithTOTPKey(sshd.FactorTestKey))
	t.Cleanup(func() { _ = svc.Close() })

	opts := sshd.Options{
		Service:      svc,
		Store:        st,
		Clock:        clk,
		Addr:         "127.0.0.1:0",
		HostKeyPath:  filepath.Join(t.TempDir(), "host_key"),
		RatePerHour:  100000,
		RateBurst:    100000,
		Commands:     cmd.RunHosted,
		CommandWords: cmd.CommandWords(),
		Logger:       slog.New(slog.DiscardHandler),
	}
	if tune != nil {
		tune(&opts)
	}
	srv, err := sshd.New(opts)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = srv.Serve(runCtx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	return hostedListener{addr: srv.Addr(), svc: svc, st: st}
}

// dial opens one authenticated client, which the test runs every session on.
func dial(t *testing.T, addr, user string, signer gossh.Signer) *gossh.Client {
	t.Helper()
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            user,
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
		Timeout:         30 * time.Second,
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

// result is what one session left behind.
type result struct {
	out, err string
	code     int
}

// run executes one command line in a fresh session, as `ssh host LINE` does.
func run(t *testing.T, client *gossh.Client, line string, stdin io.Reader, env map[string]string) result {
	t.Helper()
	sess, err := client.NewSession()
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	defer func() { _ = sess.Close() }()
	for k, v := range env {
		if err := sess.Setenv(k, v); err != nil {
			t.Fatalf("setenv %s: %v", k, err)
		}
	}
	var out, errw bytes.Buffer
	sess.Stdout, sess.Stderr = &out, &errw
	if stdin != nil {
		sess.Stdin = stdin
	}
	code := 0
	if err := sess.Run(line); err != nil {
		var exit *gossh.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("%q: %v (stderr %q)", line, err, errw.String())
		}
		code = exit.ExitStatus()
	}
	return result{out: out.String(), err: errw.String(), code: code}
}

// enrolled is a listener with one admin in tenant acme and a board "tob".
func enrolled(t *testing.T, tune func(*sshd.Options)) (hostedListener, sshd.Person, *gossh.Client) {
	t.Helper()
	l := newHostedListener(t, tune)
	p := sshd.SeedPerson(t, l.st, "acme", "ada", "ada@example.test")
	sshd.SeedBoard(t, l.st, l.svc, p)
	return l, p, dial(t, l.addr, "acme", p.Signer)
}

func TestACommandRunsInProcessAsTheKeysActor(t *testing.T) {
	l, p, client := enrolled(t, nil)

	add := run(t, client, `task add "first over ssh" -p tob`, nil, nil)
	if add.code != 0 {
		t.Fatalf("task add exited %d: %s", add.code, add.err)
	}
	ls := run(t, client, "task ls -o json -p tob", nil, nil)
	if ls.code != 0 || !strings.Contains(ls.out, "first over ssh") {
		t.Fatalf("task ls exited %d, out %q, err %q", ls.code, ls.out, ls.err)
	}
	page, err := l.svc.ListTasks(sshd.SessionContext(p.Actor), core.TaskFilter{})
	if err != nil || len(page.Tasks) != 1 {
		t.Fatalf("tasks in the tenant = %d, %v", len(page.Tasks), err)
	}
	if page.Tasks[0].CreatorActorID != p.Actor.ID {
		t.Fatalf("created by %q, want the key's actor %q", page.Tasks[0].CreatorActorID, p.Actor.ID)
	}
	who := run(t, client, "whoami", nil, nil)
	if who.code != 0 || !strings.Contains(who.out, "ada") || !strings.Contains(who.out, "ssh") {
		t.Fatalf("whoami exited %d, out %q", who.code, who.out)
	}
}

func TestStandardStreamsAndTheExitCodePassThrough(t *testing.T) {
	l, p, client := enrolled(t, nil)
	if _, err := l.svc.CreateProject(sshd.SessionContext(p.Actor), core.CreateProjectInput{Key: "ops", Name: "Ops"}); err != nil {
		t.Fatalf("creating ops: %v", err)
	}
	for _, title := range []string{"alpha", "beta"} {
		if r := run(t, client, "task add "+title+" -p tob", nil, nil); r.code != 0 {
			t.Fatalf("task add exited %d: %s", r.code, r.err)
		}
	}

	export := run(t, client, "export tasks -p tob", nil, nil)
	if export.code != 0 || !strings.Contains(export.out, "alpha") || !strings.Contains(export.out, "beta") {
		t.Fatalf("export exited %d, out %q, err %q", export.code, export.out, export.err)
	}
	imported := run(t, client, "import tasks -p ops", strings.NewReader(export.out), nil)
	if imported.code != 0 {
		t.Fatalf("import exited %d: out %q err %q", imported.code, imported.out, imported.err)
	}
	page, err := l.svc.ListTasks(sshd.SessionContext(p.Actor), core.TaskFilter{ProjectKeys: []string{"ops"}})
	if err != nil || len(page.Tasks) != 2 {
		t.Fatalf("ops holds %d tasks after the import, %v", len(page.Tasks), err)
	}

	missing := run(t, client, "task show TOB-999", nil, nil)
	if missing.code != core.ExitNotFound || missing.err == "" || missing.out != "" {
		t.Fatalf("task show of a missing task: exit %d, out %q, err %q", missing.code, missing.out, missing.err)
	}
}

func TestACommandIsAuditedAsSSH(t *testing.T) {
	l, p, client := enrolled(t, nil)
	if r := run(t, client, "task add audited -p tob", nil, nil); r.code != 0 {
		t.Fatalf("task add exited %d: %s", r.code, r.err)
	}
	entries, _, err := l.svc.ListAudit(sshd.SessionContext(p.Actor), core.AuditFilter{Actions: []string{"task.create"}})
	if err != nil || len(entries) != 1 {
		t.Fatalf("task.create entries = %d, %v", len(entries), err)
	}
	via := entries[0].Via
	if via.Kind != core.ViaSSH || via.KeyFingerprint != gossh.FingerprintSHA256(p.Signer.PublicKey()) ||
		via.ClientAddr == "" {
		t.Fatalf("the entry records via %+v, want ssh with the key and the address", via)
	}
	if entries[0].Source != core.SourceCLI {
		t.Fatalf("source = %q, want cli", entries[0].Source)
	}
}

// TestEveryExcludedClassIsRefused runs one command of each kind the listener
// must not run, and checks both that it is refused and that nothing it would
// have done happened.
func TestEveryExcludedClassIsRefused(t *testing.T) {
	l, p, client := enrolled(t, nil)
	dir := t.TempDir()
	secret := filepath.Join(dir, "server-secret.txt")
	if err := os.WriteFile(secret, []byte("title\nleaked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	written := filepath.Join(dir, "written")

	cases := []struct {
		name, line, says string
		code             int
	}{
		{"a local configuration command", "config ls", "not a command this listener runs", core.ExitUsage},
		{"contexts", "ctx ls", "not a command", core.ExitUsage},
		{"the server", "serve", "not a command", core.ExitUsage},
		{"a local database", "init", "not a command", core.ExitUsage},
		{"generated docs", "docs " + written, "not a command", core.ExitUsage},
		{"completion", "completion bash", "not a command", core.ExitUsage},
		{"cobra's completion entry", "__complete task ''", "not a command", core.ExitUsage},
		{"cobra's help", "help task", "not a command", core.ExitUsage},
		{"self update", "update", "not a command", core.ExitUsage},
		{"the doctor", "doctor", "not a command", core.ExitUsage},
		{"the mcp server", "mcp", "not a command", core.ExitUsage},
		{"demo seeding", "demo seed", "not a command", core.ExitUsage},
		{"a nested listener", "ssh", "not a command", core.ExitUsage},
		{"sign in", "login", "not a command", core.ExitUsage},
		{"sign out", "logout", "not a command", core.ExitUsage},
		{"a program run by claim exec", "claim exec -- sh -c id", "not a command", core.ExitUsage},
		{"a tenant switch in local configuration", "tenant use other", "not a command", core.ExitUsage},
		{"--db", "task ls --db " + filepath.Join(dir, "other.db"), "--db", core.ExitUsage},
		{"--db=", "task ls --db=" + filepath.Join(dir, "other.db"), "--db", core.ExitUsage},
		{"--server", "task ls --server http://127.0.0.1:1", "--server", core.ExitUsage},
		{"--token", "task ls --token tix_x", "--token", core.ExitUsage},
		{"--ctx", "task ls --ctx prod", "--ctx", core.ExitUsage},
		{"--config", "task ls --config " + secret, "--config", core.ExitUsage},
		{"--tenant", "task ls --tenant other", "--tenant", core.ExitUsage},
		{"--no-discovery", "task ls --no-discovery", "--no-discovery", core.ExitUsage},
		{"--allow-network-fs", "task ls --allow-network-fs", "--allow-network-fs", core.ExitUsage},
		{"--log-output", "task ls --log-output " + written, "--log-output", core.ExitUsage},
		{"--log-level", "task ls --log-level debug", "--log-level", core.ExitUsage},
		{"export --out", "export tasks -p tob --out " + written, "--out", core.ExitUsage},
		{"import FILE", "import tasks -p tob " + secret, "FILE", core.ExitUsage},
		{"import --lines", "import tasks -p tob --lines " + secret, "over ssh", core.ExitUsage},
		{"import --mapping", "import tasks -p tob --mapping " + secret, "--mapping", core.ExitUsage},
		{"import --save-mapping", "import tasks -p tob --dry-run --save-mapping " + written, "--save-mapping", core.ExitUsage},
		{"bundle import FILE", "bundle import --on-collision skip " + secret, "FILE", core.ExitUsage},
		{"artifact --file", "artifact put TOB-1 --kind file --name x --file " + secret, "--file", core.ExitUsage},
		{"workflow put -f", "workflow put -f " + secret, "over ssh", core.ExitUsage},
		{"project --icon-file", "project create web Web --icon-file " + secret, "--icon-file", core.ExitUsage},
		{"project icon --out", "project icon tob --out " + written, "--out", core.ExitUsage},
		{"user key --file", "user key add --file " + secret, "--file", core.ExitUsage},
		{"sync --mapping-file", "sync source add generic x --file x.csv --mapping-file " + secret, "over ssh", core.ExitUsage},
		{"sync --file on the server", "sync source add generic x --file " + secret, "--file", core.ExitUsage},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := run(t, client, tc.line, strings.NewReader(""), nil)
			if r.code != tc.code || !strings.Contains(r.err, tc.says) {
				t.Fatalf("%q: exit %d (want %d), stderr %q (want %q)", tc.line, r.code, tc.code, r.err, tc.says)
			}
			if strings.Contains(r.out+r.err, "leaked") {
				t.Fatalf("%q read the server's file: %q", tc.line, r.out+r.err)
			}
		})
	}
	if _, err := os.Stat(written); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a refused command wrote %s on the server", written)
	}
	if _, err := os.Stat(filepath.Join(dir, "other.db")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("a refused command opened another database")
	}
	page, err := l.svc.ListTasks(sshd.SessionContext(p.Actor), core.TaskFilter{IncludeDeleted: true})
	if err != nil || len(page.Tasks) != 0 {
		t.Fatalf("refused commands left %d tasks, %v", len(page.Tasks), err)
	}
	projects, _, err := l.svc.ListProjects(sshd.SessionContext(p.Actor), core.ProjectFilter{})
	if err != nil || len(projects) != 1 {
		t.Fatalf("refused commands left %d projects, %v", len(projects), err)
	}
}

func TestAShellIsRefused(t *testing.T) {
	_, _, client := enrolled(t, nil)
	r := run(t, client, "sh -c id", nil, nil)
	if r.code == 0 || !strings.Contains(r.err, "not a shell") || strings.Contains(r.out, "uid=") {
		t.Fatalf("sh -c id: exit %d, out %q, err %q", r.code, r.out, r.err)
	}
}

func TestAWordNobodyTypesIsRefusedBeforeAnythingRuns(t *testing.T) {
	_, _, client := enrolled(t, nil)
	cases := map[string]string{
		"an escape":       "task add 'a\x1b[2Jb' -p tob",
		"a nul":           "task add 'a\x00b' -p tob",
		"too many words":  "task ls" + strings.Repeat(" x", sshd.MaxCommandWords),
		"an oversize arg": "task add " + strings.Repeat("a", sshd.MaxCommandWordBytes+1),
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			r := run(t, client, line, nil, nil)
			if r.code != core.ExitUsage || !strings.Contains(r.err, "command") {
				t.Fatalf("exit %d, err %q", r.code, r.err)
			}
		})
	}
}

func TestStandardInputPastTheLimitImportsNothing(t *testing.T) {
	l, p, client := enrolled(t, func(o *sshd.Options) { o.MaxInputBytes = 64 })
	big := "title\n" + strings.Repeat("a task title\n", 50)
	r := run(t, client, "import tasks -p tob", strings.NewReader(big), nil)
	if r.code == 0 || !strings.Contains(r.err, "max_import_bytes") {
		t.Fatalf("an oversize import: exit %d, err %q", r.code, r.err)
	}
	page, err := l.svc.ListTasks(sshd.SessionContext(p.Actor), core.TaskFilter{})
	if err != nil || len(page.Tasks) != 0 {
		t.Fatalf("the refused import wrote %d tasks, %v", len(page.Tasks), err)
	}
}

func TestAGlobalAdminCommandNeedsAGlobalAdmin(t *testing.T) {
	_, _, client := enrolled(t, nil)
	r := run(t, client, "tenant create other Other", nil, nil)
	if r.code != core.ExitPermission {
		t.Fatalf("a tenant admin creating a tenant over ssh: exit %d, err %q", r.code, r.err)
	}
}

func TestADemoListenerRunsNoCommands(t *testing.T) {
	l := newHostedListener(t, func(o *sshd.Options) { o.Demo = true })
	signer := sshd.NewTestSigner(t)
	client := dial(t, l.addr, "tix", signer)
	r := run(t, client, "task ls", nil, nil)
	if r.code == 0 || !strings.Contains(r.err, "runs no commands") {
		t.Fatalf("a sandbox ran a command: exit %d, out %q, err %q", r.code, r.out, r.err)
	}
}

func TestACommandOwesTheFactorABoardSessionOwes(t *testing.T) {
	l, p, client := enrolled(t, nil)
	sshd.RequireFactor(t, l.st, p.TenantID, core.SurfaceSSH)
	secret := sshd.EnrolFactor(t, l.svc, p)

	refused := run(t, client, "task ls -p tob", nil, nil)
	if refused.code == 0 || !strings.Contains(refused.err, sshd.EnvCode) || !strings.Contains(refused.err, "ssh -t") {
		t.Fatalf("no code: exit %d, err %q", refused.code, refused.err)
	}
	wrong := run(t, client, "task ls -p tob", nil, map[string]string{sshd.EnvCode: "000000"})
	if wrong.code == 0 {
		t.Fatalf("a wrong code ran the command: %q", wrong.out)
	}
	ok := run(t, client, "task ls -p tob -o json", nil, map[string]string{sshd.EnvCode: sshd.TOTPNow(t, secret)})
	if ok.code != 0 {
		t.Fatalf("a current code: exit %d, err %q", ok.code, ok.err)
	}
}

// The factor's surface is read from the channel, so removing it over SSH is
// judged against the SSH requirement even though the command's source is cli.
func TestRemovingTheFactorOverSSHIsJudgedOnTheSSHSurface(t *testing.T) {
	l, p, client := enrolled(t, nil)
	sshd.RequireFactor(t, l.st, p.TenantID, core.SurfaceSSH)
	secret := sshd.EnrolFactor(t, l.svc, p)
	r := run(t, client, "user totp rm", nil, map[string]string{sshd.EnvCode: sshd.TOTPNow(t, secret)})
	if r.code != core.ExitPrecondtion || !strings.Contains(r.err, "ssh") {
		t.Fatalf("user totp rm over ssh: exit %d, err %q", r.code, r.err)
	}
}

// TestEverySSHBoundOperationIsReachableThroughTheListener is the parity test
// for the ssh surface: every command the registry hosts answers through a
// real session, and the ones an interface name takes are served by it.
func TestEverySSHBoundOperationIsReachableThroughTheListener(t *testing.T) {
	_, _, client := enrolled(t, nil)
	interfaces := map[string]bool{"board": true, "watch": true}
	for _, path := range capability.SSHCommands() {
		word, _, _ := strings.Cut(path, " ")
		if interfaces[word] {
			continue
		}
		r := run(t, client, path+" --help", nil, nil)
		if r.code != 0 || !strings.Contains(r.out, "tix "+path) {
			t.Errorf("%q is hosted by the registry and not reachable over ssh: exit %d, err %q", path, r.code, r.err)
		}
	}
	if !interfaces["watch"] || !contains(capability.SSHCommands(), "watch") {
		t.Fatal("tix watch is expected to be served over ssh by the watch interface")
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// A command waiting on standard input the client never closes is stopped at
// the timeout, and the session gives its slot back rather than waiting on.
func TestACommandPastItsTimeoutIsStopped(t *testing.T) {
	_, _, client := enrolled(t, func(o *sshd.Options) { o.CommandTimeout = 300 * time.Millisecond })
	sess, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()
	stdin, err := sess.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = stdin.Close() }()
	var errw bytes.Buffer
	sess.Stderr = &errw
	done := make(chan error, 1)
	go func() { done <- sess.Run("import tasks -p tob") }()
	select {
	case err := <-done:
		var exit *gossh.ExitError
		if !errors.As(err, &exit) || exit.ExitStatus() != core.ExitError || !strings.Contains(errw.String(), "ran past") {
			t.Fatalf("a stalled import ended with %v, stderr %q", err, errw.String())
		}
	case <-time.After(30 * time.Second):
		t.Fatal("a command past its timeout kept the session open")
	}
}

func TestAPanickingCommandEndsOnlyItsOwnSession(t *testing.T) {
	calls := 0
	_, _, client := enrolled(t, func(o *sshd.Options) {
		o.Commands = func(ctx context.Context, inv sshd.Invocation) int {
			calls++
			if calls == 1 {
				panic("boom")
			}
			return cmd.RunHosted(ctx, inv)
		}
	})
	first := run(t, client, "task ls", nil, nil)
	if first.code != core.ExitError || strings.Contains(first.err, "boom") {
		t.Fatalf("a panicking command: exit %d, err %q", first.code, first.err)
	}
	second := run(t, client, "task ls -p tob", nil, nil)
	if second.code != 0 {
		t.Fatalf("the listener did not survive the panic: exit %d, err %q", second.code, second.err)
	}
}

// Sessions running at once share nothing but the service, so each one's
// output is its own.
func TestConcurrentCommandsKeepTheirOwnOutput(t *testing.T) {
	_, _, client := enrolled(t, func(o *sshd.Options) { o.MaxSessionsPerKey = 3 })
	titles := []string{"one", "two", "three"}
	results := make(chan result, len(titles))
	for _, title := range titles {
		go func() {
			results <- run(t, client, "task add "+title+" -p tob -o json", nil, nil)
		}()
	}
	seen := map[string]bool{}
	for range titles {
		r := <-results
		if r.code != 0 {
			t.Fatalf("a concurrent add exited %d: %s", r.code, r.err)
		}
		for _, title := range titles {
			if strings.Contains(r.out, `"`+title+`"`) {
				if seen[title] {
					t.Fatalf("%q was reported twice", title)
				}
				seen[title] = true
			}
		}
	}
	if len(seen) != len(titles) {
		t.Fatalf("outputs named %v", seen)
	}
}

// The hosted transfer view prints these commands for a person to run, so each
// one has to work as printed.
func TestTheCommandsTheHostedTransferViewPrintsRun(t *testing.T) {
	_, _, client := enrolled(t, nil)
	if r := run(t, client, "task add moved -p tob", nil, nil); r.code != 0 {
		t.Fatalf("task add exited %d: %s", r.code, r.err)
	}
	bundle := run(t, client, "bundle export -p tob", nil, nil)
	if bundle.code != 0 || bundle.out == "" {
		t.Fatalf("bundle export: exit %d, err %q", bundle.code, bundle.err)
	}
	snapshot := run(t, client, "export", nil, nil)
	if snapshot.code != 0 || snapshot.out == "" {
		t.Fatalf("export: exit %d, err %q", snapshot.code, snapshot.err)
	}
	tasks := run(t, client, "export tasks -p tob", nil, nil)
	for _, c := range []struct{ line, in string }{
		{"bundle import --on-collision skip --preview", bundle.out},
		{"bundle import --on-collision skip", bundle.out},
		{"import --mode merge --dry-run", snapshot.out},
		{"import --mode merge", snapshot.out},
		{"import tasks -p tob --dry-run", tasks.out},
	} {
		if r := run(t, client, c.line, strings.NewReader(c.in), nil); r.code != 0 {
			t.Errorf("%q: exit %d, err %q", c.line, r.code, r.err)
		}
	}
}
