// Package sentinel provides lightweight, stack-free sentinel error values.
//
// Unlike errors.New in this module, sentinel.New does not capture a program
// counter, so it is cheap to declare at package scope where a stack would be
// meaningless (it would point at package initialization, not the failure site).
//
// # Matching by identity
//
// Sentinel deliberately has no Is method, so errors.Is falls back to pointer
// equality. A sentinel therefore matches only itself, not another sentinel that
// merely shares the same text:
//
//	var ErrNotFound = sentinel.New("not found")
//	errors.Is(err, ErrNotFound)               // true only for THIS value
//	errors.Is(err, sentinel.New("not found")) // false — distinct value, same text
//
// To be matchable, a sentinel must be a package-level variable. This mirrors the
// standard library (io.EOF, sql.ErrNoRows) and is what lets a sentinel mean one
// specific condition rather than "any error whose text happens to match".
//
// # Matching by type
//
// Every sentinel shares the concrete type *Sentinel, so AsType[*Sentinel] matches
// any sentinel indiscriminately. That makes *Sentinel the wrong tool for telling
// individual sentinels apart (define a distinct named type for that). It is,
// however, a useful feature for the inverse question: AsType[*Sentinel](err)
// distinguishes sentinels produced by this package from all third-party errors.
package sentinel

// Sentinel is a trivial implementation of error.
//
//nolint:errname // "Sentinel" is a deliberate public type name, not an ErrXxx/xxxError.
type Sentinel struct {
	s string
}

// New returns an error that formats as the given text.
// Each call to New returns a distinct error value even if the text is identical,
// so a sentinel intended for matching should be stored in a package-level variable.
func New(text string) error {
	return &Sentinel{text}
}

func (e *Sentinel) Error() string {
	return e.s
}

// IsSentinel reports that the value was produced by this package. It lets callers
// distinguish sentinels from arbitrary errors via a marker interface.
func (e *Sentinel) IsSentinel() bool { return true }
