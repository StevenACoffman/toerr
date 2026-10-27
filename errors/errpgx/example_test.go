package errpgx_test

import (
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	toerr "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/errcode"
	"github.com/StevenACoffman/toerr/errors/errpgx"
)

// A repository translates pgx's error where it leaves, and seals it: callers branch on
// the domain code and cannot match *pgconn.PgError.
func ExampleFromError() {
	cause := &pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: "users_email_key"}
	err := errcode.Link(errpgx.FromError(cause), "that email is taken", toerr.Seal(cause))

	var pgErr *pgconn.PgError
	fmt.Println(errcode.Status(err), errcode.Message(err), errors.As(err, &pgErr))
	// Output: already_exists that email is taken false
}

// A serialization failure is Aborted: the caller should retry the whole transaction.
func ExampleFromSQLState() {
	fmt.Println(errpgx.FromSQLState(pgerrcode.SerializationFailure))
	// Output: aborted
}
