package errcode_test

import (
	"errors"
	"testing"

	toerr "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/errcode"
)

var (
	errSink   error
	benchCode errcode.StatusCode
	benchMsg  string
)

func BenchmarkNew(b *testing.B) {
	for b.Loop() {
		errSink = errcode.New(errcode.StatusNotFound, "not found")
	}
}

func BenchmarkLink(b *testing.B) {
	cause := errors.New("boom")
	for b.Loop() {
		errSink = errcode.Link(errcode.StatusNotFound, "not found", cause)
	}
}

// Sealing the cause adds one small allocation and no frame to Link.
func BenchmarkLinkSealed(b *testing.B) {
	cause := errors.New("boom")
	for b.Loop() {
		errSink = errcode.Link(errcode.StatusNotFound, "not found", toerr.Seal(cause))
	}
}

func BenchmarkCode(b *testing.B) {
	err := errcode.Link(errcode.StatusNotFound, "not found", errors.New("boom"))
	for b.Loop() {
		benchCode, benchMsg = errcode.Code(err)
	}
}

func BenchmarkError(b *testing.B) {
	err := errcode.Link(errcode.StatusNotFound, "not found", errors.New("boom"))
	for b.Loop() {
		benchMsg = err.Error()
	}
}
