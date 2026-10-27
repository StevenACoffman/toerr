package errhttp

import (
	"net/http"

	"github.com/StevenACoffman/toerr/errors/errcode"
)

// StatusClientClosedRequest is 499 Client Closed Request: the client went away before
// the server answered. It is not in net/http, because no RFC defines it, but it is the
// HTTP mapping google/rpc/code.proto gives CANCELLED and the one grpc-gateway uses, so
// access logs and metrics can tell an abandoned request from a failed one.
const StatusClientClosedRequest = 499

// Status maps a domain status code to an HTTP status code. Unknown codes map to
// 500 (no logging — this is a pure mapping).
//
//nolint:cyclop // flat exhaustive value mapping; inherent, not branching logic.
func Status(code errcode.StatusCode) int {
	switch code {
	case errcode.StatusInternal, errcode.StatusUnknown, errcode.StatusDataLoss:
		return http.StatusInternalServerError
	case errcode.StatusCanceled:
		return StatusClientClosedRequest
	case errcode.StatusUnimplemented:
		return http.StatusNotImplemented
	case errcode.StatusNotFound:
		return http.StatusNotFound
	case errcode.StatusDeadlineExceeded:
		return http.StatusGatewayTimeout
	case errcode.StatusInvalidArgument,
		errcode.StatusFailedPrecondition,
		errcode.StatusOutOfRange:
		return http.StatusBadRequest
	case errcode.StatusAlreadyExists, errcode.StatusAborted:
		return http.StatusConflict
	case errcode.StatusUnavailable:
		return http.StatusServiceUnavailable
	case errcode.StatusResourceExhausted:
		return http.StatusTooManyRequests
	case errcode.StatusUnauthenticated:
		return http.StatusUnauthorized
	case errcode.StatusPermissionDenied:
		return http.StatusForbidden
	}
	// No default arm on purpose: exhaustive (default-signifies-exhaustive) then forces
	// every errcode.StatusCode to be mapped above, so a new code fails the build here.
	// This return handles values outside the defined set (StatusUnknown maps to 500).
	return http.StatusInternalServerError
}

// StatusMessage returns the HTTP status for code and a message, defaulting the
// message to the standard HTTP status text when empty.
func StatusMessage(code errcode.StatusCode, message string) (int, string) {
	status := Status(code)
	if message == "" {
		message = statusText(status)
	}
	return status, message
}

// Error maps the status code attached to err to an HTTP status and message.
// Use it at the transport boundary: status, msg := errhttp.Error(err).
//
// It is the end-user safety boundary for HTTP: the message is the error's
// user-facing message (errcode.Message), falling back to the generic HTTP status
// text when none is set. It never returns err.Error(), so wrapped internal
// detail is not exposed to clients.
//
// The code is errcode.ForTransport's, so an error without a code that wraps a context
// error answers 504 or 499 rather than 500: the request ran out of time or the client
// left, which a server's error rate should not count.
func Error(err error) (int, string) {
	code, message := errcode.ForTransport(err)
	return StatusMessage(code, message)
}

// statusText is http.StatusText, with the text for 499, which net/http does not know.
func statusText(status int) string {
	if status == StatusClientClosedRequest {
		return "Client Closed Request"
	}
	return http.StatusText(status)
}
