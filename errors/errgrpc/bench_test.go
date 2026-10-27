package errgrpc_test

import (
	"log/slog"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/errgrpc"
)

// Sinks keep the compiler from discarding the work under measurement.
var (
	errSink   error
	codeSink  errcode.StatusCode
	attrsSink []slog.Attr
)

// BenchmarkError is the price of a handler's last line: reading the code and message
// and building the status error.
func BenchmarkError(b *testing.B) {
	err := errcode.New(errcode.StatusNotFound, "no such seat")
	for b.Loop() {
		errSink = errgrpc.Error(err)
	}
}

// BenchmarkFromError is the price of translating a client error where it enters your
// code: two context checks, then the status code.
func BenchmarkFromError(b *testing.B) {
	err := status.Error(codes.Unavailable, "connection refused")
	for b.Loop() {
		codeSink = errgrpc.FromError(err)
	}
}

// BenchmarkAttrs is the price of a downstream status's fields for the log. Its
// details are unmarshalled on each call, which is most of the cost.
func BenchmarkAttrs(b *testing.B) {
	s, err := status.New(codes.ResourceExhausted, "quota").WithDetails(
		&errdetails.ErrorInfo{Reason: "RATE_LIMIT_EXCEEDED", Domain: "example.com"},
		&errdetails.RetryInfo{RetryDelay: durationpb.New(30 * time.Second)},
	)
	if err != nil {
		b.Fatalf("WithDetails: %v", err)
	}
	err = s.Err()
	for b.Loop() {
		attrsSink = errgrpc.Attrs(err)
	}
}
