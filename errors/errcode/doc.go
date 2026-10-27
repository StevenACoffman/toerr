// Package errcode attaches a transport-neutral status code (and optional
// user-facing message) to an error, so callers can react to a failure's category
// without the domain knowing about HTTP or gRPC. Map a code to a transport with an
// adapter such as errhttp. Message returns the client-safe text; unlike
// err.Error() it never exposes wrapped internal detail.
//
// New makes a coded leaf. Link attaches a code to an existing error and keeps it in
// the Unwrap chain, so callers can errors.Is/As into it; seal a dependency's error
// first (Link(code, message, errors.Seal(err))) so callers match only the code.
// Every constructor records its call site, so a coded error is a frame in the %+v
// return trace.
package errcode
