package logctx_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/StevenACoffman/toerr/errors/logctx"
)

// Sinks keep the compiler from discarding the work under measurement.
var (
	ctxSink   context.Context
	attrsSink []slog.Attr
)

// BenchmarkAdd measures recording a field that is already present, the steady state
// of a request that updates a counter.
func BenchmarkAdd(b *testing.B) {
	ctx := logctx.Init(context.Background())
	for b.Loop() {
		logctx.Add(ctx, slog.Int("rows", 1))
	}
}

// BenchmarkScope measures entering a scope that already exists.
func BenchmarkScope(b *testing.B) {
	ctx := logctx.Init(context.Background())
	for b.Loop() {
		ctxSink = logctx.Scope(ctx, "seats")
	}
}

// BenchmarkAttrs measures the boundary's snapshot of a typical request: a few
// top-level fields and two scopes.
func BenchmarkAttrs(b *testing.B) {
	ctx := logctx.Init(context.Background())
	logctx.Add(ctx, slog.String("route", "/seats"), slog.String("user", "u1"))
	logctx.Add(logctx.Scope(ctx, "db"), slog.Int("rows", 1), slog.Bool("cache_hit", true))
	logctx.Add(logctx.Scope(ctx, "payment"), slog.String("provider", "acme"))
	for b.Loop() {
		attrsSink = logctx.Attrs(ctx)
	}
}
