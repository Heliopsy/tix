// Package core holds the tix domain model and the Service contract.
//
// This package imports only the standard library. Both the local service
// implementation and the remote HTTP client depend on it, and neither depends
// on the other, so "one authoritative code path" is a compile-time property
// rather than a convention. Do not add a dependency here.
package core

import (
	"errors"
	"fmt"
)

// Kind classifies an error so every transport can map it consistently. A Kind
// determines the HTTP status code the API returns and the exit code the CLI
// exits with, which is what makes local and remote behaviour indistinguishable.
type Kind string

// Error kinds. These are the complete taxonomy; a new kind requires updating
// the HTTP and exit-code mappings below, both of which are exhaustively tested.
const (
	// KindInvalid means the request was malformed or failed validation.
	KindInvalid Kind = "invalid"
	// KindNotFound means the addressed resource does not exist, or is not
	// visible to the caller. Tenant isolation deliberately reports a
	// cross-tenant reference as not found rather than forbidden, so the API
	// does not confirm that another tenant's resource exists.
	KindNotFound Kind = "not_found"
	// KindConflict means the resource changed underneath the caller, or is
	// already claimed. Claim contention reports this.
	KindConflict Kind = "conflict"
	// KindUnauthenticated means no valid credential was presented.
	KindUnauthenticated Kind = "unauthenticated"
	// KindForbidden means the caller is known but lacks the required scope.
	KindForbidden Kind = "forbidden"
	// KindLeaseExpired means the presented lease token is no longer current.
	// A worker receiving this must re-claim before acting on the task again.
	KindLeaseExpired Kind = "lease_expired"
	// KindNoTaskAvailable means a claim-next found no eligible task. This is
	// an ordinary empty result, not a failure, and is distinct from not found.
	KindNoTaskAvailable Kind = "no_task_available"
	// KindPrecondition means a rule prevented the operation, such as an
	// illegal workflow transition or a dependency cycle.
	KindPrecondition Kind = "precondition_failed"
	// KindInternal means an unexpected failure. Details are logged, not returned.
	KindInternal Kind = "internal"
)

// Error is a domain error carrying a Kind, a human-readable message, optional
// structured details, and an optional wrapped cause.
type Error struct {
	Kind    Kind
	Message string
	// Details carries machine-readable context, such as the field that failed
	// validation. It SHALL NOT contain secrets.
	Details map[string]any
	cause   error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Kind, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Kind, e.Message)
}

// Unwrap exposes the wrapped cause to errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.cause }

// newf builds an Error with a formatted message.
func newf(kind Kind, format string, args ...any) *Error {
	return &Error{Kind: kind, Message: fmt.Sprintf(format, args...)}
}

// Invalid reports malformed input or failed validation.
func Invalid(format string, args ...any) *Error { return newf(KindInvalid, format, args...) }

// NotFound reports a missing or invisible resource.
func NotFound(format string, args ...any) *Error { return newf(KindNotFound, format, args...) }

// Conflict reports concurrent modification or contended claim.
func Conflict(format string, args ...any) *Error { return newf(KindConflict, format, args...) }

// Unauthenticated reports a missing or invalid credential.
func Unauthenticated(format string, args ...any) *Error {
	return newf(KindUnauthenticated, format, args...)
}

// Forbidden reports insufficient scope.
func Forbidden(format string, args ...any) *Error { return newf(KindForbidden, format, args...) }

// LeaseExpired reports a stale lease token.
func LeaseExpired(format string, args ...any) *Error {
	return newf(KindLeaseExpired, format, args...)
}

// NoTaskAvailable reports that no eligible task matched a claim-next.
func NoTaskAvailable(format string, args ...any) *Error {
	return newf(KindNoTaskAvailable, format, args...)
}

// Precondition reports a rule violation such as an illegal transition.
func Precondition(format string, args ...any) *Error {
	return newf(KindPrecondition, format, args...)
}

// Internal reports an unexpected failure.
func Internal(format string, args ...any) *Error { return newf(KindInternal, format, args...) }

// WithDetail attaches a machine-readable detail and returns the error, so it
// can be built in a single expression. It SHALL NOT be used for secrets.
func (e *Error) WithDetail(key string, value any) *Error {
	if e.Details == nil {
		e.Details = make(map[string]any, 1)
	}
	e.Details[key] = value
	return e
}

// Wrap attaches a cause and returns the error.
func (e *Error) Wrap(cause error) *Error {
	e.cause = cause
	return e
}

// KindOf reports the Kind of err, following the wrap chain. An error that is
// not a domain error is reported as KindInternal, so an unexpected failure
// never accidentally maps to a success-adjacent status.
func KindOf(err error) Kind {
	if err == nil {
		return ""
	}
	var domain *Error
	if errors.As(err, &domain) {
		return domain.Kind
	}
	return KindInternal
}

// IsKind reports whether err has the given Kind.
func IsKind(err error, kind Kind) bool { return KindOf(err) == kind }

// HTTPStatus maps a Kind to its HTTP status code.
func (k Kind) HTTPStatus() int {
	switch k {
	case KindInvalid:
		return 400
	case KindUnauthenticated:
		return 401
	case KindForbidden:
		return 403
	case KindNotFound, KindNoTaskAvailable:
		return 404
	case KindConflict, KindLeaseExpired:
		return 409
	case KindPrecondition:
		return 422
	case KindInternal:
		return 500
	default:
		return 500
	}
}

// Exit codes returned by the CLI. They are documented in the cli capability and
// scripts depend on them, so they SHALL NOT be renumbered.
const (
	ExitOK          = 0
	ExitError       = 1
	ExitUsage       = 2
	ExitNotFound    = 3
	ExitConflict    = 4
	ExitPermission  = 5
	ExitPrecondtion = 6
)

// ExitCode maps a Kind to the process exit code the CLI uses.
func (k Kind) ExitCode() int {
	switch k {
	case "":
		return ExitOK
	case KindInvalid:
		return ExitUsage
	case KindNotFound, KindNoTaskAvailable:
		return ExitNotFound
	case KindConflict, KindLeaseExpired:
		return ExitConflict
	case KindUnauthenticated, KindForbidden:
		return ExitPermission
	case KindPrecondition:
		return ExitPrecondtion
	default:
		return ExitError
	}
}
