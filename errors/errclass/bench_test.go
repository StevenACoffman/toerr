package errclass_test

import (
	"errors"
	"testing"

	"github.com/StevenACoffman/toerr/errors/errclass"
)

var benchClass errclass.Class

func BenchmarkGetClass(b *testing.B) {
	b.Run("single", func(b *testing.B) {
		err := errclass.WrapAs(errors.New("boom"), errclass.Transient)
		for b.Loop() {
			benchClass = errclass.GetClass(err)
		}
	})
	b.Run("join", func(b *testing.B) {
		err := errors.Join(
			errclass.WrapAs(errors.New("a"), errclass.Transient),
			errors.New("b"),
			errclass.WrapAs(errors.New("c"), errclass.Persistent),
		)
		for b.Loop() {
			benchClass = errclass.GetClass(err)
		}
	})
}
