// Package stacktrace uses the go runtime to capture stack trace data.
package stacktrace

import (
	"regexp"
	"runtime"
	"strings"
)

const (
	maxFrames     = 50
	runtimePrefix = "runtime."
	testingPrefix = "testing."
)

// match the filename of the go runtime package
// eg `/pkg/mod/golang.org/toolchain@v0.0.1-go1.22.4.linux-amd64/src/runtime/panic.go`
var runtimeRegex = regexp.MustCompile(`go[^/]*/src/runtime/[^.]+\.go`)

// match the filename of the go testing package
var testingRegex = regexp.MustCompile(`go[^/]*/src/testing/[^.]+\.go`)

// Frame represents human-readable information about a frame in a stack trace.
type Frame struct {
	File       string `json:"source"`
	LineNumber int    `json:"line"`
	Function   string `json:"func"`
}

// StackTrace represents a program stack trace as a series of frames.
type StackTrace []Frame

// GetStack captures the current program stack trace.
// skipFrames is the number of stack frames to skip, where 1 would result in GetStack itself being the first frame.
// skipRuntime when true ignores all frames that are part of the Go runtime (eg runtime.main and runtime.panic) and testing packages.
func GetStack(skipFrames int, skipRuntime bool) StackTrace {
	var stackTrace StackTrace

	pc := make([]uintptr, maxFrames)
	n := runtime.Callers(skipFrames, pc)
	pc = pc[:n]

	frames := runtime.CallersFrames(pc)
	for {
		frame, more := frames.Next()
		if !more {
			break
		}
		if skipRuntime {
			if strings.HasPrefix(frame.Function, runtimePrefix) && runtimeRegex.MatchString(frame.File) {
				continue
			} else if strings.HasPrefix(frame.Function, testingPrefix) && testingRegex.MatchString(frame.File) {
				continue
			}
		}
		stackTrace = append(stackTrace, Frame{
			File:       frame.File,
			LineNumber: frame.Line,
			Function:   frame.Function,
		})
	}

	return stackTrace
}
