package sentinel_test

import (
	"errors"
	"fmt"

	"github.com/StevenACoffman/toerr/errors/sentinel"
)

// ErrNotFound is matchable only because it is a package-level variable.
var ErrNotFound = sentinel.New("not found")

// A sentinel matches by identity even through wrapping, but a distinct value with
// the same text does not.
func ExampleNew() {
	err := fmt.Errorf("lookup user 42: %w", ErrNotFound)
	fmt.Println(errors.Is(err, ErrNotFound))
	fmt.Println(errors.Is(err, sentinel.New("not found")))
	// Output:
	// true
	// false
}
