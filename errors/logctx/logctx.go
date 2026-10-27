package logctx

import (
	"context"
	"log/slog"
	"sync"
)

// collector holds every field recorded for one request. One mutex guards the whole
// tree: writes are short and few per request, and a single lock keeps a snapshot
// consistent across scopes.
type collector struct {
	mu   sync.Mutex
	root scope
}

// scope is one namespace of fields. Its attrs hold one value per key, in the order
// the keys were first added; its children are nested scopes, in creation order.
type scope struct {
	name     string
	attrs    []slog.Attr
	children []*scope
}

// position is what a context carries: the request's collector, and the scope that
// additions through that context go to.
type position struct {
	c *collector
	s *scope
}

// ctxKey is the context key for a position. Its type is unexported, so no other
// package can read or replace the value.
type ctxKey struct{}

// Init returns a context carrying a new, empty collector. Call it once where a
// request starts. If ctx already carries a collector, Init returns ctx unchanged, so
// a second middleware cannot split one request's fields in two.
//
// Without Init, Add and Scope do nothing and Attrs returns nil, so code beneath a
// boundary that collects nothing still runs.
func Init(ctx context.Context) context.Context {
	if _, ok := from(ctx); ok {
		return ctx
	}
	c := &collector{}
	return context.WithValue(ctx, ctxKey{}, position{c: c, s: &c.root})
}

// Add records attrs in ctx's scope, where the request's Attrs will find them. It is
// safe to call from many goroutines. Within one scope a repeated key keeps the last
// value, in the position of the first; give parallel branches their own [Scope] so
// their keys cannot meet. An empty attr is ignored.
func Add(ctx context.Context, attrs ...slog.Attr) {
	p, ok := from(ctx)
	if !ok {
		return
	}
	p.c.mu.Lock()
	defer p.c.mu.Unlock()
	for _, a := range attrs {
		p.s.set(a)
	}
}

// Scope returns a context whose additions are recorded under name, nested inside
// ctx's own scope. Attrs renders each scope as a slog group, so the same key in two
// scopes yields two fields, such as shard_3.duration and shard_4.duration. Calls
// with the same name in the same scope share it, so name each parallel branch
// distinctly. An empty name adds no namespace and returns ctx.
func Scope(ctx context.Context, name string) context.Context {
	p, ok := from(ctx)
	if !ok || name == "" {
		return ctx
	}
	p.c.mu.Lock()
	child := p.s.child(name)
	p.c.mu.Unlock()
	return context.WithValue(ctx, ctxKey{}, position{c: p.c, s: child})
}

// Attrs returns every field recorded for ctx's request: the top-level fields, then
// each scope as a slog group, in the order they were created. It reads the whole
// request from any scope. The result is a copy that later additions do not change.
// It returns nil if ctx carries no collector.
func Attrs(ctx context.Context) []slog.Attr {
	p, ok := from(ctx)
	if !ok {
		return nil
	}
	p.c.mu.Lock()
	defer p.c.mu.Unlock()
	return p.c.root.snapshot()
}

// from returns the position ctx carries, if any.
func from(ctx context.Context) (position, bool) {
	p, ok := ctx.Value(ctxKey{}).(position)
	return p, ok
}

// set records a under its key, replacing an earlier value in place. A scope holds a
// handful of keys, so a scan of the slice that keeps their order is enough; a map
// beside it would be a second structure to keep in step.
func (s *scope) set(a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}
	for i := range s.attrs {
		if s.attrs[i].Key == a.Key {
			s.attrs[i] = a
			return
		}
	}
	s.attrs = append(s.attrs, a)
}

// child returns the scope called name beneath s, creating it if needed.
func (s *scope) child(name string) *scope {
	for _, c := range s.children {
		if c.name == name {
			return c
		}
	}
	c := &scope{name: name}
	s.children = append(s.children, c)
	return c
}

// snapshot copies s into new slices, omitting scopes that hold nothing.
func (s *scope) snapshot() []slog.Attr {
	out := make([]slog.Attr, 0, len(s.attrs)+len(s.children))
	out = append(out, s.attrs...)
	for _, c := range s.children {
		if inner := c.snapshot(); len(inner) > 0 {
			out = append(out, slog.Attr{Key: c.name, Value: slog.GroupValue(inner...)})
		}
	}
	return out
}
