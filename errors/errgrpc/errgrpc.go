package errgrpc

import (
	"cmp"
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/StevenACoffman/toerr/errors/errcode"
)

// Code maps a domain status code to a gRPC code. The two sets match apart from OK,
// so each domain code has exactly one gRPC code. A value outside errcode's set maps
// to Unknown.
//
//nolint:cyclop // flat exhaustive value mapping; inherent, not branching logic.
func Code(code errcode.StatusCode) codes.Code {
	switch code {
	case errcode.StatusUnknown:
		return codes.Unknown
	case errcode.StatusCanceled:
		return codes.Canceled
	case errcode.StatusInvalidArgument:
		return codes.InvalidArgument
	case errcode.StatusDeadlineExceeded:
		return codes.DeadlineExceeded
	case errcode.StatusInternal:
		return codes.Internal
	case errcode.StatusNotFound:
		return codes.NotFound
	case errcode.StatusUnauthenticated:
		return codes.Unauthenticated
	case errcode.StatusPermissionDenied:
		return codes.PermissionDenied
	case errcode.StatusAlreadyExists:
		return codes.AlreadyExists
	case errcode.StatusFailedPrecondition:
		return codes.FailedPrecondition
	case errcode.StatusUnimplemented:
		return codes.Unimplemented
	case errcode.StatusUnavailable:
		return codes.Unavailable
	case errcode.StatusResourceExhausted:
		return codes.ResourceExhausted
	case errcode.StatusAborted:
		return codes.Aborted
	case errcode.StatusOutOfRange:
		return codes.OutOfRange
	case errcode.StatusDataLoss:
		return codes.DataLoss
	}
	// No default arm on purpose: exhaustive (default-signifies-exhaustive) then forces
	// every errcode.StatusCode to be mapped above, so a new code fails the build here.
	return codes.Unknown
}

// FromCode maps a gRPC code to the domain code of the same name. OK, which is not an
// error, and a value outside gRPC's set map to StatusUnknown. It is the raw table: for
// an error a client call returned, use FromError, which adjusts the codes that
// describe your call to the downstream service rather than your caller's request, and
// keep FromCode for a call that forwarded the caller's input unchanged.
//
//nolint:cyclop // flat exhaustive value mapping; inherent, not branching logic.
func FromCode(code codes.Code) errcode.StatusCode {
	switch code {
	case codes.OK, codes.Unknown:
		return errcode.StatusUnknown
	case codes.Canceled:
		return errcode.StatusCanceled
	case codes.InvalidArgument:
		return errcode.StatusInvalidArgument
	case codes.DeadlineExceeded:
		return errcode.StatusDeadlineExceeded
	case codes.Internal:
		return errcode.StatusInternal
	case codes.NotFound:
		return errcode.StatusNotFound
	case codes.Unauthenticated:
		return errcode.StatusUnauthenticated
	case codes.PermissionDenied:
		return errcode.StatusPermissionDenied
	case codes.AlreadyExists:
		return errcode.StatusAlreadyExists
	case codes.FailedPrecondition:
		return errcode.StatusFailedPrecondition
	case codes.Unimplemented:
		return errcode.StatusUnimplemented
	case codes.Unavailable:
		return errcode.StatusUnavailable
	case codes.ResourceExhausted:
		return errcode.StatusResourceExhausted
	case codes.Aborted:
		return errcode.StatusAborted
	case codes.OutOfRange:
		return errcode.StatusOutOfRange
	case codes.DataLoss:
		return errcode.StatusDataLoss
	}
	return errcode.StatusUnknown
}

// FromError returns the domain status code for an error a gRPC client call returned,
// as the code of the operation that made the call, or StatusUnknown for nil and for an
// error without a gRPC status.
//
// A downstream code describes your call to that service, not your caller's request, so
// FromError adjusts the party responsible for it, as Google's error model asks of a
// service that depends on others. InvalidArgument, FailedPrecondition, OutOfRange, and
// Unimplemented mean your request to it was wrong, and Unauthenticated and
// PermissionDenied mean your service's credentials or grants were: all six become
// StatusInternal. NotFound and AlreadyExists, which usually concern the caller's own
// entity, and the codes that carry retry guidance pass through. A context error comes
// first, for a client wrapper that returns ctx.Err() itself.
//
// Translate the error where it enters your code, and seal it:
//
//	resp, err := client.GetUser(ctx, req)
//	if err != nil {
//		return nil, errcode.Link(errgrpc.FromError(err), "", errors.Seal(err))
//	}
//
// The seal stops callers matching the gRPC status, and the Link records your call
// site in the return trace. The remote message stays in the operator view (Error and
// %+v) and out of errcode.Message: the remote service wrote it for its own operators,
// not for your users.
//
// When your call forwarded the caller's own input unchanged, so that a downstream
// InvalidArgument is the caller's to fix, use FromCode instead.
func FromError(err error) errcode.StatusCode {
	switch {
	case err == nil:
		return errcode.StatusUnknown
	case errors.Is(err, context.DeadlineExceeded):
		return errcode.StatusDeadlineExceeded
	case errors.Is(err, context.Canceled):
		return errcode.StatusCanceled
	}
	switch code := status.Code(err); code {
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange,
		codes.Unimplemented, codes.Unauthenticated, codes.PermissionDenied:
		return errcode.StatusInternal
	default:
		return FromCode(code)
	}
}

// Error converts err into a gRPC status error for a handler to return, or nil if
// err is nil. Log err first: the status error keeps only the code and the message.
//
// It is the end-user safety boundary for gRPC. The message is the error's
// user-facing message (errcode.Message), or the gRPC code's name when none is set.
// It never uses err.Error(), so wrapped internal detail does not reach clients.
// Without it, gRPC sends err.Error() itself as the message of any error that is not
// a bare status error.
//
// A gRPC status already in err's chain, such as one a downstream call returned, is
// not passed through: its message was written for another service's operators. An
// error without an errcode status becomes Unknown, except that one wrapping
// context.DeadlineExceeded or context.Canceled becomes DeadlineExceeded or Canceled
// (errcode.ForTransport), as grpc-go's server does for a handler's bare context
// error with status.FromContextError. Unlike FromContextError, the message is still
// never err.Error(). Translate client errors with [FromCode] where they enter your
// code.
func Error(err error) error {
	if err == nil {
		return nil
	}
	domain, message := errcode.ForTransport(err)
	code := Code(domain)
	// Returned unwrapped on purpose: gRPC replaces the message of a wrapped status
	// with the wrapper's Error(), which is the very detail this function withholds.
	return status.Error(code, cmp.Or(message, code.String())) //nolint:wrapcheck // see above.
}
