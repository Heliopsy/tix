// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"errors"
	"fmt"
	"testing"
)

// allKinds is the complete taxonomy. It is Kinds itself rather than a second
// list of the same names: a hand-kept copy that calls itself complete stops
// being complete the first time a kind is added, and does it silently, because
// every test over it still passes.
var allKinds = Kinds

func TestKindOf(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want Kind
	}{
		{"nil", nil, ""},
		{"invalid", Invalid("bad %s", "input"), KindInvalid},
		{"not found", NotFound("no task"), KindNotFound},
		{"conflict", Conflict("claimed"), KindConflict},
		{"lease expired", LeaseExpired("stale token"), KindLeaseExpired},
		{"no task available", NoTaskAvailable("queue empty"), KindNoTaskAvailable},
		{"precondition", Precondition("illegal transition"), KindPrecondition},
		{"unauthenticated", Unauthenticated("no credential"), KindUnauthenticated},
		{"forbidden", Forbidden("missing scope"), KindForbidden},
		{"foreign error is internal", errors.New("boom"), KindInternal},
		{"wrapped domain error keeps kind", fmt.Errorf("ctx: %w", Conflict("claimed")), KindConflict},
		{"doubly wrapped", fmt.Errorf("a: %w", fmt.Errorf("b: %w", NotFound("x"))), KindNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := KindOf(tt.err); got != tt.want {
				t.Errorf("KindOf() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsKind(t *testing.T) {
	err := Conflict("claimed")
	if !IsKind(err, KindConflict) {
		t.Error("IsKind should match the error's own kind")
	}
	if IsKind(err, KindNotFound) {
		t.Error("IsKind should not match a different kind")
	}
}

func TestErrorWrapsCause(t *testing.T) {
	cause := errors.New("underlying")
	err := Internal("operation failed").Wrap(cause)

	if !errors.Is(err, cause) {
		t.Error("errors.Is should find the wrapped cause")
	}
	if got := err.Error(); got == "" {
		t.Error("Error() must not be empty")
	}
	if !contains(err.Error(), "underlying") {
		t.Errorf("Error() = %q, should mention the cause", err.Error())
	}
}

func TestErrorMessageWithoutCause(t *testing.T) {
	err := NotFound("task %q", "infra-42")
	want := `not_found: task "infra-42"`
	if got := err.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestWithDetail(t *testing.T) {
	err := Invalid("validation failed").
		WithDetail("field", "title").
		WithDetail("reason", "required")

	if err.Details["field"] != "title" || err.Details["reason"] != "required" {
		t.Errorf("Details = %v, want field and reason set", err.Details)
	}
	if err.Kind != KindInvalid {
		t.Errorf("WithDetail must not change Kind, got %q", err.Kind)
	}
}

func TestHTTPStatusForEveryKind(t *testing.T) {
	for _, k := range allKinds {
		got := k.HTTPStatus()
		if got < 400 || got > 599 {
			t.Errorf("Kind %q maps to HTTP %d, want a 4xx or 5xx", k, got)
		}
	}
}

func TestHTTPStatusMapping(t *testing.T) {
	tests := map[Kind]int{
		KindInvalid:         400,
		KindUnauthenticated: 401,
		KindForbidden:       403,
		KindNotFound:        404,
		KindNoTaskAvailable: 404,
		KindConflict:        409,
		KindLeaseExpired:    409,
		KindPrecondition:    422,
		KindUpstream:        502,
		KindInternal:        500,
	}
	for kind, want := range tests {
		if got := kind.HTTPStatus(); got != want {
			t.Errorf("Kind(%q).HTTPStatus() = %d, want %d", kind, got, want)
		}
	}
	assertEveryKindIsMapped(t, tests, "an HTTP status")
}

func TestExitCodeMapping(t *testing.T) {
	tests := map[Kind]int{
		"":                  ExitOK,
		KindInvalid:         ExitUsage,
		KindNotFound:        ExitNotFound,
		KindNoTaskAvailable: ExitNotFound,
		KindConflict:        ExitConflict,
		KindLeaseExpired:    ExitConflict,
		KindUnauthenticated: ExitPermission,
		KindForbidden:       ExitPermission,
		KindPrecondition:    ExitPrecondtion,
		KindUpstream:        ExitUpstream,
		KindInternal:        ExitError,
	}
	for kind, want := range tests {
		if got := kind.ExitCode(); got != want {
			t.Errorf("Kind(%q).ExitCode() = %d, want %d", kind, got, want)
		}
	}
	assertEveryKindIsMapped(t, tests, "an exit code")
}

// assertEveryKindIsMapped fails when the taxonomy has grown past a table that
// checks it. A map lookup passes for the kinds it lists and says nothing about
// the ones it does not, so without this the table quietly stops covering the
// thing it exists to cover.
func assertEveryKindIsMapped[V any](t *testing.T, table map[Kind]V, what string) {
	t.Helper()
	for _, k := range Kinds {
		if _, ok := table[k]; !ok {
			t.Errorf("kind %q has no case asserting %s", k, what)
		}
	}
}

func TestExitCodeNonZeroForEveryFailure(t *testing.T) {
	for _, k := range allKinds {
		if got := k.ExitCode(); got == ExitOK {
			t.Errorf("Kind %q maps to exit 0, want non-zero", k)
		}
	}
}

func TestUnknownKindFailsClosed(t *testing.T) {
	unknown := Kind("something_new")
	if got := unknown.HTTPStatus(); got != 500 {
		t.Errorf("unknown kind HTTPStatus() = %d, want 500", got)
	}
	if got := unknown.ExitCode(); got == ExitOK {
		t.Error("unknown kind must not map to exit 0")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
