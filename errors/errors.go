package errors

import (
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/StevenACoffman/toerr/errors/internal/trace"
)

// Compile-time checks. TracePC is the errtrace-compatible trace marker.
var (
	_ error                          = (*annotatedError)(nil)
	_ fmt.Formatter                  = (*annotatedError)(nil)
	_ interface{ TracePC() uintptr } = (*annotatedError)(nil)

	_ interface{ Attrs() []slog.Attr } = (*annotatedError)(nil)
	_ trace.Describer                  = (*annotatedError)(nil)

	_ error                          = (*marked)(nil)
	_ fmt.Formatter                  = (*marked)(nil)
	_ interface{ TracePC() uintptr } = (*marked)(nil)
	_ trace.Describer                = (*marked)(nil)
)

// annotatedError is both the leaf (New) and the wrapper (Wrap): a leaf sets msg,
// a wrapper sets cause. Both record pc, the single call site where they were
// created.
type annotatedError struct {
	msg   string // set only by New
	cause error  // set only by Wrap
	pc    uintptr
	attrs []slog.Attr
}

// marked tags an error with a marker that Is and AsType can recognize, without
// placing the marker in the Unwrap chain (so it stays transparent).
type marked struct {
	cause  error
	marker error
}

// New creates a leaf error, recording this call site and attrs.
func New(msg string, attrs ...slog.Attr) error {
	return &annotatedError{msg: msg, pc: trace.CallerPC(), attrs: attrs}
}

// Wrap annotates err with this call site and attrs. Returns nil if err is nil.
func Wrap(err error, attrs ...slog.Attr) error {
	if err == nil {
		return nil
	}
	return &annotatedError{cause: err, pc: trace.CallerPC(), attrs: attrs}
}

// WrapWithMessage is Wrap with an additional message prepended to err's message
// as "message: cause". Returns nil if err is nil.
func WrapWithMessage(err error, message string, attrs ...slog.Attr) error {
	if err == nil {
		return nil
	}
	return &annotatedError{msg: message, cause: err, pc: trace.CallerPC(), attrs: attrs}
}

// Wrapf is WrapWithMessage with the message formatted by fmt.Sprintf, so a wrap that
// says what was being attempted is as short to write as fmt.Errorf:
//
//	return errors.Wrapf(err, "load config %q", path)
//
// It takes no attributes, because the variadic arguments belong to the format; use
// WrapWithMessage to attach both. Do not use %w: err is already the cause. go vet
// reports a %w in the format, as it does for any Sprintf wrapper. Returns nil if err
// is nil.
func Wrapf(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	return &annotatedError{msg: fmt.Sprintf(format, args...), cause: err, pc: trace.CallerPC()}
}

// Mark tags err with marker so that Is(err, marker) is true and AsType[T](err)
// is true for marker's type T. The marker joins neither the message nor the
// Unwrap chain: Unwrap still reaches err, so the mark is transparent.
//
// Mark is for errors whose type you do not control — a dependency's sentinel or
// an untyped error — and either kind of marker works: a sentinel for identity
// (Is), or a typed value for category and fields (AsType, AsBehavior). An error
// type you own does not need it; give the type an Unwrap or Is method instead.
//
// If err was not produced by this package, it is Wrapped first so it still
// carries a trace frame.
func Mark(err error, marker error) error {
	if err == nil {
		return nil
	}
	if !hasTrace(err) {
		err = &annotatedError{cause: err, pc: trace.CallerPC()} // foreign error: give it a frame.
	}
	return &marked{cause: err, marker: marker}
}

// AsType finds the first error in err's tree matching type T, returning it and
// whether one was found. It matches types carried by Mark as well as those in
// the natural chain. It is a re-export of the stdlib errors.AsType (Go 1.26).
func AsType[T error](err error) (T, bool) {
	return stderrors.AsType[T](err)
}

// Attrs collects every slog attribute attached along err's tree, outermost
// first, descending into every branch of a multi-error (errors.Join) in order.
//
// Any error contributes by implementing
//
//	Attrs() []slog.Attr
//
// the method counterpart to this function, as Unwrap, Is, and As are to theirs.
// The errors this package creates implement it; an error type declared elsewhere
// can too, surfacing its own fields without being wrapped to copy them.
func Attrs(err error) []slog.Attr {
	var attrs []slog.Attr
	walkTree(err, func(e error) {
		if a, ok := e.(interface{ Attrs() []slog.Attr }); ok {
			attrs = append(attrs, a.Attrs()...)
		}
	})
	return attrs
}

// walkTree calls visit on err and every error beneath it, depth-first and
// pre-order, following both the single (Unwrap() error) and multi (Unwrap()
// []error) shapes.
func walkTree(err error, visit func(error)) {
	for err != nil {
		visit(err)
		switch x := err.(type) { //nolint:errorlint // deliberate single-node inspection.
		case interface{ Unwrap() error }:
			err = x.Unwrap()
		case interface{ Unwrap() []error }:
			for _, e := range x.Unwrap() {
				walkTree(e, visit)
			}
			return
		default:
			return
		}
	}
}

// Is reports whether any error in err's chain matches target (see errors.Is).
func Is(err, target error) bool { return stderrors.Is(err, target) }

// As finds the first error in err's chain matching target (see errors.As).
func As(err error, target any) bool { return stderrors.As(err, target) }

// Unwrap returns the result of calling Unwrap on err (see errors.Unwrap).
func Unwrap(err error) error { return stderrors.Unwrap(err) }

// Join wraps the given errors into a single multi-error (see errors.Join).
func Join(errs ...error) error { return stderrors.Join(errs...) }

func (e *annotatedError) Error() string {
	switch {
	case e.cause == nil:
		return e.msg
	case e.msg == "":
		return e.cause.Error()
	default:
		return e.msg + ": " + e.cause.Error()
	}
}

func (e *annotatedError) Unwrap() error { return e.cause }

// TracePC returns the program counter of this error's call site. It is the
// errtrace marker interface, so this error contributes a frame to both this
// package's and errtrace's return traces.
func (e *annotatedError) TracePC() uintptr { return e.pc }

func (e *annotatedError) Format(s fmt.State, verb rune) { formatError(e, s, verb) }

// Attrs returns the slog attributes recorded at this error's call site; the
// package-level Attrs collects them along the whole chain.
func (e *annotatedError) Attrs() []slog.Attr { return e.attrs }

// DescribeLayer gives LogValue this error's own message and attributes.
func (e *annotatedError) DescribeLayer(trace.Token) trace.Layer {
	return trace.Layer{Msg: e.msg, Attrs: e.attrs}
}

func (m *marked) Error() string { return m.cause.Error() }

func (m *marked) Unwrap() error { return m.cause }

// TracePC returns 0: Mark is transparent and contributes no frame of its own to
// the trace (the wrap it creates around a foreign error carries the location).
// A zero PC yields no frame, so the trace walk skips it.
func (m *marked) TracePC() uintptr { return 0 }

func (m *marked) Format(s fmt.State, verb rune) { formatError(m, s, verb) }

// DescribeLayer gives LogValue the marker's text, or its type when the text says
// nothing: a typed marker such as &RateLimitError{} often embeds a nil error, and its
// Error method then panics or returns "".
func (m *marked) DescribeLayer(trace.Token) trace.Layer {
	return trace.Layer{Mark: markerText(m.marker)}
}

// Is reports the marker's identity to errors.Is, so a sentinel marker matches.
func (m *marked) Is(target error) bool { return Is(m.marker, target) }

// As reports the marker's type to errors.As, so AsType matches it.
func (m *marked) As(target any) bool { return As(m.marker, target) }

// hasTrace reports whether the chain already carries a trace frame — from this
// package or from errtrace (both use the TracePC marker).
func hasTrace(err error) bool {
	for e := err; e != nil; e = stderrors.Unwrap(e) {
		if _, ok := e.(interface{ TracePC() uintptr }); ok {
			return true
		}
	}
	return false
}

// formatError implements fmt.Formatter for the package error types.
// %+v prints the return trace; %v and %s print the message; %q quotes it.
func formatError(err error, s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = io.WriteString(s, trace.Format(err))
			return
		}
		_, _ = io.WriteString(s, err.Error())
	case 's':
		_, _ = io.WriteString(s, err.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", err.Error())
	}
}

// LogValue renders err for a structured log line, as a JSON object that keeps the
// shape of its tree. A JSON handler nests it under the attribute's key:
//
//	logger.LogAttrs(ctx, slog.LevelError, "request failed", slog.Any("error", errors.LogValue(err)))
//
// For errors.Wrap(errors.Join(b, c), …) with c wrapping d, the object is
//
//	{"msg": "<err.Error()>",
//	 "chain": [{"msg": "a failed", "at": "/src/svc.go:40", "attrs": {"op": "a"}}],
//	 "errors": [
//	   {"chain": [{"msg": "b failed", "at": "/src/db.go:12", "attrs": {"op": "b"}}]},
//	   {"chain": [{"msg": "c failed", "at": "/src/cache.go:20", "attrs": {"op": "c"}},
//	              {"msg": "d failed", "at": "/src/cache.go:31", "attrs": {"op": "d"}}]}]}
//
// A chain lists every error from the outside in, down to a join or the root cause, and
// each branch of the join gets a tree of its own under "errors". The join is the
// chain's last layer only when it reports something of its own, such as attributes.
// The walk is the one %+v takes, so the "at" fields are the return trace.
//
// Each layer keeps its own message, call site, and attributes, so no key collides with
// another error's, however many siblings set it. A layer also carries "code" and
// "class" from errcode and errclass, "mark" from Mark, and "sealed" from Seal, whose
// cause it still shows. A layer from another package's error also carries its Go
// "type", such as "*pq.Error"; toerr's own layers do not. Empty fields are left out.
// Within one layer a repeated key keeps its first position and takes its last value.
//
// Attribute values are resolved, so a slog.LogValuer that redacts still redacts, and
// are encoded as slog's JSON handler encodes them. An error from another package is
// described by its Error, Unwrap, and Attrs methods.
//
// The value is neither an error nor a slog.LogValuer, so handlers encode it with its
// MarshalJSON; a text handler writes the same JSON as a string. A ReplaceAttr function
// sees it as one value and cannot reach inside it. For the flat list of attributes,
// use Attrs.
//
// These errors deliberately do not implement slog.LogValuer, so passing one to a
// logger does not expand it at every layer; log it once, where it is handled. A nil
// err gives the zero Value.
func LogValue(err error) slog.Value {
	if err == nil {
		return slog.Value{}
	}
	return slog.AnyValue(trace.NewJSONTree(err))
}

// markerText returns marker's Error text, or its type name if Error panics or returns "".
func markerText(marker error) string {
	if text, ok := trace.ErrorText(marker); ok && text != "" {
		return text
	}
	return fmt.Sprintf("%T", marker)
}
