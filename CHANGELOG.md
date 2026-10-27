# Changelog

Curated by hand for each release, for people upgrading. The commit log is the
developer's record; this file is the user's. Paste a release's section into its GitHub
release notes when you tag it.

## toerr v0.3.0 (Unreleased)

### Upgrading

- **`errcode.WithCode` and `errcode.WithCodeOpaque` are deprecated.** They keep their
  behaviour through v0.4.0 and are removed in the first release after it. staticcheck
  (SA1019) reports each call. Migrate as follows:

  | Before                                   | After                                       |
  | ---------------------------------------- | ------------------------------------------- |
  | `errcode.WithCode(code, msg, nil)`       | `errcode.New(code, msg)`                    |
  | `errcode.WithCode(code, msg, err)`       | `errcode.Link(code, msg, err)`              |
  | `errcode.WithCodeOpaque(code, msg, err)` | `errcode.Link(code, msg, errors.Seal(err))` |

  `Link` returns nil for a nil `err`, as `errors.Wrap` does. `WithCode` returned a
  coded leaf, so migrate a nil-cause call to `New`, not `Link`.
- **`errcode.New(code, "")` renders differently.** `Error()` of a coded leaf without
  a message is the code in words, `not found`, where it was `error: not_found`.
  `Status`, `Message`, and the `errhttp` output are unchanged.
- **`WithCodeOpaque` now keeps the operator view.** It delegates to `errors.Seal`, so
  `%+v` shows the cause's frames and `errors.Attrs` returns the cause's attributes.
  Matching is still blocked, as before.
- **Coded errors record a frame.** Every `errcode` constructor now appears in the `%+v`
  return trace at its caller's line, and `%+v` of a coded leaf no longer prints
  `<nil>`. The capture costs what a `Wrap` costs: about 133 ns, where `WithCode` cost
  12 ns.
- **`errors.LogValue` returns the error's tree as JSON.** It returned a group of the
  message and every attribute, which repeated a key that two errors both set, such as
  siblings in a join; a backend that decodes into a map kept one value. It now returns
  a value that JSON handlers write as an object: each error in a chain is a layer with
  its own message, call site (`at`), attributes, and any code, class, mark, or seal,
  and each branch of a join is a tree of its own. A layer from another package's
  error also names its Go `type`. The value's kind is `Any`, not
  `Group`, so code that read it with `Value.Group` or matched the group in
  `ReplaceAttr` must read the JSON instead. `errors.Attrs` still returns the flat
  list, and a nil error now gives the zero `Value` rather than a panic.
- **`errhttp` answers a cancelled request with 499, and a context error without a code
  with 499 or 504.** `StatusCanceled` mapped to 408 Request Timeout, which means the
  server stopped waiting for the client to send its request. It now maps to 499
  Client Closed Request (`errhttp.StatusClientClosedRequest`), the mapping
  `google/rpc/code.proto` gives and grpc-gateway uses. `errhttp.Error` now reads the
  code with `errcode.ForTransport`, so an error wrapping `context.Canceled` or
  `context.DeadlineExceeded` without a code answers 499 or 504 instead of 500. Update
  dashboards and alerts that counted these as 408 or 500.
- **`errcode.Message` returns the outermost non-empty message.** A coded wrap without
  a message only reclassifies, so it no longer hides the message beneath it. `Code`
  returns the same message.

### Added

- `errors.Seal(err)`: hides `err` from `Is`, `As`, `AsType`, `AsBehavior`, `errcode`,
  `errclass`, and marks, while keeping its text, frames, and attributes for operators.
  It is the opposite of `Mark`.
- `errors.Wrapf(err, format, args...)`: `WrapWithMessage` with a formatted message.
- `errors/logctx`: a request-scoped collector of log fields, for the request's one log
  line on the success and the failure path. `Init`, `Add`, `Scope` (a namespace per
  parallel branch), and `Attrs`. Standard library only.
- `errcode.New(code, message)`: the coded leaf.
- `errcode.Link(code, message, err)`: the coded wrap.
- `errcode.ForTransport(err)`: the code and message a transport adapter answers with.
  It is `Code`, except that an error without a code that wraps
  `context.DeadlineExceeded` or `context.Canceled` gets `StatusDeadlineExceeded` or
  `StatusCanceled`, so an abandoned or timed-out request is not counted as a failure.
- `errcode.StatusUnavailable`, `StatusResourceExhausted`, `StatusAborted`,
  `StatusOutOfRange`, and `StatusDataLoss`, appended so no existing value changes. The
  set is now gRPC's canonical codes apart from OK, and each code's godoc says whether a
  retry can help. `errhttp` maps them to 503, 429, 409, 400, and 500.

### Documentation

- The guide and the README separate three records of a failure: the context chain
  (what was being attempted), the return trace (which layers passed it on), and the
  stack trace (how it was created). New sections cover failure domains, errors across
  goroutines, and a code as the default classification.
- `errclass.Panic` is documented, along with how to set it on a recovered panic.
- The README has a `wrapcheck` configuration for projects that adopt toerr.

## errgrpc v0.1.0 (Unreleased)

The first release of `github.com/StevenACoffman/toerr/errors/errgrpc`, a module of its
own so that gRPC stays out of toerr's dependency graph. Tag it as
`errors/errgrpc/v0.1.0` after toerr v0.3.0 is tagged, since it needs that version's
codes.

- `errgrpc.Error(err)`: a gRPC status error carrying the code and user-facing message,
  never `err.Error()`. An error without a code that wraps a context error becomes
  `Canceled` or `DeadlineExceeded`, as grpc-go's server answers a bare context error.
- `errgrpc.FromError(err)`: the domain code for an error a gRPC client call returned,
  as the code of the operation that made the call. A downstream code that describes
  your call to the service (`InvalidArgument`, `FailedPrecondition`, `OutOfRange`,
  `Unimplemented`, `Unauthenticated`, `PermissionDenied`) becomes `Internal`.
- `errgrpc.FromCode(code)`: the raw table, the domain code of the same name, for a
  call that forwarded the caller's input unchanged.
- `errgrpc.Attrs(err)`: the downstream status's code, `ErrorInfo` reason and domain,
  and `RetryInfo` delay, for operators.
- `errgrpc.Code(code)`: the gRPC code for a domain code.

## errpgx v0.1.0 (Unreleased)

The first release of `github.com/StevenACoffman/toerr/errors/errpgx`, a module of its
own so that pgx stays out of toerr's dependency graph. Tag it as
`errors/errpgx/v0.1.0` after toerr v0.3.0 is tagged.

- `errpgx.FromError(err)`: the domain code for an error from `github.com/jackc/pgx/v5`,
  chosen by the retry guidance each code carries. A context error comes first, a
  failure to connect is `Unavailable`, and the database's own credentials and grants
  are `Internal`.
- `errpgx.FromSQLState(state)`: the domain code for a SQLSTATE, by code and then by
  class, using `github.com/jackc/pgerrcode`.
- `errpgx.Attrs(err)`: the server error's sqlstate, schema, table, column, and
  constraint, for operators, without the fields that can carry row values.
