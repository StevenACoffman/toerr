package errbase

// UnwrapOnce accesses the direct cause of the error if any, otherwise
// returns nil.
//
// It supports both errors implementing causer (`Cause()` method, from
// github.com/pkg/errors) and `Wrapper` (`Unwrap()` method, from the
// Go 2 error proposal).
//
// UnwrapOnce treats multi-errors (those implementing the
// `Unwrap() []error` interface as leaf-nodes since they cannot
// reasonably be iterated through to a single cause. These errors
// are typically constructed as a result of `fmt.Errorf` which results
// in a `wrapErrors` instance that contains an interpolated error
// string along with a list of causes.
//
// The go stdlib does not define output on `Unwrap()` for a multi-cause
// error, so we default to nil here.
func UnwrapOnce(err error) (cause error) {
	switch e := err.(type) {
	case interface{ Cause() error }:
		return e.Cause()
	case interface{ Unwrap() error }:
		return e.Unwrap()
	}
	return nil
}

// UnwrapAll accesses the root cause object of the error.
// If the error has no cause (leaf error), it is returned directly.
// UnwrapAll treats multi-errors as leaf nodes.
func UnwrapAll(err error) error {
	for {
		if cause := UnwrapOnce(err); cause != nil {
			err = cause
			continue
		}
		break
	}
	return err
}

// UnwrapMulti access the slice of causes that an error contains, if it is a
// multi-error.
func UnwrapMulti(err error) []error {
	if me, ok := err.(interface{ Unwrap() []error }); ok {
		return me.Unwrap()
	}
	return nil
}
