package errhttp_test

import (
	"errors"
	"testing"

	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/errhttp"
)

var (
	benchStatus int
	benchMsg    string
)

func BenchmarkError(b *testing.B) {
	err := errcode.WithCode(errcode.StatusNotFound, "", errors.New("boom"))
	for b.Loop() {
		benchStatus, benchMsg = errhttp.Error(err)
	}
}
