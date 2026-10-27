// Package trace records and renders return traces, as %+v text and as the JSON tree
// errors.LogValue returns. It is shared by the errors package and its subpackages so
// that every error type in this module records its call site the same way and renders
// the same way. It is internal because the trace
// format is this module's design decision, not part of its API.
//
// An error contributes a frame by implementing the errtrace marker interface,
//
//	TracePC() uintptr
//
// so frames recorded here and by errtrace read each other.
package trace

import (
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

// tree is an error split at its joins: the chain of errors from err down to a join or
// the root cause, outermost first, and one tree per branch of that join. Both
// renderings read it: %+v for its frames, and LogValue for its layers.
type tree struct {
	err      error
	chain    []error // a join, when there is one, is its last error
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

// buildTree splits err into its tree, following each error's Unwrap, or a seal's
// hidden cause, and branching at an Unwrap() []error.
func buildTree(err error) tree {
	t := tree{err: err}
	for err != nil {
		t.chain = append(t.chain, err)
		if multi, ok := err.(interface{ Unwrap() []error }); ok {
			for _, e := range multi.Unwrap() {
				t.children = append(t.children, buildTree(e))
			}
			break
		}
		err = next(err)
	}
	return t
}

// next returns the error beneath err in its chain, following a seal's hidden cause.
func next(err error) (cause error) {
	switch x := err.(type) { //nolint:errorlint // deliberate single-node inspection.
	case interface{ Unwrap() error }:
		cause = x.Unwrap()
	case HiddenCause:
		cause = x.TraceCause(Token{})
	}
	return cause //nolint:wrapcheck // the next node of the walk, not an error for a caller.
}

// frames returns the return trace of a chain, origin-first: one frame for each error
// that recorded a call site, read through the errtrace marker so that frames recorded
// by either package appear.
func frames(chain []error) []runtime.Frame {
	var trace []runtime.Frame
	for _, err := range slices.Backward(chain) {
		if f, ok := frame(err); ok {
			trace = append(trace, f)
		}
	}
	return trace
}

// frame returns the call site err recorded, if it recorded one. Mark records a zero
// PC, which yields no frame.
func frame(err error) (runtime.Frame, bool) {
	e, ok := err.(interface{ TracePC() uintptr })
	if !ok || e.TracePC() == 0 {
		return runtime.Frame{}, false
	}
	f, _ := runtime.CallersFrames([]uintptr{e.TracePC()}).Next()
	return f, f != (runtime.Frame{})
}

// writeTree writes the tree depth-first: children (deepest branches) first, then
// the node's own message and trace. path indexes the branch nesting.
func writeTree(b *strings.Builder, t tree, path []int) {
	for i, child := range t.children {
		writeTree(b, child, append(path, i))
	}
	writeTrace(b, t.err, frames(t.chain), path)
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
