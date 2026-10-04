// The property tests are a module of their own so that rapid stays out of toerr's
// dependency graph.
//
// A test-only requirement is not free to a library's consumers. It does not reach their
// builds and `go mod tidy` does not add it to their go.mod, but it does land in their go.sum
// and appear in `go list -m all`, which is enough to show up in a supply-chain scan, an SBOM
// and a dependency-review bot. toerr has no dependencies outside the standard library, and
// keeping it that way is worth a directory.
module github.com/StevenACoffman/toerr/proptest

go 1.26.0

// The properties are about the working tree, not the last release.
replace github.com/StevenACoffman/toerr => ../

require (
	github.com/StevenACoffman/toerr v0.0.0-00010101000000-000000000000
	pgregory.net/rapid v1.3.0
)
