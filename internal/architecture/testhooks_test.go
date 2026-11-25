// SPDX-License-Identifier: AGPL-3.0-or-later

package architecture_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testOnlyHooks are calls that weaken something on purpose to make a test
// binary fast, and that a shipped binary must never make.
var testOnlyHooks = []struct {
	call    string
	defines string
}{
	{call: "UseCheapHashingInTests(", defines: "internal/service/local.go"},
	{call: "HoldHashSlotsInTests(", defines: "internal/auth/gate.go"},
}

// TestTestHooksStayInTests keeps cheap password hashing out of the product.
// The hook is an exported function, so the compiler would let any package
// call it; this is what says only a _test.go file may.
func TestTestHooksStayInTests(t *testing.T) {
	root := repoRoot(t)
	for _, hook := range testOnlyHooks {
		used := 0
		for _, rel := range ownedSourceFiles(t) {
			if !strings.HasSuffix(rel, ".go") || strings.HasPrefix(rel, "internal/architecture/") {
				continue
			}
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatalf("reading %s: %v", rel, err)
			}
			n := strings.Count(string(body), hook.call)
			if n == 0 {
				continue
			}
			if strings.HasSuffix(rel, "_test.go") {
				used += n
				continue
			}
			if rel == hook.defines && n == 1 {
				continue
			}
			t.Errorf("%s calls %s outside a test file", rel, strings.TrimSuffix(hook.call, "("))
		}
		if used == 0 {
			t.Errorf("no test calls %s; the name is stale and this guard watches nothing",
				strings.TrimSuffix(hook.call, "("))
		}
	}
}
