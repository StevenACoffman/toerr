package errpgx_test

import (
	"fmt"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/errpgx"
)

// codeSink keeps the compiler from discarding the work under measurement.
var codeSink errcode.StatusCode

// BenchmarkFromError is the price of classifying a server error at the repository's
// boundary: the context and sentinel checks, then the SQLSTATE lookup.
func BenchmarkFromError(b *testing.B) {
	err := fmt.Errorf("insert user: %w", &pgconn.PgError{Code: pgerrcode.UniqueViolation})
	for b.Loop() {
		codeSink = errpgx.FromError(err)
	}
}
