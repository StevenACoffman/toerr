package errors

import "reflect"

// AsBehavior finds the first error in err's tree that satisfies the behavior
// interface T, returning that value and whether one was found. It is the
// behavior-interface counterpart to AsType: where AsType matches by concrete
// type (T constrained to error), AsBehavior matches by the methods an error
// answers — the "ask the error a question" idiom, as in net.Error's Timeout.
//
// T must be an interface type, typically a small predicate such as
// interface{ Retryable() bool }. Because the interface need not embed error,
// AsType cannot express it; AsBehavior can. Like As, it matches values carried
// by Mark and traverses every branch of an errors.Join.
//
//	type retryable interface{ Retryable() bool }
//	if r, ok := errors.AsBehavior[retryable](err); ok && r.Retryable() {
//	    backoffAndRetry()
//	}
//
// Passing a non-interface T is a programming error, not a runtime condition: it
// can never match, so AsBehavior panics rather than silently returning false
// (the same failure the underlying As would raise, with a clearer message).
func AsBehavior[T any](err error) (T, bool) {
	var target T
	if reflect.TypeFor[T]().Kind() != reflect.Interface {
		panic(
			"errors.AsBehavior: T must be an interface type, got " + reflect.TypeFor[T]().String(),
		)
	}
	ok := As(err, &target)
	return target, ok
}
