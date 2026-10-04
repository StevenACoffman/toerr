package errcode_test

import (
	"errors"
	"testing"

	"github.com/StevenACoffman/toerr/errors/errcode"
)

var (
	errSink   error
	benchCode errcode.StatusCode
	benchMsg  string
)

func BenchmarkWithCode(b *testing.B) {
	cause := errors.New("boom")
	for b.Loop() {
		errSink = errcode.WithCode(errcode.StatusNotFound, "not found", cause)
	}
}

// WithCodeOpaque copies the cause's text into a fresh error, so it allocates more than
// WithCode; the gap is the price of severing the chain.
func BenchmarkWithCodeOpaque(b *testing.B) {
	cause := errors.New("boom")
	for b.Loop() {
		errSink = errcode.WithCodeOpaque(errcode.StatusNotFound, "not found", cause)
	}
}

func BenchmarkCode(b *testing.B) {
	err := errcode.WithCode(errcode.StatusNotFound, "not found", errors.New("boom"))
	for b.Loop() {
		benchCode, benchMsg = errcode.Code(err)
	}
}

func BenchmarkError(b *testing.B) {
	err := errcode.WithCode(errcode.StatusNotFound, "not found", errors.New("boom"))
	for b.Loop() {
		benchMsg = err.Error()
	}
}
