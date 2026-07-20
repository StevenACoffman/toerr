package errors_test

import (
	"bytes"
	stderrors "errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"testing"

	errors "github.com/StevenACoffman/toerr/errors"
)

// fakeTraceError mimics a braces.dev/errtrace *errTrace: it exposes the exported
// TracePC marker and Unwrap. It stands in for a real errtrace error so the
// interop test needs no external dependency.
type fakeTraceError struct {
	err error
	pc  uintptr
}

type notFoundError struct{ msg string }

func (e *fakeTraceError) Error() string    { return e.err.Error() }
func (e *fakeTraceError) Unwrap() error    { return e.err }
func (e *fakeTraceError) TracePC() uintptr { return e.pc }

func (n *notFoundError) Error() string { return n.msg }

//go:noinline
func errtraceStyleWrap(err error) error {
	var pcs [1]uintptr
	runtime.Callers(1, pcs[:]) // this function's own frame
	return &fakeTraceError{err: err, pc: pcs[0]}
}

func interopOrigin() error { return errors.New("boom") }

// TestInteropWithErrtraceMarker verifies that an error implementing errtrace's
// exported TracePC marker contributes a frame to this package's return trace.
func TestInteropWithErrtraceMarker(t *testing.T) {
	err := errors.Wrap(errtraceStyleWrap(interopOrigin()))
	trace := fmt.Sprintf("%+v", err)
	for _, want := range []string{"interopOrigin", "errtraceStyleWrap"} {
		assert(t, strings.Contains(trace, want),
			fmt.Sprintf("foreign errtrace frame %q should appear in the trace:\n%s", want, trace))
	}
}

func TestNewRecordsMessageAndAttrs(t *testing.T) {
	err := errors.New("boom", slog.String("k", "v"))
	equals(t, "boom", err.Error())

	attrs := errors.Attrs(err)
	equals(t, 1, len(attrs))
	equals(t, "k", attrs[0].Key)
	equals(t, "v", attrs[0].Value.String())
}

func TestWrapPreservesChainAndCollectsAttrs(t *testing.T) {
	leaf := errors.New("leaf", slog.Int("id", 1))
	wrapped := errors.Wrap(leaf, slog.String("op", "read"))

	assert(t, errors.Is(wrapped, leaf), "Is(wrapped, leaf) should be true")

	// Attrs are collected outermost-first: the wrap's attr precedes the leaf's.
	attrs := errors.Attrs(wrapped)
	equals(t, 2, len(attrs))
	equals(t, "op", attrs[0].Key)
	equals(t, "id", attrs[1].Key)
}

func TestFormatVerbs(t *testing.T) {
	err := errors.WrapWithMessage(errors.New("root"), "context")

	equals(t, "context: root", fmt.Sprintf("%v", err))
	equals(t, "context: root", fmt.Sprintf("%s", err))
	equals(t, `"context: root"`, fmt.Sprintf("%q", err))

	// %+v adds the trace below the message.
	verbose := fmt.Sprintf("%+v", err)
	assert(
		t,
		strings.HasPrefix(verbose, "context: root\n"),
		"%+v should start with the message, got: "+verbose,
	)
	assert(t, countFrames(verbose) > 0, "%+v should include at least one frame")
}

func TestLogValueCarriesMessageAndAttrs(t *testing.T) {
	err := errors.New("disk full", slog.String("device", "sda1"))

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		// Drop time so the output is stable.
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))
	logger.InfoContext(t.Context(), "operation failed", slog.Any("err", err))

	out := buf.String()
	assert(t, strings.Contains(out, "disk full"), "log should carry the error message, got: "+out)
	assert(t, strings.Contains(out, "sda1"), "log should carry the error attr, got: "+out)
}

func TestNilInputsReturnNil(t *testing.T) {
	assert(t, errors.Wrap(nil) == nil, "Wrap(nil) should be nil")
	assert(t, errors.WrapWithMessage(nil, "m") == nil, "WrapWithMessage(nil, ...) should be nil")
	assert(t, errors.Mark(nil, errors.New("m")) == nil, "Mark(nil, ...) should be nil")
}

func TestWrapWithMessage(t *testing.T) {
	leaf := errors.New("leaf", slog.Int("id", 1))
	wrapped := errors.WrapWithMessage(leaf, "reading config", slog.String("op", "read"))

	equals(t, "reading config: leaf", wrapped.Error())
	assert(t, errors.Is(wrapped, leaf), "Is(wrapped, leaf) should be true")
	equals(t, 2, len(errors.Attrs(wrapped)))
}

func TestJoinRendersEveryBranch(t *testing.T) {
	joined := errors.Join(errors.New("first branch"), errors.New("second branch"))
	trace := fmt.Sprintf("%+v", errors.Wrap(joined))

	assert(
		t,
		strings.Contains(trace, "first branch"),
		"trace should include first branch, got: "+trace,
	)
	assert(
		t,
		strings.Contains(trace, "second branch"),
		"trace should include second branch, got: "+trace,
	)
	// Each branch is a leaf, so each contributes its own frames after the join.
	assert(
		t,
		countFrames(trace) > 2,
		fmt.Sprintf("both branches should contribute frames, got %d", countFrames(trace)),
	)
}

func TestAsType(t *testing.T) {
	external := stderrors.New("sql: no rows")
	cases := map[string]struct {
		err  error
		want bool
	}{
		"foreign error marked":   {errors.Mark(external, &notFoundError{"x"}), true},
		"library error marked":   {errors.Mark(errors.New("lib"), &notFoundError{"x"}), true},
		"unmarked library error": {errors.New("plain"), false},
		"nil error":              {nil, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, ok := errors.AsType[*notFoundError](tc.err)
			equals(t, tc.want, ok)
		})
	}
}

func TestMarkIsTransparent(t *testing.T) {
	external := stderrors.New("sql: no rows")
	marked := errors.Mark(external, &notFoundError{"missing"})

	// Message and identity of the cause survive the mark.
	equals(t, "sql: no rows", marked.Error())
	assert(t, errors.Is(marked, external), "Is(marked, external) should be true")

	// A foreign error gains a trace by being wrapped before marking.
	trace := fmt.Sprintf("%+v", marked)
	assert(t, strings.Contains(trace, "errors_test.go"),
		"%+v should include a trace mentioning errors_test.go, got: "+trace)
}

// countFrames returns the number of file:line lines in a %+v trace. Each frame
// renders its (absolute) file path indented by a single tab, so "\t/" occurs
// exactly once per frame.
func countFrames(trace string) int {
	return strings.Count(trace, "\t/")
}

// A chain where the origin and each wrap live in a distinct function, so the
// return trace records one frame per hop.
func retOrigin() error { return errors.New("boom") }
func retMid() error    { return errors.Wrap(retOrigin()) }
func retOuter() error  { return errors.Wrap(retMid()) }

func TestReturnTraceIsOneFramePerHop(t *testing.T) {
	trace := fmt.Sprintf("%+v", retOuter())

	// The message, then exactly one frame per New/Wrap. No full stack: the
	// runtime tail is never captured.
	assert(t, strings.HasPrefix(trace, "boom\n"),
		"trace should start with the message:\n"+trace)
	assert(t, !strings.Contains(trace, "runtime.goexit"),
		"trace should not capture the runtime stack:\n"+trace)
	equals(t, 3, countFrames(trace)) // origin + two wraps
}

func TestReturnTraceOriginFirst(t *testing.T) {
	// Frames appear origin-first: the New site before the outer Wrap site.
	trace := fmt.Sprintf("%+v", retOuter())
	origin := strings.Index(trace, "retOrigin")
	outer := strings.Index(trace, "retOuter")
	assert(t, origin >= 0 && outer >= 0, "both frames should appear:\n"+trace)
	assert(t, origin < outer, "origin frame should precede the outer wrap:\n"+trace)
}

// assert fails the test with msg when condition is false.
func assert(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Fatal(msg)
	}
}

// equals fails the test unless exp and act are equal.
func equals(t *testing.T, exp, act any) {
	t.Helper()
	if exp != act {
		t.Fatalf("expected %v, got %v", exp, act)
	}
}
