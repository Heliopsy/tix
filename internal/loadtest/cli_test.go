// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"syscall"
	"testing"
)

// fakeTixEnv makes the test binary stand in for tix, so a local run's
// bootstrap, seeding and server lifecycle run without building tix.
const fakeTixEnv = "TIXLOAD_FAKE_TIX"

func TestMain(m *testing.M) {
	if os.Getenv(fakeTixEnv) == "1" {
		os.Exit(fakeTix(os.Args[1:]))
	}
	os.Exit(m.Run())
}

// The stand-in runs with GODEBUG=gctrace=1 like the real server, so its own
// collections would print real gctrace lines over the scripted one and leave
// the heap the test reads to the runtime's timing. Collection stays off.
func fakeTix(args []string) int {
	debug.SetGCPercent(-1)
	switch {
	case slices.Contains(args, "serve"):
		i := slices.Index(args, "--listen")
		srv := &http.Server{Addr: args[i+1], Handler: http.HandlerFunc(fakeAPI)}
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGTERM)
		go func() { <-stop; _ = srv.Close() }()
		_, _ = fmt.Fprintln(os.Stderr, "gc 1 @0.010s 1%: 0.01 ms clock, 0.1 ms cpu, 4->4->3 MB, 4 MB goal, 8 P")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return 1
		}
		return 0
	case slices.Contains(args, "token"):
		_, _ = fmt.Fprintln(os.Stdout, `{"token":"tix_pat_fake"}`)
	}
	return 0
}

func fakeAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.URL.Path == "/readyz":
		_, _ = io.WriteString(w, `{"status":"ok"}`)
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/tasks":
		_, _ = io.WriteString(w, `{"ref":"ops-1","status":"todo","version":1}`)
	default:
		_, _ = io.WriteString(w, `{"items":[{"id":"a1","kind":"user","key":"ops","ref":"ops-1","status":"todo","version":1}]}`)
	}
}

func TestLocalRunWithAStandInTix(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the stand-in server is stopped with SIGTERM and measured through /proc")
	}
	t.Setenv(fakeTixEnv, "1")
	dir := t.TempDir()
	report := filepath.Join(dir, "out", "report.json")
	var stdout, stderr strings.Builder
	args := []string{"--tix", os.Args[0], "--users", "1", "--duration", "1s", "--mix", "agent=1", "--think", "100ms",
		"--tenants", "2", "--people", "1", "--tasks", "4", "--body-min", "1KiB", "--body-max", "2KiB",
		"--sample", "200ms", "--max-rss", "64GiB", "--report", report}
	code := run(context.Background(), args, &stdout, &stderr, os.Getenv)
	if code != ExitOK {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout.String(), stderr.String())
	}
	var r Report
	raw, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	if r.Mode != "local" || r.Requests == 0 || !r.Memory.Available || r.Memory.PeakHeap != 3*MiB || !r.Budget.Pass {
		t.Fatalf("report: mode %q, %d requests, memory %v heap %d, budget %+v", r.Mode, r.Requests, r.Memory.Available, r.Memory.PeakHeap, r.Budget)
	}
	if _, err := os.Stat(strings.TrimSuffix(report, ".json") + ".txt"); err != nil {
		t.Fatalf("no text report beside the JSON one: %v", err)
	}
	if !strings.Contains(stdout.String(), "budget: PASS") {
		t.Fatalf("stdout lacks the verdict:\n%s", stdout.String())
	}

	code = run(context.Background(), append(args[:len(args)-4:len(args)-4], "--max-rss", "1KiB", "--report", ""), &stdout, &stderr, os.Getenv)
	if code != ExitBudget {
		t.Fatalf("a 1 KiB budget: exit %d, want %d", code, ExitBudget)
	}
}

func TestRemoteRunAgainstAnInProcessServer(t *testing.T) {
	if testing.Short() {
		t.Skip("runs real load for a second")
	}
	base, acct := inProcessServer(t)
	env := func(k string) string {
		return map[string]string{"TIX_LOAD_TOKEN": acct.Token, "TIX_LOAD_PASSWORD": acct.Password}[k]
	}
	var stdout, stderr strings.Builder
	args := []string{"--target", base, "--email", acct.Email, "--allow-writes", "--no-delay",
		"--users", "2", "--mix", "browser=1,agent=1", "--duration", "1s", "--think", "100ms", "--report", ""}
	if code := run(context.Background(), args, &stdout, &stderr, env); code != ExitOK {
		t.Fatalf("exit %d\n%s\n%s", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{"REMOTE LOAD TEST", "WRITES ENABLED", "writes go to project loadtest"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
		}
	}
	if !strings.Contains(stdout.String(), "not measured") || !strings.Contains(stdout.String(), "budget: not checked") {
		t.Errorf("a remote report should say memory was not measured:\n%s", stdout.String())
	}
}

func TestRunRejectsBadFlags(t *testing.T) {
	env := func(string) string { return "" }
	for _, args := range [][]string{{"--users", "0"}, {"--mix", "x=1"}, {"--body-min", "9KiB", "--body-max", "1KiB"}, {"extra"}, {"--max-rss", "lots"}} {
		if code := run(context.Background(), args, io.Discard, io.Discard, env); code != ExitUsage {
			t.Errorf("run(%v) = %d, want %d", args, code, ExitUsage)
		}
	}
	if code := run(context.Background(), []string{"--tix", "/nonexistent/tix"}, io.Discard, io.Discard, env); code != ExitSetup {
		t.Errorf("a missing tix binary: exit %d, want %d", code, ExitSetup)
	}
}

func TestTixBinaryMustBeAnExecutableFile(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, "tix")
	if err := os.WriteFile(plain, []byte("not a program"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{plain, dir, filepath.Join(dir, "missing")} {
		if _, err := TixBinary(path); err == nil {
			t.Errorf("TixBinary(%q) accepted something that is not an executable file", path)
		}
	}
	if got, err := TixBinary(os.Args[0]); err != nil || !filepath.IsAbs(got) {
		t.Fatalf("TixBinary(test binary) = %q, %v; want an absolute path", got, err)
	}
}

func TestBodySizesAreBounded(t *testing.T) {
	if _, err := sizeInt("65MiB"); err == nil {
		t.Fatal("a 65 MiB description was accepted")
	}
	if n, err := sizeInt("50KiB"); err != nil || n != 50<<10 {
		t.Fatalf("sizeInt(50KiB) = %d, %v", n, err)
	}
}
