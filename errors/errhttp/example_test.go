package errhttp_test

import (
	"fmt"

	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/errhttp"
)

// Error maps the code attached to an error to an HTTP status and a client-safe
// message, defaulting to the standard status text when no message is set.
func ExampleError() {
	denied := errcode.WithCode(errcode.StatusPermissionDenied, "you cannot edit this", nil)
	status, msg := errhttp.Error(denied)
	fmt.Println(status, msg)

	missing := errcode.WithCode(errcode.StatusNotFound, "", nil)
	status, msg = errhttp.Error(missing)
	fmt.Println(status, msg)
	// Output:
	// 403 you cannot edit this
	// 404 Not Found
}
