package errors

import (
	"fmt"
	"io"

	"log/slog"
	"runtime"

	"github.com/StevenACoffman/toerr/errors/errtrace"
)

// AnnotatedError includes more context than a plain error that is useful for troubleshooting.
// This is generally a root error cause
// as it does not contain any other errors.
type AnnotatedError struct {
	// msg is the error message.
	msg string
	// pc is the program counter for the location of the error provided by runtime.Callers.
	pc uintptr
	// attrs are slog attributes that are added to the log event to provide more context for the error.
	attrs []slog.Attr
}

// New creates a new [AnnotatedError] with the given message and attributes.
func New(msg string, attrs ...slog.Attr) AnnotatedError {
	return newAnnotatedError(msg, attrs...)
}

func (e *AnnotatedError) Format(s fmt.State, verb rune) {
	if verb == 'v' && s.Flag('+') {
		fprintf(s, fmt.FormatString(s, verb), errtrace.FormatString(e))
	}

	fprintf(s, fmt.FormatString(s, verb), e.msg)
	fprintf(s, fmt.FormatString(s, verb), e.attrs)
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

// TracePC returns the program counter for the location
// in the frame that the error originated with.
//
// The returned PC is intended to be used with
// runtime.CallersFrames or runtime.FuncForPC
// to aid in generating the error return trace
func (e *AnnotatedError) TracePC() uintptr {
	return e.pc
}

// compile time tracePCprovider interface check
// this lets errtrace work on these
var _ interface{ TracePC() uintptr } = &AnnotatedError{}

// newAnnotatedError is a constructor that ensures that the program counter is set correctly.
//
// It must always be called directly by an exported function or method
// because it uses a fixed call depth to obtain the pc.
func newAnnotatedError(msg string, attrs ...slog.Attr) AnnotatedError {
	var pcs [1]uintptr
	runtime.Callers(3, pcs[:]) //nolint:mnd // Skip runtime.Callers, this function, and the calling function.

	return AnnotatedError{
		msg:   msg,
		pc:    pcs[0],
		attrs: attrs,
	}
}

// https://github.com/shogo82148/logrus-slog-hook/blob/fa80edaeb83a80050066d547940b06308e6656cc/hook.go#L117
// "github.com/sirupsen/logrus"
//func FieldToAttrs(f logrus.Fields) []slog.Attr {
//	sorter := h.newSorter()
//	keys := sorter.keys(f)
//
//	attrs := make([]slog.Attr, 0, len(f))
//	for _, k := range keys {
//		attrs = append(attrs, slog.Any(k, f[k]))
//	}
//	h.sorter.Put(sorter)
//	return attrs
//}

func (e *AnnotatedError) Error() string {
	return e.msg
}
