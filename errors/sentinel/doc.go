// Package sentinel provides cheap, stack-free sentinel error values.
//
// Unlike errors.New in the parent module, sentinel.New captures no program
// counter, so it is cheap to declare at package scope where a stack would only
// point at package initialization, not the failure site.
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
// however, useful for the inverse question: AsType[*Sentinel](err) distinguishes
// sentinels produced by this package from all third-party errors.
package sentinel
