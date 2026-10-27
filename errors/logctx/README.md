# Logctx

**Collect a request's log fields wherever they are found, and write them once at the boundary.**

[![Go Reference](https://pkg.go.dev/badge/github.com/StevenACoffman/toerr/errors/logctx.svg)](https://pkg.go.dev/github.com/StevenACoffman/toerr/errors/logctx)

An error carries the context of a failure, but a request that succeeds has no error
to carry anything. The fields that explain it, such as a cache hit, a row count, or
the authenticated user, turn up deep in the handler, and the one log line for the
request is written at the top. `logctx` keeps those fields on the request's context
and hands them to the boundary, for the success path and the failure path alike.

## Usage

- `logctx.Init(ctx) context.Context`: install a collector where the request starts.
  A second `Init` on the same request returns `ctx` unchanged.
- `logctx.Add(ctx, attrs...)`: record fields anywhere beneath it. Within one scope a
  repeated key keeps the last value.
- `logctx.Scope(ctx, name) context.Context`: a context whose fields are recorded under
  `name`, which renders as a slog group. Give each parallel branch its own scope, so
  `shard_3.duration` and `shard_4.duration` cannot overwrite each other.
- `logctx.Attrs(ctx) []slog.Attr`: a copy of everything recorded for the request,
  top-level fields first and then each scope.

Without `Init`, `Add` and `Scope` do nothing and `Attrs` returns nil. The collector
is safe for concurrent use, and it depends only on the standard library.

## Example

```go
func logRequests(logger *slog.Logger, next func(context.Context) error) func(context.Context) {
	return func(ctx context.Context) {
		ctx = logctx.Init(ctx)
		err := next(ctx)

		level, msg, attrs := slog.LevelInfo, "request served", logctx.Attrs(ctx)
		if err != nil {
			level, msg = slog.LevelError, err.Error()
			attrs = append(attrs, slog.Any("error", errors.LogValue(err)))
		}
		logger.LogAttrs(ctx, level, msg, attrs...)
	}
}
```

Fields from a goroutine land under the scope the code that started it chose:

```go
for n, shard := range shards {
	g.Go(func() error {
		ctx := logctx.Scope(ctx, fmt.Sprintf("shard_%d", n))
		start := time.Now()
		err := render(ctx, shard)
		logctx.Add(ctx, slog.Duration("duration", time.Since(start)))
		return err
	})
}
```

`logctx` sends nothing to a logger itself and adds nothing to other log lines. It
holds the fields for the one line the boundary writes, so a request is logged once.
