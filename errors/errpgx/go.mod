// errpgx is a module of its own so that pgx stays out of toerr's dependency graph: a
// program that never talks to PostgreSQL should not find pgx in its go.sum or its SBOM.
module github.com/StevenACoffman/toerr/errors/errpgx

go 1.26.0

// Until toerr v0.3.0 is tagged, build against the working tree. Consumers ignore a replace
// in a module they depend on, so this line is harmless once that tag exists.
replace github.com/StevenACoffman/toerr => ../..

require (
	github.com/StevenACoffman/toerr v0.3.0
	github.com/jackc/pgerrcode v0.0.0-20250907135507-afb5586c32a6
	github.com/jackc/pgx/v5 v5.11.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	golang.org/x/text v0.29.0 // indirect
)
