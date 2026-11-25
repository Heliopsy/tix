// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/docsmd"
)

// refusalReasons reads every Refused constant out of refusal.go, so the
// documented list is checked against the code rather than against a copy.
func refusalReasons(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "refusal.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.ValueSpec)
		if !ok || len(spec.Names) != 1 || !strings.HasPrefix(spec.Names[0].Name, "Refused") || len(spec.Values) != 1 {
			return true
		}
		if lit, ok := spec.Values[0].(*ast.BasicLit); ok {
			v, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, v)
		}
		return true
	})
	if len(out) < 20 {
		t.Fatalf("read %d reasons out of refusal.go, the parse is not finding them", len(out))
	}
	return out
}

func TestEveryRefusalReasonIsDocumented(t *testing.T) {
	page, err := docsmd.Read("docs/deployment.md")
	if err != nil {
		t.Fatal(err)
	}
	section, err := docsmd.Section(page, "### Refused authentication")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := docsmd.Table(section, "Reason", "Status", "Meaning")
	if err != nil {
		t.Fatal(err)
	}
	var documented []string
	for _, cell := range docsmd.Column(rows, 0) {
		documented = append(documented, docsmd.Codes(cell)...)
	}
	code := refusalReasons(t)
	for _, r := range code {
		if !slices.Contains(documented, r) {
			t.Errorf("reason %q is not in the docs/deployment.md table", r)
		}
	}
	for _, r := range documented {
		if !slices.Contains(code, r) {
			t.Errorf("docs/deployment.md lists %q, which the code never logs", r)
		}
	}
}
