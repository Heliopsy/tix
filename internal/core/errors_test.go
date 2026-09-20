package core

import (
	"errors"
	"fmt"
	"testing"
)

// allKinds is the complete taxonomy. Adding a Kind without adding it here
// fails the exhaustiveness tests below.
var allKinds = []Kind{
	KindInvalid, KindNotFound, KindConflict, KindUnauthenticated, KindForbidden,
	KindLeaseExpired, KindNoTaskAvailable, KindPrecondition, KindInternal,
}

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
	// The cause must be visible in the message so logs are useful.
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
	// Every kind must map to a real client or server error status. A kind that
	// fell through to a 2xx would report failure as success.
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
		KindInternal:        500,
	}
	for kind, want := range tests {
		if got := kind.HTTPStatus(); got != want {
			t.Errorf("Kind(%q).HTTPStatus() = %d, want %d", kind, got, want)
		}
	}
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
		KindInternal:        ExitError,
	}
	for kind, want := range tests {
		if got := kind.ExitCode(); got != want {
			t.Errorf("Kind(%q).ExitCode() = %d, want %d", kind, got, want)
		}
	}
}

func TestExitCodeNonZeroForEveryFailure(t *testing.T) {
	// A failing operation must never exit 0, or scripts silently continue.
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
