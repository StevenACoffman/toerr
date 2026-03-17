package stacktrace_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/StevenACoffman/toerr/errors/stacktrace"
)

var errTest = fmt.Errorf("this is a test error")

func a() error {
	return stacktrace.Wrap(b())
}

func b() error {
	return stacktrace.Wrap(c())
}

func c() error {
	return stacktrace.Wrap(errTest)
}

// TestStackTrace checks that the stack trace is correct.
// WARNING: This test is extremely fragile if line numbers in this file change.
func TestStackTrace(t *testing.T) {
	t.Parallel()

	// wrapping nil error is still nil.
	err := stacktrace.Wrap(nil)
	if err != nil {
		t.Errorf("unexpected error: got %v", err)
	}

	// extracting stack trace from nil produces nil
	trace := stacktrace.Extract(err)
	if trace != nil {
		t.Errorf("expected nil stack trace: got %v", trace)
	}

	// wrapping non-nil error is non-nil
	err = a()
	if err == nil {
		t.Errorf("expected error: got %v", err)
	}

	// stack trace should not be nil in this case
	trace = stacktrace.Extract(err)
	if trace == nil {
		t.Errorf("expected stack trace: got %v", trace)
	}

	if len(trace) != 4 {
		t.Errorf("unexpected stack trace len: want: %d got %d.\n-----\n%v\n-----\n", 4, len(trace), trace)
	}

	expected := []stacktrace.Frame{
		{
			File:       "xerrors/stacktrace/error_test.go",
			LineNumber: 22,
			Function:   "xerrors/stacktrace_test.c",
		},
		{
			File:       "xerrors/stacktrace/error_test.go",
			LineNumber: 18,
			Function:   "xerrors/stacktrace_test.b",
		},
		{
			File:       "xerrors/stacktrace/error_test.go",
			LineNumber: 14,
			Function:   "xerrors/stacktrace_test.a",
		},
		{
			File:       "xerrors/stacktrace/error_test.go",
			LineNumber: 43,
			Function:   "xerrors/stacktrace_test.TestStackTrace",
		},
	}

	for i, frame := range trace {
		if !strings.HasSuffix(frame.File, expected[i].File) {
			t.Errorf("unexpected file name suffix: want: %s got %s", expected[i].File, frame.File)
		}
		if !strings.HasSuffix(frame.File, expected[i].File) {
			t.Errorf("unexpected function name suffix: want: %s got %s", expected[i].Function, frame.Function)
		}
		if frame.LineNumber != expected[i].LineNumber {
			t.Errorf("unexpected line number: want: %d got %d", expected[i].LineNumber, frame.LineNumber)
		}
	}
}
