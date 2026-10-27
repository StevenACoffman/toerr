// Package trace records and renders return traces, as %+v text and as the JSON tree
// errors.LogValue returns. It is shared by the errors package and its subpackages so
// that every error type in this module records its call site the same way and renders
// the same way. It is internal because the trace format is this module's design
// decision, not part of its API.
//
// An error contributes a frame by implementing the errtrace marker interface,
//
//	TracePC() uintptr
//
// so frames recorded here and by errtrace read each other.
//
// logvalue.go renders the JSON tree. It is written by hand, not with encoding/json, so
// that each layer keeps its own keys in a fixed order and every value is encoded as
// slog's JSON handler encodes it.
package trace
