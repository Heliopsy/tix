// SPDX-License-Identifier: AGPL-3.0-or-later

package capability_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/docsmd"
	"github.com/heliopsy/tix/internal/wire"
)

// documentedRoutes returns the paths the routes table of docs/api.md names,
// with the method a cell may lead with dropped. Only that table is read: the
// page quotes routes in prose and examples too, and a route mentioned in
// passing is not a route a reader can find by scanning the table.
func documentedRoutes(t *testing.T) map[string]bool {
	t.Helper()
	md, err := docsmd.Read("docs/api.md")
	if err != nil {
		t.Fatalf("reading docs/api.md: %v", err)
	}
	section, err := docsmd.Section(md, "## Routes")
	if err != nil {
		t.Fatalf("docs/api.md: %v", err)
	}
	rows, err := docsmd.Table(section, "Area", "Routes")
	if err != nil {
		t.Fatalf("docs/api.md: %v", err)
	}
	out := map[string]bool{}
	for _, cell := range docsmd.Column(rows, 1) {
		for _, code := range docsmd.Codes(cell) {
			fields := strings.Fields(code)
			out[fields[len(fields)-1]] = true
		}
	}
	return out
}

// wireAPIRoutes reads every Route constant under the API prefix out of the
// wire package's source. The constants are the routes the server registers,
// including the few no registry operation names, such as a task's own audit
// trail, so reading them covers what the registry alone would miss.
func wireAPIRoutes(t *testing.T) []string {
	t.Helper()
	root, err := docsmd.Root()
	if err != nil {
		t.Fatalf("locating the module root: %v", err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "internal", "wire", "wire.go"), nil, 0)
	if err != nil {
		t.Fatalf("parsing wire.go: %v", err)
	}
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range spec.Names {
			if !strings.HasPrefix(name.Name, "Route") || i >= len(spec.Values) {
				continue
			}
			sum, ok := spec.Values[i].(*ast.BinaryExpr)
			if !ok {
				continue
			}
			prefix, isIdent := sum.X.(*ast.Ident)
			lit, isLit := sum.Y.(*ast.BasicLit)
			if !isIdent || prefix.Name != "APIPrefix" || !isLit {
				continue
			}
			path, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatalf("%s: %v", name.Name, err)
			}
			out = append(out, path)
		}
		return true
	})
	return out
}

// TestEveryAPIRouteIsInTheRoutesTable fails when the server gains a route the
// table of docs/api.md does not list. Fourteen routes, the second factor, SSH
// keys, statistics, status and live connections among them, shipped without a
// row while every one of them had a command and a browser screen.
func TestEveryAPIRouteIsInTheRoutesTable(t *testing.T) {
	t.Parallel()
	documented := documentedRoutes(t)
	routes := wireAPIRoutes(t)
	if len(routes) < 40 {
		t.Fatalf("read %d routes from wire.go, which is not the whole contract", len(routes))
	}
	for _, path := range routes {
		if !documented[path] {
			t.Errorf("%s%s is served and missing from the routes table of docs/api.md", wire.APIPrefix, path)
		}
	}
	for _, op := range capability.Operations() {
		if op.HTTP.Zero() {
			continue
		}
		path := strings.TrimPrefix(op.HTTP.Pattern, wire.APIPrefix)
		if !documented[path] {
			t.Errorf("%s: %s is missing from the routes table of docs/api.md", op.Name, op.HTTP.Pattern)
		}
	}
}
