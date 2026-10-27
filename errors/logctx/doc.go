// Package logctx collects structured log fields for one request on its context, so
// the request's single log line at the boundary can carry fields discovered anywhere
// beneath it. That includes success-path fields an error never could, such as a cache
// hit or a row count.
//
// Install a collector once at the request boundary, add fields wherever they are
// known, and read them back when the request ends:
//
//	ctx = logctx.Init(ctx)        // in the middleware, before the handler runs
//	logctx.Add(ctx, slog.Bool("cache_hit", true)) // anywhere beneath it
//	attrs := logctx.Attrs(ctx)    // in the middleware, after the handler returns
//	if err != nil {
//		attrs = append(attrs, slog.Any("error", errors.LogValue(err)))
//	}
//	logger.LogAttrs(ctx, level, msg, attrs...)
//
// A context is shared by every goroutine started beneath it, so the collector is
// safe for concurrent use. Fields from parallel branches must not overwrite each
// other: give each branch its own [Scope], and its fields appear under that name as a
// slog group, as in shard_3.duration and shard_4.duration. A branch can instead
// return its fields to the goroutine that started it, which adds them under its own
// scope; that shares nothing between goroutines at all.
//
// The package imports nothing from toerr. The caller adds the error beside the
// fields, so the collector also works for code that does not use toerr's errors.
package logctx
