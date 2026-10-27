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
return fmt.Errorf("dial database %s: %w", host, err)
// dial database db1: connection refused

// This package: what failed, and where.
return errors.Wrapf(err, "dial database %s", host)
// dial database db1: connection refused
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
dependency and handled by no one on the way up. That engineer can ask three
questions of it, and a failure leaves a different record for each:

| Record        | Answers                                                  | `fmt.Errorf` keeps | This package adds           |
| ------------- | -------------------------------------------------------- | ------------------ | --------------------------- |
| Context chain | What was the program trying to do?                       | the messages       | `slog.Attr` beside each one |
| Return trace  | Which layers passed the error on instead of handling it? | nothing            | one frame per `New` or wrap |
| Stack trace   | How did the program reach the creation site?             | nothing            | only for a recovered panic  |

- **The context chain carries intent.** `charge card: authorize payment: connection
refused` says what failed in the program's own terms, and nothing else does. `%w`
keeps that chain of messages. But the identifiers and state that explain the failure
are in scope only near the origin, and a plain string carries them forward only as
text to grep.
- **The return trace carries propagation.** Each frame marks a function that saw the
error and returned it instead of handling it. Without frames, "where did this come
from?" means grepping the source for message fragments.
- **The stack trace carries call structure at creation.** It is the record to want
when the bug is at the creation site, and it is the one this package mostly leaves
out (see [Return Traces, Not Stack Traces](#return-traces-not-stack-traces)).

Apart from the three records, a caller needs to know **what kind of failure it is**.
Reacting to a category (not found, rate limited, retryable) across layers of wrapping
takes more than comparing message strings.

This package keeps the **context chain** and adds structured **`slog.Attr`** to it,
adds the **return trace**, and gives type **marks** and codes for *what kind*.

## What You Get

- **[Messages that say what was attempted](#messages-and-location)**: `Wrapf` is as
short as `fmt.Errorf`, and every wrap also records its file, line, and function.
- **[Structured context](#structured-attributes)**: `New`, `Wrap`, and
`WrapWithMessage` take trailing `slog.Attr` values, surfaced at your log boundary with
`errors.LogValue` / `errors.Attrs`.
- **[Request fields on the success path too](#request-fields)**: `logctx` collects
fields found anywhere in a request, on its context, for the request's one log line.
- **[A return trace](#return-trace)** under `%+v`, not a heavyweight stack trace.
- **[Type marks for control flow](#marks-and-astype)**: `Mark` / `AsType` tag an
error by type without disturbing its message or `Unwrap` chain, and `Seal` does the
opposite: it hides an error from matching while operators still see all of it.
- **[A true drop-in](#drop-in-replacement)**: `Is`, `As`, `Unwrap`, `Join`
re-exported unchanged, and foreign errors interoperate.

Intended for larger projects where the trade-offs of a batteries-included error
package outweigh YAGNI.

```go
import errors "github.com/StevenACoffman/toerr/errors"
```

## The Context Chain

### Messages and Location

[`New`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#New) creates a
leaf error and records the call site.
[`Wrapf`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#Wrapf) prefixes a
formatted message to an existing error, as `fmt.Errorf` does, and records the call
site.
[`WrapWithMessage`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#WrapWithMessage)
does the same with a fixed message and trailing attributes.
[`Wrap`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#Wrap) records only
the call site, for a hop that adds nothing about intent.

```go
err := errors.New("connect")
err = errors.Wrapf(err, "dial database %s", "db1")
fmt.Println(err) // "dial database db1: connect"
```

Write a message wherever the code knows what it was attempting. That message is the
only record of intent, because a frame names only a function and a line. Unlike
`fmt.Errorf("...: %w", err)`, which only concatenates strings, every one of these also
captures the file, line, and function at the point it is called.

### Structured Attributes

`New`, `Wrap`, and `WrapWithMessage` take trailing `slog.Attr` values for context
that belongs in structured logs rather than in the message:

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

`Attrs` walks the whole tree, including every branch of an `errors.Join`. An error
type declared outside this package contributes its own fields by implementing
`Attrs() []slog.Attr`, the method counterpart of the function, as `Unwrap` is to
`errors.Unwrap`. That way the fields are not copied into a `Wrap`.

The flat list repeats a key that two errors both set, such as two siblings in a join.
To log the error with its shape, ask for it explicitly with
[`LogValue`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#LogValue):

```go
logger.LogAttrs(ctx, slog.LevelError, "request failed", slog.Any("error", errors.LogValue(err)))
```

A JSON handler then writes the error as an object. Each error in a chain is a layer
with its own message, call site, and attributes, and each branch of a join is a tree
of its own, so two errors' `op` never meet in one object:

```json
"error": {
  "msg": "a failed: b failed\nc failed: d failed",
  "chain": [{"msg": "a failed", "at": "/src/svc.go:40", "attrs": {"op": "a"}}],
  "errors": [
    {"chain": [{"msg": "b failed", "at": "/src/db.go:12", "attrs": {"op": "b"}}]},
    {"chain": [{"msg": "c failed", "at": "/src/cache.go:20", "attrs": {"op": "c"}},
               {"msg": "d failed", "at": "/src/cache.go:31", "attrs": {"op": "d"}}]}
  ]
}
```

A layer also shows an `errcode` code, an `errclass` class, a `Mark`, and a `Seal`,
whose hidden cause it still describes. A layer from another package's error, such as a
`*pq.Error`, also names its Go `type`. Attribute values are resolved first, so a
`slog.LogValuer` that redacts a secret still redacts it.

With a logger other than slog, such as zerolog's `Interface` or a canonical-line
library's `Add`, pass the value inside: `errors.LogValue(err).Any()` encodes itself as
JSON, while the `slog.Value` around it encodes as `{}`.

Errors deliberately do **not** implement `slog.LogValuer` themselves, so passing one
straight to a logger does not expand it at every layer.

Google's Go client for Cloud Logging decodes a JSON payload into a map, so a repeated
key keeps one value; `LogValue`'s object repeats none. When those logs are exported to BigQuery, note
that a column nests at most 13 levels (each join inside a join adds two), that
numbers become floats (log an ID above 2^53 as a string), and that an attribute key
must keep one type across call sites.

The reasoning behind carrying `slog.Attr` on the error is in
[Design → Context as `slog.Attr`](#context-as-slogattr).

### Request Fields

Fields such as the authenticated user or a cache hit describe the whole request, and
a request that succeeds has no error to carry them. The
[`logctx`](errors/logctx/) package keeps them on the request's context.
`logctx.Init` installs a collector where the request starts, `logctx.Add` records a
field from any layer, and `logctx.Attrs` returns them all for the request's one log
line, beside the error when it failed:

```go
func serve(ctx context.Context) (err error) {
	ctx = logctx.Init(ctx)
	defer func() {
		recovered := recover()
		if p := errors.Recover(recovered); p != nil {
			err = errclass.WrapAs(p, errclass.Panic)
		}
		level, msg, attrs := slog.LevelInfo, "request served", logctx.Attrs(ctx)
		if err != nil {
			level, msg = slog.LevelError, err.Error()
			attrs = append(attrs, slog.Any("error", errors.LogValue(err)))
		}
		logger.LogAttrs(ctx, level, msg, attrs...)
		if recovered != nil {
			panic(recovered) // net/http, or the goroutine's own recover, still sees it
		}
	}()
	return next(ctx)
}
```

The line is written in a `defer`, so a handler that panics still gets it, at error
level with the panic's stack; the panic then carries on as before.

It is safe for the goroutines a request starts. `logctx.Scope` gives each parallel
branch a namespace of its own, so two branches' `duration` fields render as
`shard_3.duration` and `shard_4.duration` instead of overwriting each other.

## Return Trace

Each `New` and wrap records a single frame for its own call site. Together they
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
Go 1.26 `errors.AsType`, so it returns the found value and a boolean. A sentinel
works as a marker too: `errors.Is(errors.Mark(err, ErrQuota), ErrQuota)` is true.

`Mark` exists for errors whose type you do not control. If you construct the error
type yourself, give it an `Unwrap` or `Is` method that returns your sentinel instead.
That keeps identity matching inside the type and needs no extra wrapper.

```go
type NotFoundError struct{ error }

err = errors.Mark(sql.ErrNoRows, &NotFoundError{})
if _, ok := errors.AsType[*NotFoundError](err); ok {
// control flow, anywhere up the chain
}
```

[`Seal`](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors#Seal) is the
opposite of `Mark`. `Mark` adds a match; `Seal` removes every match beneath it, so
`Is`, `As`, `AsType`, `AsBehavior`, `errcode`, and `errclass` all stop at the seal.
The message, the `%+v` trace, `Attrs`, and `LogValue` are unchanged. Seal a
dependency's error where it enters your domain, then mark or code it outside the seal,
so callers match your meaning and never the dependency's:

```go
err = errors.Mark(errors.Seal(err), ErrNotFound) // matches ErrNotFound, not sql.ErrNoRows
```

Seal the cause, not the result: a seal around a mark or a code hides that too.

## Design

This package makes three decisions that plain `fmt.Errorf` wrapping does not, each
about a record from [The Problem](#the-problem). It records a return trace and no
stack trace, it carries the context chain's evidence as `slog.Attr`, and it keeps an
error's own structure in typed fields.

### Return Traces, Not Stack Traces

**A stack trace shows how an error was born. In production you usually need to know
how it got away.**

An error surfaces at the top of a request handler. It was created deep in some
dependency, handled by no one on the way up, and is now being read by an on-call
engineer who did not write the code that produced it. The reader needs three answers:
what was the program trying to do, where did this come from, and why did nothing deal
with it sooner? The first belongs to the context chain, the messages and attributes,
and neither kind of trace answers it. The traces split the other two.

A stack trace answers only where it came from, and it answers at a cost. It is
captured once, at creation, for the goroutine that was running then. The instant the
error is buffered, sent over a channel, or returned from a worker, that recorded
stack describes a call structure that no longer has anything to do with how the
error reached you. It is also complete: every runtime and library frame is included,
so the frames of your own code are buried in dispatch noise. And it points at the
creation site, which the context chain has often already named.
`connect to db1: connection refused` rarely leaves you wondering *what* the operation
was.

A return trace answers the last question, the one you could not already answer. It
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

- **It follows the value, not the goroutine.** Across channels, buffers, and worker
pools, where a creation-time stack becomes useless, it still shows which layers
passed the error on. It cannot show which task the error came from: the frames
after the hop record plumbing, not purpose. Attach the task's identity as a message
or attribute where the task starts.
- **Its length is a signal.** The number of frames is the number of layers that
declined to handle the error. This length is the error's escalation distance. A long return trace is a
measurement, not merely a location.
- **It is short.** Frames appear only where your code wrapped, so there is no runtime
tail and no dispatch noise to read past. Each frame is still mechanical, a function
and a line, so it locates intent in the source without stating it.
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

### What a Frame Costs

A Wrap costs about 135 ns, and roughly 120 ns of that is the `runtime.Callers` call.
I measured on an Apple M1 Pro, 6 runs each; the numbers barely varied between runs.

| Benchmark                                   | ns/op | B/op | allocs/op | Calls `runtime.Callers`? |
| ------------------------------------------- | ----: | ---: | --------: | ------------------------ |
| `Wrap`                                      |   134 |   64 |         1 | yes                      |
| `WrapWithMessage`                           |   134 |   64 |         1 | yes                      |
| `Wrapf`                                     |   200 |   80 |         2 | yes                      |
| `New`                                       |   135 |   64 |         1 | yes                      |
| `errcode.New`                               |   136 |   48 |         1 | yes                      |
| `errcode.Link`                              |   133 |   48 |         1 | yes                      |
| `errcode.Link` over a `Seal`ed cause        |   148 |   64 |         2 | yes                      |
| `Mark` on an error that already has a trace |    14 |   32 |         1 | no                       |
| `Seal`                                      |     9 |   16 |         1 | no                       |
| `fmt.Errorf("…: %w")`                       |    76 |   48 |         2 | no                       |
| `runtime.Callers(3, pcs[:1])` alone         |  ~128 |    0 |         0 | (it is the call)         |

These numbers are from Go 1.27.1 on arm64, and the `errcode`, `Seal`, and `Wrapf`
rows from Go 1.27.2 (3 runs each). `Wrapf` adds the `Sprintf` that `fmt.Errorf` also
pays. A
copy of the same error struct without the frame capture costs 14 ns. The capture
itself allocates nothing, so the one allocation is the 64-byte error (48 bytes for a
coded one); attributes add a second. Stack depth doesn't change the cost. Against
`fmt.Errorf`, a 10-hop chain costs about 0.5 µs more to build (1.3 µs against 0.9 µs),
and it uses fewer bytes (640 B against 920 B). It is cheap next to any I/O and
matters only in hot loops that create and discard errors as normal control flow. Use a
sentinel there. Reading the trace costs more than building it. On a 10-hop chain, `%+v`
takes about 4 µs, and `Error()` rebuilds the message on every call, about 0.3 µs each.

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
  with `errors.LogValue`. Lower layers wrap (for location) and
  return, not log. A field a lower layer wants in that line goes to `logctx.Add`,
  which is how a canonical log line gets built without a log call per layer.
- **Do not leak dependencies at boundaries.** `errcode.Link` keeps the cause
  matchable; `errors.Seal` the cause first when translating a dependency error you do
  not want callers coupling to.
- **Not every project needs this.** For a small library or CLI, the standard library
  plus disciplined `fmt.Errorf` wrapping is usually enough. This package earns its
  keep in larger, long-lived services where the debugging cost of thin errors is
  real.
- An error can be planful state and it doesn't necessarily mean something went wrong. 
  It carries semantic meaning that may not change until the API changes. If an error has 
  no locus (clear point of origin or structural anchor) then associating a trace makes
  no sense. If an error is a normal operational result used as a standard 
  control-flow value or an opaque, lightweight "domain value", use a simple sentinel.

None of this makes the package wrong. It makes the trade-off explicit. If your
context lives naturally in per-request logs and your call graph is shallow and
synchronous, you may not need it.

## Drop-in Replacement

- Replace the import `errors` with `github.com/StevenACoffman/toerr/errors`.
- `Is`, `As`, `Unwrap`, and `Join` are re-exported unchanged.
- Replace `fmt.Errorf("load %q: %w", path, err)` with
`errors.Wrapf(err, "load %q", path)` to gain location. Where logs should query an
identifier, move it into an attribute:
`errors.WrapWithMessage(err, "charge card", slog.Int("order", id))`.
- Use the `sentinel` package for package-level sentinel values.

### Linting an Adopting Project

`wrapcheck` reports an error from another package that is returned without a wrap.
It already counts `Wrap`, `Wrapf`, `errors.New`, and `Join` as wraps. Add the rest of
this package's constructors to your `.golangci.yaml`:

```yaml
linters:
  settings:
    wrapcheck:
      extra-ignore-sigs:
        - .WrapWithMessage(
        - .Mark(
        - errcode.New(
        - .Link(
        - .WithCode(       # deprecated; drop once migrated to Link
        - .WithCodeOpaque( # deprecated; drop once migrated to Link
        - errclass.WrapAs(
        - errgrpc.Error(
```

Leave `.Seal(` out. A seal records no frame, so a bare `return errors.Seal(err)` is a
hop with no location, which is what `wrapcheck` should report; wrap it with `Link`,
`Mark`, or `Wrap`.

## Subpackages

| Package    | Purpose                                                                                                |
| ---------- | ------------------------------------------------------------------------------------------------------ |
| `errors`   | Primary API: `New`, `Wrap`, `Wrapf`, `WrapWithMessage`, `Mark`, `Seal`, `AsType`, `Attrs`, `LogValue`. |
| `sentinel` | Cheap, stack-free sentinel values for `errors.Is` matching.                                            |
| `errcode`  | Transport-neutral status codes (`New`/`Link`/`Code`/`Status`/`Message`).                               |
| `errhttp`  | HTTP adapter for `errcode`: maps a domain code to an HTTP status.                                      |
| `errgrpc`  | gRPC adapter for `errcode`, in both directions. A module of its own.                                   |
| `logctx`   | Request-scoped log fields for the request's one log line, on success and failure.                      |
| `errclass` | Coarse severity classification that folds across joined errors.                                        |

Each subpackage has its own README:

- [`sentinel`](errors/sentinel/): stack-free sentinels that match by identity.
- [`errcode`](errors/errcode/): attach transport-neutral status codes to errors.
- [`errhttp`](errors/errhttp/): map a coded error to an HTTP status and safe message.
- [`errgrpc`](errors/errgrpc/): map a coded error to a gRPC status, and a client
  call's code back to a domain code.
- [`logctx`](errors/logctx/): collect a request's log fields and write them once.
- [`errclass`](errors/errclass/): classify error severity for retry decisions.

## Prior Art

- [Failure is Your Domain](https://www.gobeyond.dev/failure-is-your-domain/) - Philosophy of Errors
- [Ow! My Foot!](https://n8z.dev/posts/ow-my-foot) - errors across languages, error
  domains and blast radius, and the rakes stepped on
- [braces.dev/errtrace](https://github.com/bracesdev/errtrace) - the Zig style return trace
- [remko/go-errors](https://github.com/remko/go-errors): the `errcode` model.
- [AnnotatedError](https://github.com/myrjola/sheerluck/blob/ba6715f2118eba0677889afb58d77f6f3f33f345/internal/errors/annotatederror.go#L24)
by @myrjola, message + `pc` + `slog.Attr`.
- [`xerrors` / `errcontext`](https://github.com/zircuit-labs/zkr-go-common-public/blob/dc1effe2259f5592f9c38fcc4079aeca0f555cd9/xerrors/errcontext/errcontext.go)
by @alif-zrc, attaching `slog.Attr` context.
- [errors/errors.go](https://github.com/upspin/upspin/blob/master/errors/errors.go)
