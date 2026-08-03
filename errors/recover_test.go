package errors_test

import (
	stderrors "errors"
	"fmt"
	"strings"
	"testing"

	errors "github.com/StevenACoffman/toerr/errors"
)

func TestRecoverNilReturnsNil(t *testing.T) {
	assert(t, errors.Recover(nil) == nil, "Recover(nil) should be nil")
}

func TestRecoverPreservesError(t *testing.T) {
	orig := stderrors.New("orig failure")
	err := errors.Recover(orig)

	assert(t, err != nil, "Recover(error) should not be nil")
	assert(t, errors.Is(err, orig), "Recover should preserve the recovered error for errors.Is")
	equals(t, "orig failure", err.Error())
}

func TestRecoverStringAndValue(t *testing.T) {
	equals(t, "boom", errors.Recover("boom").Error())
	equals(t, "42", errors.Recover(42).Error())
}

func TestRecoverAttachesStackAttr(t *testing.T) {
	var stack string
	var found bool
	for _, a := range errors.Attrs(errors.Recover("boom")) {
		if a.Key == "stack" {
			found = true
			stack = a.Value.String()
		}
	}
	assert(t, found, `Recover should attach a "stack" attr`)
	assert(t, strings.Contains(stack, "goroutine "),
		"stack attr should contain a goroutine stack, got: "+stack)
}

// TestRecoverInDeferredFlow exercises the real usage: a panic recovered inside a
// deferred function. The resulting error keeps the panic message and records a
// frame at the recovery site.
func TestRecoverInDeferredFlow(t *testing.T) {
	err := panicThenRecover()

	assert(t, err != nil, "recovered panic should yield an error")
	equals(t, "kaboom", err.Error())

	trace := fmt.Sprintf("%+v", err)
	assert(t, countFrames(trace) > 0, "recovered error should carry at least one frame:\n"+trace)
	assert(t, strings.Contains(trace, "panicThenRecover"),
		"trace should point at the recovery site:\n"+trace)
}

func panicThenRecover() (err error) {
	defer func() {
		err = errors.Recover(recover())
	}()
	panic("kaboom")
}
