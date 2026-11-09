// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Exit codes Main returns.
const (
	ExitOK     = 0
	ExitBudget = 1
	ExitUsage  = 2
	ExitSetup  = 3
)

// defaultBudgetUsers is the largest profile the 128 MiB default applies to.
// Larger profiles carry no default: name one with --max-rss once a run has
// shown what that profile needs.
const defaultBudgetUsers = 4

type options struct {
	cfg     Config
	mix     string
	maxRSS  string
	report  string
	tix     string
	dir     string
	keep    bool
	pg      string
	bodyMin string
	bodyMax string
	remote  RemoteOptions
	noDelay bool
	getenv  func(string) string
}

func parse(args []string, stderr io.Writer, getenv func(string) string) (*options, error) {
	o := &options{getenv: getenv}
	fs := flag.NewFlagSet("tixload", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.IntVar(&o.cfg.Users, "users", 4, "virtual users")
	fs.DurationVar(&o.cfg.Duration, "duration", 2*time.Minute, "how long the load runs")
	fs.StringVar(&o.mix, "mix", DefaultMix, "weights of browser, mobile and agent users")
	fs.DurationVar(&o.cfg.Think, "think", 2*time.Second, "mean pause between two actions of one user")
	fs.StringVar(&o.maxRSS, "max-rss", "", "fail when the server's peak RSS exceeds this (default 128MiB up to 4 users, none above; 0 disables)")
	fs.DurationVar(&o.cfg.Sample, "sample", time.Second, "how often the server's memory is read")
	fs.Float64Var(&o.cfg.Warmup, "warmup", 0.25, "share of the run left out of the steady-state memory figure")
	fs.StringVar(&o.report, "report", "out/load/report.json", "where the JSON report is written; the text report goes beside it")
	fs.StringVar(&o.tix, "tix", "bin/tix", "tix binary a local run seeds and serves with")
	fs.StringVar(&o.dir, "dir", "", "empty scratch directory for a local run, kept afterwards (default: a new temporary directory, removed afterwards)")
	fs.BoolVar(&o.keep, "keep", false, "keep the temporary scratch directory after a local run")
	fs.StringVar(&o.pg, "pg", "", "empty, disposable PostgreSQL DSN to serve from instead of SQLite")
	fs.IntVar(&o.cfg.Seed.Tenants, "tenants", DefaultSeed.Tenants, "tenants a local run seeds")
	fs.IntVar(&o.cfg.Seed.UsersPerTenant, "people", DefaultSeed.UsersPerTenant, "people, each with a password and a token, per tenant")
	fs.IntVar(&o.cfg.Seed.Projects, "projects", DefaultSeed.Projects, "projects per tenant")
	fs.IntVar(&o.cfg.Seed.Tasks, "tasks", DefaultSeed.Tasks, "tasks a local run seeds, over all tenants")
	fs.StringVar(&o.bodyMin, "body-min", "5KiB", "smallest seeded description")
	fs.StringVar(&o.bodyMax, "body-max", "50KiB", "largest seeded description")
	fs.StringVar(&o.remote.URL, "target", "", "url of a running server to load instead of a local scratch one")
	fs.StringVar(&o.remote.Token, "token", "", "api token for a remote run (or TIX_LOAD_TOKEN)")
	fs.StringVar(&o.remote.Tenant, "tenant", "", "tenant key a remote sign-in names")
	fs.StringVar(&o.remote.Email, "email", "", "account browser users sign in as on a remote run; the password is read from TIX_LOAD_PASSWORD")
	fs.BoolVar(&o.remote.AllowWrites, "allow-writes", false, "let agents write on a remote run, confined to --project or a new scratch project")
	fs.StringVar(&o.remote.Project, "project", "", "project a remote run writes to; created when missing")
	fs.BoolVar(&o.noDelay, "no-delay", false, "skip the five second pause after the remote warning banner")
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	if fs.NArg() > 0 {
		return nil, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	var err error
	if o.cfg.Mix, err = ParseMix(o.mix); err != nil {
		return nil, err
	}
	if o.cfg.Seed.MinBody, err = sizeInt(o.bodyMin); err != nil {
		return nil, err
	}
	if o.cfg.Seed.MaxBody, err = sizeInt(o.bodyMax); err != nil {
		return nil, err
	}
	if o.cfg.Seed.MaxBody < o.cfg.Seed.MinBody {
		return nil, fmt.Errorf("--body-max is smaller than --body-min")
	}
	if o.cfg.MaxRSS, err = budgetFor(o.maxRSS, o.cfg.Users); err != nil {
		return nil, err
	}
	tokenFlag := o.remote.Token != ""
	if !tokenFlag {
		o.remote.Token = getenv("TIX_LOAD_TOKEN")
	}
	o.remote.Password = getenv("TIX_LOAD_PASSWORD")
	if o.remote.URL != "" {
		if err := CheckRemote(o.remote); err != nil {
			return nil, err
		}
		if o.remote.Email == "" && o.cfg.Mix[KindBrowser] > 0 {
			o.cfg.Mix[KindBrowser] = 0
			if o.cfg.Mix.total() == 0 {
				return nil, fmt.Errorf("browser users need --email and TIX_LOAD_PASSWORD, and the mix has no other kind")
			}
		}
	} else if o.remote.AllowWrites || o.remote.Project != "" || tokenFlag || o.remote.Email != "" {
		return nil, fmt.Errorf("--token, --email, --allow-writes and --project are for a remote run with --target; a local run writes only to its scratch database")
	}
	if s := o.cfg.Seed; o.remote.URL == "" && (s.Tenants < 1 || s.UsersPerTenant < 1 || s.Projects < 1 || s.Tasks < 0) {
		return nil, fmt.Errorf("--tenants, --people and --projects must be at least 1")
	}
	return o, o.cfg.Validate()
}

func sizeInt(s string) (int, error) {
	n, err := ParseSize(s)
	return int(n), err
}

// budgetFor is the memory budget a run enforces: what --max-rss says, or the
// 128 MiB default for a profile of up to four users.
func budgetFor(flagValue string, users int) (int64, error) {
	if flagValue != "" {
		return ParseSize(flagValue)
	}
	if users <= defaultBudgetUsers {
		return DefaultMaxRSS, nil
	}
	return 0, nil
}

// Main runs tixload with args and returns its exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return run(ctx, args, stdout, stderr, os.Getenv)
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	o, err := parse(args, stderr, getenv)
	if errors.Is(err, flag.ErrHelp) {
		return ExitOK
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tixload: %v\n", err)
		return ExitUsage
	}
	logf := func(format string, a ...any) { _, _ = fmt.Fprintf(stderr, format+"\n", a...) }

	var rep Report
	if o.remote.URL != "" {
		rep, err = runRemote(ctx, o, stderr, logf)
	} else {
		rep, err = runLocal(ctx, o, logf)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "tixload: %v\n", err)
		return ExitSetup
	}
	if err := writeReports(&rep, o.report, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "tixload: %v\n", err)
		return ExitSetup
	}
	if rep.Failed() {
		return ExitBudget
	}
	return ExitOK
}

func runLocal(ctx context.Context, o *options, logf func(string, ...any)) (Report, error) {
	dir := o.dir
	if dir == "" {
		d, err := os.MkdirTemp("", "tixload-")
		if err != nil {
			return Report{}, fmt.Errorf("making a scratch directory: %w", err)
		}
		dir = d
		if o.keep {
			logf("scratch directory kept at %s", dir)
		} else {
			defer func() { _ = os.RemoveAll(dir) }()
		}
	} else if entries, err := os.ReadDir(dir); err != nil || len(entries) > 0 {
		return Report{}, fmt.Errorf("--dir %s must be an existing, empty directory; it is left in place afterwards", dir)
	}
	tix, err := filepath.Abs(o.tix)
	if err != nil {
		return Report{}, fmt.Errorf("resolving --tix: %w", err)
	}
	if _, err := os.Stat(tix); err != nil {
		return Report{}, fmt.Errorf("no tix binary at %s: run `just build` first or pass --tix", tix)
	}
	l, err := StartLocal(ctx, LocalOptions{Tix: tix, Dir: dir, DSN: o.pg, Seed: o.cfg.Seed, Logf: logf})
	if err != nil {
		return Report{}, err
	}
	defer func() {
		if err := l.Stop(); err != nil {
			logf("stopping the server: %v", err)
		}
	}()
	logf("running %d users (%s) for %s", o.cfg.Users, o.cfg.Mix, o.cfg.Duration)
	tgt := Target{Base: l.Base, AllowWrites: true, Accounts: l.Accounts}
	return Run(ctx, o.cfg, tgt, l.Probe, Report{Mode: "local", Target: l.Base, Engine: l.Engine, Seeded: l.Seeded}, logf)
}

func runRemote(ctx context.Context, o *options, stderr io.Writer, logf func(string, ...any)) (Report, error) {
	Banner(stderr, o.remote, o.cfg)
	if !o.noDelay {
		logf("starting in 5 seconds, interrupt to cancel")
		select {
		case <-ctx.Done():
			return Report{}, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	acct := Account{Tenant: o.remote.Tenant, Email: o.remote.Email, Password: o.remote.Password, Token: o.remote.Token}
	tgt := Target{Base: strings.TrimRight(o.remote.URL, "/"), Remote: true, Accounts: []Account{acct}}
	if o.remote.AllowWrites {
		if acct.Token == "" {
			return Report{}, fmt.Errorf("--allow-writes needs --token: the scratch project is created with it")
		}
		key, err := ScratchProject(ctx, tgt.Base, acct, o.remote.Project)
		if err != nil {
			return Report{}, err
		}
		logf("writes go to project %s", key)
		tgt.AllowWrites, tgt.Project = true, key
	}
	probe := Probe{Why: "the server is remote; read its memory with kubectl top or the platform's metrics"}
	return Run(ctx, o.cfg, tgt, probe, Report{Mode: "remote", Target: tgt.Base}, logf)
}

func writeReports(r *Report, path string, stdout io.Writer) error {
	if err := r.WriteText(stdout); err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("making the report directory: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating the report: %w", err)
	}
	if err := r.WriteJSON(f); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing the report: %w", err)
	}
	txt, err := os.Create(strings.TrimSuffix(path, filepath.Ext(path)) + ".txt")
	if err != nil {
		return fmt.Errorf("creating the text report: %w", err)
	}
	defer func() { _ = txt.Close() }()
	return r.WriteText(txt)
}
