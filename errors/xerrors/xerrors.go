// Package xerrors provides a generic implementation of error wrapping, allowing any data type to be captured alongside an error.
package xerrors

import (
	stderrors "errors"
)

// ExtendedError is a custom error type that contains additional data using generics.
type ExtendedError[T any] struct {
	Data T
	err  error
}

// Error meets the error interface by returning the Error() of the underlying.
func (e *ExtendedError[T]) Error() string {
	return e.err.Error()
}

// Unwrap returns the underlying wrapped error.
func (e *ExtendedError[T]) Unwrap() error {
	return e.err
}

// Extend creates an ExtendedError wrapping an original error with additional data.
func Extend[T any](data T, err error) error {
	if err == nil {
		return nil
	}
	return &ExtendedError[T]{Data: data, err: err}
}

// Extract returns the desired data if possible, even in cases of deeply nested wrapping.
// NOTE: If an error is extended multiple times with the same data type, only the first match is returned.
func Extract[T any](err error) (T, bool) {
	var extendedError *ExtendedError[T]
	ok := stderrors.As(err, &extendedError)
	return extendedError.Data, ok
}

// Unjoin returns the underlying stderrors if the error was joined with errors.Join.
func Unjoin(err error) []error {
	if err == nil {
		return nil
	}
	if joinedErrs, ok := err.(interface{ Unwrap() []error }); ok {
		return joinedErrs.Unwrap()
	}
	return []error{err}
}

//func printError(err error, s fmt.State, verb rune) {
//	var withTrace bool
//	switch verb {
//	case 'v':
//		if s.Flag('+') {
//			withTrace = true
//		}
//	}
//	str := ToString(err, withTrace)
//	_, _ = io.WriteString(s, str)
//}

// compile time error interface check
var _ error = &ExtendedError[string]{}
