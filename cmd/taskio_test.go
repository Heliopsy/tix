// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// tixCSV is a small file in the shape tix export writes.
const tixCSV = "title,status,priority\nFix login,todo,high\nWrite release notes,todo,low\n"

// A dry run reports the plan and writes nothing; the real run creates the
// tasks, and the saved mapping can be passed back.
func TestImportTasksDryRunsThenWrites(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	file := filepath.Join(c.home, "tasks.csv")
	if err := os.WriteFile(file, []byte(tixCSV), 0o600); err != nil {
		t.Fatal(err)
	}
	mapping := filepath.Join(c.home, "mapping.yaml")

	dry := c.mustRun("import", "tasks", "-p", "infra", file, "--dry-run", "--save-mapping", mapping)
	if !strings.HasPrefix(dry.out, "dry run: 2 rows read as csv") || !strings.Contains(dry.out, "2 create") {
		t.Errorf("the dry run report does not lead with its counts:\n%s", dry.out)
	}
	for _, want := range []string{"Fix login", "Write release notes", "title"} {
		if !strings.Contains(dry.out, want) {
			t.Errorf("the dry run report lacks %q:\n%s", want, dry.out)
		}
	}
	if got := c.mustRun("task", "ls", "-p", "infra"); strings.Contains(got.out, "Fix login") {
		t.Fatalf("a dry run wrote a task:\n%s", got.out)
	}
	saved, err := os.ReadFile(mapping)
	if err != nil || !strings.Contains(string(saved), "title") {
		t.Fatalf("--save-mapping wrote %q, %v", saved, err)
	}

	real := c.mustRun("import", "tasks", "-p", "infra", file, "--mapping", mapping, "-o", "json")
	var res core.TaskImportResult
	if err := json.Unmarshal([]byte(real.out), &res); err != nil {
		t.Fatalf("json report: %v\n%s", err, real.out)
	}
	if !res.Committed || res.Created != 2 || res.DryRun {
		t.Errorf("the import reported %+v, want two tasks committed", res)
	}
	if got := c.mustRun("task", "ls", "-p", "infra"); !strings.Contains(got.out, "Fix login") ||
		!strings.Contains(got.out, "Write release notes") {
		t.Errorf("the imported tasks are not listed:\n%s", got.out)
	}

	again := c.mustRun("import", "tasks", "-p", "infra", file, "--mode", "skip")
	if !strings.HasPrefix(again.out, "imported:") {
		t.Errorf("a second import does not report itself:\n%s", again.out)
	}
}

// A list in quick-add syntax arrives on standard input with --lines -.
func TestImportTasksReadsALineList(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	got := c.runIn("Fix login #bug\nWrite release notes\n", "import", "tasks", "-p", "infra", "--lines", "-")
	if got.code != core.ExitOK {
		t.Fatalf("--lines exited %d\n%s", got.code, got.err)
	}
	if !strings.Contains(got.out, "2 create") {
		t.Errorf("--lines did not create two tasks:\n%s", got.out)
	}
	list := c.mustRun("task", "ls", "-p", "infra")
	if !strings.Contains(list.out, "Fix login") {
		t.Errorf("the line list was not imported:\n%s", list.out)
	}

	both := c.run("import", "tasks", "-p", "infra", "--lines", "-", "extra.csv")
	if both.code != core.ExitUsage || !strings.Contains(both.err, "--lines") {
		t.Errorf("--lines beside a FILE exited %d: %s", both.code, both.err)
	}
}

// Every refusal names what was wrong and exits with the documented code, and
// a file with a row in error writes nothing.
func TestImportTasksRefusals(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	missing := filepath.Join(c.home, "nope.csv")

	cases := []struct {
		name string
		args []string
		code int
		want string
	}{
		{"missing file", []string{"import", "tasks", "-p", "infra", missing}, core.ExitUsage, "opening"},
		{"missing mapping", []string{"import", "tasks", "-p", "infra", "--mapping", missing}, core.ExitUsage, "reading mapping"},
		{"bad mode", []string{"import", "tasks", "-p", "infra", "--mode", "merge"}, core.ExitUsage, "mode"},
		{"unknown project", []string{"import", "tasks", "-p", "ghost"}, core.KindNotFound.ExitCode(), "ghost"},
	}
	for _, tc := range cases {
		got := c.runIn(tixCSV, tc.args...)
		if got.code != tc.code || !strings.Contains(got.err, tc.want) {
			t.Errorf("%s: exit %d (want %d), stderr %q lacks %q", tc.name, got.code, tc.code, got.err, tc.want)
		}
	}

	bad := c.runIn("title,status\n,todo\nGood row,todo\n", "import", "tasks", "-p", "infra")
	if bad.code != core.KindInvalid.ExitCode() || !strings.Contains(bad.err, "nothing was imported") {
		t.Fatalf("a row in error exited %d: %s", bad.code, bad.err)
	}
	if !strings.HasPrefix(bad.out, "nothing imported:") || !strings.Contains(bad.out, "line 2:") {
		t.Errorf("the report does not name the row in error:\n%s", bad.out)
	}
	if got := c.mustRun("task", "ls", "-p", "infra"); strings.Contains(got.out, "Good row") {
		t.Errorf("a file with a row in error wrote its good rows:\n%s", got.out)
	}
}

// An export streams to standard output or to --out, and round-trips through
// import into another project.
func TestExportTasksWritesAFileThatImportsBack(t *testing.T) {
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("project", "create", "ops", "Operations")
	c.mustRun("task", "add", "-p", "infra", "Fix login")
	c.mustRun("task", "add", "-p", "infra", "=SUM(A1)")

	csv := c.mustRun("export", "tasks", "-p", "infra", "--columns", "title,status", "--delimiter", ";")
	if !strings.HasPrefix(csv.out, "title;status\n") || !strings.Contains(csv.out, "Fix login;") {
		t.Errorf("the csv export does not honour --columns and --delimiter:\n%s", csv.out)
	}
	if !strings.Contains(csv.out, "'=SUM(A1)") {
		t.Errorf("a formula cell was not quoted:\n%s", csv.out)
	}

	out := filepath.Join(c.home, "infra.json")
	c.mustRun("export", "tasks", "-p", "infra", "--format", "json", "--dates", "display", "--out", out)
	raw, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(raw), "Fix login") {
		t.Fatalf("--out wrote %q, %v", raw, err)
	}
	c.mustRun("import", "tasks", "-p", "ops", out)
	if got := c.mustRun("task", "ls", "-p", "ops"); !strings.Contains(got.out, "Fix login") {
		t.Errorf("the exported file did not import into another project:\n%s", got.out)
	}

	refused := filepath.Join(c.home, "refused.csv")
	bad := c.run("export", "tasks", "-p", "ghost", "--out", refused)
	if bad.code != core.KindNotFound.ExitCode() {
		t.Errorf("an unknown project exited %d: %s", bad.code, bad.err)
	}
	if _, err := os.Stat(refused); !os.IsNotExist(err) {
		t.Errorf("a refused export left %s behind: %v", refused, err)
	}
	if got := c.run("export", "tasks", "-p", "infra", "--columns", "nonsense"); got.code != core.ExitUsage {
		t.Errorf("an unknown column exited %d: %s", got.code, got.err)
	}
	if got := c.run("export", "tasks", "-p", "infra", "--out", filepath.Join(c.home, "no", "such", "dir.csv")); got.code != core.ExitUsage ||
		!strings.Contains(got.err, "creating") {
		t.Errorf("an unwritable --out exited %d: %s", got.code, got.err)
	}
}
