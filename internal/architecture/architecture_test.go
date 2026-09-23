// SPDX-License-Identifier: AGPL-3.0-or-later

package architecture_test

import (
	"go/build"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// modulePath is the import path prefix every package in this repository shares.
const modulePath = "github.com/heliopsy/tix"

// pkg is one package of this module and the packages it imports.
type pkg struct {
	// path is the import path relative to the module, "cmd" or "internal/core".
	path string
	// imports are the packages its non-test files import.
	imports []string
	// testImports are the packages its test files import, internal and external.
	testImports []string
}

// all returns every package of this module, keyed by module-relative path. It
// walks the source tree rather than taking a list, because a guard that names
// the packages it checks stops covering the one somebody adds next.
func all(t *testing.T) map[string]pkg {
	t.Helper()

	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolving repository root: %v", err)
	}

	found := map[string]pkg{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if path != root && (strings.HasPrefix(name, ".") || name == "testdata" || name == "node_modules") {
			return filepath.SkipDir
		}
		// Build tags hide whole packages from the default context, which is an
		// empty directory as far as go/build is concerned rather than an error
		// worth failing on.
		built, err := build.Default.ImportDir(path, 0)
		if err != nil {
			return nil //nolint:nilerr // a directory with no buildable Go files is not a package
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		found[rel] = pkg{
			path:        rel,
			imports:     built.Imports,
			testImports: append(slices.Clone(built.TestImports), built.XTestImports...),
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if len(found) < 20 {
		t.Fatalf("found only %d packages under %s; the walk is broken", len(found), root)
	}
	return found
}

// TestNoInternalPackageImportsCobra enforces "cmd/ is Cobra flags and wiring
// only. No package under internal/ may import Cobra." A command tree that
// reaches into internal, or an internal package that grows a *cobra.Command
// argument, makes the CLI the only way to call the behaviour it carries.
func TestNoInternalPackageImportsCobra(t *testing.T) {
	const cobra = "github.com/spf13/cobra"

	for path, p := range all(t) {
		if !strings.HasPrefix(path, "internal/") {
			continue
		}
		for _, imp := range p.imports {
			if imp == cobra || strings.HasPrefix(imp, cobra+"/") {
				t.Errorf("%s imports %s; Cobra belongs to cmd/ alone", path, imp)
			}
		}
		for _, imp := range p.testImports {
			if imp == cobra || strings.HasPrefix(imp, cobra+"/") {
				t.Errorf("%s has a test importing %s; Cobra belongs to cmd/ alone", path, imp)
			}
		}
	}
}

// clientMayImport is every package of this module internal/client is allowed to
// reach, transitively. AGENTS.md says the client "marshals and nothing else":
// that it performs no validation cannot be checked mechanically, but that it
// cannot reach the code holding the rules can, and a client that only sees the
// contract and the wire format has nowhere to put a rule.
var clientMayImport = []string{
	modulePath + "/internal/core",
	modulePath + "/internal/wire",
}

// TestClientReachesOnlyTheContractAndTheWireFormat walks internal/client's
// transitive dependencies inside this module and holds them to clientMayImport.
// Transitive, because a rule one hop away is still a rule the client can call.
func TestClientReachesOnlyTheContractAndTheWireFormat(t *testing.T) {
	packages := all(t)
	const client = "internal/client"
	if _, ok := packages[client]; !ok {
		t.Fatalf("%s is not in the walk; the guard is watching nothing", client)
	}

	seen := map[string]bool{}
	var walk func(path string)
	walk = func(path string) {
		if seen[path] {
			return
		}
		seen[path] = true
		for _, imp := range packages[path].imports {
			rel, ok := strings.CutPrefix(imp, modulePath+"/")
			if !ok {
				continue
			}
			if !slices.Contains(clientMayImport, imp) {
				t.Errorf("%s imports %s; internal/client may reach only %s",
					path, imp, strings.Join(clientMayImport, " and "))
				continue
			}
			walk(rel)
		}
	}
	walk(client)
}

// TestAuthzIsCalledOnlyByService enforces "the authorization policy lives in
// internal/authz, and internal/service is its only caller". A second caller is
// a second place a permission decision is made, which is how a surface ends up
// enforcing a policy that is one edit behind the real one.
func TestAuthzIsCalledOnlyByService(t *testing.T) {
	const authz = modulePath + "/internal/authz"
	allowed := []string{"internal/authz", "internal/service"}

	for path, p := range all(t) {
		if slices.Contains(allowed, path) {
			continue
		}
		for _, imp := range p.imports {
			if imp == authz || strings.HasPrefix(imp, authz+"/") {
				t.Errorf("%s imports %s; only internal/service may ask the policy", path, imp)
			}
		}
	}
}

// TestCommandsReachNoStore enforces "cmd/ is Cobra flags and wiring only. No
// business logic, no database access." A command that opens a store itself has
// gone around the service, and with it around the audit entry and the outbox
// event that every mutation owes.
func TestCommandsReachNoStore(t *testing.T) {
	const store = modulePath + "/internal/store"

	for _, imp := range all(t)["cmd"].imports {
		if imp == store || strings.HasPrefix(imp, store+"/") {
			t.Errorf("cmd imports %s; a command wires a service, it does not open a store", imp)
		}
	}
}

// dialectDrivers are the database drivers, one per engine.
var dialectDrivers = []string{
	"modernc.org/sqlite",
	"github.com/jackc/pgx/v5",
}

// TestDialectsStayInTheirOwnPackages enforces "dialect differences live only in
// internal/store/sqlite and internal/store/postgres". Whether a difference is a
// dialect difference is a judgement, but importing an engine's driver is not:
// anywhere else it is a second place that knows which database is underneath.
func TestDialectsStayInTheirOwnPackages(t *testing.T) {
	allowed := []string{"internal/store/sqlite", "internal/store/postgres"}

	var reached int
	for path, p := range all(t) {
		for _, imp := range p.imports {
			for _, driver := range dialectDrivers {
				if imp != driver && !strings.HasPrefix(imp, driver+"/") {
					continue
				}
				reached++
				if !slices.Contains(allowed, path) {
					t.Errorf("%s imports %s; the engines are known only to %s",
						path, imp, strings.Join(allowed, " and "))
				}
			}
		}
	}
	if reached == 0 {
		t.Fatalf("no package imports any of %v; the driver names are stale and this guard watches nothing",
			dialectDrivers)
	}
}
