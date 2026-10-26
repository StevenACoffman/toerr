// Package trace records and renders return traces. It is shared by the errors
// package and its subpackages so that every error type in this module records its
// call site the same way and prints the same %+v. It is internal because the trace
// format is this module's design decision, not part of its API.
//
// An error contributes a frame by implementing the errtrace marker interface,
//
//	TracePC() uintptr
//
// so frames recorded here and by errtrace read each other.
package trace

import (
	stderrors "errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
)

// Token is the argument type of HiddenCause. Code outside this module cannot name
// it, so it cannot write the HiddenCause interface and reach a hidden cause through
// it; only this module's renderer can. Reflection still can: the token keeps the
// cause out of the ordinary API, not out of deliberate reach.
type Token struct{}

// HiddenCause is implemented by an error that keeps a cause for the operator view
// while hiding it from Unwrap, and so from errors.Is and errors.As. The renderer
// follows TraceCause where Unwrap stops, so the frames beneath still print.
type HiddenCause interface {
	TraceCause(Token) error
}

// tree represents an error and its return trace as a tree. Children are the
// branches of a multi-error (errors.Join); a single-cause chain has none.
type tree struct {
	err      error
	trace    []runtime.Frame // origin-first
	children []tree
}

// CallerPC records the PC of whoever called the exported constructor that calls
// CallerPC. It must be called directly from that constructor.
// skip: 0=Callers, 1=CallerPC, 2=the constructor, 3=the constructor's caller.
func CallerPC() uintptr {
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) //nolint:mnd // skip Callers, CallerPC, and the constructor.
	return pcs[0]
}

// Format renders err's message followed by its return trace, origin-first. A
// multi-error renders each branch, deepest first, joined by a "|" gutter.
func Format(err error) string {
	var b strings.Builder
	writeTree(&b, buildTree(err), nil)
	return b.String()
}

// buildTree builds a trace tree from an error, following the single-cause chain
// and splitting into children at a multi-error. Mirrors errtrace's
// implementation.
func buildTree(err error) tree {
	current := tree{err: err}
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
			current.children = make([]tree, 0, len(errs))
			for _, e := range errs {
				current.children = append(current.children, buildTree(e))
			}
			break loop

		case HiddenCause:
			err = x.TraceCause(Token{})

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
func writeTree(b *strings.Builder, t tree, path []int) {
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
			_, _ = fmt.Fprintf(b, "\t%s:%d\n", frame.File, frame.Line)
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
