// SPDX-License-Identifier: AGPL-3.0-or-later

package architecture_test

import (
	"go/build"
	"path/filepath"
	"strings"
	"testing"
)

// TestTheBinaryReachesNoCgo enforces "builds stay CGO_ENABLED=0" for the tix
// binary now that the repository holds one cgo package, the C ABI in
// embedded/libtix. The walk runs with cgo enabled, which is the context a cgo
// import would hide in under CGO_ENABLED=0: the file would simply drop out of
// that build and nothing would fail until somebody built with cgo on.
func TestTheBinaryReachesNoCgo(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	ctx := build.Default
	ctx.CgoEnabled = true

	seen := map[string]bool{}
	cgo := map[string]bool{}
	var visit func(rel string)
	visit = func(rel string) {
		if seen[rel] {
			return
		}
		seen[rel] = true
		p, err := ctx.ImportDir(filepath.Join(root, filepath.FromSlash(rel)), 0)
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		if len(p.CgoFiles) > 0 {
			cgo[rel] = true
		}
		for _, imp := range p.Imports {
			if imp == "C" {
				cgo[rel] = true
			}
			if next, ok := strings.CutPrefix(imp, modulePath+"/"); ok {
				visit(next)
			}
		}
	}
	visit(".")
	if len(seen) < 20 {
		t.Fatalf("the walk from the main package reached only %d packages; it is broken", len(seen))
	}
	for rel := range cgo {
		t.Errorf("the tix binary reaches %s, which uses cgo", rel)
	}

	lib, err := ctx.ImportDir(filepath.Join(root, "embedded", "libtix"), 0)
	if err != nil || len(lib.CgoFiles) == 0 {
		t.Fatalf("embedded/libtix has no cgo files (%v); this guard no longer knows where the cgo lives", err)
	}
}
