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

## 1. Reconcile the logging guidance — high impact, medium effort

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

## 2. Document "when not to use this" + wrap-density guidance — medium impact, low effort

- [ ] Add a short "when not to reach for this" list (shallow call graphs,
      one-structured-line-per-request shops).
- [ ] Give explicit guidance on wrap-at-every-hop vs wrap-at-boundaries; the
      mechanism is neutral (a bare `Wrap` adds a frame with no message), so the
      choice belongs to the caller and should be stated.
- Source: Counterargument 2; remainder of analysis next step #2. Cheapest real win.

## 3. Draw the `panic` app-vs-library line — low-medium impact, very low effort

- [ ] In "Knowing When to Give Up," state that the `panic`-on-invariant advice is
      app-facing, and that library code should not panic across its boundary
      (recover internally). The guidance ships inside a library, so a reader could
      misapply it.
- Source: Counterargument 6. A one-paragraph doc fix — bundle with item 2.

## 4. PC-capture cost story + opt-out — medium impact, medium effort

- [ ] Document the per-wrap `runtime.Callers` cost (paid on every `Wrap`, even for
      swallowed errors) and add a benchmark.
- [ ] Consider a hot-path opt-out or a build-tag / debug-gated PC capture.
- Source: Gap 5; analysis next step #4 (optional).

## 5. Recover-at-boundary helper — medium impact, medium effort — DONE

- [x] Provide a helper that turns a recovered panic into a traced error, serving
      the "packages recover internally at their boundary" guidance.
- Done: `errors.Recover(recover())` (`errors/recover.go`, tests in
  `errors/recover_test.go`). Returns nil when there was no panic, preserves a
  recovered error as the cause so `errors.Is`/`As` still reach it, records a frame
  at the recovery site, and attaches the panic stack as a `"stack"` `slog.Attr`
  rather than in the message (honoring return-trace-not-stack-trace).
- Source: Gap 6; analysis next step #5 (optional).

## 6. Behavior-interface idiom — low/niche impact, medium effort — DONE

- [x] Provide an "ask the error a question" pattern to complement identity
      (`sentinel`), type (`AsType`/`Mark`), and stored classification (`errclass`).
- Done: `errors.AsBehavior[T]` (`errors/behavior.go`, tests in
  `errors/behavior_test.go`) — the behavior counterpart to `AsType` for interfaces
  that do not embed `error`. Blessed in `philosophy.md` ("React by Behavior"),
  including the `Mark`-to-attach trick and when to prefer it over code/class/sentinel.
  Named domain predicates (`Temporary`/`Retryable`) are deliberately left to the
  application, per the mechanism/domain split.
- Source: Gap 2.

## 7. `Error()` actionable vs operator-only — low impact, likely won't-fix

- [ ] Decide whether to note the trade-off: `toerr` deliberately makes `Error()`
      operator-only (users read `errcode.Message`), whereas some advice wants
      `Error()` self-sufficient. Mostly a "document the trade-off" item, not a change.
- Source: Counterargument 5.
