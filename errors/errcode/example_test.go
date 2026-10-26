package errcode_test

import (
	"errors"
	"fmt"

	toerr "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/errcode"
)

// New is the coded leaf: the failure is understood here, so it gets a code and the
// message an end user may see, and no cause.
func ExampleNew() {
	err := errcode.New(errcode.StatusNotFound, "user not found")

	fmt.Println(err)                  // operator view: the message
	fmt.Println(errcode.Status(err))  // application view: the code
	fmt.Println(errcode.Message(err)) // end-user view: safe message
	// Output:
	// user not found
	// not_found
	// user not found
}

// A single Link call carries a code, a user-facing message, and a cause together,
// and each consumer reads the view it needs.
func ExampleLink() {
	cause := errors.New("disk full")
	err := errcode.Link(errcode.StatusInternal, "try again later", cause)

	fmt.Println(errors.Is(err, cause)) // the cause stays matchable
	fmt.Println(err)                   // operator view: cause (message)
	fmt.Println(errcode.Status(err))   // application view: the code
	fmt.Println(errcode.Message(err))  // end-user view: safe message
	// Output:
	// true
	// disk full (try again later)
	// internal
	// try again later
}

// Seal a dependency's error before linking a code to it: callers match the code,
// never the dependency, and operators still read the cause.
func ExampleLink_sealed() {
	cause := errors.New("sql: no rows in result set")
	err := errcode.Link(errcode.StatusNotFound, "user not found", toerr.Seal(cause))

	fmt.Println(errors.Is(err, cause)) // sealed: the dependency is unreachable
	fmt.Println(err)                   // operator text still shows the cause
	fmt.Println(errcode.Message(err))  // clean user message
	// Output:
	// false
	// sql: no rows in result set (user not found)
	// user not found
}
