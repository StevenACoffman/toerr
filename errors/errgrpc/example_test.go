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
// sealed, so callers branch on the domain code and cannot match the gRPC status. A
// downstream PermissionDenied is about your service's grants, not your caller's, so it
// becomes Internal; an outage passes through.
func ExampleFromError() {
	for _, remote := range []error{
		status.Error(codes.PermissionDenied, "service account lacks users.get"),
		status.Error(codes.Unavailable, "connection refused"),
	} {
		err := errcode.Link(errgrpc.FromError(remote), "", toerr.Seal(remote))
		fmt.Println(errcode.Status(err), status.Code(err))
	}
	// Output:
	// internal Unknown
	// unavailable Unknown
}

// FromCode is the raw table, for a call that forwarded the caller's input unchanged:
// a downstream InvalidArgument is then the caller's to fix.
func ExampleFromCode() {
	fmt.Println(errgrpc.FromCode(codes.InvalidArgument))
	// Output: invalid_argument
}
