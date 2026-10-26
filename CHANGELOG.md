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
  never `err.Error()`.
- `errgrpc.FromCode(code)`: the domain code for a client call's gRPC code.
- `errgrpc.Code(code)`: the gRPC code for a domain code.
