package proptest_test

import (
	stderrors "errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/StevenACoffman/toerr/errors"
)

// keyCounter makes attribute keys unique across a whole test case; see genAttrs.
var keyCounter int

// attrTree is a recipe for an error tree plus the attributes Attrs should report for it.
type attrTree struct {
	kind     string // "new", "foreign", "wrap", "join"
	attrs    []slog.Attr
	children []attrTree
}

// sealChain is a chain of Wrap, Mark, and Seal over a root, with the matches it should allow:
// the root only if nothing sealed it, and the markers added after the last seal.
type sealChain struct {
	root, err       error
	rootVisible     bool
	visible, hidden []error
}

// foreignError is an error type declared outside the errors package that surfaces its own
// fields through the Attrs method rather than by being wrapped.
type foreignError struct{ attrs []slog.Attr }

// TestWrapAndMarkAreTransparent checks that no sequence of Wrap and Mark changes what an error
// says or what it is.
//
// Both are documented as transparent: Wrap adds a frame and attributes, Mark adds a marker
// outside the Unwrap chain, and neither touches the message. Callers lean on that when they
// match on the root with errors.Is, or show err.Error() to an operator expecting the text the
// origin wrote. A counterexample is a chain shape in which a wrapper leaks into the message,
// hides the root from errors.Is, or puts a marker where Unwrap can reach it.
func TestWrapAndMarkAreTransparent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		root := genRoot().Draw(t, "root")
		err := root
		var markers []error
		for i, op := range rapid.SliceOfN(rapid.SampledFrom([]string{"wrap", "mark"}), 0, 8).
			Draw(t, "ops") {
			switch op {
			case "wrap":
				err = errors.Wrap(err, genAttrs().Draw(t, fmt.Sprintf("attrs%d", i))...)
			case "mark":
				marker := fmt.Errorf("marker %d", i)
				markers = append(markers, marker)
				err = errors.Mark(err, marker)
			}
		}

		if err.Error() != root.Error() {
			t.Fatalf("message changed: got %q, want %q", err.Error(), root.Error())
		}
		if got := fmt.Sprintf("%v", err); got != root.Error() {
			t.Fatalf("%%v changed: got %q, want %q", got, root.Error())
		}
		if !errors.Is(err, root) {
			t.Fatalf("errors.Is lost the root %q", root)
		}
		for _, marker := range markers {
			checkMarker(t, err, marker)
		}
	})
}

// checkMarker fails unless errors.Is finds marker in err while Unwrap never reaches it.
func checkMarker(t *rapid.T, err, marker error) {
	t.Helper()

	if !errors.Is(err, marker) {
		t.Fatalf("errors.Is lost the marker %q", marker)
	}
	for e := err; e != nil; e = errors.Unwrap(e) {
		if e == marker { //nolint:errorlint // identity of each chain node, not a match.
			t.Fatalf("marker %q is reachable through Unwrap", marker)
		}
	}
}

// TestWrapWithMessageComposesOutermostFirst checks that each WrapWithMessage prepends its
// message, so the text reads from the outermost layer down to the origin.
func TestWrapWithMessageComposesOutermostFirst(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		root := genRoot().Draw(t, "root")
		messages := rapid.SliceOfN(rapid.StringMatching(`[a-z ]{1,8}`), 0, 6).Draw(t, "messages")

		err := root
		want := root.Error()
		for _, m := range messages {
			err = errors.WrapWithMessage(err, m)
			want = m + ": " + want
		}

		if err.Error() != want {
			t.Fatalf("got %q, want %q", err.Error(), want)
		}
	})
}

// TestTraceHasOneFramePerHop checks that %+v records exactly one frame for each New, Wrap and
// WrapWithMessage in the chain, plus one for a Mark that had to give a foreign error a frame,
// and none for a Mark of an error that already had one. A Seal adds none, and hides the frames
// behind it from Mark, so a Mark just outside a seal records one.
//
// The trace is a return trace, not a stack trace: a frame per hop is the whole promise. An
// extra frame is a captured stack leaking in; a missing one is a hop the trace walk skipped.
func TestTraceHasOneFramePerHop(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		var err error
		want := 0
		traced := rapid.Bool().Draw(t, "rootIsTraced")
		if traced {
			err = errors.New("origin")
			want++
		} else {
			err = stderrors.New("origin")
		}

		ops := rapid.SliceOfN(
			rapid.SampledFrom([]string{"wrap", "message", "mark", "seal"}), 1, 8,
		).Draw(t, "ops")
		for _, op := range ops {
			switch op {
			case "wrap":
				err = errors.Wrap(err)
			case "message":
				err = errors.WrapWithMessage(err, "context")
			case "mark":
				err = errors.Mark(err, stderrors.New("marker"))
				if traced {
					continue
				}
			case "seal":
				// A seal is not a hop, and Mark cannot see the frames behind it, so the
				// next Mark records one.
				err = errors.Seal(err)
				traced = false
				continue
			}
			want++
			traced = true
		}

		trace := fmt.Sprintf("%+v", err)
		if got := strings.Count(trace, ".go:"); got != want {
			t.Fatalf("ops %v: got %d frames, want %d:\n%s", ops, got, want, trace)
		}
	})
}

// TestSealHidesEverythingBeneathFromMatching checks that after any sequence of Wrap, Mark,
// and Seal, errors.Is reaches a marker exactly when it was added after the last seal, reaches
// the root exactly when nothing sealed it, and the message never changes.
//
// This is the contract a boundary relies on: whatever a dependency's error carried, callers
// can match only what the boundary attached outside the seal.
func TestSealHidesEverythingBeneathFromMatching(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		c := genSealChain().Draw(t, "chain")

		if c.err.Error() != c.root.Error() {
			t.Fatalf("message changed: got %q, want %q", c.err.Error(), c.root.Error())
		}
		if errors.Is(c.err, c.root) != c.rootVisible {
			t.Fatalf("errors.Is(err, root) = %v, want %v", !c.rootVisible, c.rootVisible)
		}
		for _, marker := range c.visible {
			if !errors.Is(c.err, marker) {
				t.Fatalf("errors.Is lost %q, added outside every seal", marker)
			}
		}
		for _, marker := range c.hidden {
			if errors.Is(c.err, marker) {
				t.Fatalf("errors.Is reaches %q through a seal", marker)
			}
		}
	})
}

// TestSealKeepsTheOperatorView checks that sealing any error tree, joins and foreign Attrs
// types included, changes none of what an operator reads: the message, every attribute in
// order, and every frame of the %+v trace.
func TestSealKeepsTheOperatorView(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		_, tree := genAttrTree(3).Draw(t, "tree").build()
		// Both go under a Wrap so both print through this package's renderer: a bare
		// Join or foreign error would not print its frames at all.
		err, sealed := errors.Wrap(tree), errors.Wrap(errors.Seal(tree))

		if sealed.Error() != err.Error() {
			t.Fatalf("message changed: got %q, want %q", sealed.Error(), err.Error())
		}
		if got, want := keys(errors.Attrs(sealed)), keys(errors.Attrs(err)); got != want {
			t.Fatalf("Attrs changed: got %s, want %s", got, want)
		}
		got, want := fmt.Sprintf("%+v", sealed), fmt.Sprintf("%+v", err)
		if strings.Count(got, ".go:") != strings.Count(want, ".go:") {
			t.Fatalf("frames changed:\ngot:\n%s\nwant:\n%s", got, want)
		}
	})
}

// TestAttrsArePreOrder checks that Attrs returns every attribute in an error tree, outermost
// first and each Join branch in order, including attributes from a type declared outside the
// package that implements Attrs() []slog.Attr.
//
// The expected order is computed independently of the library, by building the tree and the
// list of attributes it should yield side by side.
func TestAttrsArePreOrder(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		want, err := genAttrTree(3).Draw(t, "tree").build()

		got := errors.Attrs(err)
		if keys(got) != keys(want) {
			t.Fatalf("got %s, want %s", keys(got), keys(want))
		}
	})
}

// TestRecoverPreservesThePanic checks that Recover turns any panic value into an error that
// still says, or still is, what was panicked with.
func TestRecoverPreservesThePanic(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		msg := rapid.String().Draw(t, "msg")

		if got := errors.Recover(msg); got.Error() != msg {
			t.Fatalf("Recover(%q).Error() = %q", msg, got.Error())
		}

		cause := stderrors.New(msg)
		if got := errors.Recover(cause); !errors.Is(got, cause) {
			t.Fatalf("Recover(err) lost the recovered error %q", msg)
		}
	})
}

func genRoot() *rapid.Generator[error] {
	return rapid.Custom(func(t *rapid.T) error {
		msg := rapid.String().Draw(t, "rootMessage")
		if rapid.Bool().Draw(t, "foreignRoot") {
			return stderrors.New(msg)
		}
		return errors.New(msg, genAttrs().Draw(t, "rootAttrs")...)
	})
}

func genSealChain() *rapid.Generator[sealChain] {
	return rapid.Custom(func(t *rapid.T) sealChain {
		c := sealChain{root: genRoot().Draw(t, "root"), rootVisible: true}
		c.err = c.root
		for i, op := range rapid.SliceOfN(rapid.SampledFrom([]string{"wrap", "mark", "seal"}), 0, 8).
			Draw(t, "ops") {
			switch op {
			case "wrap":
				c.err = errors.Wrap(c.err)
			case "mark":
				marker := fmt.Errorf("marker %d", i)
				c.visible = append(c.visible, marker)
				c.err = errors.Mark(c.err, marker)
			case "seal":
				c.err = errors.Seal(c.err)
				c.rootVisible = false
				c.hidden, c.visible = append(c.hidden, c.visible...), nil
			}
		}
		return c
	})
}

// genAttrs draws attributes with keys unique across the whole test case, so that comparing
// key sequences compares positions and not just multisets.
func genAttrs() *rapid.Generator[[]slog.Attr] {
	return rapid.Custom(func(t *rapid.T) []slog.Attr {
		n := rapid.IntRange(0, 3).Draw(t, "attrCount")
		attrs := make([]slog.Attr, n)
		for i := range attrs {
			attrs[i] = slog.Int(fmt.Sprintf("k%d", nextKey()), i)
		}
		return attrs
	})
}

func nextKey() int {
	keyCounter++
	return keyCounter
}

// build returns the attributes Attrs should report for the tree, and the tree itself.
func (n attrTree) build() (want []slog.Attr, err error) {
	switch n.kind {
	case "new":
		return n.attrs, errors.New("leaf", n.attrs...)
	case "foreign":
		return n.attrs, &foreignError{attrs: n.attrs}
	case "wrap":
		innerAttrs, inner := n.children[0].build()
		return append(
				append([]slog.Attr{}, n.attrs...),
				innerAttrs...), errors.Wrap(
				inner,
				n.attrs...)
	default: // join
		var errs []error
		for _, c := range n.children {
			a, e := c.build()
			errs = append(errs, e)
			want = append(want, a...)
		}
		return want, errors.Join(errs...)
	}
}

func genAttrTree(depth int) *rapid.Generator[attrTree] {
	return rapid.Custom(func(t *rapid.T) attrTree {
		kinds := []string{"new", "foreign"}
		if depth > 0 {
			kinds = append(kinds, "wrap", "join")
		}
		n := attrTree{kind: rapid.SampledFrom(kinds).Draw(t, "kind")}
		switch n.kind {
		case "wrap":
			n.attrs = genAttrs().Draw(t, "attrs")
			n.children = []attrTree{genAttrTree(depth-1).Draw(t, "child")}
		case "join":
			n.children = rapid.SliceOfN(genAttrTree(depth-1), 1, 3).Draw(t, "children")
		default:
			n.attrs = genAttrs().Draw(t, "attrs")
		}
		return n
	})
}

func (e *foreignError) Error() string      { return "foreign" }
func (e *foreignError) Attrs() []slog.Attr { return e.attrs }

func keys(attrs []slog.Attr) string {
	ks := make([]string, len(attrs))
	for i, a := range attrs {
		ks[i] = a.Key
	}
	return "[" + strings.Join(ks, " ") + "]"
}
