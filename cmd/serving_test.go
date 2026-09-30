// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTheSignalHandlerIsInstalledBeforeAnythingBinds is the structural half of
// the guard serveUntilSignal is the other half of. The helper makes the order
// right in the one place it is written; this makes the wrong order anywhere
// else in the package fail loudly instead of intermittently.
//
// The defect it protects against: `tix serve` and `tix ssh` each bound their
// listener and only then called signal.NotifyContext. In that window the
// process is accepting connections while Go's default action for SIGINT is
// still to terminate, so an operator interrupting a busy server killed it
// instead of draining it.
//
// A test that signals a running server cannot guard this. On the fixed code it
// passes, but on the broken code it only sometimes fails, which is a guard
// nobody can watch fail. Statement order is decidable, so it is asserted here.
func TestTheSignalHandlerIsInstalledBeforeAnythingBinds(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the command package: %v", err)
	}

	checked := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			bind := firstCallTo(fn.Body, "Listen")
			handler := firstCallTo(fn.Body, "NotifyContext")
			if bind == token.NoPos {
				continue
			}
			checked++
			switch {
			case handler == token.NoPos:
				t.Errorf("%s binds a listener at %s and installs no signal handler; "+
					"call serveUntilSignal instead", fn.Name.Name, fset.Position(bind))
			case handler > bind:
				t.Errorf("%s binds a listener at %s before installing its signal handler at %s; "+
					"between the two, SIGINT kills a process that is already accepting",
					fn.Name.Name, fset.Position(bind), fset.Position(handler))
			}
		}
	}
	if checked == 0 {
		t.Fatal("no function in cmd binds a listener; the walk is broken")
	}
}

// firstCallTo reports the position of the earliest call to a function or
// method of this name inside body, or NoPos when there is none.
func firstCallTo(body *ast.BlockStmt, name string) token.Pos {
	found := token.NoPos
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		called := ""
		switch fun := call.Fun.(type) {
		case *ast.SelectorExpr:
			called = fun.Sel.Name
		case *ast.Ident:
			called = fun.Name
		}
		if called == name && (found == token.NoPos || call.Pos() < found) {
			found = call.Pos()
		}
		return true
	})
	return found
}
