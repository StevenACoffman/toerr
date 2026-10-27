package errpgx

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/StevenACoffman/toerr/errors/errcode"
)

// Status returns the domain status code for an error pgx returned, or StatusUnknown
// for nil and for an error it cannot classify.
//
// The code describes the operation to its caller, not the database's view of it. A
// caller of a repository did not log in to PostgreSQL, so a credentials or privilege
// failure is the program's own misconfiguration, StatusInternal, and never
// StatusUnauthenticated or StatusPermissionDenied.
//
// A context error comes first: context.DeadlineExceeded gives StatusDeadlineExceeded
// and context.Canceled gives StatusCanceled. pgx reports a query it cancelled for a
// context as SQLSTATE 57014 as well, and the seal around a translated error hides the
// context error from errcode.ForTransport, so the code has to carry it. A missing row,
// pgx.ErrNoRows or sql.ErrNoRows, is StatusNotFound, and a server error is mapped by
// FromSQLState, and a commit that rolled back is StatusAborted. A failure to connect,
// or one pgconn reports reached nothing on the server, is StatusUnavailable, even when
// the connection timed out: no statement ran. A timeout after a statement was sent is
// StatusDeadlineExceeded, since the statement may have run.
func Status(err error) errcode.StatusCode {
	var pgErr *pgconn.PgError
	var connectErr *pgconn.ConnectError
	switch {
	case err == nil:
		return errcode.StatusUnknown
	case errors.Is(err, context.DeadlineExceeded):
		return errcode.StatusDeadlineExceeded
	case errors.Is(err, context.Canceled):
		return errcode.StatusCanceled
	case errors.Is(err, sql.ErrNoRows): // pgx.ErrNoRows unwraps to it.
		return errcode.StatusNotFound
	case errors.As(err, &pgErr):
		return FromSQLState(pgErr.Code)
	case errors.Is(err, pgx.ErrTxCommitRollback):
		return errcode.StatusAborted
	case errors.Is(err, pgx.ErrTooManyRows), errors.Is(err, pgx.ErrTxClosed):
		return errcode.StatusInternal // the program assumed one row, or an open transaction.
	case pgconn.SafeToRetry(err), errors.As(err, &connectErr):
		return errcode.StatusUnavailable
	case pgconn.Timeout(err):
		return errcode.StatusDeadlineExceeded
	}
	return errcode.StatusUnknown
}

// FromSQLState returns the domain status code for a PostgreSQL SQLSTATE, or
// StatusUnknown for one it does not know. A few codes are mapped on their own and the
// rest by their class, the first two characters, so a code a later PostgreSQL adds to
// a class is still classified.
//
// The retry guidance of each code is what decides the mapping. A serialization failure
// or a deadlock is StatusAborted, because retrying the whole transaction can succeed. A
// statement whose commit is unknown (40003) is StatusUnknown, because it may have
// committed and must not be retried automatically. A broken connection, a shutting
// down server, and a read-only standby after a failover are StatusUnavailable.
func FromSQLState(state string) errcode.StatusCode {
	if code, ok := fromSpecificState(state); ok {
		return code
	}
	return fromClass(state)
}

// Attrs returns the server error's fields for operators, as a "pg" group: its
// sqlstate and the schema, table, column, and constraint it names, each left out when
// empty. It returns nil when err holds no *pgconn.PgError. Attach them with the wrap
// that says what was being attempted:
//
//	return errcode.Link(errpgx.Status(err), "",
//		errors.WrapWithMessage(errors.Seal(err), "insert user", errpgx.Attrs(err)...))
//
// The Detail, Hint, Where, and InternalQuery fields are left out, because they can
// carry row values, such as the email a unique violation rejected, or whole
// statements. The server's message is already in the error's text.
func Attrs(err error) []slog.Attr {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}
	var fields []any
	add := func(key, value string) {
		if value != "" {
			fields = append(fields, slog.String(key, value))
		}
	}
	add("sqlstate", pgErr.Code)
	add("schema", pgErr.SchemaName)
	add("table", pgErr.TableName)
	add("column", pgErr.ColumnName)
	add("constraint", pgErr.ConstraintName)
	return []slog.Attr{slog.Group("pg", fields...)}
}

// fromSpecificState maps the SQLSTATEs whose code differs from their class's.
//

func fromSpecificState(state string) (errcode.StatusCode, bool) {
	switch state {
	case pgerrcode.UniqueViolation:
		return errcode.StatusAlreadyExists, true
	case pgerrcode.NotNullViolation, pgerrcode.CheckViolation:
		return errcode.StatusInvalidArgument, true
	case pgerrcode.StatementCompletionUnknown:
		return errcode.StatusUnknown, true
	case pgerrcode.LockNotAvailable:
		return errcode.StatusAborted, true
	case pgerrcode.QueryCanceled: // statement_timeout; a context's cancel is caught first.
		return errcode.StatusDeadlineExceeded, true
	case pgerrcode.AdminShutdown, pgerrcode.CrashShutdown, pgerrcode.CannotConnectNow,
		pgerrcode.IdleSessionTimeout, pgerrcode.IdleInTransactionSessionTimeout,
		pgerrcode.ReadOnlySQLTransaction:
		return errcode.StatusUnavailable, true
	case pgerrcode.DataCorrupted, pgerrcode.IndexCorrupted:
		return errcode.StatusDataLoss, true
	}
	return errcode.StatusUnknown, false
}

// fromClass maps a SQLSTATE by its class.
//

func fromClass(state string) errcode.StatusCode {
	if len(state) < len("00") {
		return errcode.StatusUnknown
	}
	switch state[:2] {
	case class(pgerrcode.IntegrityConstraintViolation):
		return errcode.StatusFailedPrecondition
	case class(pgerrcode.DataException):
		return errcode.StatusInvalidArgument
	case class(pgerrcode.TransactionRollback):
		return errcode.StatusAborted
	case class(pgerrcode.ConnectionException):
		return errcode.StatusUnavailable
	case class(pgerrcode.InsufficientResources):
		return errcode.StatusResourceExhausted
	case class(pgerrcode.InvalidAuthorizationSpecification),
		class(pgerrcode.SyntaxErrorOrAccessRuleViolation),
		class(pgerrcode.FeatureNotSupported),
		class(pgerrcode.InvalidTransactionState),
		class(pgerrcode.ProgramLimitExceeded),
		class(pgerrcode.OperatorIntervention),
		class(pgerrcode.InternalError):
		return errcode.StatusInternal
	}
	return errcode.StatusUnknown
}

// class returns the class of a SQLSTATE: its first two characters.
func class(state string) string { return state[:2] }
