// SPDX-License-Identifier: AGPL-3.0-or-later

package presence

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

// theUnscopedServerMethods are the only methods allowed to reach the server
// registry without a tenant, and mayNameThem is every file that may spell one.
//
// A server serves every tenant, so its row belongs to none and there is no
// scope to build with. That makes this the second unscoped thing in the system
// after the SSH fingerprint lookup, and it is confined the same way: the door
// is narrow, it is named here, and a sixth file walking through it fails the
// build rather than reading as ordinary code.
var theUnscopedServerMethods = []string{
	"DeregisterServer",
	"ForgetServersBefore",
	"HeartbeatServer",
	"ListServers",
	"RegisterServer",
}

var mayNameThem = []string{
	"internal/presence/registrar.go",
	"internal/service/status.go",
	"internal/store/postgres/server.go",
	"internal/store/sqlite/server.go",
	"internal/store/store.go",
}

// TestTheServerRegistryHasOneDoor asserts in two shapes, because either shape
// alone misses one of the two ways the claim stops being true.
//
// Reflecting over store.UnscopedTx catches a sixth door being cut: a
// ListServersForEveryTenant would compile, pass every other test, and be
// spelled nowhere a search for the existing names looks. Walking the tree
// catches a sixth caller reaching through a door that already exists, which
// reflection cannot see at all, because the interface is unchanged when
// somebody reaches through it from a new place.
func TestTheServerRegistryHasOneDoor(t *testing.T) {
	t.Run("the interface offers these and no others", func(t *testing.T) {
		unscoped := reflect.TypeOf((*store.UnscopedTx)(nil)).Elem()

		var named []string
		for i := range unscoped.NumMethod() {
			if strings.Contains(unscoped.Method(i).Name, "Server") {
				named = append(named, unscoped.Method(i).Name)
			}
		}
		sort.Strings(named)
		if !reflect.DeepEqual(named, theUnscopedServerMethods) {
			t.Fatalf("store.UnscopedTx reaches the server registry through %v, want exactly %v",
				named, theUnscopedServerMethods)
		}
	})

	t.Run("nothing else carries a server row across the boundary", func(t *testing.T) {
		unscoped := reflect.TypeOf((*store.UnscopedTx)(nil)).Elem()
		server := reflect.TypeOf(core.Server{})

		var carriers []string
		for i := range unscoped.NumMethod() {
			m := unscoped.Method(i)
			if signatureMentions(m.Type, server) {
				carriers = append(carriers, m.Name)
			}
		}
		sort.Strings(carriers)
		want := []string{"ListServers", "RegisterServer"}
		if !reflect.DeepEqual(carriers, want) {
			t.Fatalf("store.UnscopedTx moves core.Server through %v, want exactly %v", carriers, want)
		}
	})

	t.Run("five files reach through them", func(t *testing.T) {
		named := filesNamingAny(t, theUnscopedServerMethods)
		if !reflect.DeepEqual(named, mayNameThem) {
			t.Fatalf("the unscoped server methods are named in %v, want exactly %v: the registry "+
				"belongs to the registrar and the status reader and nowhere else", named, mayNameThem)
		}
	})
}

// TestARowNamesNoTenant is the other half of the isolation claim. The
// tenant-isolation suite is written method by method against tenant-scoped
// reads, so a new table is invisible to it; this asserts the property that
// makes the table showable at all, from the struct rather than from the
// migration, so a field added in Go is caught as surely as a column.
func TestARowNamesNoTenant(t *testing.T) {
	forbidden := []string{"tenant", "hostname", "domain", "actor", "user", "key"}
	for _, typ := range []reflect.Type{
		reflect.TypeOf(core.Server{}),
		reflect.TypeOf(core.ServerStatus{}),
	} {
		for i := range typ.NumField() {
			name := strings.ToLower(typ.Field(i).Name)
			for _, word := range forbidden {
				if strings.Contains(name, word) {
					t.Errorf("%s.%s names %q; a server row must carry nothing that identifies a tenant",
						typ.Name(), typ.Field(i).Name, word)
				}
			}
		}
	}
}

// signatureMentions reports whether a method's arguments or results carry the
// type, directly or inside a slice, pointer or map.
func signatureMentions(fn reflect.Type, want reflect.Type) bool {
	for i := range fn.NumIn() {
		if carries(fn.In(i), want) {
			return true
		}
	}
	for i := range fn.NumOut() {
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

// filesNamingAny returns every non-test Go file in the repository naming at
// least one of the identifiers, relative to the repository root and sorted.
func filesNamingAny(t *testing.T, identifiers []string) []string {
	t.Helper()
	root := repositoryRoot(t)
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Any dot-directory, not just .git: the harness puts agent
			// worktrees under .claude/worktrees/, and a nested checkout holds a
			// second copy of every file this counts.
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
		if !mentionsAny(string(body), identifiers) {
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

func mentionsAny(body string, identifiers []string) bool {
	for _, id := range identifiers {
		if strings.Contains(body, id) {
			return true
		}
	}
	return false
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
