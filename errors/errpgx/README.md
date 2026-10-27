# Errpgx

**Turn an error from pgx into the domain code that tells its caller what to do.**

[![Go Reference](https://pkg.go.dev/badge/github.com/StevenACoffman/toerr/errors/errpgx.svg)](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/errpgx)

A repository built on [pgx](https://github.com/jackc/pgx) should not hand
`*pgconn.PgError` to its callers: they would couple to the database driver, and branch
on SQLSTATEs. `errpgx` maps the error to an [`errcode`](../errcode/) status where it
leaves the repository, so the caller, and every transport adapter after it, branches
on one small set of codes. SQLSTATEs come from
[pgerrcode](https://github.com/jackc/pgerrcode).

It is a module of its own, like [`errgrpc`](../errgrpc/), so pgx stays out of the
dependency graph of a program that does not use it:

```sh
go get github.com/StevenACoffman/toerr/errors/errpgx
```

## Usage

- `errpgx.FromError(err) errcode.StatusCode`: the code for an error pgx returned, or
  `StatusUnknown` when it cannot say.
- `errpgx.FromSQLState(state) errcode.StatusCode`: the code for a SQLSTATE.
- `errpgx.Attrs(err) []slog.Attr`: a `pg` group of the server error's sqlstate,
  schema, table, column, and constraint, for operators. It leaves out `Detail`,
  `Hint`, `Where`, and `InternalQuery`, which can carry row values or whole
  statements.

Translate and seal in one line, where the error leaves the repository:

```go
if err != nil {
	return nil, errcode.Link(errpgx.FromError(err), "", errors.Seal(err))
}
```

To keep the server's fields in the log, attach them with the wrap that says what was
being attempted:

```go
return errcode.Link(errpgx.FromError(err), "",
	errors.WrapWithMessage(errors.Seal(err), "insert user", errpgx.Attrs(err)...))
```

The seal stops callers matching `*pgconn.PgError` or `pgx.ErrNoRows`, while operators
keep the error's text and trace. errpgx builds no error itself, so the frame the
`Link` records is your repository's line.

## The Mapping

A code describes the operation to its caller, and each one carries its retry
guidance, so the mapping follows what a retry would do:

| pgx error or SQLSTATE                                                                                                        | Code                           | Why                                                            |
| ---------------------------------------------------------------------------------------------------------------------------- | ------------------------------ | -------------------------------------------------------------- |
| `context.DeadlineExceeded`, `context.Canceled`                                                                               | `DeadlineExceeded`, `Canceled` | checked first: the seal hides them from `errcode.ForTransport` |
| `pgx.ErrNoRows`, `sql.ErrNoRows`                                                                                             | `NotFound`                     |                                                                |
| `23505` unique violation                                                                                                     | `AlreadyExists`                |                                                                |
| `23502` not null, `23514` check, class `22` data exception                                                                   | `InvalidArgument`              | the input is wrong whatever the state                          |
| `23503` foreign key, `23P01` exclusion, rest of class `23`                                                                   | `FailedPrecondition`           | another row must change first                                  |
| `40001` serialization, `40P01` deadlock, `55P03` lock not available, class `40`, `pgx.ErrTxCommitRollback`                   | `Aborted`                      | retry the whole transaction                                    |
| `40003` statement completion unknown                                                                                         | `Unknown`                      | it may have committed: never retry it automatically            |
| `57014` statement timeout, a timeout after the statement was sent                                                            | `DeadlineExceeded`             | it may have run                                                |
| class `08`, a failure to connect, shutdowns, idle timeouts, `25006` read-only after a failover, `53300` too many connections | `Unavailable`                  | nothing ran: retry with backoff                                |
| rest of class `53`, such as a full disk                                                                                      | `ResourceExhausted`            | do not retry automatically                                     |
| `XX001` data corrupted, `XX002` index corrupted                                                                              | `DataLoss`                     |                                                                |
| class `28` credentials, `42` syntax or privilege, `0A`, `25`, `54`, `57`, `XX`, `pgx.ErrTooManyRows`, `pgx.ErrTxClosed`      | `Internal`                     | the program's SQL or configuration is wrong                    |
| anything else                                                                                                                | `Unknown`                      | a code or nothing                                              |

A constraint or data violation is `InvalidArgument` on the assumption that the
caller's input reached the database unchecked. When the program's own data was bad,
the program is the party responsible: link `StatusInternal` at that call site instead.

A wrong database password or a missing grant is `Internal`, not `Unauthenticated` or
`PermissionDenied`: the repository's caller did not log in to PostgreSQL, and telling
it so would send it the wrong way. A code a later PostgreSQL adds to a known class is
classified by its class.

## Example

```go
func (s *Store) CreateUser(ctx context.Context, u *User) error {
	_, err := s.pool.Exec(ctx, insertUser, u.Email)
	if err != nil {
		return errcode.Link(errpgx.FromError(err), "", errors.Seal(err))
	}
	return nil
}
```

A duplicate email answers `409 Conflict` through `errhttp` and `AlreadyExists`
through `errgrpc`; a cancelled request answers `499` and `Canceled`.
