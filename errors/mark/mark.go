package mark

import (
	"github.com/StevenACoffman/toerr/errors/errbase"
)

// Mark creates an explicit mark for the given error, using
// the same mark as some reference error.
func Mark(err error, reference error) error {
	if err == nil {
		return nil
	}
	refMark := getMark(reference)
	return &withMark{cause: err, mark: refMark}
}

// withMark carries an explicit mark.
type withMark struct {
	cause error
	mark  error
}

var _ error = (*withMark)(nil)

// var _ fmt.Formatter = (*withMark)(nil)

func (m *withMark) Error() string { return m.cause.Error() }
func (m *withMark) Cause() error  { return m.cause }
func (m *withMark) Unwrap() error { return m.cause }

// getMark computes a marker for the given error.
func getMark(err error) error {
	if err == nil {
		return nil
	}
	if m, ok := err.(*withMark); ok {
		return m.mark
	}

	for c := errbase.UnwrapOnce(err); c != nil; c = errbase.UnwrapOnce(c) {
		if m, ok := err.(*withMark); ok {
			return m.mark
		}
		if me, ok := err.(interface{ Unwrap() []error }); ok {
			// accumulate marked multi-errors
			var merrs []error
			for _, innerErr := range me.Unwrap() {
				if getMark(innerErr) != nil {
					merrs = append(merrs, innerErr)
				}
			}
			if len(merrs) > 0 {
				// first mark wins
				return merrs[0]
			}
		}
	}

	return nil
}
