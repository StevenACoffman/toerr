// Package errpgx is the PostgreSQL adapter for errcode, for errors from
// github.com/jackc/pgx/v5. FromError turns an error pgx returned into the domain status
// code that describes the failed operation to its caller, FromSQLState does the same
// for a SQLSTATE, and Attrs gives operators the server's error fields. Translate the
// error where it leaves your repository, and seal it, so callers branch on the code
// and cannot couple to pgx:
//
//	if err != nil {
//		return nil, errcode.Link(errpgx.FromError(err), "", errors.Seal(err))
//	}
//
// errpgx is a module of its own, so only a program that imports it depends on pgx.
package errpgx
