// Package errors is a drop-in replacement for the standard errors package that
// records the call site of New/Wrap, carries slog attributes for structured
// logging, and supports transparent type marks for control flow.
//
// The return trace is built on the same exported marker interface as
// braces.dev/errtrace — any error with a TracePC() uintptr method contributes a
// frame — so the two packages interoperate: errtrace-wrapped errors appear in
// this package's %+v output, and these errors appear in errtrace.Format.
package errors

import (
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"slices"
	"strings"
)

// Compile-time checks. TracePC is the errtrace-compatible trace marker.
var (
	_ error                          = (*annotatedError)(nil)
	_ fmt.Formatter                  = (*annotatedError)(nil)
	_ slog.LogValuer                 = (*annotatedError)(nil)
	_ interface{ TracePC() uintptr } = (*annotatedError)(nil)

	_ error                          = (*marked)(nil)
	_ fmt.Formatter                  = (*marked)(nil)
	_ slog.LogValuer                 = (*marked)(nil)
	_ interface{ TracePC() uintptr } = (*marked)(nil)
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

// marked tags an error with a marker whose type AsType can recognize, without
// placing the marker in the Unwrap chain (so it stays transparent).
type marked struct {
	cause  error
	marker error
	attrs  []slog.Attr
}

// traceTree represents an error and its return trace as a tree. Children are the
// branches of a multi-error (errors.Join); a single-cause chain has none.
type traceTree struct {
	err      error
	trace    []runtime.Frame // origin-first
	children []traceTree
}

// New creates a leaf error, recording this call site and attrs.
func New(msg string, attrs ...slog.Attr) error {
	return &annotatedError{msg: msg, pc: callerPC(), attrs: attrs}
}

// Wrap annotates err with this call site and attrs. Returns nil if err is nil.
func Wrap(err error, attrs ...slog.Attr) error {
	if err == nil {
		return nil
	}
	return &annotatedError{cause: err, pc: callerPC(), attrs: attrs}
}

// WrapWithMessage is Wrap with an additional message prepended to err's message
// as "message: cause". Returns nil if err is nil.
func WrapWithMessage(err error, message string, attrs ...slog.Attr) error {
	if err == nil {
		return nil
	}
	return &annotatedError{msg: message, cause: err, pc: callerPC(), attrs: attrs}
}

// Mark tags err with marker so AsType[T](err) is true for marker's type T.
// If err was not produced by this package, it is Wrapped first so it still
// carries a trace frame.
func Mark(err error, marker error) error {
	if err == nil {
		return nil
	}
	if !hasTrace(err) {
		err = &annotatedError{cause: err, pc: callerPC()} // foreign error: give it a frame.
	}
	return &marked{cause: err, marker: marker}
}

// AsType finds the first error in err's tree matching type T, returning it and
// whether one was found. It matches types carried by Mark as well as those in
// the natural chain. It is a re-export of the stdlib errors.AsType (Go 1.26).
func AsType[T error](err error) (T, bool) {
	return stderrors.AsType[T](err)
}

// Attrs collects every slog attribute attached along the chain, outermost first.
func Attrs(err error) []slog.Attr {
	var attrs []slog.Attr
	for e := err; e != nil; e = stderrors.Unwrap(e) {
		if a, ok := e.(interface{ attributes() []slog.Attr }); ok {
			attrs = append(attrs, a.attributes()...)
		}
	}
	return attrs
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

func (e *annotatedError) LogValue() slog.Value { return logValue(e) }

func (e *annotatedError) attributes() []slog.Attr { return e.attrs }

func (m *marked) Error() string { return m.cause.Error() }

func (m *marked) Unwrap() error { return m.cause }

// TracePC returns 0: Mark is transparent and contributes no frame of its own to
// the trace (the wrap it creates around a foreign error carries the location).
// A zero PC yields no frame, so the trace walk skips it.
func (m *marked) TracePC() uintptr { return 0 }

func (m *marked) Format(s fmt.State, verb rune) { formatError(m, s, verb) }

func (m *marked) LogValue() slog.Value { return logValue(m) }

// As reports the marker's type to errors.As, so AsType matches it.
func (m *marked) As(target any) bool { return As(m.marker, target) }

func (m *marked) attributes() []slog.Attr { return m.attrs }

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

// callerPC records the PC of whoever called the exported New/Wrap/Mark.
// skip: 0=Callers, 1=callerPC, 2=New/Wrap/Mark, 3=the caller.
func callerPC() uintptr {
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) //nolint:mnd // skip Callers, callerPC, and the exported entry point.
	return pcs[0]
}

// formatError implements fmt.Formatter for the package error types.
// %+v prints the return trace; %v and %s print the message; %q quotes it.
func formatError(err error, s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			_, _ = io.WriteString(s, formatTrace(err))
			return
		}
		_, _ = io.WriteString(s, err.Error())
	case 's':
		_, _ = io.WriteString(s, err.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", err.Error())
	}
}

// logValue renders the error and every attribute in its chain for slog.
func logValue(err error) slog.Value {
	return slog.GroupValue(append([]slog.Attr{slog.String("msg", err.Error())}, Attrs(err)...)...)
}

func formatTrace(err error) string {
	var b strings.Builder
	writeTree(&b, buildTraceTree(err), nil)
	return b.String()
}

// buildTraceTree builds a trace tree from an error, following the single-cause
// chain and splitting into children at a multi-error. Mirrors errtrace's
// implementation.
func buildTraceTree(err error) traceTree {
	current := traceTree{err: err}
loop:
	for {
		if frame, ok, inner := unwrapFrame(err); ok {
			current.trace = append(current.trace, frame)
			err = inner
			continue
		}

		// Inspect this node's own Unwrap shape (single vs multi) to build the
		// tree; do not use errors.As, which would traverse the whole chain.
		switch x := err.(type) { //nolint:errorlint // deliberate single-node inspection.
		case interface{ Unwrap() error }:
			err = x.Unwrap()

		case interface{ Unwrap() []error }:
			errs := x.Unwrap()
			current.children = make([]traceTree, 0, len(errs))
			for _, e := range errs {
				current.children = append(current.children, buildTraceTree(e))
			}
			break loop

		default:
			break loop
		}
	}

	slices.Reverse(current.trace)
	return current
}

// unwrapFrame unwraps the outermost trace frame from err, returning the frame,
// whether one was present, and the inner error. Mirrors errtrace.UnwrapFrame so
// both packages read each other's frames.
func unwrapFrame(err error) (frame runtime.Frame, ok bool, inner error) {
	e, ok := err.(interface{ TracePC() uintptr })
	if !ok {
		return runtime.Frame{}, false, err
	}
	inner = stderrors.Unwrap(err)
	f, _ := runtime.CallersFrames([]uintptr{e.TracePC()}).Next()
	if f == (runtime.Frame{}) {
		return runtime.Frame{}, false, inner
	}
	return f, true, inner
}

// writeTree writes the tree depth-first: children (deepest branches) first, then
// the node's own message and trace. path indexes the branch nesting.
func writeTree(b *strings.Builder, t traceTree, path []int) {
	for i, child := range t.children {
		writeTree(b, child, append(path, i))
	}
	writeTrace(b, t.err, t.trace, path)
}

func writeTrace(b *strings.Builder, err error, trace []runtime.Frame, path []int) {
	// The message may have newlines in it, so print each line separately.
	for i, line := range strings.Split(err.Error(), "\n") {
		if i == 0 {
			pipes(b, path, "+- ")
		} else {
			pipes(b, path, "|  ")
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(trace) > 0 {
		// Empty line between the message and the trace.
		pipes(b, path, "|  ")
		b.WriteString("\n")

		for _, frame := range trace {
			pipes(b, path, "|  ")
			b.WriteString(frame.Function)
			b.WriteString("\n")

			pipes(b, path, "|  ")
			fmt.Fprintf(b, "\t%s:%d\n", frame.File, frame.Line)
		}
	}

	// Connecting "|" line between sibling traces.
	if len(path) > 0 {
		pipes(b, path, "|  ")
		b.WriteString("\n")
	}
}

// pipes draws the "| | |" gutter prefix. path leads to the current node; last is
// this node's own connector ("+- " or "|  "). A 0 index is a first child with
// nothing above to connect to, so its column is blank.
func pipes(b *strings.Builder, path []int, last string) {
	for depth, idx := range path {
		switch {
		case depth == len(path)-1:
			b.WriteString(last)
		case idx == 0:
			b.WriteString("   ")
		default:
			b.WriteString("|  ")
		}
	}
}
