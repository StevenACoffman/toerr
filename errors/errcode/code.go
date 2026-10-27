package errcode

// from https://github.com/remko/go-errors/blob/main/code.go

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	toerr "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/internal/trace"
)

// The gRPC canonical status codes (https://grpc.github.io/grpc/core/md_doc_statuscodes.html),
// apart from OK, named without reference to any transport. A code tells the caller
// what to do, not only what happened: each one says whether retrying can help, so a
// boundary decides retries from Status alone, and so do the proxies and retry
// middleware between it and the client.
//
// Values are assigned by iota, so append new codes at the end only: inserting or
// reordering renumbers every code below it. Each switch on StatusCode omits its
// default arm so the exhaustive linter fails the build until a new code is handled;
// no manual search for switches is needed.
const (
	// StatusUnknown means no code was attached. A boundary answers it as it answers
	// Internal, but it means unclassified, not a bug.
	StatusUnknown StatusCode = iota
	// StatusCanceled means the caller gave up, usually through its context.
	StatusCanceled
	// StatusInvalidArgument means the request is wrong whatever the system's state.
	// Retrying the same request cannot succeed.
	StatusInvalidArgument
	// StatusDeadlineExceeded means time ran out. The operation may still have
	// completed, so do not retry it automatically; the application may repeat an
	// idempotent one.
	StatusDeadlineExceeded
	// StatusInternal means an invariant broke: a bug, not a condition to retry. It is
	// not a default: an error nobody classified needs no code, and a dependency that is
	// down is Unavailable.
	StatusInternal
	// StatusNotFound means the requested entity does not exist.
	StatusNotFound
	// StatusUnauthenticated means the caller's identity could not be established.
	StatusUnauthenticated
	// StatusPermissionDenied means the caller is known but not allowed.
	StatusPermissionDenied
	// StatusAlreadyExists means the entity a create would make is already there.
	StatusAlreadyExists
	// StatusFailedPrecondition means the system is not in the state the operation
	// needs. Retrying cannot help until something changes that state.
	StatusFailedPrecondition
	// StatusUnimplemented means the operation is not supported.
	StatusUnimplemented
	// StatusUnavailable means a transient failure, such as a dependency being
	// down. Retry with backoff, if the operation is idempotent.
	StatusUnavailable
	// StatusResourceExhausted means a quota or rate limit ran out. Do not retry it
	// automatically: a quota may take hours to refill, and each retry may be billed.
	// Background work may retry after a long delay.
	StatusResourceExhausted
	// StatusAborted means a concurrency conflict, such as a failed transaction or
	// a stale version. Retry the whole read-modify-write, not the last step.
	StatusAborted
	// StatusOutOfRange means the request reached past a valid range, such as an
	// offset past the end. Unlike InvalidArgument, it may succeed once the range
	// grows.
	StatusOutOfRange
	// StatusDataLoss means unrecoverable loss or corruption of data.
	StatusDataLoss
)

// Compile-time interface implementation checks. TracePC is the errtrace-compatible
// trace marker the root errors package reads.
var (
	_ error                          = (*withCodeError)(nil)
	_ fmt.Formatter                  = (*withCodeError)(nil)
	_ interface{ TracePC() uintptr } = (*withCodeError)(nil)
	_ trace.Describer                = (*withCodeError)(nil)
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
	case StatusUnavailable:
		return "unavailable"
	case StatusResourceExhausted:
		return "resource_exhausted"
	case StatusAborted:
		return "aborted"
	case StatusOutOfRange:
		return "out_of_range"
	case StatusDataLoss:
		return "data_loss"
	}
	// No default arm on purpose: exhaustive (default-signifies-exhaustive) then forces
	// every StatusCode to be named above, so a new code fails the build here. This
	// return handles values outside the defined set.
	return "unknown"
}

// New creates a leaf error carrying code and a user-facing message, and records
// this call site. Use it where a failure is first understood; use [Link] to
// attach a code to an error that already exists.
//
// Give every leaf a message. The leaf is where the failure is understood, and its
// message is the one record of it that a user may see. Error() returns message, or
// the code in words ("not found") when message is empty, so the operator reads at
// least the category in the chain, as in "load seat 7: not found".
func New(code StatusCode, message string) error {
	return &withCodeError{code: code, message: message, pc: trace.CallerPC()}
}

// Link attaches a status code (and optional user-facing message) to err, and
// records this call site. Returns nil if err is nil, as errors.Wrap does.
//
// err stays in the Unwrap chain, so callers can still errors.Is/As into it. That
// suits an error of your own. Where err is a dependency's error that callers must
// not couple to, seal it first, so callers match only the code:
//
//	errcode.Link(code, message, errors.Seal(err))
//
// Seal the cause, not the result: errors.Seal(errcode.Link(…)) hides this code too.
func Link(code StatusCode, message string, err error) error {
	if err == nil {
		return nil
	}
	return &withCodeError{code: code, message: message, cause: err, pc: trace.CallerPC()}
}

// WithCode attaches a status code (and optional user-facing message) to cause, and
// records this call site. A nil cause yields a coded leaf.
//
// Deprecated: Use [Link], which returns nil for a nil cause, or [New] for a leaf.
// WithCode keeps its behaviour until it is removed in the first release after
// v0.4.0, so an existing call never changes meaning; it stops compiling instead.
func WithCode(code StatusCode, message string, cause error) error {
	return &withCodeError{code: code, message: message, cause: cause, pc: trace.CallerPC()}
}

// WithCodeOpaque is [WithCode] with cause sealed by errors.Seal: callers match the
// code but not the cause, and operators still see the cause's text, trace, and
// attributes. A nil cause yields a coded leaf.
//
// Deprecated: Use [Link] with errors.Seal, as in
// errcode.Link(code, message, errors.Seal(err)), or [New] for a leaf. It is removed
// in the first release after v0.4.0.
func WithCodeOpaque(code StatusCode, message string, cause error) error {
	return &withCodeError{
		code:    code,
		message: message,
		cause:   toerr.Seal(cause),
		pc:      trace.CallerPC(),
	}
}

// Code returns [Status] and [Message] together.
func Code(err error) (StatusCode, string) {
	var wc *withCodeError
	if !errors.As(err, &wc) {
		return StatusUnknown, ""
	}
	return wc.code, wc.userMessage()
}

// Status returns the outermost status code in err's chain, or StatusUnknown. The
// outermost wins so that a wrap can reclassify a failure for its own callers.
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
// cmp.Or) — never fall back to err.Error().
//
// It reads the outermost coded error that has a message. A coded wrap without one,
// such as Link(code, "", err), only reclassifies, so it leaves the message
// beneath it visible; an outer message still replaces an inner one. A sealed cause
// (errors.Seal) hides any message it carried.
func Message(err error) string {
	var wc *withCodeError
	if !errors.As(err, &wc) {
		return ""
	}
	return wc.userMessage()
}

// ForTransport returns the code and message a transport adapter answers err with.
// It is [Code], except for an error whose code is StatusUnknown that wraps a context
// error: context.DeadlineExceeded gives StatusDeadlineExceeded and context.Canceled
// gives StatusCanceled, checked in that order, as gRPC's status.FromContextError
// checks them. A request the caller abandoned, or that ran out of time, is then
// answered, counted, and logged apart from a failure.
//
// An explicit code wins, so a wrap that classified the context error keeps its code,
// and a [toerr.Seal] hides the context error as it hides everything beneath it.
// Status and Code are unchanged: they report only codes that were attached.
func ForTransport(err error) (StatusCode, string) {
	code, message := Code(err)
	if code != StatusUnknown {
		return code, message
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return StatusDeadlineExceeded, message
	case errors.Is(err, context.Canceled):
		return StatusCanceled, message
	}
	return code, message
}

func (e *withCodeError) Unwrap() error {
	return e.cause
}

// TracePC returns the program counter of the call site that attached the code, so
// a coded error contributes a frame to the return trace like any other hop.
func (e *withCodeError) TracePC() uintptr { return e.pc }

// DescribeLayer gives errors.LogValue the code and this error's own message.
func (e *withCodeError) DescribeLayer(trace.Token) trace.Layer {
	return trace.Layer{Msg: e.message, Code: e.code.String()}
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
		return strings.ReplaceAll(e.code.String(), "_", " ")
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

// userMessage returns the first non-empty message from e down its chain. Code and
// Message start it from the outermost coded error they already found, so the usual
// case, a message on that error, costs no second search.
func (e *withCodeError) userMessage() string {
	for e.message == "" {
		var inner *withCodeError
		if !errors.As(e.cause, &inner) {
			return ""
		}
		e = inner
	}
	return e.message
}
