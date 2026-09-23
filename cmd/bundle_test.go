// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"gopkg.in/yaml.v3"
)

// reviewWorkflow is the fixture every bundle test shares.
const reviewWorkflow = `
key: review
name: Review
definition:
  initial: draft
  states:
    - key: draft
      label: Draft
    - key: shipped
      label: Shipped
      terminal: true
  transitions:
    - from: draft
      to: shipped
`

// putReviewWorkflow defines the fixture workflow in the given database.
func putReviewWorkflow(t *testing.T, c *cli, db string) {
	t.Helper()
	got := c.runIn(reviewWorkflow, "--db", db, "workflow", "put", "-f", "-")
	if got.code != core.ExitOK {
		t.Fatalf("workflow put exited %d\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
	}
}

// exportReview streams a bundle carrying the fixture workflow.
func exportReview(t *testing.T, c *cli, db string) string {
	t.Helper()
	got := c.run("--db", db, "bundle", "export", "--kind", "workflow", "--workflow", "review", "--name", "review-kit")
	if got.code != core.ExitOK {
		t.Fatalf("bundle export exited %d\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
	}
	return got.out
}

func TestBundleExportProducesABundle(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(c.home, "source.db")
	putReviewWorkflow(t, c, db)

	got := c.run("--db", db, "bundle", "export", "--kind", "workflow", "--workflow", "review", "--name", "review-kit")
	if got.code != core.ExitOK {
		t.Fatalf("exit = %d\nstderr: %s", got.code, got.err)
	}
	if !strings.Contains(got.out, "review") {
		t.Fatalf("bundle does not name the workflow: %s", got.out)
	}
	if got.err != "" {
		t.Fatalf("stderr = %q, want a clean stream so the bundle pipes onward", got.err)
	}
}

func TestBundleExportRejectsAnUnknownKind(t *testing.T) {
	c := newCLI(t)
	if got := c.run("bundle", "export", "--kind", "task"); got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitUsage)
	}
}

// Import deliberately has no default collision policy, because replace
// overwrites a component somebody else may be using.
func TestBundleImportRequiresACollisionPolicy(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(c.home, "source.db")
	putReviewWorkflow(t, c, db)
	bundle := exportReview(t, c, db)

	got := c.runIn(bundle, "--db", filepath.Join(c.home, "target.db"), "bundle", "import")
	if got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d\nstderr: %s", got.code, core.ExitUsage, got.err)
	}
	if !strings.Contains(got.err, "collision policy") {
		t.Fatalf("stderr = %q, want it to name the missing policy", got.err)
	}
}

func TestBundleImportRejectsAnUnknownCollisionPolicy(t *testing.T) {
	c := newCLI(t)
	if got := c.run("bundle", "import", "--on-collision", "overwrite"); got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitUsage)
	}
}

func TestBundleImportPreviewWritesNothing(t *testing.T) {
	c := newCLI(t)
	source := filepath.Join(c.home, "source.db")
	target := filepath.Join(c.home, "target.db")
	putReviewWorkflow(t, c, source)
	bundle := exportReview(t, c, source)

	got := c.runIn(bundle, "--db", target, "bundle", "import", "--on-collision", "skip", "--preview")
	if got.code != core.ExitOK {
		t.Fatalf("exit = %d\nstderr: %s", got.code, got.err)
	}
	if !strings.Contains(got.out, "preview") {
		t.Fatalf("stdout = %q, want it to say nothing was written", got.out)
	}

	after := c.run("--db", target, "workflow", "get", "review")
	if after.code == core.ExitOK {
		t.Fatalf("the preview wrote the workflow: %s", after.out)
	}
}

func TestBundleRoundTripBetweenTwoDatabases(t *testing.T) {
	c := newCLI(t)
	source := filepath.Join(c.home, "source.db")
	target := filepath.Join(c.home, "target.db")
	putReviewWorkflow(t, c, source)
	bundle := exportReview(t, c, source)

	got := c.runIn(bundle, "--db", target, "bundle", "import", "--on-collision", "skip")
	if got.code != core.ExitOK {
		t.Fatalf("import exited %d\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
	}

	after := c.run("--db", target, "workflow", "get", "review", "-o", "json")
	if after.code != core.ExitOK {
		t.Fatalf("the workflow did not arrive: exit %d, %s", after.code, after.err)
	}
	var wf core.Workflow
	if err := json.Unmarshal([]byte(after.out), &wf); err != nil {
		t.Fatalf("not json: %v\n%s", err, after.out)
	}
	if wf.Key != "review" || wf.Definition.Initial != "draft" {
		t.Fatalf("imported workflow = %+v", wf)
	}
}

func TestBundleImportSecondTimeReportsTheCollision(t *testing.T) {
	c := newCLI(t)
	source := filepath.Join(c.home, "source.db")
	target := filepath.Join(c.home, "target.db")
	putReviewWorkflow(t, c, source)
	bundle := exportReview(t, c, source)

	if got := c.runIn(bundle, "--db", target, "bundle", "import", "--on-collision", "skip"); got.code != core.ExitOK {
		t.Fatalf("first import exited %d: %s", got.code, got.err)
	}
	got := c.runIn(bundle, "--db", target, "bundle", "import", "--on-collision", "rename", "-o", "json")
	if got.code != core.ExitOK {
		t.Fatalf("second import exited %d: %s", got.code, got.err)
	}
	var result core.BundleResult
	if err := json.Unmarshal([]byte(got.out), &result); err != nil {
		t.Fatalf("not json: %v\n%s", err, got.out)
	}
	if len(result.Outcomes) == 0 {
		t.Fatalf("no outcome was reported: %s", got.out)
	}
	for _, o := range result.Outcomes {
		if o.Action == core.ActionRenamed && o.NewKey == "" {
			t.Fatalf("a rename reported no new key: %+v", o)
		}
	}
}

func TestBundleImportReadsAFileArgumentAndStandardInput(t *testing.T) {
	c := newCLI(t)
	source := filepath.Join(c.home, "source.db")
	putReviewWorkflow(t, c, source)
	bundle := exportReview(t, c, source)

	path := filepath.Join(c.home, "review.bundle")
	if err := os.WriteFile(path, []byte(bundle), 0o600); err != nil {
		t.Fatalf("write bundle: %v", err)
	}

	t.Run("file argument", func(t *testing.T) {
		db := filepath.Join(c.home, "from-file.db")
		if got := c.run("--db", db, "bundle", "import", "--on-collision", "skip", path); got.code != core.ExitOK {
			t.Fatalf("exit = %d: %s", got.code, got.err)
		}
		if got := c.run("--db", db, "workflow", "get", "review"); got.code != core.ExitOK {
			t.Fatalf("workflow missing after a file import: %s", got.err)
		}
	})

	t.Run("standard input", func(t *testing.T) {
		db := filepath.Join(c.home, "from-stdin.db")
		if got := c.runIn(bundle, "--db", db, "bundle", "import", "--on-collision", "skip"); got.code != core.ExitOK {
			t.Fatalf("exit = %d: %s", got.code, got.err)
		}
		if got := c.run("--db", db, "workflow", "get", "review"); got.code != core.ExitOK {
			t.Fatalf("workflow missing after a stdin import: %s", got.err)
		}
	})

	t.Run("explicit dash", func(t *testing.T) {
		db := filepath.Join(c.home, "from-dash.db")
		if got := c.runIn(bundle, "--db", db, "bundle", "import", "--on-collision", "skip", "-"); got.code != core.ExitOK {
			t.Fatalf("exit = %d: %s", got.code, got.err)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		got := c.run("bundle", "import", "--on-collision", "skip", filepath.Join(c.home, "absent.bundle"))
		if got.code != core.ExitUsage {
			t.Fatalf("exit = %d, want %d", got.code, core.ExitUsage)
		}
	})
}

func TestBundleImportInEveryOutputFormat(t *testing.T) {
	c := newCLI(t)
	source := filepath.Join(c.home, "source.db")
	putReviewWorkflow(t, c, source)
	bundle := exportReview(t, c, source)

	t.Run("table", func(t *testing.T) {
		got := c.runIn(bundle, "--db", filepath.Join(c.home, "table.db"),
			"bundle", "import", "--on-collision", "skip", "--preview")
		if got.code != core.ExitOK {
			t.Fatalf("exit = %d: %s", got.code, got.err)
		}
		for _, want := range []string{"KIND", "KEY", "ACTION", "preview"} {
			if !strings.Contains(got.out, want) {
				t.Fatalf("table output is missing %q: %s", want, got.out)
			}
		}
	})

	t.Run("json", func(t *testing.T) {
		got := c.runIn(bundle, "--db", filepath.Join(c.home, "json.db"),
			"bundle", "import", "--on-collision", "skip", "--preview", "-o", "json")
		if got.code != core.ExitOK {
			t.Fatalf("exit = %d: %s", got.code, got.err)
		}
		var result core.BundleResult
		if err := json.Unmarshal([]byte(got.out), &result); err != nil {
			t.Fatalf("not json: %v\n%s", err, got.out)
		}
		if !result.Preview {
			t.Fatalf("json result does not report the preview: %s", got.out)
		}
	})

	t.Run("yaml", func(t *testing.T) {
		got := c.runIn(bundle, "--db", filepath.Join(c.home, "yaml.db"),
			"bundle", "import", "--on-collision", "skip", "--preview", "-o", "yaml")
		if got.code != core.ExitOK {
			t.Fatalf("exit = %d: %s", got.code, got.err)
		}
		var result core.BundleResult
		if err := yaml.Unmarshal([]byte(got.out), &result); err != nil {
			t.Fatalf("not yaml: %v\n%s", err, got.out)
		}
		if !result.Preview {
			t.Fatalf("yaml result does not report the preview: %s", got.out)
		}
	})

	t.Run("ndjson", func(t *testing.T) {
		got := c.runIn(bundle, "--db", filepath.Join(c.home, "ndjson.db"),
			"bundle", "import", "--on-collision", "skip", "--preview", "-o", "ndjson")
		if got.code != core.ExitOK {
			t.Fatalf("exit = %d: %s", got.code, got.err)
		}
		for _, line := range strings.Split(strings.TrimSpace(got.out), "\n") {
			if !json.Valid([]byte(line)) {
				t.Fatalf("ndjson line is not json: %q", line)
			}
		}
	})
}

func TestBundleExportInEveryOutputFormatStaysAStream(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(c.home, "source.db")
	putReviewWorkflow(t, c, db)

	for _, format := range []string{"table", "json", "yaml", "ndjson"} {
		t.Run(format, func(t *testing.T) {
			got := c.run("--db", db, "-o", format, "bundle", "export", "--kind", "workflow")
			if got.code != core.ExitOK {
				t.Fatalf("exit = %d: %s", got.code, got.err)
			}
			if got.out == "" {
				t.Fatal("export wrote nothing")
			}
			if got.err != "" {
				t.Fatalf("stderr = %q, want diagnostics kept off the bundle", got.err)
			}
		})
	}
}

// A webhook travels without its secret, so the import warns that the endpoint
// is inactive. The warning is a diagnostic and must not land on standard output.
func TestBundleImportWarningsGoToStandardError(t *testing.T) {
	c := newCLI(t)
	source := filepath.Join(c.home, "source.db")
	target := filepath.Join(c.home, "target.db")
	c.mustRun("--db", source, "webhook", "put", "https://hooks.example.com/tix",
		"--secret", "shhhhh", "--event", "task.created")

	bundle := c.mustRun("--db", source, "bundle", "export", "--kind", "webhook")
	if strings.Contains(bundle.out, "shhhhh") {
		t.Fatalf("the bundle carries the signing secret: %s", bundle.out)
	}

	got := c.runIn(bundle.out, "--db", target, "bundle", "import", "--on-collision", "skip")
	if got.code != core.ExitOK {
		t.Fatalf("exit = %d: %s", got.code, got.err)
	}
	if !strings.Contains(got.err, "warning:") {
		t.Fatalf("stderr = %q, want the inactive endpoint warning", got.err)
	}
	if strings.Contains(got.out, "warning:") {
		t.Fatalf("stdout = %q, want diagnostics kept off the data stream", got.out)
	}
}

func TestBundleGroupShowsHelpWithoutASubcommand(t *testing.T) {
	c := newCLI(t)
	got := c.mustRun("bundle")
	for _, want := range []string{"export", "import"} {
		if !strings.Contains(got.out, want) {
			t.Fatalf("bundle help is missing %q: %s", want, got.out)
		}
	}
}
