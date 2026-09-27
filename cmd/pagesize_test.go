// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
)

// seedRows fills a store with more rows than any page size used below, so a
// short page is a page size taking effect rather than a store running out.
func seedRows(t *testing.T, c *cli) {
	t.Helper()
	for i := range 6 {
		c.mustRun("task", "add", "row "+strconv.Itoa(i))
	}
}

// rowsOf counts the records one ndjson listing printed. ndjson rather than the
// table, because one record is one line and nothing else on stdout is.
func rowsOf(t *testing.T, out string) int {
	t.Helper()
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "{") {
			n++
		}
	}
	return n
}

// listingCommands are the commands whose --limit now falls back to
// cli.page_size. Each one is listed with an invocation a fresh store answers
// with more rows than the page size under test.
var listingCommands = [][]string{
	{"task", "ls"},
	{"project", "ls"},
	{"audit", "ls"},
}

// limitCommands is every command carrying a --limit that pages a listing. The
// three in listingCommands above are covered end to end; the rest are here
// because a fresh store cannot be seeded past a page of tenants, actors, users
// or webhook deliveries from the command line, so what is checked for them is
// that their flag declares the configured default rather than the contract's.
var limitCommands = []string{
	"task ls", "project ls", "audit ls",
	"actor ls", "tenant ls", "user ls", "webhook deliveries",
}

// TestEveryLimitFlagDeclaresTheConfiguredDefault catches a listing added with
// core.DefaultPageLimit as its --limit default, which would print fifty rows
// and report cli.page_size as the source of a number nobody used.
func TestEveryLimitFlagDeclaresTheConfiguredDefault(t *testing.T) {
	want := strconv.Itoa(config.DefaultPageSize)
	for _, path := range limitCommands {
		t.Run(path, func(t *testing.T) {
			flag := commandNamed(t, path).Flags().Lookup("limit")
			if flag == nil {
				t.Fatalf("tix %s has no --limit flag", path)
			}
			if flag.DefValue != want {
				t.Errorf("tix %s declares --limit %s, want the configured default %s",
					path, flag.DefValue, want)
			}
		})
	}
}

// TestEveryListingShowsTheConfiguredNumberOfRows is the end-to-end half: the
// key in a configuration file, no --limit, and the assertion is how many
// records the command printed.
//
// Counting printed records is the point. Asserting on the resolved
// configuration, or on the flag's variable, would pass while the number never
// reached a query, which is how the fallback branch this replaced came to be
// dead code: query.Parse ends in Validate, which had already filled the limit
// in, so the branch that was supposed to apply the flag never ran.
func TestEveryListingShowsTheConfiguredNumberOfRows(t *testing.T) {
	const want = 2
	for _, args := range listingCommands {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			c := newCLI(t)
			writeUserConfig(t, c, "cli:\n  page_size: "+strconv.Itoa(want)+"\n")
			seedRows(t, c)

			got := c.mustRun(append(args, "-o", "ndjson")...)
			if rows := rowsOf(t, got.out); rows != want {
				t.Errorf("tix %s printed %d rows with cli.page_size %d, want %d",
					strings.Join(args, " "), rows, want, want)
			}
			// The short page has to be a page, not the end of the listing:
			// without this a limit that reached nothing would still pass
			// whenever the store happened to hold exactly that many rows.
			if !strings.Contains(got.err, "next cursor") {
				t.Errorf("tix %s reported no further page: %q", strings.Join(args, " "), got.err)
			}
		})
	}
}

// TestTheEnvironmentSetsTheListingPageSize is the layer a container reaches.
func TestTheEnvironmentSetsTheListingPageSize(t *testing.T) {
	c := newCLI(t)
	seedRows(t, c)

	var out, errb strings.Builder
	env := append(c.environ(), config.EnvName("cli.page_size")+"=3")
	if code := Run([]string{"task", "ls", "-o", "ndjson"},
		strings.NewReader(""), &out, &errb, env, c.home); code != core.ExitOK {
		t.Fatalf("task ls exited %d: %s", code, errb.String())
	}
	if rows := rowsOf(t, out.String()); rows != 3 {
		t.Errorf("task ls printed %d rows with TIX_CLI_PAGE_SIZE=3, want 3", rows)
	}
}

// TestWhatOutranksTheConfiguredPageSize keeps the fallback from becoming the
// opposite defect, a configured value that now wins over what was asked for.
func TestWhatOutranksTheConfiguredPageSize(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want int
	}{
		{"the flag", []string{"--limit", "4"}, 4},
		{"a limit in the filter expression", []string{"--filter", "limit:5"}, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			writeUserConfig(t, c, "cli:\n  page_size: 1\n")
			seedRows(t, c)

			got := c.mustRun(append([]string{"task", "ls", "-o", "ndjson"}, tc.args...)...)
			if rows := rowsOf(t, got.out); rows != tc.want {
				t.Errorf("task ls %s printed %d rows over cli.page_size 1, want %d",
					strings.Join(tc.args, " "), rows, tc.want)
			}
		})
	}
}

// TestAnUnusablePageSizeIsRefusedRatherThanReplaced states what happens at the
// command line, not only in Validate: the run fails, names the key, and prints
// no listing at all.
func TestAnUnusablePageSizeIsRefusedRatherThanReplaced(t *testing.T) {
	for _, value := range []string{"0", "-1", "100000"} {
		t.Run(value, func(t *testing.T) {
			c := newCLI(t)
			writeUserConfig(t, c, "cli:\n  page_size: "+value+"\n")

			got := c.run("task", "ls", "-o", "ndjson")
			switch {
			case got.code == core.ExitOK:
				t.Errorf("a page size of %s was accepted: %q", value, got.out)
			case !strings.Contains(got.err, "cli.page_size"):
				t.Errorf("the refusal does not name the key: %q", got.err)
			case rowsOf(t, got.out) != 0:
				t.Errorf("a refused page size still printed rows: %q", got.out)
			}
		})
	}
}

// taskRow matches one row of the browser's task list. The whole document would
// not do: a count taken over the page would also find the titles the shortcut
// data carries, so a listing that rendered nothing could still look full.
var taskRow = regexp.MustCompile(`<li id="t-[^"]+" class="task`)

// TestServeGivesTheBrowserTheConfiguredPageSize covers the call site rather
// than the option. internal/web asserts that WithPageSize decides how many
// rows a listing carries; nothing there can notice that runServe stopped
// passing it, and a forgotten call site is how server.listen died.
//
// The assertion is rows in a page fetched over HTTP from a real `tix serve`.
func TestServeGivesTheBrowserTheConfiguredPageSize(t *testing.T) {
	const want = 2
	c := newCLI(t)
	seedRows(t, c)

	var created struct {
		Token string `json:"token"`
	}
	out := c.mustRun("token", "create", "browser", "--scope", "task:read",
		"--scope", "project:read", "--scope", "workflow:read", "-o", "json").out
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("token create: %v (%s)", err, out)
	}
	writeUserConfig(t, c, "web:\n  page_size: "+strconv.Itoa(want)+"\n")

	addr := freePort(t)
	done := make(chan result, 1)
	go func() { done <- c.run("serve", "--listen", addr) }()
	waitFor(t, "http://"+addr+"/healthz")
	defer stopServe(t, done)

	page := fetchPage(t, "http://"+addr+"/tasks", created.Token)
	if rows := len(taskRow.FindAllString(page, -1)); rows != want {
		t.Errorf("the browser task list carried %d rows with web.page_size %d, want %d",
			rows, want, want)
	}
}

// fetchPage reads one browser screen from a running server, authenticated the
// way an API client is. A redirect or a refusal is a failure rather than a page
// with no rows in it, which would pass a row count of zero.
func fetchPage(t *testing.T, url, token string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("building the request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("getting %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading %s: %v", url, err)
	}
	return string(body)
}
