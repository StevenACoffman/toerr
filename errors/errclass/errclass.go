package errclass

import (
	"errors"

	"github.com/StevenACoffman/toerr/errors/internal/trace"
)

// These are the allowed error classifications.
// The values are arbitrary but provide a strict ordering,
// where the higher the value, the more severe the error.
// When determining the class of a joined error, the highest
// class is returned.
const (
	// Nil is the class of a nil error.
	Nil Class = -1
	// Unknown is the class of an error nothing classified.
	Unknown Class = 0

	// Transient means the same operation may succeed if tried again.
	Transient Class = 100
	// Persistent means trying again cannot help.
	Persistent Class = 110

	// Panic marks an error recovered from a panic. errors.Recover does not set it,
	// because the root errors package imports none of its subpackages; classify the
	// recovered error yourself:
	//
	//	errclass.WrapAs(errors.Recover(recover()), errclass.Panic)
	Panic Class = 900
)

// Compile-time check that errors.LogValue can describe a classified layer.
var _ trace.Describer = (*withClassError)(nil)

// Class represents a type of error.
type Class int

// withClassError carries a Class classification for an error. It is transparent to
// Error and Unwrap so it does not disturb message or identity matching.
type withClassError struct {
	cause error
	class Class
}

// String implements the fmt.Stringer interface.
func (c Class) String() string {
	switch c {
	case Nil:
		return "nil"
	case Panic:
		return "panic"
	case Transient:
		return "transient"
	case Persistent:
		return "persistent"
	default:
		return "unknown"
	}
}

// WrapAs extends an error with the given class data.
func WrapAs(err error, class Class) error {
	if err == nil {
		return nil
	}
	return &withClassError{cause: err, class: class}
}

// GetClass extracts the Class from an error. For a joined error it returns the
// highest class among its members; an unclassified non-nil error is Unknown.
func GetClass(err error) Class {
	if err == nil {
		return Nil
	}

	maxClass := Nil
	for _, joinedErr := range unjoin(err) {
		var wc *withClassError
		if errors.As(joinedErr, &wc) {
			if wc.class > maxClass {
				maxClass = wc.class
			}
		} else if maxClass < Unknown {
			maxClass = Unknown
		}
	}
	return maxClass
}

func (e *withClassError) Error() string { return e.cause.Error() }
func (e *withClassError) Unwrap() error { return e.cause }

// DescribeLayer gives errors.LogValue the class.
func (e *withClassError) DescribeLayer(trace.Token) trace.Layer {
	return trace.Layer{Class: e.class.String()}
}

// unjoin returns the members of a joined error, or the error itself if it is not
// a multi-error.
func unjoin(err error) []error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}
	return []error{err}
}
