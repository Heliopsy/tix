package cmd

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

// annotatedCode matches one "N meaning" pair of the exitCodes annotation.
var annotatedCode = regexp.MustCompile(`(?:^|, )(\d+) ([a-z][a-z ]*)`)

// docsCodeRow matches one row of an exit code table in docs/.
var docsCodeRow = regexp.MustCompile(`(?m)^\| (\d+) \| ([^|]+?) \|`)

// mentionedCode matches an exit code named in a command's own description.
var mentionedCode = regexp.MustCompile(`(?:^|, )(\d+) `)

// coreExitCodes returns every exit code the CLI can produce, derived from
// core.Kind.ExitCode(), which AGENTS.md makes the one owner of the mapping. The
// empty Kind is included because that is what a successful command carries.
func coreExitCodes(t *testing.T) []int {
	t.Helper()

	var codes []int
	for _, kind := range append(slices.Clone(core.Kinds), "") {
		if code := kind.ExitCode(); !slices.Contains(codes, code) {
			codes = append(codes, code)
		}
	}
	slices.Sort(codes)
	if len(codes) < 2 {
		t.Fatalf("core maps every kind onto %v; the derivation is broken", codes)
	}
	return codes
}

// TestAnnotatedExitCodesMatchCore ties the exitCodes annotation on the root
// command, which `tix --help` prints and `tix docs` writes into the generated
// reference, to the mapping in core. It was a hand-kept copy of that mapping.
func TestAnnotatedExitCodesMatchCore(t *testing.T) {
	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())
	annotation := root.Annotations["exitCodes"]
	if annotation == "" {
		t.Fatal("the root command carries no exitCodes annotation; --help prints nothing")
	}

	var annotated []int
	for _, pair := range annotatedCode.FindAllStringSubmatch(annotation, -1) {
		code, err := strconv.Atoi(pair[1])
		if err != nil {
			t.Fatalf("reading %q out of the annotation: %v", pair[1], err)
		}
		if strings.TrimSpace(pair[2]) == "" {
			t.Errorf("exit code %d is annotated with no meaning", code)
		}
		annotated = append(annotated, code)
	}
	if len(annotated) == 0 {
		t.Fatalf("parsed no codes out of %q; the annotation changed shape", annotation)
	}
	if !slices.IsSorted(annotated) {
		t.Errorf("the annotation lists %v out of order; it is read as a table", annotated)
	}
	if want := coreExitCodes(t); !slices.Equal(annotated, want) {
		t.Errorf("the exitCodes annotation lists %v, core produces %v", annotated, want)
	}
}

// exitCodeDocs are the documents that reproduce the exit code table, and the
// heading each one keeps it under. docs/agents.md carries a second, different
// table for `tix claim exec`, which is why the section matters.
var exitCodeDocs = []string{"scripting.md", "agents.md"}

// TestDocumentedExitCodesMatchCore ties the exit code tables in docs/ to the
// mapping in core, in both directions. docs/agents.md claims outright that
// "every command uses the same table, which tix --help also prints", and
// nothing made that true.
func TestDocumentedExitCodesMatchCore(t *testing.T) {
	want := coreExitCodes(t)

	for _, name := range exitCodeDocs {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("..", "docs", name)
			body, err := os.ReadFile(path) // #nosec G304 -- a fixed path inside the repository
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}

			var documented []int
			for _, row := range docsCodeRow.FindAllStringSubmatch(exitCodeSection(t, path, string(body)), -1) {
				code, err := strconv.Atoi(row[1])
				if err != nil {
					t.Fatalf("reading %q out of %s: %v", row[1], path, err)
				}
				if strings.TrimSpace(row[2]) == "" {
					t.Errorf("%s documents exit code %d with no meaning", path, code)
				}
				if slices.Contains(documented, code) {
					t.Errorf("%s documents exit code %d twice", path, code)
				}
				documented = append(documented, code)
			}
			if len(documented) == 0 {
				t.Fatalf("%s has no exit code rows; the parser or the table changed shape", path)
			}
			if !slices.IsSorted(documented) {
				t.Errorf("%s lists %v out of order", path, documented)
			}
			for _, code := range documented {
				if !slices.Contains(want, code) {
					t.Errorf("%s documents exit %d, which core.Kind.ExitCode never returns", path, code)
				}
			}
			for _, code := range want {
				if !slices.Contains(documented, code) {
					t.Errorf("core returns exit %d, which %s does not document", code, path)
				}
			}
		})
	}
}

// interruptExit is the one exit code no Kind produces. `tix tui` returns it on
// SIGINT by the 128+signal convention, which is a process-level outcome rather
// than a domain error, and docs/scripting.md says so beside the table.
const interruptExit = 130

// TestCommandsNameOnlyRealExitCodes holds the per-command "Exit codes:" lines
// in every description to the same mapping. Most of them are written by hand,
// so a code that core cannot produce is a promise the binary does not keep.
func TestCommandsNameOnlyRealExitCodes(t *testing.T) {
	want := coreExitCodes(t)
	root, _ := newRoot([]string{"HOME=" + t.TempDir()}, t.TempDir())

	var checked int
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, line := range strings.Split(cmd.Long, "\n") {
			rest, ok := strings.CutPrefix(strings.TrimSpace(line), "Exit codes: ")
			if !ok {
				continue
			}
			checked++
			for _, match := range mentionedCode.FindAllStringSubmatch(rest, -1) {
				code, err := strconv.Atoi(match[1])
				if err != nil {
					t.Fatalf("reading %q out of %q: %v", match[1], cmd.CommandPath(), err)
				}
				if !slices.Contains(want, code) && code != interruptExit {
					t.Errorf("%q documents exit %d, which core.Kind.ExitCode never returns",
						cmd.CommandPath(), code)
				}
			}
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)

	if checked == 0 {
		t.Fatal("no command description names an exit code; this guard is watching nothing")
	}
}

// exitCodeSection returns the body under the "## Exit codes" heading alone.
func exitCodeSection(t *testing.T, path, body string) string {
	t.Helper()
	const heading = "\n## Exit codes\n"
	at := strings.Index(body, heading)
	if at < 0 {
		t.Fatalf("%s has no %q heading", path, "## Exit codes")
	}
	rest := body[at+len(heading):]
	if end := strings.Index(rest, "\n## "); end >= 0 {
		return rest[:end]
	}
	return rest
}
