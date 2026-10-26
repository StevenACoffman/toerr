package errgrpc_test

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	toerr "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/errgrpc"
)

// A handler returns errgrpc.Error(err): the client sees the code and the user
// message, and never the wrapped detail.
func ExampleError() {
	err := toerr.Wrapf(
		errcode.New(errcode.StatusNotFound, "no such seat"),
		"reserve seat %d", 7,
	)
	s := status.Convert(errgrpc.Error(err))
	fmt.Println(s.Code(), s.Message())
	// Output: NotFound no such seat
}

// An error from a gRPC client call is translated where it enters your code, and
// sealed, so callers branch on the domain code and cannot match the gRPC status.
func ExampleFromCode() {
	remote := status.Error(codes.Unavailable, "connection refused")
	err := errcode.Link(errgrpc.FromCode(status.Code(remote)), "", toerr.Seal(remote))

	fmt.Println(errcode.Status(err))
	fmt.Println(status.Code(err))
	// Output:
	// unavailable
	// Unknown
}
