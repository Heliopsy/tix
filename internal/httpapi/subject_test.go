// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// workSubjects are the event subjects that any holder of event:subscribe may
// see. They are the point of the scope: an agent watching a queue needs task,
// comment, artifact and dependency traffic, and a board needs project, label,
// workflow and field changes. Nothing here describes a person, a credential or
// the shape of the deployment.
var workSubjects = []string{
	"artifact",
	"comment",
	"field",
	"field_def",
	"noop",
	"project",
	"tag",
	"task",
	"workflow",
	"",
}

// TestEveryEventSubjectIsClassified reads the subject of every event the
// service layer emits and requires it to be either administrative, and so
// gated by a scope in subjectScope, or ordinary work listed above.
//
// Both lists are written by hand, which is the failure this test exists to
// prevent. A hand-maintained list of things to protect protects nothing once
// somebody adds the tenth item and edits only the code: the omission is
// silent, it defaults to open, and the new object joins the stream for every
// subscriber. So the lists are checked against the calls themselves rather
// than trusted. Adding an event with a new subject fails here until its
// classification is stated.
func TestEveryEventSubjectIsClassified(t *testing.T) {
	for subject, file := range eventSubjectsInService(t) {
		_, gated := subjectScope[subject]
		work := slices.Contains(workSubjects, subject)
		switch {
		case gated && work:
			t.Errorf("event subject %q (%s) is in subjectScope and in workSubjects; it cannot be both", subject, file)
		case !gated && !work:
			t.Errorf("event subject %q (%s) is classified nowhere.\n"+
				"Decide what a holder of event:subscribe alone may learn from it.\n"+
				"If it describes a person, a credential, or the deployment, add it to subjectScope in hub.go\n"+
				"with the scope that reads it over REST. If it is ordinary work, add it to workSubjects here.", subject, file)
		}
	}
}

// TestSubjectScopeHasNoDeadEntries keeps the gate honest in the other
// direction: a subject that no longer exists suggests the code moved on and
// the map did not, and the next reader would take a stale entry for cover.
func TestSubjectScopeHasNoDeadEntries(t *testing.T) {
	found := eventSubjectsInService(t)
	for subject := range subjectScope {
		if _, ok := found[subject]; !ok {
			t.Errorf("subjectScope gates %q, but no event in internal/service carries that subject any more", subject)
		}
	}
}

// eventSubjectsInService returns every subject passed to the two mutation
// methods that reach the outbox, mapped to the file it came from.
//
// Record(action, type, subjectType, ...) has the subject third; Event(type,
// subjectType, ...) has it second. Reading the arguments is what makes this a
// check rather than a second copy of the same list.
//
// The argument is not always a literal. Most of the service names its subjects
// with package constants, and the bundle importer indexes a map of them by
// import kind, so the scan resolves both against the package's own
// declarations. Anything it still cannot resolve is reported rather than
// skipped: a subject this test cannot read is a subject nothing classifies.
func eventSubjectsInService(t *testing.T) map[string]string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join("..", "service", "*.go"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no sources in internal/service: %v", err)
	}

	fset := token.NewFileSet()
	files := make(map[string]*ast.File, len(paths))
	for _, path := range paths {
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		files[path] = f
	}

	consts, maplits := declaredStrings(files)
	argOf := map[string]int{"Record": 2, "Event": 1}
	out := map[string]string{}

	for path, f := range files {
		base := filepath.Base(path)
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			idx, ok := argOf[sel.Sel.Name]
			if !ok || len(call.Args) <= idx {
				return true
			}
			// tx.go declares Record and forwards to Event with its own
			// parameter. That is the definition, not a call site that names a
			// subject, and it is the one place the argument is legitimately a
			// plain identifier with no constant behind it.
			if base == "tx.go" {
				return true
			}
			values, why := resolveSubject(call.Args[idx], consts, maplits)
			if why != "" {
				t.Errorf("%s: %s takes its event subject from %s, which this test cannot resolve. "+
					"Name it with a constant so it can be classified.", base, sel.Sel.Name, why)
				return true
			}
			for _, v := range values {
				out[v] = base
			}
			return true
		})
	}

	if len(out) < 10 {
		t.Fatalf("found only %d event subjects across %d service files, which means the scan stopped working "+
			"rather than that the service shrank", len(out), len(files))
	}
	return out
}

// resolveSubject reduces one call argument to the subject strings it can
// produce, or explains what it is when it cannot.
func resolveSubject(arg ast.Expr, consts map[string]string, maplits map[string][]string) ([]string, string) {
	switch e := arg.(type) {
	case *ast.BasicLit:
		if e.Kind != token.STRING {
			return nil, "a non-string literal"
		}
		v, err := strconv.Unquote(e.Value)
		if err != nil {
			return nil, "an unreadable string literal"
		}
		return []string{v}, ""
	case *ast.Ident:
		if v, ok := consts[e.Name]; ok {
			return []string{v}, ""
		}
		return nil, "the identifier " + e.Name + ", which is not a package string constant"
	case *ast.IndexExpr:
		id, ok := e.X.(*ast.Ident)
		if !ok {
			return nil, "an index of something that is not a named map"
		}
		if vs, ok := maplits[id.Name]; ok {
			return vs, ""
		}
		return nil, "an index of " + id.Name + ", which is not a package map of string constants"
	}
	return nil, "an expression built at run time"
}

// declaredStrings collects the package's string constants and the string
// values of its package-level map literals, which is everything the service
// currently uses to name a subject.
func declaredStrings(files map[string]*ast.File) (map[string]string, map[string][]string) {
	consts := map[string]string{}
	maplits := map[string][]string{}

	unquote := func(e ast.Expr) (string, bool) {
		lit, ok := e.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return "", false
		}
		v, err := strconv.Unquote(lit.Value)
		return v, err == nil
	}

	for _, f := range files {
		for _, d := range f.Decls {
			gen, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					if v, ok := unquote(vs.Values[i]); ok {
						consts[name.Name] = v
						continue
					}
					cl, ok := vs.Values[i].(*ast.CompositeLit)
					if !ok {
						continue
					}
					var got []string
					for _, elt := range cl.Elts {
						kv, ok := elt.(*ast.KeyValueExpr)
						if !ok {
							continue
						}
						if v, ok := unquote(kv.Value); ok {
							got = append(got, v)
						}
					}
					if len(got) > 0 {
						maplits[name.Name] = got
					}
				}
			}
		}
	}
	return consts, maplits
}

// TestSubjectScopeNamesRealScopes guards against a typo turning a gate into a
// permanent refusal that looks like a working one.
func TestSubjectScopeNamesRealScopes(t *testing.T) {
	for subject, scope := range subjectScope {
		if !slices.Contains(core.AllScopes, scope) {
			t.Errorf("subjectScope[%q] is %q, which is not a scope any credential can hold", subject, scope)
		}
	}
}

// TestAdministrativeEventsNeedTheirScope is the behaviour the map exists for,
// stated once against the thing a subscriber actually holds. A token minted
// for an agent -- event:subscribe and nothing else -- watches its queue and
// learns nothing about the tenant's people, credentials or delivery targets.
func TestAdministrativeEventsNeedTheirScope(t *testing.T) {
	agent := &wsConn{actor: testActor("t1", core.ScopeTaskRead, core.ScopeEventSubscribe)}
	admin := &wsConn{actor: testActor("t1", core.ScopeEventSubscribe, core.ScopeUserAdmin,
		core.ScopeTokenAdmin, core.ScopeTenantAdmin, core.ScopeWebhookAdmin, core.ScopeSyncAdmin)}

	event := func(subject string) core.Event {
		return core.Event{TenantID: "t1", SubjectType: subject, SubjectID: "s1"}
	}

	for subject := range subjectScope {
		if agent.entitled(event(subject)) {
			t.Errorf("a credential holding only event:subscribe was served the %q event; "+
				"it is refused that object over REST", subject)
		}
		if !admin.entitled(event(subject)) {
			t.Errorf("a credential holding %s was refused the %q event it is entitled to",
				subjectScope[subject], subject)
		}
	}

	// The scope still does what it is for.
	for _, subject := range workSubjects {
		if !agent.entitled(event(subject)) {
			t.Errorf("event:subscribe did not deliver the ordinary %q event", subject)
		}
	}

	// A wildcard credential is not caught by the gate.
	star := &wsConn{actor: testActor("t1", core.ScopeAll)}
	if !star.entitled(event("user")) {
		t.Error("a credential holding * was refused the user event")
	}
}
