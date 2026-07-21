package errcode_test

import (
	"errors"
	"fmt"

	"github.com/StevenACoffman/toerr/errors/errcode"
)

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
