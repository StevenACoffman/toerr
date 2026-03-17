package stacktrace

import (
	"github.com/StevenACoffman/toerr/errors/xerrors"
)

const (
	// depth of stack to ignore so that callers of Wrap don't see the call to Wrap itself.
	wrapStackDepth = 3
)

// Wrap extends an error by including a stack trace at the point where this was called.
// If the error already contains a stack trace, it is NOT wrapped again.
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := xerrors.Extract[StackTrace](err); !ok {
		return xerrors.Extend(GetStack(wrapStackDepth, true), err)
	}
	return err
}

// Extract returns the StackTrace embedded in the error if it exists.
func Extract(err error) StackTrace {
	st, ok := xerrors.Extract[StackTrace](err)
	if !ok {
		return nil
	}
	return st
}
