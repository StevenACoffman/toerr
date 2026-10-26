package errgrpc_test

import (
	"testing"

	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/errgrpc"
)

// errSink keeps the compiler from discarding the work under measurement.
var errSink error

// BenchmarkError is the price of a handler's last line: reading the code and message
// and building the status error.
func BenchmarkError(b *testing.B) {
	err := errcode.New(errcode.StatusNotFound, "no such seat")
	for b.Loop() {
		errSink = errgrpc.Error(err)
	}
}
