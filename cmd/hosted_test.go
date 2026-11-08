// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bufio"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
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

// TestEveryGoroutineIsGuarded fails on a bare go statement in cmd/: a panic
// in a goroutine a hosted command started would end the whole server.
func TestEveryGoroutineIsGuarded(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
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
			if !ok || fn.Body == nil || fn.Name.Name == "goGuarded" {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				if g, ok := n.(*ast.GoStmt); ok {
					t.Errorf("%s: %s starts a goroutine without goGuarded", fset.Position(g.Pos()), fn.Name.Name)
				}
				return true
			})
		}
	}
}

// A panic in a goroutine a hosted command started is recovered and the
// goroutine's own defers still run.
func TestGoGuardedRecoversOnlyWhenHosted(t *testing.T) {
	done := make(chan struct{})
	goGuarded(context.WithValue(context.Background(), hostedKey{}, true), func() {
		defer close(done)
		panic("inside a hosted command")
	})
	<-done
}

// pathFlags are the hosted flags and arguments that name a file, each refused
// over ssh by localFile and each with a refusal case in internal/sshd. A path
// flag that is handed on to an internal package, rather than opened here, is
// invisible to TestEveryFileCallIsGuarded, so every flag that looks like a
// path has to be named here before it can ship.
var pathFlags = map[string]bool{
	"artifact put --file":            true,
	"bundle import FILE":             true,
	"domain add --cert":              true,
	"domain add --key":               true,
	"export tasks --out":             true,
	"import tasks --mapping":         true,
	"import tasks --save-mapping":    true,
	"import tasks --lines":           true,
	"import tasks FILE":              true,
	"project create --icon-file":     true,
	"project edit --icon-file":       true,
	"project icon --out":             true,
	"sync source add --file":         true,
	"sync source add --mapping":      true,
	"sync source add --mapping-file": true,
	"user key add --file":            true,
	"workflow put --file":            true,
}

// pathish matches a flag whose name or usage says it takes a path.
var pathish = regexp.MustCompile(`(?i)\b(file|path|directory|dir|out|lines|cert|key file)\b`)

// notPaths are flags pathish matches that take no path.
var notPaths = map[string]bool{
	"artifact put --kind":    true,
	"export tasks --format":  true,
	"import tasks --format":  true,
	"domain add --cert-mode": true,
	"task add --parse":       true,
}

func TestEveryHostedPathFlagIsKnown(t *testing.T) {
	root, _ := newRoot([]string{}, "")
	pruneForSSH(root, capability.SSHCommands())
	seen := map[string]bool{}
	var visit func(c *cobra.Command)
	visit = func(c *cobra.Command) {
		path := strings.TrimPrefix(c.CommandPath(), "tix ")
		if strings.Contains(c.Use, "FILE") {
			seen[path+" FILE"] = true
		}
		c.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
			name := path + " --" + f.Name
			seen[name] = true
			if (pathish.MatchString(f.Usage) || pathish.MatchString(f.Name)) && !notPaths[name] && !pathFlags[name] {
				t.Errorf("%s looks like it takes a path and is not in pathFlags: %q", name, f.Usage)
			}
		})
		if strings.Contains(c.Use, "FILE") && !pathFlags[path+" FILE"] {
			t.Errorf("%s takes a FILE argument and is not in pathFlags", path)
		}
		for _, k := range c.Commands() {
			visit(k)
		}
	}
	visit(root)
	for name := range pathFlags {
		if !seen[name] {
			t.Errorf("pathFlags names %s, which the hosted tree does not have", name)
		}
	}
}

func TestAnAnswerEndsAtEitherLineEnding(t *testing.T) {
	cases := map[string][]string{
		"123456\n654321\n":   {"123456", "654321"},
		"123456\r654321\r":   {"123456", "654321"},
		"123456\r\n654321\n": {"123456", "654321"},
	}
	for in, want := range cases {
		r := bufio.NewReader(strings.NewReader(in))
		afterCR := false
		for _, w := range want {
			got, err := readAnswer(r, &afterCR)
			if err != nil || got != w {
				t.Errorf("%q: read %q, %v; want %q", in, got, err, w)
			}
		}
	}
}
