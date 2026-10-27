package errors_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"testing"

	errors "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/sentinel"
)

var (
	errRateLimited  = sentinel.New("rate limited")
	errUserNotFound = sentinel.New("user not found")
)

type exampleRateLimitError struct{ error }

// errors.Is finds a target in any branch of a joined error.
func ExampleJoin() {
	err := errors.Join(errors.New("write cache"), errRateLimited)
	fmt.Println(errors.Is(err, errRateLimited))
	// Output: true
}

// Wrapf says what was being attempted, in the same breath as fmt.Errorf, and also
// records where.
func ExampleWrapf() {
	err := errors.Wrapf(fs.ErrNotExist, "load config %q", "/etc/app.yaml")
	fmt.Println(err)
	fmt.Println(errors.Is(err, fs.ErrNotExist))
	// Output:
	// load config "/etc/app.yaml": file does not exist
	// true
}

// AsType matches a type attached by Mark, anywhere up the chain.
func ExampleMark() {
	err := errors.Mark(errors.New("upstream 429"), &exampleRateLimitError{})
	_, ok := errors.AsType[*exampleRateLimitError](err)
	fmt.Println(ok)
	// Output: true
}

// Seal turns a dependency's error into a domain sentinel: callers match the mark
// outside the seal and nothing inside it, while the message keeps the detail.
func ExampleSeal() {
	dependency := fmt.Errorf("open users.db: %w", fs.ErrNotExist)
	err := errors.Mark(errors.Seal(dependency), errUserNotFound)

	fmt.Println(errors.Is(err, errUserNotFound))
	fmt.Println(errors.Is(err, fs.ErrNotExist))
	fmt.Println(err)
	// Output:
	// true
	// false
	// open users.db: file does not exist
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

// LogValue keeps each error's attributes in its own layer, so siblings that set the
// same key do not collide.
func ExampleLogValue() {
	err := errors.WrapWithMessage(errors.Join(
		errors.New("b failed", slog.String("op", "b")),
		errors.WrapWithMessage(errors.New("d failed", slog.String("op", "d")),
			"c failed", slog.String("op", "c")),
	), "a failed", slog.String("op", "a"))

	data, _ := json.Marshal(errors.LogValue(err).Any())
	type layer struct {
		Attrs struct {
			Op string `json:"op"`
		} `json:"attrs"`
	}
	type branch struct {
		Chain  []layer  `json:"chain"`
		Errors []branch `json:"errors"`
	}
	var tree branch
	_ = json.Unmarshal(data, &tree)

	fmt.Println(tree.Chain[0].Attrs.Op)
	for _, sibling := range tree.Errors {
		var ops []string
		for _, l := range sibling.Chain {
			ops = append(ops, l.Attrs.Op)
		}
		fmt.Println(strings.Join(ops, " "))
	}
	// Output:
	// a
	// b
	// c d
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
