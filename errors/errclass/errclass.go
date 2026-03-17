// Package errclass provides functions for simple error classification.
package errclass

import (
	"github.com/StevenACoffman/toerr/errors/xerrors"
)

// Class represents a type of error.
type Class int

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

// String implements stringer interface.
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
	return xerrors.Extend(class, err)
}

// GetClass extracts the Class from an error.
func GetClass(err error) Class {
	if err == nil {
		return Nil
	}

	maxClass := Nil
	joinedErrs := xerrors.Unjoin(err)
	for _, joinedErr := range joinedErrs {
		class, ok := xerrors.Extract[Class](joinedErr)
		if ok && class > maxClass {
			maxClass = class
		} else if !ok && maxClass < Unknown {
			maxClass = Unknown
		}
	}
	return maxClass
}
