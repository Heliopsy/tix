// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// runBare runs the command tree with nothing opting into the local database,
// which is the environment a person installing tix starts in.
func (c *cli) runBare(extra []string, args ...string) result {
	c.t.Helper()
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errb, append(c.bare(), extra...), c.home)
	return result{code: code, out: out.String(), err: errb.String()}
}

// TestFirstRunRefusesToCreateADatabaseUnasked is the reported failure: with
// nothing configured, tix created a database on this machine and every task
// meant for a server went into it without a word.
func TestFirstRunRefusesToCreateADatabaseUnasked(t *testing.T) {
	for _, args := range [][]string{
		{"task", "add", "meant for the server"},
		{"task", "ls", "-o", "json"},
		{"whoami"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			c := newCLI(t)
			got := c.runBare(nil, args...)
			if got.code != core.ExitPrecondtion {
				t.Fatalf("exit = %d, want %d\nstdout: %s\nstderr: %s", got.code, core.ExitPrecondtion, got.out, got.err)
			}
			for _, want := range []string{"tix ctx add NAME --server URL --token TOKEN --use", "tix init", c.dbPath()} {
				if !strings.Contains(got.err, want) {
					t.Errorf("stderr does not name %q: %s", want, got.err)
				}
			}
			if got.out != "" {
				t.Errorf("stdout = %q, want nothing", got.out)
			}
			if _, err := os.Stat(c.dbPath()); err == nil {
				t.Fatal("the database was created anyway")
			}
		})
	}
}

// TestFirstRunOnATerminalAsks covers the person at a keyboard, who is asked
// rather than refused.
func TestFirstRunOnATerminalAsks(t *testing.T) {
	cases := []struct {
		answer  string
		code    int
		created bool
	}{
		{"1\n", core.ExitOK, true},
		{"local\n", core.ExitOK, true},
		{"2\n", core.ExitPrecondtion, false},
		{"\n", core.ExitPrecondtion, false},
	}
	for _, tc := range cases {
		t.Run(strings.TrimSpace(tc.answer), func(t *testing.T) {
			c := newCLI(t)
			var out, errb bytes.Buffer
			in := &tty{reply: func() string { return tc.answer }}
			code := Run([]string{"task", "add", "asked first"}, in, &out, &errb, c.bare(), c.home)
			if code != tc.code {
				t.Fatalf("exit = %d, want %d\nstderr: %s", code, tc.code, errb.String())
			}
			if !strings.Contains(errb.String(), "1) create a local database at "+c.dbPath()) {
				t.Errorf("the question does not offer the local database: %s", errb.String())
			}
			if !strings.Contains(errb.String(), "2) connect to a tix server") {
				t.Errorf("the question does not offer a server: %s", errb.String())
			}
			_, err := os.Stat(c.dbPath())
			if created := err == nil; created != tc.created {
				t.Fatalf("database created = %v, want %v", created, tc.created)
			}
		})
	}
}

// TestLocalDatabaseOptIns lists every way a person or a script says the local
// database is what they want. Each one keeps working exactly as before.
func TestLocalDatabaseOptIns(t *testing.T) {
	cases := []struct {
		name  string
		setup func(c *cli) []string
		db    func(c *cli) string
	}{
		{"target local in the environment", func(*cli) []string { return []string{"TIX_TARGET=local"} }, (*cli).dbPath},
		{"target local in the configuration file", func(c *cli) []string {
			writeUserConfig(t, c, "target: local\n")
			return nil
		}, (*cli).dbPath},
		{"tix init", func(c *cli) []string {
			if got := c.runBare(nil, "init"); got.code != core.ExitOK {
				t.Fatalf("tix init exited %d: %s", got.code, got.err)
			}
			return nil
		}, (*cli).dbPath},
		{"a database already at the default path", func(c *cli) []string {
			if got := c.runBare(nil, "--db", c.dbPath(), "task", "add", "older install"); got.code != core.ExitOK {
				t.Fatalf("seeding exited %d: %s", got.code, got.err)
			}
			return nil
		}, (*cli).dbPath},
		{"TIX_DATABASE_DSN", func(c *cli) []string {
			return []string{"TIX_DATABASE_DSN=" + filepath.Join(c.home, "env.db")}
		}, func(c *cli) string { return filepath.Join(c.home, "env.db") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			extra := tc.setup(c)
			got := c.runBare(extra, "task", "add", "kept here")
			if got.code != core.ExitOK {
				t.Fatalf("exit = %d\nstderr: %s", got.code, got.err)
			}
			if _, err := os.Stat(tc.db(c)); err != nil {
				t.Fatalf("database %s: %v", tc.db(c), err)
			}
		})
	}
	t.Run("--db", func(t *testing.T) {
		c := newCLI(t)
		db := filepath.Join(c.home, "flag.db")
		if got := c.runBare(nil, "--db", db, "task", "add", "kept here"); got.code != core.ExitOK {
			t.Fatalf("exit = %d\nstderr: %s", got.code, got.err)
		}
		if _, err := os.Stat(db); err != nil {
			t.Fatalf("database %s: %v", db, err)
		}
		if _, err := os.Stat(c.dbPath()); err == nil {
			t.Fatal("--db also created the default database")
		}
	})
}

// TestTheLocalTargetIsNamed checks the one line that says work went to this
// machine: on standard error, past --quiet, never in the data, and gone when
// asked or when a target was named.
func TestTheLocalTargetIsNamed(t *testing.T) {
	c := newCLI(t)
	optIn := []string{"TIX_TARGET=local"}
	notice := "using local database " + c.dbPath() + " (no server configured; run `tix ctx add` to use a server)"

	for _, args := range [][]string{
		{"task", "add", "first"},
		{"task", "ls"},
		{"-q", "task", "add", "quiet"},
		{"task", "ls", "-o", "json"},
	} {
		got := c.runBare(optIn, args...)
		if got.code != core.ExitOK {
			t.Fatalf("tix %s exited %d: %s", strings.Join(args, " "), got.code, got.err)
		}
		if !strings.Contains(got.err, notice) {
			t.Errorf("tix %s stderr = %q, want the notice", strings.Join(args, " "), got.err)
		}
		if strings.Contains(got.out, "using local database") {
			t.Errorf("tix %s put the notice on stdout: %q", strings.Join(args, " "), got.out)
		}
	}
	listing := c.runBare(optIn, "task", "ls", "-o", "json")
	var rows []map[string]any
	if err := json.Unmarshal([]byte(listing.out), &rows); err != nil {
		t.Fatalf("json listing does not parse: %v\n%s", err, listing.out)
	}

	if got := c.runBare(append(optIn, "TIX_CLI_TARGET_NOTICE=false"), "task", "ls"); strings.Contains(got.err, "using local database") {
		t.Errorf("TIX_CLI_TARGET_NOTICE=false still printed it: %q", got.err)
	}
	if got := c.runBare(nil, "--db", c.dbPath(), "task", "ls"); strings.Contains(got.err, "using local database") {
		t.Errorf("an explicit --db still printed it: %q", got.err)
	}
}

// TestWhoAmIReportsTheLocalDatabase checks that the local case is named as
// such, with the path, rather than looking like a server.
func TestWhoAmIReportsTheLocalDatabase(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("whoami")
	for _, want := range []string{"local database " + c.dbPath(), "no server configured", "tenant  default", "token   none"} {
		if !strings.Contains(got.out, want) {
			t.Errorf("whoami does not say %q:\n%s", want, got.out)
		}
	}
	var report identity
	if err := json.Unmarshal([]byte(c.mustRun("whoami", "-o", "json").out), &report); err != nil {
		t.Fatalf("whoami -o json: %v", err)
	}
	if report.Target != "local" || report.Database != c.dbPath() || report.Token {
		t.Fatalf("whoami -o json = %+v", report)
	}
}

// TestWhoAmIPrintsTheTenantTheme pins the theme line and the theme in JSON,
// read from the one line rather than the whole report.
func TestWhoAmIPrintsTheTenantTheme(t *testing.T) {
	c := newCLI(t)
	c.mustRun("tenant", "edit", "default", "--theme", "ocean")
	var line string
	for _, l := range strings.Split(c.mustRun("whoami").out, "\n") {
		if strings.HasPrefix(l, "theme ") {
			line = strings.TrimSpace(strings.TrimPrefix(l, "theme"))
		}
	}
	if line != "ocean (#0b7285, dark #67e8f9)" {
		t.Errorf("theme line = %q, want ocean with both accents", line)
	}
	var report identity
	if err := json.Unmarshal([]byte(c.mustRun("whoami", "-o", "json").out), &report); err != nil {
		t.Fatalf("whoami -o json: %v", err)
	}
	if report.Theme == nil || report.Theme.Name != "ocean" || report.Theme.AccentSoftDark != "#0e3440" {
		t.Fatalf("whoami -o json theme = %+v", report.Theme)
	}
}

// authRecorder sits in front of a real tix serve and keeps the Authorization
// header of every request that passes through it.
type authRecorder struct {
	mu   sync.Mutex
	seen []string
}

func (r *authRecorder) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.seen
	r.seen = nil
	return out
}

// TestEveryWayOfGivingATokenReachesTheServer is the second report: a token
// given to tix ctx add, or written in the configuration, was presented only
// by the commands that happened to read the configuration before dialling, so
// TIX_TOKEN looked like the only way in. Each command below runs once per way
// of supplying the token, and the header is read where the server receives it.
func TestEveryWayOfGivingATokenReachesTheServer(t *testing.T) {
	host := newCLI(t)
	host.mustRun("task", "add", "on the server")
	var created struct {
		Token string `json:"token"`
	}
	out := host.mustRun("token", "create", "every-way", "--scope", "*", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &created); err != nil || created.Token == "" {
		t.Fatalf("token create: %v (%s)", err, out)
	}
	token := created.Token

	addr := freePort(t)
	done := make(chan result, 1)
	go func() { done <- host.run("serve", "--listen", addr, "--no-registry") }()
	waitFor(t, "http://"+addr+"/healthz")
	defer stopServe(t, done)

	upstream, err := url.Parse("http://" + addr)
	if err != nil {
		t.Fatal(err)
	}
	rec := &authRecorder{}
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.seen = append(rec.seen, r.Header.Get("Authorization"))
		rec.mu.Unlock()
		proxy.ServeHTTP(w, r)
	}))
	defer front.Close()
	server := front.URL

	ways := []struct {
		name  string
		setup func(c *cli) (env, flags []string)
	}{
		{"ctx add --token --use", func(c *cli) ([]string, []string) {
			c.mustRun("ctx", "add", "x", "--server", server, "--token", token, "--use")
			return nil, nil
		}},
		{"--ctx with a context holding the token", func(c *cli) ([]string, []string) {
			c.mustRun("ctx", "add", "x", "--server", server, "--token", token)
			return nil, []string{"--ctx", "x"}
		}},
		{"server.token in the configuration file", func(c *cli) ([]string, []string) {
			writeUserConfig(t, c, "server:\n  url: "+server+"\n  token: "+token+"\n")
			return nil, nil
		}},
		{"TIX_SERVER_TOKEN", func(*cli) ([]string, []string) {
			return []string{"TIX_SERVER_URL=" + server, "TIX_SERVER_TOKEN=" + token}, nil
		}},
		{".env", func(c *cli) ([]string, []string) {
			body := "TIX_SERVER_URL=" + server + "\nTIX_SERVER_TOKEN=" + token + "\n"
			if err := os.WriteFile(filepath.Join(c.home, ".env"), []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			return nil, nil
		}},
		{"TIX_TOKEN", func(*cli) ([]string, []string) {
			return []string{"TIX_TOKEN=" + token}, []string{"--server", server}
		}},
		{"--token", func(*cli) ([]string, []string) {
			return nil, []string{"--server", server, "--token", token}
		}},
	}
	commands := [][]string{
		{"task", "show", "default-1"},
		{"task", "ls"},
		{"task", "add", "from the client"},
		{"status"},
		{"whoami"},
		{"claim", "next"},
	}
	for _, way := range ways {
		t.Run(way.name, func(t *testing.T) {
			c := newCLI(t)
			env, flags := way.setup(c)
			for _, command := range commands {
				rec.take()
				args := append(append([]string{}, flags...), command...)
				got := c.runBare(env, args...)
				if got.code != core.ExitOK {
					t.Errorf("tix %s exited %d: %s", strings.Join(command, " "), got.code, strings.TrimSpace(got.err))
					continue
				}
				seen := rec.take()
				if len(seen) == 0 {
					t.Errorf("tix %s reached no server", strings.Join(command, " "))
				}
				for _, header := range seen {
					if header != "Bearer "+token {
						t.Errorf("tix %s sent Authorization %q, want the token", strings.Join(command, " "), redactAuth(header, token))
					}
				}
			}
			if got := c.runBare(env, append(flags, "whoami")...); !strings.Contains(got.out, "token   yes, from ") {
				t.Errorf("whoami does not say a token is in use:\n%s", got.out)
			}
		})
	}
}

// redactAuth keeps a token out of a failure message.
func redactAuth(header, token string) string {
	return strings.ReplaceAll(header, token, "<token>")
}
