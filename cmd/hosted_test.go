// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// neverHosted are the files whose file calls run only in commands the SSH
// listener never hosts, each because the registry binds none of its commands.
var neverHosted = map[string]string{
	"ctx.go":                "named contexts are local configuration",
	"docs.go":               "generated documentation is written on the machine running it",
	"completion_install.go": "shell completion is installed on the machine running it",
	"update.go":             "self update replaces the running binary",
}

// fileCalls are the os functions that open, read, write or inspect a path.
var fileCalls = map[string]bool{
	"Open": true, "OpenFile": true, "Create": true, "ReadFile": true, "WriteFile": true,
	"Stat": true, "Lstat": true, "Remove": true, "RemoveAll": true, "Rename": true,
	"Mkdir": true, "MkdirAll": true, "ReadDir": true, "Truncate": true, "Chmod": true,
}

// TestEveryFileCallIsGuarded fails on a function in cmd/ that touches a path
// without asking localFile first, so a flag added later that names a file
// cannot reach the server's disk from an SSH session.
func TestEveryFileCallIsGuarded(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	checked := 0
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") || neverHosted[name] != "" {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			var touches []string
			guarded := false
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.SelectorExpr:
					if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "os" && fileCalls[fun.Sel.Name] {
						touches = append(touches, fset.Position(call.Pos()).String()+" os."+fun.Sel.Name)
					}
				case *ast.Ident:
					if fun.Name == "localFile" {
						guarded = true
					}
				}
				return true
			})
			checked += len(touches)
			if len(touches) > 0 && !guarded {
				t.Errorf("%s touches a path without localFile: %s", fn.Name.Name, strings.Join(touches, ", "))
			}
		}
	}
	if checked == 0 {
		t.Fatal("no file call was found, so nothing was checked")
	}
}

// TestTheHostedTreeHoldsOnlyHostedCommands builds the tree an SSH session
// gets and checks nothing outside the registry's list survived the pruning.
func TestTheHostedTreeHoldsOnlyHostedCommands(t *testing.T) {
	root, _ := newRoot([]string{}, "")
	allowed := []string{"task ls", "task add", "export", "export tasks"}
	pruneForSSH(root, allowed)
	var paths []string
	var visit func(prefix string, cmds []*cobra.Command)
	visit = func(prefix string, cmds []*cobra.Command) {
		for _, c := range cmds {
			path := strings.TrimSpace(prefix + " " + c.Name())
			paths = append(paths, path)
			visit(path, c.Commands())
		}
	}
	visit("", root.Commands())
	want := map[string]bool{"task": true, "task ls": true, "task add": true, "export": true, "export tasks": true}
	for _, p := range paths {
		if !want[p] {
			t.Errorf("%q survived the pruning", p)
		}
		delete(want, p)
	}
	for p := range want {
		t.Errorf("%q was pruned away", p)
	}
	if root.PersistentPreRunE == nil {
		t.Fatal("the root lost the pre-run that refuses local flags")
	}
}

// TestOnlyTheRootRunsAPersistentPreRun guards the refusal of local flags,
// which lives in the root's pre-run: cobra runs only the nearest one, so a
// command declaring its own would walk past it.
func TestOnlyTheRootRunsAPersistentPreRun(t *testing.T) {
	root, _ := newRoot([]string{}, "")
	var visit func(cmds []*cobra.Command)
	visit = func(cmds []*cobra.Command) {
		for _, c := range cmds {
			if c.PersistentPreRunE != nil || c.PersistentPreRun != nil {
				t.Errorf("%s declares its own persistent pre-run", c.CommandPath())
			}
			visit(c.Commands())
		}
	}
	visit(root.Commands())
}
