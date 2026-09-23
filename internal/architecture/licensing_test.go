// SPDX-License-Identifier: AGPL-3.0-or-later

package architecture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spdxLine is the notice every file tix owns carries. The root LICENSE governs
// the work as distributed, but it does not travel: a file copied out of this
// repository has to say what it is on its own, and AGPL's network clause only
// binds whoever knows it applies.
const spdxLine = "SPDX-License-Identifier: AGPL-3.0-or-later"

// vendored is every path in this tree that belongs to somebody else. Its terms
// are recorded in REUSE.toml, not in a header we wrote. Stamping our licence on
// third-party code is worse than leaving it bare, so these are excluded from
// the header check and asserted to stay bare by TestVendoredFilesAreNotStamped.
var vendored = []string{
	"internal/web/assets/htmx.min.js",
}

// ownedSuffixes are the extensions whose files this project authors and ships
// as source. Templates are deliberately absent: they are fragments that mean
// nothing outside internal/web, which is stamped, and the only comment form
// html/template drops from its output would sit outside their {{define}}
// blocks.
var ownedSuffixes = []string{".go", ".js", ".mjs", ".css", ".sql"}

// repoRoot is this repository's root, resolved from the package directory so
// the walk does not depend on where the test binary was started.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolving repository root: %v", err)
	}
	return root
}

// ownedSourceFiles walks the tree and returns every module-relative path this
// project owns and ships as source. It walks rather than takes a list, because
// a guard that names the files it checks stops covering the one added next.
func ownedSourceFiles(t *testing.T) []string {
	t.Helper()

	root := repoRoot(t)
	skipped := map[string]bool{}
	for _, v := range vendored {
		skipped[v] = true
	}

	var found []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			// Dot directories, build output and fixtures hold nothing we ship
			// as source.
			if path != root && (strings.HasPrefix(name, ".") || name == "testdata" ||
				name == "node_modules" || name == "dist" || name == "bin") {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if skipped[rel] {
			return nil
		}
		for _, suffix := range ownedSuffixes {
			if strings.HasSuffix(name, suffix) {
				found = append(found, rel)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if len(found) < 400 {
		t.Fatalf("found only %d source files under %s; the walk is broken", len(found), root)
	}
	return found
}

// head returns the opening bytes of a file, which is as far as a licence notice
// is allowed to be.
func head(t *testing.T, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("reading %s: %v", rel, err)
	}
	const window = 512
	if len(data) > window {
		data = data[:window]
	}
	return string(data)
}

// TestEverySourceFileCarriesItsLicence fails when a file this project owns
// ships without saying so.
func TestEverySourceFileCarriesItsLicence(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range ownedSourceFiles(t) {
		if !strings.Contains(head(t, root, rel), spdxLine) {
			t.Errorf("%s has no %q near the top of the file; add it as the first line, followed by a blank line", rel, spdxLine)
		}
	}
}

// TestLicenceLineIsFirstAndNotADocComment pins the placement. A comment sitting
// immediately before the package clause becomes the package doc comment, so the
// notice has to be separated from whatever follows by a blank line, and it has
// to come first so //go:build constraints and doc comments keep their meaning.
func TestLicenceLineIsFirstAndNotADocComment(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range ownedSourceFiles(t) {
		if !strings.HasSuffix(rel, ".go") {
			continue
		}
		lines := strings.SplitN(head(t, root, rel), "\n", 3)
		if len(lines) < 3 {
			t.Errorf("%s is too short to carry a licence header", rel)
			continue
		}
		if lines[0] != "// "+spdxLine {
			t.Errorf("%s line 1 is %q, want %q", rel, lines[0], "// "+spdxLine)
		}
		if strings.TrimSpace(lines[1]) != "" {
			t.Errorf("%s line 2 is %q, want a blank line; without it the notice becomes a doc comment", rel, lines[1])
		}
	}
}

// TestVendoredFilesAreNotStamped is the other half of the exclusion. A path
// listed as vendored that has gone missing means the list is stale, and one
// carrying our notice means somebody stamped this project's licence onto
// somebody else's work.
func TestVendoredFilesAreNotStamped(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range vendored {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("vendored path %s does not exist; the exclusion list is stale: %v", rel, err)
			continue
		}
		if strings.Contains(head(t, root, rel), spdxLine) {
			t.Errorf("%s is vendored third-party code carrying this project's licence; remove the notice", rel)
		}
		if !strings.Contains(reuseManifest(t, root), rel) {
			t.Errorf("%s is vendored but REUSE.toml does not record its terms", rel)
		}
	}
}

// reuseManifest returns REUSE.toml, where the terms of everything a per-file
// header cannot cover are recorded.
func reuseManifest(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "REUSE.toml"))
	if err != nil {
		t.Fatalf("reading REUSE.toml: %v", err)
	}
	return string(data)
}
