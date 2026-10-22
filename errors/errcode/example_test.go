package errcode_test

import (
	"errors"
	"fmt"

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

// A single WithCode call carries a code, a user-facing message, and a cause
// together, and each consumer reads the view it needs.
func ExampleWithCode() {
	cause := errors.New("sql: no rows in result set")
	err := errcode.WithCode(errcode.StatusNotFound, "user not found", cause)

	fmt.Println(err)                  // operator view: cause (message)
	fmt.Println(errcode.Status(err))  // application view: the code
	fmt.Println(errcode.Message(err)) // end-user view: safe message
	// Output:
	// sql: no rows in result set (user not found)
	// not_found
	// user not found
}

// WithCodeOpaque severs the cause at a boundary: callers cannot couple to the
// dependency's error type, but the operator text and user message survive.
func ExampleWithCodeOpaque() {
	cause := errors.New("sql: no rows in result set")
	err := errcode.WithCodeOpaque(errcode.StatusNotFound, "user not found", cause)

	fmt.Println(errors.Is(err, cause)) // severed: dependency error is unreachable
	fmt.Println(err)                   // operator text still shows the cause
	fmt.Println(errcode.Message(err))  // clean user message
	// Output:
	// false
	// sql: no rows in result set (user not found)
	// user not found
}
