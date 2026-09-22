// Package core holds the tix domain model and the Service contract.
package core

import (
	"errors"
	"fmt"
)

// Kind classifies an error so every transport can map it consistently.
type Kind string

// Error kinds.
const (
	KindInvalid         Kind = "invalid"
	KindNotFound        Kind = "not_found"
	KindConflict        Kind = "conflict"
	KindUnauthenticated Kind = "unauthenticated"
	KindForbidden       Kind = "forbidden"
	KindLeaseExpired    Kind = "lease_expired"
	KindNoTaskAvailable Kind = "no_task_available"
	KindPrecondition    Kind = "precondition_failed"
	KindInternal        Kind = "internal"
)

// Kinds lists every error kind, in the order the exit code table reads.
var Kinds = []Kind{
	KindInvalid, KindNotFound, KindConflict, KindUnauthenticated, KindForbidden,
	KindLeaseExpired, KindNoTaskAvailable, KindPrecondition, KindInternal,
}

// Error is a domain error with a Kind, a message and optional details.
type Error struct {
	Kind    Kind
	Message string
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

// WithDetail attaches a machine-readable detail. Never use it for secrets.
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

// KindOf reports the Kind of err, following the wrap chain.
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

// Exit codes returned by the CLI.
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
