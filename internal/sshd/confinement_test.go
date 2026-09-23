// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// theOneUnscopedLookup is the single method allowed to reach enrolled keys
// without a tenant, and the files that may name it: the interface that declares
// it, the two engines that implement it, and the one caller.
const theOneUnscopedLookup = "FindSSHKeysByFingerprint"

var mayNameTheUnscopedLookup = []string{
	"internal/sshd/enrolled.go",
	"internal/store/postgres/sshkey.go",
	"internal/store/sqlite/sshkey.go",
	"internal/store/store.go",
}

// TestTheUnscopedLookupIsTheOnlyDoorToEnrolledKeys is the half of the
// confinement requirement that no single test can reach by running code:
// TestEveryResolutionNarrowsToOneTenant proves no session runs without a
// tenant scope, and this proves there is nowhere else to go.
//
// It asserts in two shapes because one shape catches only one of the two ways
// the claim can stop being true. Reflecting over store.UnscopedTx catches a
// second unscoped door being cut, which a search for the existing name would
// never see: a new ListSSHKeysAcrossTenants would compile, pass every test, and
// be spelled nowhere this file looks. Walking the tree catches a second caller
// of the door that already exists, which reflection cannot see at all, because
// the interface is unchanged when somebody reaches through it from a new place.
func TestTheUnscopedLookupIsTheOnlyDoorToEnrolledKeys(t *testing.T) {
	t.Run("the interface offers one", func(t *testing.T) {
		unscoped := reflect.TypeOf((*store.UnscopedTx)(nil)).Elem()
		sshKey := reflect.TypeOf(core.SSHKey{})

		var doors []string
		for i := 0; i < unscoped.NumMethod(); i++ {
			m := unscoped.Method(i)
			if signatureMentions(m.Type, sshKey) {
				doors = append(doors, m.Name)
			}
		}
		sort.Strings(doors)
		if len(doors) != 1 || doors[0] != theOneUnscopedLookup {
			t.Fatalf("store.UnscopedTx reaches enrolled keys through %v; only %s may",
				doors, theOneUnscopedLookup)
		}
	})

	t.Run("one caller reaches through it", func(t *testing.T) {
		named := filesNaming(t, theOneUnscopedLookup)
		if !reflect.DeepEqual(named, mayNameTheUnscopedLookup) {
			t.Fatalf("%s is named in %v, want exactly %v: the cross-tenant lookup belongs to the "+
				"authentication path and nowhere else",
				theOneUnscopedLookup, named, mayNameTheUnscopedLookup)
		}
	})
}

// signatureMentions reports whether a method's arguments or results carry the
// type, directly or inside a slice, pointer or map.
func signatureMentions(fn reflect.Type, want reflect.Type) bool {
	for i := 0; i < fn.NumIn(); i++ {
		if carries(fn.In(i), want) {
			return true
		}
	}
	for i := 0; i < fn.NumOut(); i++ {
		if carries(fn.Out(i), want) {
			return true
		}
	}
	return false
}

func carries(have, want reflect.Type) bool {
	if have == want {
		return true
	}
	switch have.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Chan:
		return carries(have.Elem(), want)
	case reflect.Map:
		return carries(have.Key(), want) || carries(have.Elem(), want)
	default:
		return false
	}
}

// filesNaming returns every non-test Go file in the repository that mentions
// the identifier, relative to the repository root and sorted.
func filesNaming(t *testing.T, identifier string) []string {
	t.Helper()
	root := repositoryRoot(t)
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", "bin":
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
		if !strings.Contains(string(body), identifier) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		found = append(found, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	sort.Strings(found)
	return found
}

// repositoryRoot locates the module root from this package.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("%q is not the repository root: %v", dir, err)
	}
	return dir
}
