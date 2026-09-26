// SPDX-License-Identifier: AGPL-3.0-or-later

package capability_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store/sqlite"
)

// scopeVariesByInput names the operations whose required scope is decided by
// what they are handed rather than by the operation, so no single scope
// describes one. An entry here is the reason an empty Scope on that operation
// does not read as "needs only a session".
var scopeVariesByInput = map[string]string{
	"ImportBundle": "authorises each component kind the bundle carries, so a " +
		"bundle of workflows needs workflow:write and a bundle of projects needs " +
		"project:write; the registry cannot name one scope for both",
}

// validArgs supplies the least input that gets past a method's own validation,
// for the methods that check their arguments before they ask the policy. The
// probe has to reach the authorization to read the scope off its refusal.
var validArgs = map[string][]any{
	"AddComment":     {core.TaskRef{ID: "t"}, "body"},
	"EditComment":    {"c1", "body"},
	"AddTag":         {core.TaskRef{ID: "t"}, "urgent"},
	"RemoveTag":      {core.TaskRef{ID: "t"}, "urgent"},
	"CreateTask":     {core.CreateTaskInput{Title: "a title"}},
	"TransitionTask": {core.TaskRef{ID: "t"}, core.TransitionInput{To: "done"}},
	"PutArtifact":    {core.TaskRef{ID: "t"}, core.ArtifactInput{Kind: core.ArtifactKind("log")}},
}

// TestEveryDeclaredScopeIsTheScopeTheServiceEnforces is why the registry may
// carry a scope at all. The terminal interface offers a view by asking whether
// the reader holds the scope the registry names, and internal/tui may not ask
// the policy itself, so a declaration that drifted from enforcement would open
// a screen the service then refuses, or hide one it would have allowed.
//
// Nothing here is a second opinion about authority: the scope is read back off
// the real refusal the real service produces for a reader holding nothing, and
// compared with what the registry says. A declaration nobody enforces, or an
// enforcement nobody declared, fails.
func TestEveryDeclaredScopeIsTheScopeTheServiceEnforces(t *testing.T) {
	t.Parallel()
	for _, op := range capability.Operations() {
		op := op
		t.Run(op.Name, func(t *testing.T) {
			t.Parallel()
			reason, varies := scopeVariesByInput[op.Method]
			if varies {
				if strings.TrimSpace(reason) == "" {
					t.Fatalf("%s is recorded as varying by input without a reason", op.Method)
				}
				if op.Scope != "" {
					t.Fatalf("%s varies by input and still declares scope %q", op.Method, op.Scope)
				}
				if op.TUI != "" {
					t.Fatalf("%s varies by input and is bound to the %q view, which would be "+
						"gated on a scope nothing declares", op.Method, op.TUI)
				}
				return
			}
			enforced, err := refusedScope(t, op.Method)
			if err != nil {
				t.Fatalf("probing %s: %v", op.Method, err)
			}
			if enforced != op.Scope {
				t.Fatalf("%s: the registry declares scope %q and the service enforces %q",
					op.Method, op.Scope, enforced)
			}
		})
	}
}

// TestTheProbeCanActuallySeeARefusal is the guard on the guard. Every check
// above compares two values the probe produced, so a probe that stopped
// reaching the policy would report every operation as needing nothing and
// agree with a registry that said so.
func TestTheProbeCanActuallySeeARefusal(t *testing.T) {
	t.Parallel()
	got, err := refusedScope(t, "ListTasks")
	if err != nil {
		t.Fatalf("probing ListTasks: %v", err)
	}
	if got != core.ScopeTaskRead {
		t.Fatalf("the probe read %q off a task listing refused to a reader holding nothing", got)
	}
	if got, err = refusedScope(t, "WhoAmI"); err != nil || got != "" {
		t.Fatalf("the probe read %q, %v off an operation that needs no scope", got, err)
	}
}

// refusedScope calls a service method as an actor holding no scope at all and
// returns the scope the policy named in its refusal, or the empty scope when
// the call was not refused for want of one.
func refusedScope(t *testing.T, method string) (core.Scope, error) {
	t.Helper()
	svc := scopelessService(t)
	actor := &core.Actor{ID: "u1", TenantID: "t1", Kind: core.ActorUser, Handle: "nobody"}
	ctx := core.WithSource(core.WithActor(context.Background(), actor), core.SourceTUI)

	fn := reflect.ValueOf(svc).MethodByName(method)
	if !fn.IsValid() {
		return "", errors.New("the service declares no such method")
	}
	ft := fn.Type()
	args := make([]reflect.Value, ft.NumIn())
	supplied := validArgs[method]
	for i := range ft.NumIn() {
		switch {
		case i == 0 && ft.In(0) == reflect.TypeOf((*context.Context)(nil)).Elem():
			args[i] = reflect.ValueOf(ctx)
		case i-1 < len(supplied):
			args[i] = reflect.ValueOf(supplied[i-1])
		default:
			args[i] = reflect.New(ft.In(i)).Elem()
		}
	}
	var out []reflect.Value
	if ft.IsVariadic() {
		out = fn.CallSlice(args)
	} else {
		out = fn.Call(args)
	}
	var failure error
	for _, v := range out {
		if e, ok := v.Interface().(error); ok && e != nil {
			failure = e
		}
	}
	if failure == nil || !core.IsKind(failure, core.KindForbidden) {
		return "", nil
	}
	var cerr *core.Error
	if !errors.As(failure, &cerr) {
		return "", errors.New("a refusal that is not a core error carries no scope")
	}
	named, ok := cerr.Details["scope"].(string)
	if !ok {
		return "", errors.New("the refusal names no scope")
	}
	return core.Scope(named), nil
}

// scopelessService builds the real service over its own temporary database, so
// the refusal under test is the one production code produces.
func scopelessService(t *testing.T) core.Service {
	t.Helper()
	clk := clock.NewFakeAt()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "authority.db"), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return service.New(st, service.WithClock(clk), service.WithHooks(service.HookOff))
}
