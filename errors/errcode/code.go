package errcode

// from https://github.com/remko/go-errors/blob/main/code.go

import (
	"errors"
	"fmt"
	"io"

	"github.com/StevenACoffman/toerr/errors/internal/trace"
)

// Union of HTTP status codes & gRPC status codes
// (https://grpc.github.io/grpc/core/md_doc_statuscodes.html).
//
// Values are assigned by iota, so append new codes at the end only: inserting or
// reordering renumbers every code below it. Each switch on StatusCode omits its
// default arm so the exhaustive linter fails the build until a new code is handled;
// no manual search for switches is needed.
const (
	StatusUnknown StatusCode = iota
	StatusCanceled
	StatusInvalidArgument
	StatusDeadlineExceeded
	StatusInternal
	StatusNotFound
	StatusUnauthenticated
	StatusPermissionDenied
	StatusAlreadyExists
	StatusFailedPrecondition
	StatusUnimplemented
)

// Compile-time interface implementation checks. TracePC is the errtrace-compatible
// trace marker the root errors package reads.
var (
	_ error                          = (*withCodeError)(nil)
	_ fmt.Formatter                  = (*withCodeError)(nil)
	_ interface{ TracePC() uintptr } = (*withCodeError)(nil)
)

// StatusCode is a transport-neutral error status code.
type StatusCode int

type withCodeError struct {
	code StatusCode

	// Custom message (visible to user)
	message string

	cause error

	pc uintptr // the call site that attached the code
}

// String returns a transport-neutral name for the code.
//
//nolint:cyclop // flat exhaustive value mapping; inherent, not branching logic.
func (c StatusCode) String() string {
	switch c {
	case StatusUnknown:
		return "unknown"
	case StatusCanceled:
		return "canceled"
	case StatusInvalidArgument:
		return "invalid_argument"
	case StatusDeadlineExceeded:
		return "deadline_exceeded"
	case StatusInternal:
		return "internal"
	case StatusNotFound:
		return "not_found"
	case StatusUnauthenticated:
		return "unauthenticated"
	case StatusPermissionDenied:
		return "permission_denied"
	case StatusAlreadyExists:
		return "already_exists"
	case StatusFailedPrecondition:
		return "failed_precondition"
	case StatusUnimplemented:
		return "unimplemented"
	}
	// No default arm on purpose: exhaustive (default-signifies-exhaustive) then forces
	// every StatusCode to be named above, so a new code fails the build here. This
	// return handles values outside the defined set.
	return "unknown"
}

// New creates a leaf error carrying code and a user-facing message, and records
// this call site. Use it where a failure is first understood; use [WithCode] to
// attach a code to an error that already exists. Error() returns message, or
// "error: <code>" when message is empty.
func New(code StatusCode, message string) error {
	return &withCodeError{code: code, message: message, pc: trace.CallerPC()}
}

// WithCode attaches a status code (and optional user-visible message) to cause,
// and records this call site.
//
// WithCode is a wrap. A nil cause still yields a coded leaf, for compatibility,
// but that use is deprecated: call [New] instead. A future release will make
// WithCode return nil for a nil cause, as errors.Wrap does, so that a wrap needs
// no nil check of its own. Replace every nil-cause call with New before taking
// that release: after it, such a call returns nil and reports success on a
// failure path.
//
// The cause stays in the Unwrap chain, so callers can still errors.Is/As into it.
// That is the %w-style contract: convenient when the cause is your own error, but
// at a boundary it lets callers couple to a dependency's error types. To sever the
// chain there, use [WithCodeOpaque].
func WithCode(code StatusCode, message string, cause error) error {
	return &withCodeError{code: code, message: message, cause: cause, pc: trace.CallerPC()}
}

// WithCodeOpaque is like [WithCode] but severs the chain: the cause's message is
// preserved in the error text (for operators and logs) yet the cause itself is no
// longer reachable via errors.Is, errors.As, or Unwrap.
//
// Use it at a boundary to translate a dependency's error into a domain code without
// letting callers couple to that dependency's error types. This is the %v-style
// translation from error_translation.md: after WithCodeOpaque, switching your
// storage engine cannot break a caller who matched on sql.ErrNoRows, because the
// match never succeeds. The user-facing Message and the domain Status are
// unaffected.
//
// As with WithCode, a nil cause is deprecated; call [New] for a leaf.
func WithCodeOpaque(code StatusCode, message string, cause error) error {
	e := &withCodeError{code: code, message: message, pc: trace.CallerPC()}
	if cause != nil {
		// Flatten to a plain string error: the text survives in Error()/%+v, but
		// the original type and identity are gone, so errors.Is/As cannot reach it.
		e.cause = errors.New(cause.Error())
	}
	return e
}

// Code returns the status code and message attached to err, or StatusUnknown.
func Code(err error) (StatusCode, string) {
	var wc *withCodeError
	if !errors.As(err, &wc) {
		return StatusUnknown, ""
	}
	return wc.code, wc.message
}

// Status returns the status code attached to err, or StatusUnknown.
func Status(err error) StatusCode {
	var wc *withCodeError
	if !errors.As(err, &wc) {
		return StatusUnknown
	}
	return wc.code
}

// Message returns the user-facing message attached to err, or "" if none is set.
//
// It is the safe, end-user view of an error: unlike err.Error(), it never
// exposes wrapped internal detail, so it is what you show to an end user or a
// client. When it returns "", supply your own generic message (for example with
// cmp.Or) — never fall back to err.Error(). It reads the outermost coded error
// in the chain.
func Message(err error) string {
	var wc *withCodeError
	if !errors.As(err, &wc) {
		return ""
	}
	return wc.message
}

func (e *withCodeError) Unwrap() error {
	return e.cause
}

// TracePC returns the program counter of the call site that attached the code, so
// a coded error contributes a frame to the return trace like any other hop.
func (e *withCodeError) TracePC() uintptr { return e.pc }

func (e *withCodeError) Error() string {
	switch {
	case e.message != "" && e.cause != nil:
		return fmt.Sprintf("%s (%s)", e.cause.Error(), e.message)
	case e.message != "":
		return e.message
	case e.cause != nil:
		return e.cause.Error()
	default:
		return fmt.Sprintf("error: %s", e.code)
	}
}

// Format prints the return trace for %+v, as the root errors package does, and the
// message otherwise.
func (e *withCodeError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = io.WriteString(s, trace.Format(e))
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, e.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	}
}
