# Specification: Context-Based Log-Field Collector

Status: **draft / proposal — decision required.** Tracks TODO item 1 (Gap 3):
"Address the absence of a `context`-based field collector that also captures
success-path fields (cache hit/miss, durations) an error can never carry."

This spec exists to be reviewed *before* code. There is mature prior art
(`veqryn/slog-context`), so the headline question is **build vs adopt** (§5), and the
**dependency impact** (§4) is the central input to that decision; the rest specifies the
design for whichever path is chosen.

______________________________________________________________________

## 1. Purpose and Scope

A request-scoped collector that accumulates structured `slog.Attr` fields across the
layers of one request and makes them available at the boundary — for **both** the
success and the failure paths.

- **Success path:** fields an error can never carry (`cache=hit`, `db_duration=8ms`,
  `rows=42`), because on success there is no error.
- **Failure path:** the same request-scoped fields, merged with the error's own deep
  attributes (`errors.Attrs(err)`).

It is the "Option B" mechanism from the logging analysis: move context accumulation
off the error value and into the request `context`.

**Non-goals:** the log transport (the caller owns the `slog.Logger`); tracing/span
propagation; sampling. (Demoting `errors`' `LogValuer` — the incentive half of the
log-once fix — is handled separately, done under TODO 1a.)

______________________________________________________________________

## 2. The Problem This Must Solve

The naive implementation — a pointer to a mutable field list stashed once in the
`context` and appended to as the stack descends — has two failure modes:

1. **Data race.** A `context` is inherited by child goroutines. Two parallel branches
   appending to one shared holder are concurrent writers to shared mutable state — a
   race that `go test -race` (CI runs `GOFLAGS=-trimpath -race`) will fail.
2. **Silent key collision.** Parallel branches sharing an inherited context are
   *likely* to reuse keys (`duration`, `id`, `count`). On collision the pertinent data
   is lost or ambiguous — Kleppmann's Last-Write-Wins hazard ("writes are silently
   dropped with no error to the application", `kleppmann_rules.md` §6). `slog`'s
   built-in handlers do **not** deduplicate, so slog will not save us.

The design must make both **impossible by construction** (Ousterhout §6, "define the
special case out of existence"), not merely detectable.

______________________________________________________________________

## 3. Prior Art

### 3.1 `github.com/veqryn/slog-context` (`slogctx`)

A mature, published library (Go 1.21, `-race` in CI, with otel/grpc/http/logr
adapters). It already implements **both** concurrency models this spec weighs, plus an
emit model this spec does not have:

- **`Prepend` / `Append`** (`attrs.go`) — **immutable copy-on-write**:
  `context.WithValue(parent, key, append(slices.Clip(v), new...))`. `slices.Clip`
  forces a fresh backing array so concurrent adds can't alias; deep additions are
  therefore **not** visible to ancestors. This is Model 2 in §6.1.
- **`propagate`** (+ the `sloghttp.AttrCollection` middleware and `sloghttp.With`) —
  **mutex-guarded shared holder**: a `*syncAttrs{ sync.RWMutex; []slog.Attr }` stored
  once in the context; `With` appends under `Lock` and returns the *same* context (a
  shared pointer, so additions *are* visible to ancestors); `ExtractAttrs` returns a
  copied slice under `RLock` (a snapshot). This is Model 3 in §6.1.
- **`Handler`** — a wrapping `slog.Handler` that injects context attrs into **every**
  log line via pluggable `AttrExtractor`s (including OTel trace/span). This is a
  *per-line* emit model, not the *log-once-at-the-boundary* model this spec assumes.

What it validates (this spec's decisions, confirmed by a shipping library): copy-on-
write for the common case; a mutex on the shared holder; a **snapshot on read**;
private `context`-key types; graceful nil/uninitialized handling; reuse of slog's own
arg-to-attr parsing.

**What it does *not* solve — the crux.** `propagate` is a **flat, un-namespaced,
ordered append** (its own comment: "append-only synchronized ordered slice"). It is
**race-safe** (mutex) but **not collision-safe**: two parallel branches that both
`With(ctx, "duration", …)` produce two `duration` attrs in one slice, which slog emits
verbatim → the silent-loss hazard of §2.2 is unaddressed.

### 3.2 `github.com/veqryn/slog-dedup` (`slogdedup`)

The ecosystem's answer to collisions is a **separate** library: a sink-level middleware
that **deduplicates and sorts** attributes at handling time (keep-first / keep-last /
keep-all-with-suffix). So slog-context's philosophy is **dedup at the sink**, with a
policy that determines whether data is lost (keep-last) or preserved-without-provenance
(`duration`, `duration#01`).

This contrasts with this spec's philosophy — **avoid collisions at write time** via
per-scope namespacing (`slog.Group`), which preserves both values *with provenance*
(`shard_3.duration` vs `shard_4.duration`) and needs no sink dedup.

______________________________________________________________________

## 4. Dependency Impact

toerr today has **zero** third-party dependencies: a bare `go.mod` and an **empty
`go.sum`**. For a foundational, drop-in replacement for the standard library `errors`
package this is a deliberate, load-bearing property — every downstream consumer inherits
toerr's dependency graph, so a dependency added *here* propagates to *all* of them.

Adopting slog-context is therefore toerr's **first** external dependency. Measured with
`go mod tidy` (local `replace`, offline):

| Path                                             | New `require` in toerr `go.mod`                                | Recorded in `go.sum`    | Compiled into binaries                     |
| ------------------------------------------------ | -------------------------------------------------------------- | ----------------------- | ------------------------------------------ |
| **Build from scratch** (`logctx`, §6)            | none                                                           | none                    | none — stdlib `context`/`slog`/`sync` only |
| **Adopt — import root `slogctx`**                | `veqryn/slog-context` (direct) **+ `go-logr/logr` (indirect)** | both                    | slog-context **+ logr**                    |
| **Adopt — import only `slog-context/propagate`** | `veqryn/slog-context` (direct)                                 | slog-context **+ logr** | slog-context (logr **not** linked)         |

Notes for reviewers:

- **The root package links `logr`** because `slogctx/ctx.go` (the logger-in-context
  workflow) imports `github.com/go-logr/logr`; importing the package compiles all of it.
- **Importing only `propagate` avoids linking `logr`** (that subpackage is stdlib-only),
  and `go mod tidy` keeps `logr` out of `require` — but **`logr`'s checksums still land
  in `go.sum`**, because the pruned module graph still references it via slog-context's
  own `go.mod`. So even the leanest adopt path enlarges the supply-chain surface.
- **otel and grpc are separate nested modules** (their own `go.mod`); their heavy trees
  (OTel SDK, gRPC, protobuf) are **not** pulled unless those subpackages are imported.
- **Direction of travel.** Adding the first dependency is easy; removing it later is a
  module-graph change every consumer feels. Each dependency is also a version to track
  and a supply-chain surface to vet.

**Bottom line:** build = **0** new modules; adopt = **1 direct** (+1 indirect *linked*
via the root package, or +1 recorded in `go.sum` via `propagate`). This is the central
cost side of the trade in §5.

______________________________________________________________________

## 5. The Decision: Adopt Vs Build

### 5.1 Option a — Adopt Slog-Context + a `Scope` Namespacing Layer

Use `slog-context/propagate` (or the http middleware) for the shared collector, and add
a **thin `Scope` layer on top** to supply the collision-safety propagate lacks: a small
wrapper that requires each parallel branch to write its attrs under a `slog.Group`
before `propagate.With`, e.g.

```go
// Scope wraps propagate.With so a branch's fields land under one group.
func Scope(ctx context.Context, name string, attrs ...slog.Attr) {
    propagate.With(ctx, slog.Group(name, attrsToArgs(attrs)...))
}
```

Optionally add `slog-dedup` at the sink for within-scope duplicates.

- **Pros:** battle-tested; snapshot/mutex/copy-on-write already correct; otel/grpc/http/
  logr adapters; less code to own.
- **Cons:** toerr's first dependency (§4); the per-line `Handler` emit model does **not**
  enforce log-once; collision-safety is bolted on by convention, not enforced — the base
  library's flat ordered slice stays collision-prone if a caller bypasses the `Scope`
  wrapper and calls `propagate.With` directly.

### 5.2 Option B — Build `logctx` from Scratch (§6)

A ~90-line stdlib-only package where `Scope` namespacing is first-class and
collision-safety holds **by construction**.

- **Pros:** keeps toerr at **zero dependencies** (§4); write-time collision avoidance
  with provenance (no sink dedup needed); single canonical-line emit model (serves the
  log-once goal); tuned exactly to `errors.Attrs` integration.
- **Cons:** code to own and test; no otel/grpc adapters (out of scope anyway);
  reinvents ~80% of what slog-context already does.

### 5.3 Recommendation

**Build (Option B)** unless the otel/grpc/logr ecosystem is a near-term need. Rationale:
the dependency in §4 is toerr's *first*, against an empty `go.sum` that is part of the
package's identity; and the one capability that actually motivated this spec — collision-
safe parallel accumulation with provenance — is precisely what adoption does **not**
give (it lives in the `Scope` layer either way), so adopting buys maturity for the parts
that are easy while still requiring the custom part. If the team weights ecosystem over
zero-dep, choose Option A and treat §6's API as the shape of the `Scope` wrapper.

(If reuse-without-dependency is wanted, vendoring `propagate` is possible only if
`slog-context/LICENSE` permits — verify first — but reimplementing per §6 is cleaner and
adds the collision-safety regardless.)

______________________________________________________________________

## 6. Recommended Design (The Build Path)

Design principles applied (rule references name the rule, to avoid confusion with this
document's section numbers). Ousterhout: *make modules deep* and *pull complexity
downward* — the module owns locking, namespacing, and snapshotting, so callers see 3–4
trivial calls; *define special cases out of existence* — collisions and races are removed
by structure, not runtime checks; *information hiding*; *decompose by knowledge, not
call-order*. Kleppmann: *conflict avoidance over last-write-wins*; *immutable values
across goroutines with a single owner*; *consistent snapshot reads*; *append-only and
order-independent*.

### 6.1 Options for the Concurrency Model

| Model                                                                      | Deep additions visible at boundary? | Race-free? | Collision-free? | Verdict                        |
| -------------------------------------------------------------------------- | ----------------------------------- | ---------- | --------------- | ------------------------------ |
| 1. Unguarded shared holder                                                 | yes                                 | ✗          | ✗               | Rejected (the §2 anti-pattern) |
| 2. Immutable copy-on-write (= slog-context `Prepend`/`Append`)             | **no**                              | ✓          | ✓ per-branch    | Secondary mode                 |
| 3. Guarded shared holder + per-branch scopes (= `propagate` **+ `Scope`**) | yes                                 | ✓ (mutex)  | ✓ (scopes)      | **Recommended**                |

Model 3 is `propagate`'s design **plus** the `Scope` namespacing that closes its
collision gap.

### 6.2 API Surface (Illustrative)

```go
package logctx // github.com/StevenACoffman/toerr/errors/logctx

// Init installs a fresh collector. Call once at the request boundary. Add/Attrs
// on a ctx without Init are a safe no-op / empty read, so a missed Init degrades
// gracefully.
func Init(ctx context.Context) context.Context

// Add records fields under ctx's current scope. Safe from multiple goroutines.
// Within one scope a repeated key is last-write-wins; across scopes keys never collide.
func Add(ctx context.Context, attrs ...slog.Attr)

// Scope derives a child context whose fields are namespaced under name. Give each
// parallel branch its own scope so their fields cannot collide.
func Scope(ctx context.Context, name string) context.Context

// Attrs returns an immutable snapshot, each scope rendered as a slog.Group. Call
// once at the boundary emit.
func Attrs(ctx context.Context) []slog.Attr
```

### 6.3 Semantics

- **Storage:** one `*collector{ sync.Mutex; map[scopePath]orderedAttrs }` installed by
  `Init` under an unexported `context` key; `Scope` returns a new context carrying the
  **same** collector pointer plus an extended scope path (so deep/parallel additions are
  visible at the boundary). `Add` locks and writes the ctx's scope bucket; `Attrs` locks
  and deep-copies into nested `slog.Group`s (the snapshot).
- **Collisions:** across scopes — impossible by construction (distinct `slog.Group`s).
  Within one scope, same key — last-write-wins, documented.
- **Fan-out (mandated shapes):** (A) each branch `Scope`s under a distinct name before
  `Add` (mutex-safe, collision-safe); or (B) immutable fan-in — each goroutine returns
  its own `[]slog.Attr`, the owner merges them under per-branch groups (no shared mutable
  state at all, Kleppmann-preferred). Appending to an unscoped shared ctx from multiple
  goroutines is a documented anti-pattern.

______________________________________________________________________

## 7. Integration with `errors`

At the single boundary emit, merge request-scoped fields with the error's deep attrs:

```go
attrs := logctx.Attrs(ctx)
if err != nil {
    attrs = append(attrs, errors.Attrs(err)...) // deep failure context from New/Wrap
}
logger.LogAttrs(ctx, level, msg, attrs...)
```

`logctx` does not import `errors` (or vice versa); the merge is at the call site, so no
cycle. `logctx` depends only on `context`/`slog`/`sync` — no I/O — preserving the
zero-dependency property (§4). This closes Gap 3 while `errors.Attrs` continues to carry
failure-path fields, and enables the "log once" resolution.

______________________________________________________________________

## 8. Testing Requirements

Stdlib `testing` only; `package logctx_test`; exported-API; no type-system tautologies.

1. Round-trip: `Init` → `Add` → `Attrs`.
2. Scope namespacing: same leaf key under `Scope("a")` and `Scope("b")` both survive as
   `a.k` / `b.k` (assert rendered slog output).
3. Within-scope LWW (documented behavior).
4. Uninitialized ctx: `Add`/`Attrs` on `context.Background()` are no-op/empty (no panic).
5. **Concurrency (load-bearing):** N goroutines each `Scope`+`Add`, then `Attrs`, under
   `-race`; must be race-free **and lossless** (every branch's field present under its
   group). This is the proof that §2's two failure modes are designed out — and the test
   that would *fail* against `propagate` alone (no namespacing), motivating the `Scope`
   layer whichever path is chosen.
6. Snapshot isolation: `Attrs` during a concurrent `Add` returns a coherent slice.
7. Integration: boundary merge with `errors.Attrs(err)`.

______________________________________________________________________

## 9. Open Questions

1. **Build vs adopt (§5) — the headline decision**, driven mainly by the dependency
   impact (§4). Everything below assumes the outcome; confirm first.
2. Package name/placement: `errors/logctx` (proposed) vs top-level.
3. Scope nesting: flat vs nested groups (proposal: nested via repeated `Scope`).
4. Within-scope duplicate policy: LWW (proposed) vs keep-both vs configurable.
5. Output ordering: insertion order (proposed); correctness must not depend on it.
6. `LogValuer` follow-up: per-error `LogValuer` has already been demoted (TODO 1a, done);
   the remaining question is whether to also add lint enforcement of "log once" (item 1).

______________________________________________________________________

## 10. Summary

Prior art (`veqryn/slog-context`) already implements the copy-on-write and mutex-guarded-
shared models — validating those choices — but its shared collector is **race-safe, not
collision-safe**, and it punts collisions to a separate sink-level library
(`slog-dedup`). The one capability that motivated this spec — collision-safe parallel
accumulation with provenance, via write-time `Scope` namespacing — is *not* in the
off-the-shelf library and must be built either way. Given toerr's zero-dependency
`go.sum` (§4), the recommendation is **build `logctx`** (stdlib-only, `Scope`
first-class); adopt slog-context only if its otel/grpc/logr ecosystem outweighs taking
toerr's first dependency, in which case §6's API becomes the shape of the `Scope` layer
added on top of `propagate` (with `slog-dedup` optional at the sink).
