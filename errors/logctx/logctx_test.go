package logctx_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/StevenACoffman/toerr/errors/logctx"
)

// render formats attrs the way a text handler would, without time, level, or message,
// so a test can compare the fields as one string, such as "a.k=1 b.k=2".
func render(attrs []slog.Attr) string {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 &&
				(a.Key == slog.TimeKey || a.Key == slog.LevelKey || a.Key == slog.MessageKey) {
				return slog.Attr{}
			}
			return a
		},
	}))
	logger.LogAttrs(context.Background(), slog.LevelInfo, "", attrs...)
	return strings.TrimSpace(buf.String())
}

func TestAttrs(t *testing.T) {
	cases := map[string]struct {
		// record adds fields to a request context and returns the context to read from.
		record func(ctx context.Context) context.Context
		want   string
	}{
		"top level": {func(ctx context.Context) context.Context {
			logctx.Add(ctx, slog.Int("k", 1))
			return ctx
		}, "k=1"},
		"scopes keep the same key apart": {func(ctx context.Context) context.Context {
			logctx.Add(logctx.Scope(ctx, "a"), slog.Int("k", 1))
			logctx.Add(logctx.Scope(ctx, "b"), slog.Int("k", 2))
			return ctx
		}, "a.k=1 b.k=2"},
		"nested scopes nest": {func(ctx context.Context) context.Context {
			logctx.Add(logctx.Scope(logctx.Scope(ctx, "a"), "b"), slog.Int("k", 1))
			return ctx
		}, "a.b.k=1"},
		"a repeated key keeps the last value in the first position": {
			func(ctx context.Context) context.Context {
				logctx.Add(ctx, slog.Int("k", 1), slog.Int("j", 2))
				logctx.Add(ctx, slog.Int("k", 3))
				return ctx
			}, "k=3 j=2",
		},
		"the same name shares a scope": {func(ctx context.Context) context.Context {
			logctx.Add(logctx.Scope(ctx, "a"), slog.Int("k", 1))
			logctx.Add(logctx.Scope(ctx, "a"), slog.Int("j", 2))
			return ctx
		}, "a.k=1 a.j=2"},
		"top-level fields come first": {func(ctx context.Context) context.Context {
			logctx.Add(logctx.Scope(ctx, "a"), slog.Int("k", 1))
			logctx.Add(ctx, slog.Int("j", 2))
			return ctx
		}, "j=2 a.k=1"},
		"an empty attr is ignored": {func(ctx context.Context) context.Context {
			logctx.Add(ctx, slog.Attr{}, slog.Int("k", 1))
			return ctx
		}, "k=1"},
		"an empty scope name adds no namespace": {func(ctx context.Context) context.Context {
			logctx.Add(logctx.Scope(ctx, ""), slog.Int("k", 1))
			return ctx
		}, "k=1"},
		"an empty scope is omitted": {func(ctx context.Context) context.Context {
			logctx.Scope(ctx, "a")
			logctx.Add(ctx, slog.Int("k", 1))
			return ctx
		}, "k=1"},
		"a scoped context reads the whole request": {func(ctx context.Context) context.Context {
			logctx.Add(ctx, slog.Int("k", 1))
			scoped := logctx.Scope(ctx, "a")
			logctx.Add(scoped, slog.Int("j", 2))
			return scoped
		}, "k=1 a.j=2"},
		"Init inside a request keeps its collector": {
			func(ctx context.Context) context.Context {
				logctx.Add(ctx, slog.Int("k", 1))
				logctx.Add(logctx.Init(logctx.Scope(ctx, "a")), slog.Int("j", 2))
				return ctx
			}, "k=1 a.j=2",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := tc.record(logctx.Init(t.Context()))
			if got := render(logctx.Attrs(ctx)); got != tc.want {
				t.Errorf("Attrs = %q, want %q", got, tc.want)
			}
		})
	}
}

// Code beneath a boundary that collects nothing must still run.
func TestWithoutInit(t *testing.T) {
	ctx := t.Context()
	logctx.Add(ctx, slog.Int("k", 1))
	if logctx.Scope(ctx, "a") != ctx {
		t.Error("Scope without Init should return ctx unchanged")
	}
	if attrs := logctx.Attrs(ctx); attrs != nil {
		t.Errorf("Attrs without Init = %v, want nil", attrs)
	}
}

// A snapshot taken mid-request must not change as the request goes on, and changing
// it must not reach the collector.
func TestAttrsIsACopy(t *testing.T) {
	ctx := logctx.Init(t.Context())
	logctx.Add(logctx.Scope(ctx, "a"), slog.Int("k", 1))
	snapshot := logctx.Attrs(ctx)

	logctx.Add(logctx.Scope(ctx, "a"), slog.Int("k", 2), slog.Int("j", 3))
	logctx.Add(ctx, slog.Int("top", 4))
	snapshot[0] = slog.Int("mutated", 0)

	if got := render(logctx.Attrs(ctx)); got != "top=4 a.k=2 a.j=3" {
		t.Errorf("Attrs after more additions = %q", got)
	}
	if got := render(snapshot); got != "mutated=0" {
		t.Errorf("snapshot = %q, want only the caller's own change", got)
	}
}

// Parallel branches that each take a scope must lose nothing, and reading while they
// write must be safe. Run under -race, this is the check that both failure modes of a
// shared collector, the race and the key collision, are designed out.
func TestParallelBranchesAreLossless(t *testing.T) {
	const branches = 50
	ctx := logctx.Init(t.Context())

	var wg sync.WaitGroup
	for n := range branches {
		wg.Go(func() {
			branch := logctx.Scope(ctx, fmt.Sprintf("shard_%d", n))
			logctx.Add(branch, slog.Int("duration", n))
			logctx.Add(ctx, slog.Int("last_shard", n))
		})
		wg.Go(func() { _ = logctx.Attrs(ctx) })
	}
	wg.Wait()

	got := render(logctx.Attrs(ctx))
	for n := range branches {
		if want := fmt.Sprintf("shard_%d.duration=%d", n, n); !strings.Contains(got, want) {
			t.Errorf("Attrs is missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "last_shard=") != 1 {
		t.Errorf("an unscoped key should keep one value:\n%s", got)
	}
}
