package errors

import (
	"fmt"
	"log/slog"
	"runtime/debug"
)

// Recover converts the result of the builtin recover() into an error, or nil if
// there was no panic (recovered == nil). Call it inside a deferred function:
// recover() only works when called directly by the deferral, so the value is
// passed in rather than recovered here.
//
//	defer func() {
//	    if err := errors.Recover(recover()); err != nil {
//	        // err has a frame at this recovery site and a "stack" attr holding
//	        // the panic's stack; log, translate, or write a response.
//	    }
//	}()
//
// A recovered error is preserved as the cause, so errors.Is/As still reach it
// (e.g. errors.Is(err, http.ErrAbortHandler)); any other value becomes the
// message. The panic's stack is attached as a "stack" slog.Attr for the operator
// view rather than spliced into the message, so Error() stays a clean one-liner —
// consistent with recording a return trace, not a full stack, in the error value.
func Recover(recovered any) error {
	if recovered == nil {
		return nil
	}
	e := &annotatedError{
		pc:    callerPC(),
		attrs: []slog.Attr{slog.String("stack", string(debug.Stack()))},
	}
	switch v := recovered.(type) {
	case error:
		e.cause = v
	case string:
		e.msg = v
	default:
		e.msg = fmt.Sprint(v)
	}
	return e
}
