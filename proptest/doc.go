// Package proptest holds toerr's property-based tests.
//
// They live outside the library's own module so that rapid, the generator library they need,
// stays out of the dependency graph every consumer of toerr inherits. See go.mod for why a
// test-only requirement is not free.
//
// Everything here drives toerr through its public API. The properties are the contracts
// callers rely on — transparency of Wrap and Mark, attribute order, the end-user safety of
// errhttp — stated over chains of any shape rather than the few a table test spells out.
package proptest
