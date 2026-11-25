// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build smoke

package smoke

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/testenv"
)

// examplesRequiredEnv makes a missing toolchain a failure instead of a skip.
// CI sets it, so an example cannot stop being run because a runner lost node.
const examplesRequiredEnv = "TIX_EXAMPLES_REQUIRED"

// exampleScopes are the scopes docs/examples.md tells a reader to mint. The
// examples run under exactly these, so the list cannot fall short of the walk.
var exampleScopes = []string{
	"project:write", "task:read", "task:write", "task:transition", "comment:write", "event:subscribe",
}

// example is one of the programs under examples/, and what it needs to run.
type example struct {
	name  string
	dir   string
	cmd   []string
	needs []string
	check func() string
}

func examples() []example {
	return []example{
		{name: "sh", dir: ".", cmd: []string{"sh", "examples/sh/tix-example.sh"}, needs: []string{"sh", "curl", "jq"}},
		{name: "python", dir: ".", cmd: []string{"python3", "examples/python/tix_example.py"}, needs: []string{"python3"}},
		{
			name: "typescript", dir: ".", needs: []string{"node"}, check: nodeCanStripTypes,
			cmd: []string{"node", "--experimental-strip-types", "--no-warnings", "examples/typescript/tix-example.ts"},
		},
		{name: "go", dir: "examples/go", cmd: []string{"go", "run", "."}, needs: []string{"go"}},
	}
}

// TestTheAPIExamplesRunAgainstAServer runs every program docs/examples.md
// shows against a scratch server and then reads back what each one left, so
// an example that prints the right lines without doing the work still fails.
func TestTheAPIExamplesRunAgainstAServer(t *testing.T) {
	s := newScratch(t)
	db := s.path("examples.db")
	args := []string{"--db", db, "token", "create", "examples", "-o", "json"}
	for _, sc := range exampleScopes {
		args = append(args, "--scope", sc)
	}
	var minted struct{ Token string }
	if out := s.mustRun(args...).out; json.Unmarshal([]byte(out), &minted) != nil || minted.Token == "" {
		t.Fatalf("no token in %s", out)
	}
	admin := mintToken(t, s, db)

	addr := freePort(t)
	base := "http://" + addr
	server := s.start("--db", db, "serve", "--listen", addr, "--insecure-no-tls")
	waitForHTTP(t, base+"/healthz", server)

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, ex := range examples() {
		t.Run(ex.name, func(t *testing.T) {
			requireToolchain(t, ex)
			key := "ex" + ex.name
			cmd := exec.Command(ex.cmd[0], ex.cmd[1:]...) // #nosec G204 -- a fixed table of example programs
			cmd.Dir = filepath.Join(root, ex.dir)
			cmd.Env = append(os.Environ(), "TIX_URL="+base, "TIX_TOKEN="+minted.Token, "TIX_PROJECT="+key, "GOWORK=off")
			var out, errb bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &errb
			if err := cmd.Run(); err != nil {
				t.Fatalf("%s: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(ex.cmd, " "), err, out.String(), errb.String())
			}
			if strings.Contains(out.String()+errb.String(), minted.Token) {
				t.Fatalf("%s printed the token", ex.name)
			}
			assertExampleOutput(t, out.String(), key)
			assertExampleState(t, base, admin, key)
		})
	}
}

// assertExampleOutput holds a run to the lines docs/examples.md describes.
func assertExampleOutput(t *testing.T, out, key string) {
	t.Helper()
	ref := key + "-1"
	for _, want := range []string{
		"signed in as examples (",
		"created project " + key,
		"created " + ref + " at version 1",
		"updated " + ref + " to version 2",
		"conflict: " + ref + " is at version 2, retrying",
		"retitled " + ref + " at version 3",
		ref + " is now doing",
		"listed " + ref + " doing p1 Try the tix HTTP API",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the output never says %q:\n%s", want, out)
		}
	}
	events := regexp.MustCompile(`(?m)^event \d+ (\S+)$`).FindAllStringSubmatch(out, -1)
	seen := map[string]int{}
	for _, e := range events {
		seen[e[1]]++
	}
	for typ, n := range map[string]int{"task.created": 1, "task.updated": 2, "task.transitioned": 1, "comment.added": 1} {
		if seen[typ] < n {
			t.Errorf("read %d %s events, want at least %d:\n%s", seen[typ], typ, n, out)
		}
	}
}

// assertExampleState reads the task back with another credential, which is
// what proves the requests were made and not merely printed.
func assertExampleState(t *testing.T, base, token, key string) {
	t.Helper()
	body, code := get(t, base+"/api/v1/tasks/"+key+"-1", token)
	if code != 200 {
		t.Fatalf("GET the example's task = %d\n%s", code, body)
	}
	var task struct {
		Title    string    `json:"title"`
		Status   string    `json:"status"`
		Priority int       `json:"priority"`
		Version  int       `json:"version"`
		DueAt    time.Time `json:"due_at"`
	}
	if err := json.Unmarshal([]byte(body), &task); err != nil {
		t.Fatalf("decoding the task: %v\n%s", err, body)
	}
	due := time.Date(2030, 1, 15, 17, 0, 0, 0, time.UTC)
	if task.Title != "Try the tix HTTP API" || task.Status != "doing" || task.Priority != 1 ||
		task.Version != 4 || !task.DueAt.Equal(due) {
		t.Errorf("the example left the task as %+v", task)
	}
	body, code = get(t, base+"/api/v1/tasks/"+key+"-1/comments", token)
	if code != 200 || strings.Count(body, "Started from the API example.") != 1 {
		t.Errorf("GET the example's comments = %d, want the one comment:\n%s", code, body)
	}
}

// requireToolchain skips an example whose runtime is missing, or fails when
// CI asked for every one of them.
func requireToolchain(t *testing.T, ex example) {
	t.Helper()
	why := ""
	for _, bin := range ex.needs {
		if _, err := exec.LookPath(bin); err != nil {
			why = bin + " is not on PATH"
			break
		}
	}
	if why == "" && ex.check != nil {
		why = ex.check()
	}
	if why == "" {
		return
	}
	if os.Getenv(examplesRequiredEnv) != "" {
		t.Fatalf("the %s example cannot run: %s, and %s is set", ex.name, why, examplesRequiredEnv)
	}
	testenv.Skip(t, testenv.Capability{
		Name: "example-" + ex.name,
		Why:  why + ", so examples/ in that language was not run against a server",
		How:  "install it and rerun: just smoke",
	})
}

// nodeCanStripTypes reports why node cannot run a .ts file, or "" when it can:
// type stripping arrived in Node 22.6.
func nodeCanStripTypes() string {
	out, err := exec.Command("node", "--version").Output()
	if err != nil {
		return "node --version failed: " + err.Error()
	}
	v := strings.TrimPrefix(strings.TrimSpace(string(out)), "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return "node reports the version " + v
	}
	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	if major < 22 || major == 22 && minor < 6 {
		return "node " + v + " predates type stripping (22.6)"
	}
	return ""
}
