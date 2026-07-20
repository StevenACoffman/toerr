# Errors

A drop-in replacement for the Go standard library `errors` package that records
where errors are created and wrapped, carries `slog` attributes for structured
logging, and supports transparent type marks for control flow.

[![Go Reference](https://pkg.go.dev/badge/github.com/StevenACoffman/toerr/errors.svg)](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors)

```go
import errors "github.com/StevenACoffman/toerr/errors"
```

## Features

- [Message and location](#message-and-location)
- [Structured attributes](#structured-attributes)
- [Return trace](#return-trace)
- [Marks and `AsType`](#marks-and-astype)
- [Design: structure vs. context](#design-structure-vs-context)
- [Drop-in replacement for the std `errors` package](#drop-in-replacement)

## Message and Location

[`New`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#New) creates a
leaf error and records the call site.
[`Wrap`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#Wrap) adds the
current call site to an existing error (no message change).
[`WrapWithMessage`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#WrapWithMessage)
also prepends a message.

```go
err := errors.New("connect")
err = errors.WrapWithMessage(err, "dial database")
fmt.Println(err) // "dial database: connect"
```

Unlike `fmt.Errorf("...: %w", err)`, which only concatenates strings, `New` and
`Wrap` capture the file, line, and function at the point they are called.

## Structured Attributes

Every constructor takes trailing `slog.Attr` values for context that belongs in
structured logs rather than in the message:

```go
err := errors.New("connect", slog.String("host", "db1"))
err = errors.Wrap(err, slog.Int("retry", 3))
```

Collect every attribute along the chain with
[`Attrs`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#Attrs) and
hand them to `LogAttrs`:

```go
logger.LogAttrs(ctx, slog.LevelError, err.Error(), errors.Attrs(err)...)
```

Errors also implement `slog.LogValuer`, so logging one directly promotes its
message and attributes:

```go
logger.Error("request failed", slog.Any("err", err))
```

## Return Trace

Each `New` and `Wrap` records a single frame — its own call site. Together they
form a **return trace**: the path the error took out to the caller, one frame per
hop, ordered origin-first. Unlike a stack trace it is captured incrementally, so
it works even when the error is returned across goroutines. The verbose format
verb `%+v` prints it below the message:

```text
dial database: connect

main.dialDB
	/app/db.go:20
main.startup
	/app/main.go:14
main.main
	/app/main.go:9
```

There is no full stack trace and no runtime tail — only the frames you wrapped
through. (This matches [`braces.dev/errtrace`](https://github.com/bracesdev/errtrace),
whose return-trace model and tree formatting this package follows.)

### Interoperability with Errtrace

The trace is built on errtrace's exported marker interface — any error with a
`TracePC() uintptr` method contributes a frame — and this package's error types
implement it. So the two packages interoperate in both directions: an
`errtrace.Wrap`-ed error nested in one of these chains shows up in `%+v`, and
these errors show up in `errtrace.Format`. You can mix the two freely.

A joined error (`errors.Join`) renders as a tree, with each branch drawn under a
`+-` connector and `|` gutters, and the wrapping error's own trace at the bottom:

```text
+- connection refused
|
|  main.dbConnect
|  	/app/db.go:14
|
+- cache miss
|
|  main.cacheLookup
|  	/app/cache.go:9
|
startup failed: connection refused
cache miss

main.startup
	/app/main.go:12
```

## Marks and `AsType`

[`Mark`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#Mark) tags an
error with a marker whose type
[`AsType`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#AsType) then
recognizes — without changing the error's message or `Unwrap` chain. A foreign
error is wrapped first so it still carries a trace. `AsType` is a re-export of the
Go 1.26 `errors.AsType`, so it returns the found value and a boolean.

```go
type NotFoundError struct{ error }

err = errors.Mark(sql.ErrNoRows, &NotFoundError{})
if _, ok := errors.AsType[*NotFoundError](err); ok {
	// control flow, anywhere up the chain
}
```

## Design: Structure Vs. Context

`New` and `Wrap` take trailing `attrs ...slog.Attr` for open-ended, caller-supplied
context, but the error's own fields (`msg`, `cause`, `pc`) are typed struct fields,
**not** attrs with well-known keys. The two carry different kinds of data and want
different representations:

- **Context** is arbitrary key/values chosen by the caller. That is genuinely
  open-ended, so it belongs in `[]slog.Attr`.
- **Structure** is the error's identity and shape, and it is contract-bearing:
  - `cause` feeds `Unwrap() error`, which `errors.Is`/`As`/`AsType` walk. It must
    be a statically-typed `error`, not `slog.Any("cause", err)` boxed into a value
    and read back with a string lookup and a type assertion that can silently fail.
  - `slog.Value` has no first-class `error` or `uintptr` kind, so folding
    `cause`/`pc` into attrs loses the type the compiler otherwise enforces.
  - Keeping `msg`/`cause`/`pc` out of the caller-owned attr namespace means a
    caller who passes `slog.String("msg", …)` cannot collide with the error's own
    message.
  - The struct keeps the shape invariant expressible — a leaf has `msg` and no
    `cause`; a wrapper has `cause` and no `msg`; both always have a `pc`.

In short: open-ended and caller-owned → `[]slog.Attr`; fixed, typed, and
contract-bearing → named fields.

## Drop-in Replacement

- Replace the import `errors` with `github.com/StevenACoffman/toerr/errors`.
- `Is`, `As`, `Unwrap`, and `Join` are re-exported unchanged.
- Replace `fmt.Errorf("some message: %w", err)` with
  `errors.WrapWithMessage(err, "some message")` to gain location and attributes.
- Use the `sentinel` package for package-level sentinel values.

## Packages

| Package    | Purpose                                                                   |
| ---------- | ------------------------------------------------------------------------- |
| `errors`   | Primary API: `New`, `Wrap`, `WrapWithMessage`, `Mark`, `AsType`, `Attrs`. |
| `sentinel` | Cheap, stack-free sentinel values for `errors.Is` matching.               |
| `errcode`  | Transport-neutral status codes (`WithCode`/`Code`/`Status`/`Payload`).    |
| `errhttp`  | HTTP adapter for `errcode`: maps a domain code to an HTTP status.         |
| `errclass` | Coarse severity classification that folds across joined errors.           |

## Prior Art

- [remko/go-errors](https://github.com/remko/go-errors) — the `errcode` model.
- [AnnotatedError](https://github.com/myrjola/sheerluck/blob/ba6715f2118eba0677889afb58d77f6f3f33f345/internal/errors/annotatederror.go#L24)
  by @myrjola — message + `pc` + `slog.Attr`.
- [xerrors / errcontext](https://github.com/zircuit-labs/zkr-go-common-public/blob/dc1effe2259f5592f9c38fcc4079aeca0f555cd9/xerrors/errcontext/errcontext.go)
  by @alif-zrc — attaching `slog.Attr` context.

## Why?

This repository is intended to explore various techniques to create a complete custom error framework for use as a drop-in replacement for the
Go standard library `errors` package.

Unlike the standard library `errors` package, where wrapping errors using Go 1.13's `fmt.Errorf` using `%w` only concatenates strings, in this
library wrapping errors using `errors.Wrap` will record the file/line/function of the wrap operation. Creating a new error using this library
using `New` will also record the initial file/line/function of the error creation.

Since errors are often ultimately reported using structured logging, this library seeks to leverage the slog.Attr key-value pair for storing
custom error contextual information, so that LogAttrs(ctx context.Context, level Level, msg string, attrs ...Attr) can be used for reporting
the error as it is more efficient. Both `Wrap` and `New` have `attrs ...Attr` as their optional, final trailing argument.

Errors are also used for control flow. An error that was created by this library via `New` or wrapped via `Wrap` can be transparently marked
using `Mark` such that errors.AsType will return true for that type. Passing as the first parameter to `Mark` an error that was not created
via this library `New` or `Wrap` will first `Wrap` and then `Mark`.

Please examine these functional requirements against the advice in ~/Documents/agent-orange/go-advice/summary_rules.md and identify what would
need to be altered in the code to better meet these requirements and adhere to the advice in
~/Documents/agent-orange/go-advice/summary_rules.md

## StackTraces Vs Return Traces

With stack traces, caller information for the goroutine is captured once when the error is created.

In constrast, errtrace records the caller information incrementally, following the return path the error takes to get to the user. This
approach works even if the error isn't propagated directly through function returns, and across goroutines.

Aside from stylistic formatting, in straightforwardly explicit, synchronous Go code where the frames are ordered from origin to deepest, a
full stack trace like:

```text
[Pasted text #1 +10 lines]
```

Would instead be a return trace of:

```text
[Pasted text #2 +5 lines]
```

These look similar, as the package and function names are duplicative (`braces.dev/errtrace_test.rateLimitDialer`), and the call locations
have the same files (`/errtrace/example_stack_test.go`) but each call line position (`/errtrace/example_stack_test.go:81`) differs from the
return line position (`/path/to/errtrace/example_http_test.go:72`).

This can be conceptualized as two sides of a symmetrical arcing parabolic path through the codebase.

```text
  main.coreOuter   :15   ┐
  main.coreMid     :14   │  return trace  (how it escalated out)
  main.coreOrigin  :13   ┤  ← apex: where the error was created
  main.coreMid     :14   │  origin stack  (how we got to creation)
  main.coreOuter   :15   │
  main.TestOrder   :22   │
  runtime.goexit         ┘
```

The two sides can differ significantly
when errors are passed outside of functions and across goroutines (e.g., channels).

Generally, an error is only handled once, and is otherwise just returned. The further the distance from error origination to final error
handling, the more impactful the error, and the more cognitively difficult to reconstruct it's journey.

A stacktrace provides the answer to the question "how did we get to where the error was originally created?"
A return trace provides the answer to the question "how did not handling this earlier escalate and exacerbate the problem?"

Error codepaths are typically much less frequently executed and triggering them in tests can be difficult. As a result, they usually are only
cognitively reviewed once or twice during development, and never truly exercised until a user hits them in production.

By its nature, the state of the world when an error occurs is often more complex than the
success path (since it is also may be dependent on where the error occurred).

In the course of troubleshooting an issue where the error is passed outside of functions and across goroutines, I would like to make an
ergonomic and helpful display of both sides of this parabolic path in the situations where that is helpful context.

However, when both sides are neatly symmetric, I do not want to add noise.
