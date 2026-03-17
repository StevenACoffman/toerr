package errtrace

import (
	"fmt"
	"io"
	"log/slog"
	"runtime"
	"strings"

	stderrors "errors"

	"github.com/StevenACoffman/toerr/errors/errtrace/pc"
)

func AddReturnTrace(err error) error {
	if err == nil {
		return nil
	}
	return wrap(err, pc.GetCallerSkip1())
}

func AddReturnTraceWithDepth(err error, callerPC uintptr) error {
	if err == nil {
		return nil
	}
	return wrap(err, callerPC)
}

// New returns an error with the supplied text.
//
// It's equivalent to [errors.New] followed by [Wrap] to add caller information.
//
//go:noinline due to GetCaller (see [Wrap] for details).
func New(text string) error {
	return wrap(stderrors.New(text), pc.GetCaller())
}

// Errorf creates an error message
// according to a format specifier
// and returns the string as a value that satisfies error.
//
// It's equivalent to [fmt.Errorf] followed by [Wrap] to add caller information.
//
//go:noinline due to GetCaller (see [Wrap] for details).
func Errorf(format string, args ...any) error {
	return wrap(fmt.Errorf(format, args...), pc.GetCaller())
}

func wrap(err error, callerPC uintptr) error {
	if err == nil {
		return nil
	}
	return &errTrace{err: err, pc: callerPC}
}

type errTrace struct {
	err error
	pc  uintptr
}

// Format writes the return trace for given error to the writer.
// The output takes a format similar to the following:
//
//	<error message>
//
//	<function>
//		<file>:<line>
//	<caller of function>
//		<file>:<line>
//	[...]
//
// Any error that has a method `TracePC() uintptr` will
// contribute to the trace.
// If the error doesn't have a return trace attached to it,
// only the error message is reported.
// If the error consists of multiple errors (e.g. with [errors.Join]),
// the return trace of each error is reported as a tree.
//
// Returns an error if the writer fails.
func Format(w io.Writer, target error) (err error) {
	return writeTree(w, buildTraceTree(target))
}

// FormatString writes the return trace for err to a string.
// Any error that has a method `TracePC() uintptr` will
// contribute to the trace.
// See [Format] for details of the output format.
func FormatString(target error) string {
	var s strings.Builder
	_ = Format(&s, target)
	return s.String()
}

type StackFrame uintptr

func newStackFrame() StackFrame {
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) //nolint:mnd // Skip runtime.Callers, this function, and the calling function.

	return StackFrame(pcs[0])
}

func (e *errTrace) Error() string {
	return e.err.Error()
}

func (e *errTrace) Unwrap() error {
	return e.err
}

func (e *errTrace) Format(s fmt.State, verb rune) {
	if verb == 'v' && s.Flag('+') {
		_ = Format(s, e)
		return
	}

	fprintf(s, fmt.FormatString(s, verb), e.err)
}

// fprintf - convenience to quell the linters
// since errors are impossible, so we do
// not need return values
func fprintf(w io.Writer, format string, a ...any) {
	_, err := fmt.Fprintf(w, format, a...)
	if err != nil { // this cannot happen
		panic(err)
	}
}

// LogValue implements the [slog.LogValuer] interface.
func (e *errTrace) LogValue() slog.Value {
	return slog.StringValue(FormatString(e))
}

// TracePC returns the program counter for the location
// in the frame that the error originated with.
//
// The returned PC is intended to be used with
// runtime.CallersFrames or runtime.FuncForPC
// to aid in generating the error return trace
func (e *errTrace) TracePC() uintptr {
	return e.pc
}

// compile time tracePCprovider interface check
var _ interface{ TracePC() uintptr } = &errTrace{}

// compile time error interface check
var _ error = &errTrace{}
