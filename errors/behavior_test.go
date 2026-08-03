package errors_test

import (
	stderrors "errors"
	"fmt"
	"strings"
	"testing"

	errors "github.com/StevenACoffman/toerr/errors"
)

// retryable is a behavior interface: it asks an error whether it can be retried
// and, if so, after how long. AsBehavior extracts the value so both can be read.
type retryable interface {
	Retryable() bool
	RetryAfter() int
}

type rateLimitError struct{ after int }

func (rateLimitError) Error() string     { return "rate limited" }
func (rateLimitError) Retryable() bool   { return true }
func (r rateLimitError) RetryAfter() int { return r.after }

func TestAsBehaviorFindsIntrinsicThroughWrapping(t *testing.T) {
	err := errors.Wrap(errors.WrapWithMessage(rateLimitError{after: 5}, "call upstream"))

	r, ok := errors.AsBehavior[retryable](err)
	assert(t, ok, "AsBehavior should find the behavior through wrapping")
	assert(t, r.Retryable(), "Retryable() should be true")
	equals(t, 5, r.RetryAfter()) // the matched value is returned, not just a bool
}

func TestAsBehaviorFindsMarkedForeignError(t *testing.T) {
	foreign := stderrors.New("dial tcp: connection reset")
	err := errors.Mark(foreign, rateLimitError{after: 2})

	r, ok := errors.AsBehavior[retryable](err)
	assert(t, ok, "AsBehavior should find a behavior attached with Mark")
	equals(t, 2, r.RetryAfter())
}

func TestAsBehaviorTraversesJoinBranch(t *testing.T) {
	err := errors.Join(errors.New("validation failed"), rateLimitError{after: 1})

	_, ok := errors.AsBehavior[retryable](err)
	assert(t, ok, "AsBehavior should traverse Join branches")
}

func TestAsBehaviorAbsent(t *testing.T) {
	_, ok := errors.AsBehavior[retryable](errors.New("boom"))
	assert(t, !ok, "AsBehavior should be false when no error answers the behavior")

	_, okNil := errors.AsBehavior[retryable](nil)
	assert(t, !okNil, "AsBehavior(nil) should be false")
}

func TestAsBehaviorNonInterfaceTypePanics(t *testing.T) {
	defer func() {
		r := recover()
		assert(t, r != nil, "AsBehavior[int] should panic on a non-interface T")
		msg, _ := r.(string)
		assert(t, strings.Contains(msg, "must be an interface"),
			fmt.Sprintf("panic should explain the misuse, got: %v", r))
	}()
	errors.AsBehavior[int](errors.New("x"))
}
