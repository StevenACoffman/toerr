package errors

import (
	"fmt"
	"log/slog"

	"github.com/StevenACoffman/toerr/errors/internal/trace"
)

// Compile-time checks. A seal has no Unwrap method on purpose, and no TracePC: it is
// not a hop of its own.
var (
	_ error                            = (*sealed)(nil)
	_ fmt.Formatter                    = (*sealed)(nil)
	_ interface{ Attrs() []slog.Attr } = (*sealed)(nil)
	_ trace.HiddenCause                = (*sealed)(nil)
)

// sealed hides cause from matching and keeps it for the operator view.
type sealed struct {
	cause error
}

// Seal hides err from matching while keeping it visible to operators. Is, As,
// AsType, and AsBehavior stop at the seal, and so does everything that searches
// the chain the same way: an errcode code or message, an errclass class, and any
// Mark beneath it. The message, the %+v return trace, and Attrs still include
// everything beneath it. Returns nil if err is nil.
//
// Seal is the opposite of Mark. Mark adds a match without changing what operators
// see; Seal removes every match without changing it either. Seal a dependency's
// error where it enters your domain, so callers cannot couple to its types, then
// attach your own meaning outside the seal:
//
//	errors.Mark(errors.Seal(err), ErrNotFound)                   // a sentinel
//	errcode.Link(errcode.StatusNotFound, msg, errors.Seal(err)) // a code
//
// Seal the cause, not the result: errors.Seal(errcode.Link(…)) hides the code
// that was just attached.
//
// Unlike Mark, Seal records no frame. The wrap around it records the location, and
// a frame of its own would repeat that wrap's line. errtrace.Format follows only
// Unwrap, so it does not show the frames beneath a seal; this package's %+v does.
func Seal(err error) error {
	if err == nil {
		return nil
	}
	return &sealed{cause: err}
}

func (s *sealed) Error() string { return s.cause.Error() }

func (s *sealed) Format(f fmt.State, verb rune) { formatError(s, f, verb) }

// Attrs returns every attribute beneath the seal: attributes are operator context,
// which the seal keeps.
func (s *sealed) Attrs() []slog.Attr { return Attrs(s.cause) }

// TraceCause hands the hidden cause to this module's renderer only; see trace.Token.
func (s *sealed) TraceCause(trace.Token) error { return s.cause }
