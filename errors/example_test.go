package errors_test

import (
	"fmt"
	"log/slog"
	"strings"
	"testing"

	errors "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/sentinel"
)

var errRateLimited = sentinel.New("rate limited")

type exampleRateLimitError struct{ error }

// errors.Is finds a target in any branch of a joined error.
func ExampleJoin() {
	err := errors.Join(errors.New("write cache"), errRateLimited)
	fmt.Println(errors.Is(err, errRateLimited))
	// Output: true
}

// AsType matches a type attached by Mark, anywhere up the chain.
func ExampleMark() {
	err := errors.Mark(errors.New("upstream 429"), &exampleRateLimitError{})
	_, ok := errors.AsType[*exampleRateLimitError](err)
	fmt.Println(ok)
	// Output: true
}

// Attrs collects every slog attribute along the chain, outermost first.
func ExampleAttrs() {
	err := errors.New("checkout", slog.Int("user_id", 7))
	err = errors.Wrap(err, slog.String("op", "pay"))
	for _, a := range errors.Attrs(err) {
		fmt.Printf("%s=%v\n", a.Key, a.Value.Any())
	}
	// Output:
	// op=pay
	// user_id=7
}

// TestJoinedTraceRendersTree pins the "%+v renders a joined error as a tree"
// claim structurally: the exact file/line frames vary by machine, so we assert on
// the branch connectors and both messages rather than the full output.
func TestJoinedTraceRendersTree(t *testing.T) {
	err := errors.Wrap(errors.Join(
		errors.New("connection refused"),
		errors.New("cache miss"),
	))
	out := fmt.Sprintf("%+v", err)
	for _, want := range []string{"connection refused", "cache miss", "+-", "|"} {
		if !strings.Contains(out, want) {
			t.Errorf("%%+v output missing %q\n%s", want, out)
		}
	}
}
