package proptest_test

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/StevenACoffman/toerr/errors/logctx"
)

// modelScope is the collector's contract written the plain way: the keys in the order
// they were first added, the latest value for each, and child scopes in the order they
// were created.
type modelScope struct {
	keys     []string
	values   map[string]int
	children []string
	child    map[string]*modelScope
}

func newModelScope() *modelScope {
	return &modelScope{values: map[string]int{}, child: map[string]*modelScope{}}
}

func (m *modelScope) at(path []string) *modelScope {
	if len(path) == 0 {
		return m
	}
	c, ok := m.child[path[0]]
	if !ok {
		c = newModelScope()
		m.child[path[0]] = c
		m.children = append(m.children, path[0])
	}
	return c.at(path[1:])
}

func (m *modelScope) add(key string, value int) {
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// fields flattens the model to "a.b.key=value" strings, the order Attrs promises.
func (m *modelScope) fields(prefix string) []string {
	var out []string
	for _, k := range m.keys {
		out = append(out, fmt.Sprintf("%s%s=%d", prefix, k, m.values[k]))
	}
	for _, name := range m.children {
		out = append(out, m.child[name].fields(prefix+name+".")...)
	}
	return out
}

// flatten renders attrs the same way, descending into groups.
func flatten(prefix string, attrs []slog.Attr) []string {
	var out []string
	for _, a := range attrs {
		if a.Value.Kind() == slog.KindGroup {
			out = append(out, flatten(prefix+a.Key+".", a.Value.Group())...)
			continue
		}
		out = append(out, fmt.Sprintf("%s%s=%v", prefix, a.Key, a.Value))
	}
	return out
}

// TestCollectorMatchesItsModel checks that after any sequence of additions, each made
// through a scope path of up to two names, Attrs reports exactly what the model does:
// every key once with its last value, top-level fields before scopes, and scopes nested
// and ordered by creation.
//
// The names and keys are drawn from small sets on purpose, so that repeated keys and
// shared scopes, the cases the collector must merge, come up in most runs.
func TestCollectorMatchesItsModel(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		ctx := logctx.Init(context.Background())
		model := newModelScope()

		for i := range rapid.IntRange(0, 12).Draw(t, "additions") {
			path := rapid.SliceOfN(rapid.SampledFrom([]string{"a", "b"}), 0, 2).
				Draw(t, fmt.Sprintf("path%d", i))
			key := rapid.SampledFrom([]string{"k", "j", "m"}).Draw(t, fmt.Sprintf("key%d", i))

			scoped := ctx
			for _, name := range path {
				scoped = logctx.Scope(scoped, name)
			}
			logctx.Add(scoped, slog.Int(key, i))
			model.at(path).add(key, i)
		}

		got, want := flatten("", logctx.Attrs(ctx)), model.fields("")
		if !slices.Equal(got, want) {
			t.Fatalf("Attrs:\n  %s\nwant:\n  %s",
				strings.Join(got, " "), strings.Join(want, " "))
		}
	})
}
