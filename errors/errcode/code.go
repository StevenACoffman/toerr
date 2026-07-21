package errcode

// from https://github.com/remko/go-errors/blob/main/code.go

import (
	"errors"
	"fmt"
	"io"
)

// Union of HTTP status codes & gRPC status codes
// (https://grpc.github.io/grpc/core/md_doc_statuscodes.html).
// If you add a value here, search for all switches on this enum.
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

// Compile-time interface implementation checks.
var (
	_ error         = (*withCodeError)(nil)
	_ fmt.Formatter = (*withCodeError)(nil)
)

// StatusCode is a transport-neutral error status code.
type StatusCode int

type withCodeError struct {
	code StatusCode

	// Custom message (visible to user)
	message string

	cause error
}

// String returns a transport-neutral name for the code.
//
//nolint:cyclop // flat exhaustive value mapping; inherent, not branching logic.
func (c StatusCode) String() string {
	switch c {
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
	default:
		return "unknown"
	}
}

// WithCode attaches a status code (and optional user-visible message) to cause.
func WithCode(code StatusCode, message string, cause error) error {
	return &withCodeError{code: code, message: message, cause: cause}
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

func (e *withCodeError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = fmt.Fprintf(s, "%+v", e.Unwrap())
			if e.message != "" {
				_, _ = fmt.Fprintf(s, "\n(%s)", e.message)
			}
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, e.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	}
}
