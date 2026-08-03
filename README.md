# Errors

**Go's `%w` tells you *what* failed, not *where*. This package adds the where.**

[![Go Reference](https://pkg.go.dev/badge/github.com/StevenACoffman/toerr/errors.svg)](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors)

When a Go service fails in production, `fmt.Errorf("...: %w", err)` gives you a
string like `dial database: connect: connection refused`, but not the file,
line, or function that produced it. Finding the failure means grepping the source
for message fragments and guessing which of three call sites logged it.

This is a drop-in replacement for the standard library `errors` package. `New` and
`Wrap` record the call site every time, so the same failure arrives with the path
it took attached:

```go
// Standard library: what failed.
return fmt.Errorf("dial database: %w", err)
// dial database: connection refused

// This package: what failed, and where.
return errors.WrapWithMessage(err, "dial database")
// dial database: connection refused
//
// main.dialDB
//     /app/db.go:20
// main.startup
//     /app/main.go:14
```

The cost is one import change. Everything the standard library gives you
(`Is`, `As`, `Unwrap`, `Join`) is re-exported unchanged, and errors from other
packages flow through untouched.

## The Problem

Error paths are the least-exercised code in a program: triggered rarely, reviewed
once or twice during development, and often first run for real when a user hits them
in production. This is exactly where the state of the world is more complicated than the success
path, and also where the engineer reading the log likely did not write the code.

By the time an error surfaces at the top of a handler, it was created deep in some
dependency and handled by no one on the way up. To act on it, that engineer needs
three things that the standard library discards as the error travels
outward:

- **Where it happened.** `fmt.Errorf` with `%w` concatenates strings; it keeps
no file, line, or function. `errors.Is` still works, but "where did this come
from?" means grepping the source for message fragments.
- **What the state was.** The inputs, identifiers, and state that explain the failure
are in scope only near the origin. A plain error string carries none of it forward
to where the error is logged.
- **What kind of failure it is.** Reacting to a category (not found, rate limited,
retryable) across layers of wrapping takes more than comparing message strings.

This package restores all three: a **return trace** for *where*, structured
**`slog.Attr`** for *what state*, and type **marks** for *what kind*.

## What You Get

- **[Location at every hop](#message-and-location)**: `New` and `Wrap` capture
file, line, and function.
- **[Structured context](#structured-attributes)**: every constructor takes
trailing `slog.Attr` values, surfaced at your log boundary with `errors.LogValue`
/ `errors.Attrs`.
- **[A return trace](#return-trace)** under `%+v`, not a heavyweight stack trace.
- **[Type marks for control flow](#marks-and-astype)**: `Mark` / `AsType` tag an
error by type without disturbing its message or `Unwrap` chain.
- **[A true drop-in](#drop-in-replacement)**: `Is`, `As`, `Unwrap`, `Join`
re-exported unchanged, and foreign errors interoperate.

Intended for larger projects where the trade-offs of a batteries-included error
package outweigh YAGNI.

```go
import errors "github.com/StevenACoffman/toerr/errors"
```

## Message and Location

[`New`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#New) creates a
leaf error and records the call site.
[`Wrap`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#Wrap) adds the
current call site to an existing error (no message change).
[`WrapWithMessage`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#WrapWithMessage)
also prefixes a message.

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

Errors deliberately do **not** implement `slog.LogValuer`, so passing one straight to
a logger does not auto-expand its fields at every layer. Ask for the grouped form
explicitly with `errors.LogValue`:

```go
logger.LogAttrs(ctx, slog.LevelError, "request failed", slog.Any("err", errors.LogValue(err)))
```

The reasoning behind carrying `slog.Attr` on the error is in
[Design → Context as `slog.Attr`](#context-as-slogattr).

## Return Trace

Each `New` and `Wrap` records a single frame for its own call site. Together they
form a **return trace**: the path the error took out to the caller, one frame per
hop, ordered origin-first. Unlike a stack trace, a return trace is captured incrementally, so
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

No full stack trace and no runtime tail, only the frames you wrapped
through. (This matches [`braces.dev/errtrace`](https://github.com/bracesdev/errtrace),
whose return-trace model and tree formatting this package follows.) For why a
return trace is usually the more useful artifact, see
[Design → Return traces, not stack traces](#return-traces-not-stack-traces).

### Interoperability with `errtrace`

The trace is built on `errtrace`'s exported marker interface, so any error with a
`TracePC() uintptr` method contributes a frame. This package's error types
satisfy that interface, so the two packages interoperate in both directions. An
`errtrace.Wrap`-ed error nested in one of these chains shows up in `%+v`, and
these errors show up in `errtrace.Format`. You can mix the two freely.

A joined error (`errors.Join`) renders as a tree, with each branch drawn under a
`+-` connector and `|` gutters, and the wrapping error's own trace at the bottom:

```text
+- connection refused
|
| main.dbConnect
| /app/db.go:14
|
+- cache miss
|
| main.cacheLookup
| /app/cache.go:9
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
recognizes. [`Mark`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#Mark)
does this without changing the error's message or `Unwrap` chain. A foreign
error is wrapped first, so it still carries a trace. `AsType` is a re-export of the
Go 1.26 `errors.AsType`, so it returns the found value and a boolean.

```go
type NotFoundError struct{ error }

err = errors.Mark(sql.ErrNoRows, &NotFoundError{})
if _, ok := errors.AsType[*NotFoundError](err); ok {
// control flow, anywhere up the chain
}
```

## Design

This package makes three decisions that plain `fmt.Errorf` wrapping does not, one
for each need from [The Problem](#the-problem).

### Return Traces, Not Stack Traces

**A stack trace shows how an error was born. In production you usually need to know
how it got away.**

An error surfaces at the top of a request handler. It was created deep in some
dependency, handled by no one on the way up, and is now being read by an on-call
engineer who did not write the code that produced it. The reader needs two answers:
where did this come from, and why did nothing deal with it sooner?

A stack trace answers only the first, and it answers it at a cost. It is captured
once, at creation, for the goroutine that was running then. The instant the error
is buffered, sent over a channel, or returned from a worker, that recorded stack
describes a call structure that no longer has anything to do with how the error
reached you. It is also mechanical and complete. Every runtime and library frame
is included, so the signal you want, the intent behind your own code, is buried in
dispatch noise. It also points at the creation site, which the error message has
often already told you.
`connection refused` rarely leaves you wondering *what* the operation was.

A return trace answers the second question, the one you could not already answer. It
is not snapshotted; it is assembled on the way out, one frame per wrap, following
the error value itself. Because Go errors are values that are passed rather than
exceptions that are thrown, each frame marks a function that saw the error and chose
to return it instead of handling it. The trace is a log of declined-to-handle
decisions, and it ends at the one place the error was finally dealt with. That is
where the propagation bug lives (the layer that should have retried, translated, or
swallowed the failure and did not), and it is nowhere in a creation-time stack.

Stack trace and return trace are two sides of one symmetric, arcing path. Creation is
the shared apex: the origin stack descends to it, the return trace climbs back out.

```text
main.coreOuter :15 ┐
main.coreMid :14 │ return trace (how it escalated out)
main.coreOrigin :13 ┤ ← apex: where the error was created
main.coreMid :14 │ origin stack (how we got to creation)
main.coreOuter :15 │
main.TestOrder :22 │
runtime.goexit ┘
```

What makes the return trace the more useful side for most production
errors:

- **It follows the value, not the goroutine.** It stays useful across channels,
buffers, and worker pools, exactly the cases where a creation-time stack becomes
useless.
- **Its length is a signal.** The number of frames is the number of layers that
declined to handle the error. This length is the error's escalation distance. A long return trace is a
measurement, not merely a location.
- **It is authored, not mechanical.** Frames appear only where your code wrapped,
so there is no runtime tail and no dispatch noise to read past.
- **It branches.** Combined failures (`errors.Join`) each carry their own return
path, so the trace renders as a tree. A linear stack cannot express "this failure
is two independent failures that met here."

This is not a claim that stack traces are useless. When the bug is *at* the creation
site (a bad input, a violated precondition, a nil dereference), the descending
stack is exactly what you want, and this package still records the creation frame to
give it to you. In practice the case is narrower. The error you debug in production
has usually already escaped its origin, so the path out is the path worth recording.
When the code is straightforwardly synchronous, the two sides are nearly symmetric
and the return trace alone suffices. They diverge, and both become worth showing,
when an error crosses a goroutine or channel boundary.

### Context as `slog.Attr`

**The origin has the evidence; the log site has the need. Let them speak the same
language.**

Recording a failure with no context is like calling in a crime after the scene has
been cleared. The values that explain what went wrong (the inputs, the
identifiers, the state) are all in reach at the moment the error arises, and mostly
out of scope by the time it surfaces at the top of a handler. In Go you rarely
handle an error where it occurs. You return it. Each return defers handling and lets
the context decay another hop.

That is why an error value is more than a name for a failure. It is the vehicle that
carries evidence forward, gathering more at each decision point, from the origin
(which has the most context) to the log site (which has the least).

The log site is where that evidence is spent, and a log is worth writing only if it is
structured, key/value pairs you can search, filter, and aggregate across voluminous
output. "error occurred," with no fields, is not evidence.

Both ends should meet in one representation. If the error accumulates
evidence, and the logger consumes `slog.Attr`, then the error should accumulate that
evidence *as* `slog.Attr`, the exact type the logger reads. Store it any other way
(a formatted message, a `map[string]any`, a bespoke field set) and something must
re-key and re-type it at the log site, the point of least context and the easiest
place to flatten a typed value into untyped text. When the error already holds
`slog.Attr`, one `errors.Attrs(err)` call pours straight into `LogAttrs` with
nothing in between.

**An error is evidence. Store it in the type your logger already reads.**

### Structure Versus Context

Caller context is open-ended and belongs in `[]slog.Attr` (above). The error's own
fields (`msg`, `cause`, `pc`) are the opposite. They are typed struct fields, not
attrs with well-known keys. Structure is the error's identity and shape, and it is
contract-bearing:

- `cause` feeds `Unwrap() error`, which `errors.Is`/`As`/`AsType` walk. It must
be a statically-typed `error`, not `slog.Any("cause", err)` boxed into a value
and read back with a string lookup and a type assertion that can silently fail.
- `slog.Value` has no first-class `error` or `uintptr` kind, so folding
`cause`/`pc` into attributes loses the type the compiler otherwise enforces.
- Keeping `msg`/`cause`/`pc` out of the caller-owned attr namespace means a
caller who passes `slog.String("msg", …)` cannot collide with the error's own
message.
- The struct keeps the shape invariant expressible: a leaf has `msg` and no
`cause`; a wrapper has a `cause` (and may or may not have a `msg`); both always have a `pc`.

In short: open-ended and caller-owned → `[]slog.Attr`; fixed, typed, and
contract-bearing → typed fields.

## When Not to Reach for This

This package takes a side in an unsettled debate: it puts location and structured
context *on the error value*. A well-supported school argues the opposite, and it is
worth knowing before you adopt it.

- **"Keep errors clean. Put context in logs."** Dave Cheney, whose `pkg/errors`
  popularized stack-carrying errors, later moved away from that approach in favor of
  structured logging at the boundary. If you already emit one structured line per
  request (a "canonical log line"), a return trace on the error can duplicate what
  your logs already show.
- **Wrapping has a cost.** Wrapping at every hop buries the one useful frame among
  redundant ones, and `%w` makes the wrapped error part of your API. The `wrapcheck`
  linter only asks you to wrap across *package* boundaries; inside a package, a bare
  `return err` is fine. Treat "escalation distance" as signal only where each frame
  adds information.
- **Log once, not per layer.** Errors do not implement `slog.LogValuer`, so they do
  not auto-expand when logged. Log the error once, at a single top-level handler,
  with `errors.Attrs` / `errors.LogValue`. Lower layers wrap (for location) and
  return, not log.
- **Do not leak dependencies at boundaries.** `WithCode` keeps the cause matchable;
  use `WithCodeOpaque` when translating a dependency error you do not want callers
  coupling to.
- **Not every project needs this.** For a small library or CLI, the standard library
  plus disciplined `fmt.Errorf` wrapping is usually enough. This package earns its
  keep in larger, long-lived services where the debugging cost of thin errors is
  real.

None of this makes the package wrong. It makes the trade-off explicit. If your
context lives naturally in per-request logs and your call graph is shallow and
synchronous, you may not need it.

## Drop-in Replacement

- Replace the import `errors` with `github.com/StevenACoffman/toerr/errors`.
- `Is`, `As`, `Unwrap`, and `Join` are re-exported unchanged.
- Replace `fmt.Errorf("some message: %w", err)` with
`errors.WrapWithMessage(err, "some message")` to gain location and attributes.
- Use the `sentinel` package for package-level sentinel values.

## Subpackages

| Package    | Purpose                                                                   |
| ---------- | ------------------------------------------------------------------------- |
| `errors`   | Primary API: `New`, `Wrap`, `WrapWithMessage`, `Mark`, `AsType`, `Attrs`. |
| `sentinel` | Cheap, stack-free sentinel values for `errors.Is` matching.               |
| `errcode`  | Transport-neutral status codes (`WithCode`/`Code`/`Status`/`Message`).    |
| `errhttp`  | HTTP adapter for `errcode`: maps a domain code to an HTTP status.         |
| `errclass` | Coarse severity classification that folds across joined errors.           |

Each subpackage has its own README:

- [`sentinel`](errors/sentinel/): stack-free sentinels that match by identity.
- [`errcode`](errors/errcode/): attach transport-neutral status codes to errors.
- [`errhttp`](errors/errhttp/): map a coded error to an HTTP status and safe message.
- [`errclass`](errors/errclass/): classify error severity for retry decisions.

## Prior Art

- [Failure is Your Domain](https://www.gobeyond.dev/failure-is-your-domain/) - Philosophy of Errors
- [braces.dev/errtrace](https://github.com/bracesdev/errtrace) - the Zig style return trace
- [remko/go-errors](https://github.com/remko/go-errors): the `errcode` model.
- [AnnotatedError](https://github.com/myrjola/sheerluck/blob/ba6715f2118eba0677889afb58d77f6f3f33f345/internal/errors/annotatederror.go#L24)
by @myrjola, message + `pc` + `slog.Attr`.
- [`xerrors` / `errcontext`](https://github.com/zircuit-labs/zkr-go-common-public/blob/dc1effe2259f5592f9c38fcc4079aeca0f555cd9/xerrors/errcontext/errcontext.go)
by @alif-zrc, attaching `slog.Attr` context.
- [errors/errors.go](https://github.com/upspin/upspin/blob/master/errors/errors.go)
