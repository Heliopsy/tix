// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// mayNameTheMembershipListing are the files allowed to name the one unscoped
// read of actors and memberships: the interface, the two engines, and this
// package's one helper. Every other caller goes through that helper, which
// keeps the cross-tenant read in the place its rules live.
var mayNameTheMembershipListing = []string{
	"internal/service/memberships.go",
	"internal/store/postgres/memberships.go",
	"internal/store/sqlite/memberships.go",
	"internal/store/store.go",
}

func TestTheMembershipListingHasOneCaller(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("%q is not the repository root: %v", root, err)
	}
	var named []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			switch d.Name() {
			case "node_modules", "dist", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path) // #nosec G304 -- the walk names every file it reads
		if err != nil {
			return err
		}
		if strings.Contains(string(body), "ListMemberships") {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			named = append(named, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(named)
	if !reflect.DeepEqual(named, mayNameTheMembershipListing) {
		t.Fatalf("ListMemberships is named in %v, want exactly %v: the cross-tenant read of an account's "+
			"memberships goes through membershipsOf", named, mayNameTheMembershipListing)
	}
}
