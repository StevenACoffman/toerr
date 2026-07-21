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
