# TODO

Outstanding items for `toerr`, in priority order. Derived from
`~/Documents/agent-orange/toerr_analysis.md` (comparison against the
error-handling advice corpus). Each item notes its source finding, rough effort,
and rationale.

Resolved and therefore **not** listed: chain-severing at boundaries
(`errcode.WithCodeOpaque`), and acknowledging the enrich-vs-log debate (README's
"unsettled debate" section). Deliberately **out of scope**: namespaced/global
domain codes (Gap 7) — a documented, intentional divergence, not a defect.

______________________________________________________________________

## 1. Reconcile the Logging Guidance — High Impact, Medium Effort

- [ ] Resolve the contradiction between "log once" (preached in `philosophy.md`)
      and shipping `slog.LogValuer` on every error, which makes per-layer
      `logger.Error("…", "err", err)` frictionless — the stacked-logline
      anti-pattern (Gap 4).
- [ ] Address the absence of a `context`-based field collector that also captures
      **success-path** fields (cache hit/miss, durations) an error can never carry
      (Gap 3).
- Options: add a `ctx` field-collector helper (`AddLogField(ctx, …)` style),
  and/or scope `LogValuer` to "log at the boundary only" with an explicit
  warning against per-layer logging.
- Source: Counterarguments 3; Gaps 3-4; analysis next step #3 (the top-ranked
  remaining actionable item).

### 1A. Scope `slog.LogValuer` to Boundary-Only (The Incentive Half of Gap 4) — DONE

- [x] Remove `LogValue()` from `annotatedError`/`marked` (and the `_ slog.LogValuer`
      compile assertions) so passing an error to slog no longer auto-expands into a
      structured group at every layer.
- [x] Expose `errors.LogValue(err) slog.Value` as an explicit boundary helper (the old
      grouped rendering, now opt-in); `errors.Attrs(err)` remains the flat-merge option.
- [x] Add a doc warning (philosophy.md / README) that rich structured error logging is a
      deliberate boundary act via `errors.Attrs` / `errors.LogValue`, not a per-layer habit.
- Note: this is the *incentive* half of Gap 4. The *enforcement* half — a lint that flags
  error-to-logger calls outside boundary packages — remains open under item 1.

## 2. Document "When Not to Use This" + Wrap-Density Guidance — Medium Impact, Low Effort — DONE

- [x] "When not to reach for this" list — already carried by the README's
      "When Not to Reach for This" section (Cheney debate, shallow/synchronous call
      graphs, "not every project needs this").
- [x] Wrap-at-every-hop vs wrap-at-boundaries — added the "How Often to Wrap"
      subsection to `philosophy.md`: the mechanism is neutral (a bare `Wrap` adds a
      frame with no message), so density is the caller's choice; wrap where a frame
      adds information. Cross-links the README section rather than duplicating it.
- Source: Counterargument 2; remainder of analysis next step #2.

## 3. Draw the `panic` App-Vs-Library Line — Low-Medium Impact, Very Low Effort — DONE

- [x] In "Knowing When to Give Up," state that the `panic`-on-invariant advice is
      app-facing, and that library code should not panic across its boundary
      (recover internally). The guidance ships inside a library, so a reader could
      misapply it.
- Done: appended to the unrecoverable-failures bullet in `philosophy.md` — library
  code returns errors (recovering internally with `errors.Recover`), reserving
  panics for programmer misuse or unrecoverable invariants.
- Source: Counterargument 6.

## 4. PC-Capture Cost Story + Opt-Out — Medium Impact, Medium Effort

- [x] Document the per-wrap `runtime.Callers` cost (paid on every `Wrap`, even for
      swallowed errors). Done: README "What a Frame Costs", backed by
      `errors/bench_test.go`, with allocations gated by `benchgate` in CI.
- [ ] Consider a hot-path opt-out or a build-tag / debug-gated PC capture.
- Source: Gap 5; analysis next step #4 (optional).

## 5. Recover-at-Boundary Helper — Medium Impact, Medium Effort — DONE

- [x] Provide a helper that turns a recovered panic into a traced error, serving
      the "packages recover internally at their boundary" guidance.
- Done: `errors.Recover(recover())` (`errors/recover.go`, tests in
  `errors/recover_test.go`). Returns nil when there was no panic, preserves a
  recovered error as the cause so `errors.Is`/`As` still reach it, records a frame
  at the recovery site, and attaches the panic stack as a `"stack"` `slog.Attr`
  rather than in the message (honoring return-trace-not-stack-trace).
- Source: Gap 6; analysis next step #5 (optional).

## 6. Behavior-Interface Idiom — Low/niche Impact, Medium Effort — DONE

- [x] Provide an "ask the error a question" pattern to complement identity
      (`sentinel`), type (`AsType`/`Mark`), and stored classification (`errclass`).
- Done: `errors.AsBehavior[T]` (`errors/behavior.go`, tests in
  `errors/behavior_test.go`) — the behavior counterpart to `AsType` for interfaces
  that do not embed `error`. Blessed in `philosophy.md` ("React by Behavior"),
  including the `Mark`-to-attach trick and when to prefer it over code/class/sentinel.
  Named domain predicates (`Temporary`/`Retryable`) are deliberately left to the
  application, per the mechanism/domain split.
- Source: Gap 2.

## 7. `Error()` Actionable Vs Operator-Only — Low Impact, Likely Won't-Fix

- [ ] Decide whether to note the trade-off: `toerr` deliberately makes `Error()`
      operator-only (users read `errcode.Message`), whereas some advice wants
      `Error()` self-sufficient. Mostly a "document the trade-off" item, not a change.
- Source: Counterargument 5.

______________________________________________________________________

## 8. Adoption / Rollout Across the skillet Family — the real open lever

The library is feature-complete at **v0.1.0** (HEAD == tag; the items above are docs
and enforcement, not core code). Its actual gap is **adoption**: toerr is the intended
consolidated replacement for the hand-rolled error machinery in the skillet family, but
uptake is nascent. Current state (2026-08-05 survey):

| Repo | toerr use |
| --- | --- |
| **canonizer** | direct `v0.1.0` — the reference adopter |
| **skillet** | depends on `v0.1.0`, but imports it in **one** file (`ruleset/synthesize`) while still shipping its own `errs` (Ben Johnson `Error`) for `proof` + the kernel |
| **exegesis** | transitive only (via skillet) |
| **skillsaw**, **adh** | not adopted (adh re-exports `skillet/errs.Error` as `adh.Error`) |

- [ ] **Publish a consumer-side `wrapcheck` snippet.** Every repo that adopts toerr must
      whitelist `Wrap`/`WrapWithMessage`/`Mark` in its own `.golangci.yaml` (toerr's own
      config whitelists only `fmt.Errorf`; skillet already hand-registered
      `WrapWithMessage`). Shipping a documented, copy-pasteable `extra-ignore-sigs` block
      (README or a `docs/` snippet) removes the main mechanical barrier to adoption.
      Include `.WithCode(` alongside the wrappers: gnosis had to register it separately
      (2026-10-22), because a coded wrap keeps its cause's `Unwrap` chain and is a real wrap.
- [ ] **Land the skillet consolidation first (skillet's call, tracked there).** The
      highest-leverage rollout is top-down: if `skillet/errs` becomes a thin layer over
      toerr, exegesis/skillsaw/adh inherit it on their next skillet bump — no per-repo
      migration. This item just records that toerr's adoption is gated on that decision.

______________________________________________________________________

## 9. Findings from the Gnosis `errs.Error` → `toerr` Migration (2026-10-22)

gnosis moved 436 `skillet/errs.Error` literals to toerr (gnosis commit `ec3253b`,
against toerr `v0.1.0`; re-checked against `v0.2.0`, where `errcode` is unchanged
apart from benchmarks). Its coded leaves are `errcode.WithCode(code, msg, nil)`, and
its coded wraps are `WithCode(code, "", WrapWithMessage(err, op))`. Four library
issues surfaced, in priority order.

- [ ] **`errcode.WithCode` and `WithCodeOpaque` record no frame — High Impact, Low
      Effort.** `New` and `Wrap` call `callerPC()`, but nothing in `errcode` does, so
      a coded leaf is missing from the `%+v` return trace. A probe printed the wrapper's
      frame and no origin for `WrapWithMessage(WithCode(StatusNotFound, "…", nil), …)`,
      while the same chain built on `errors.New` printed both. That contradicts
      README's "Location at every hop", and the coded leaf is the hop a reader most
      needs. Fix: capture the caller PC in both constructors and implement `TracePC`
      on `withCodeError`; add a test that a coded leaf contributes one frame. This is
      also the precondition for any adopter dropping manual `Op` strings as
      `philosophy.md` recommends: gnosis cannot, because most of its leaves are coded
      and would vanish from the trace.
- [ ] **Make leaf and wrap constructors agree about nil — Medium Impact, Low Effort
      (plus a deprecation window).** `WrapWithMessage(nil, …)` returns nil, but
      `WithCode(code, msg, nil)` returns a real error, because `WithCode` doubles as the
      only coded-leaf constructor. The mismatch caused a silent bug: gnosis's first,
      regexp-based migration turned two `{Op, Message}` leaves into
      `WrapWithMessage(nil, …)`, and those failure paths would have returned nil. Fix:
      add a leaf constructor (for example `errcode.New(code, message)`) and document
      `WithCode` as a wrap. After a deprecation window, make `WithCode` return nil for
      a nil cause, as `Wrap` does. Adopters have many `WithCode(…, nil)` leaves (gnosis
      has 34 or more), so the semantics change needs the window and a changelog entry.
- [ ] **Decide what `errcode.Message` returns for classify-only wraps — Medium Impact,
      Low Effort.** `Message` reads only the *outermost* coded error. Attaching a code
      to an existing error is naturally written `WithCode(code, "", err)`, and that
      hides any inner coded message: `Message` returns `""`. Either return the first
      non-empty message in the chain, or keep outermost-wins and document that a
      classify-only wrap masks inner messages, with the preferred spelling for adding a
      code without a message. If a dedicated classify constructor is added (for example
      `errcode.Classify(err, code)`, nil-in nil-out), state its `Message` behaviour too.
- [ ] **Document migrating *from* `Op` strings — Low-Medium Impact, Low Effort.**
      `philosophy.md` says "No `Op` strings" and offers `WrapWithMessage(err, op)` for
      keeping the one-line form, but says nothing about the two ways that form breaks
      once both conventions mix. First, a helper that borrows its caller's `op` returns an
      error already carrying it, and a caller that wraps again prints `op: op: …`; gnosis
      had 36 such sites before its migration, plus four routes through a shared helper.
      Second, `WithCode(code, op+": "+msg, nil)` puts a developer-facing `op` into what
      `errcode.Message` documents as the user-facing message.
      Add a short "Coming from Ben Johnson's `Error{Code, Message, Op, Err}`" section:
      the field-by-field mapping (`{Op, Err}` → `WrapWithMessage(err, op)`;
      `{Code, Message}` → coded leaf; `{Code, Op, Err}` →
      `WithCode(code, "", WrapWithMessage(err, op))`, because `WithCode`'s
      `"cause (message)"` rendering would otherwise reorder the text), and a note
      that `Op` belongs in wraps and never in `Message`.
- [ ] **Not planned: de-duplicating repeated wraps at runtime.** A `WrapWithMessage`
      that skips itself when the cause already starts with the same message would hide
      the structural mistake and swallow real repeated frames from recursion. If
      enforcement is wanted, a `go/analysis` checker for `WrapWithMessage(err, X)`
      where `err` already carries an `X` frame fits better. Recorded so the idea is not
      re-proposed.
- Source: gnosis PLAN.md §6.60 and TODO.md (the `errs.Error` retirement entry and the
  open `readConcept` item); probes run 2026-10-22 against `toerr` `v0.1.0`.
