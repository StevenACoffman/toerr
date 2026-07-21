// Package errcode attaches a transport-neutral status code (and optional
// user-facing message) to an error, so callers can react to a failure's category
// without the domain knowing about HTTP or gRPC. Map a code to a transport with an
// adapter such as errhttp. Message returns the client-safe text; unlike
// err.Error() it never exposes wrapped internal detail.
package errcode
