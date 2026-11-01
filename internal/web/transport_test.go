// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"os"
	"path/filepath"
	"testing"
)

// TestTestsNeverUseTheSharedDefaultTransport pins the fix for a web test that
// failed at random with "transport connection broken: http: CloseIdleConnections
// called". httptest.Server.Close closes the idle connections of
// http.DefaultTransport, so any test that sent its requests through the default
// transport or client could lose one when a parallel test shut its own server
// down. Each test talks to its server through that server's transport instead.
func TestTestsNeverUseTheSharedDefaultTransport(t *testing.T) {
	t.Parallel()
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := gotoken.NewFileSet()
	for _, name := range files {
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if ok && pkg.Name == "http" && (sel.Sel.Name == "DefaultTransport" || sel.Sel.Name == "DefaultClient") {
				t.Errorf("%s uses http.%s; use the test server's own client so a parallel server's Close cannot break it", fset.Position(sel.Pos()), sel.Sel.Name)
			}
			return true
		})
	}
}
