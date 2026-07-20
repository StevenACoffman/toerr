// Package errhttp maps transport-neutral errcode status codes to HTTP. It is the
// adapter layer: HTTP concerns live here, not in the domain errcode package.
package errhttp

import (
	"net/http"

	"github.com/StevenACoffman/toerr/errors/errcode"
)

// Status maps a domain status code to an HTTP status code. Unknown codes map to
// 500 (no logging — this is a pure mapping).
//
//nolint:cyclop // flat exhaustive value mapping; inherent, not branching logic.
func Status(code errcode.StatusCode) int {
	switch code {
	case errcode.StatusInternal, errcode.StatusUnknown:
		return http.StatusInternalServerError
	case errcode.StatusCanceled:
		return http.StatusRequestTimeout
	case errcode.StatusUnimplemented:
		return http.StatusNotImplemented
	case errcode.StatusNotFound:
		return http.StatusNotFound
	case errcode.StatusDeadlineExceeded:
		return http.StatusGatewayTimeout
	case errcode.StatusInvalidArgument, errcode.StatusFailedPrecondition:
		return http.StatusBadRequest
	case errcode.StatusAlreadyExists:
		return http.StatusConflict
	case errcode.StatusUnauthenticated:
		return http.StatusUnauthorized
	case errcode.StatusPermissionDenied:
		return http.StatusForbidden
	default:
		return http.StatusInternalServerError
	}
}

// StatusMessage returns the HTTP status for code and a message, defaulting the
// message to the standard HTTP status text when empty.
func StatusMessage(code errcode.StatusCode, message string) (int, string) {
	status := Status(code)
	if message == "" {
		message = http.StatusText(status)
	}
	return status, message
}

// Error maps the status code attached to err to an HTTP status and message.
// Use it at the transport boundary: status, msg := errhttp.Error(err).
func Error(err error) (int, string) {
	code, message := errcode.Code(err)
	return StatusMessage(code, message)
}
