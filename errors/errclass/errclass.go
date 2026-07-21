package errclass

import "errors"

// These are the allowed error classifications.
// The values are arbitrary but provide a strict ordering,
// where the higher the value, the more severe the error.
// When determining the class of a joined error, the highest
// class is returned.
const (
	Nil     Class = -1
	Unknown Class = 0

	Transient  Class = 100
	Persistent Class = 110

	Panic Class = 900
)

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

// unjoin returns the members of a joined error, or the error itself if it is not
// a multi-error.
func unjoin(err error) []error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}
	return []error{err}
}
