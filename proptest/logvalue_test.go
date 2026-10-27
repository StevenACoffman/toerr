package proptest_test

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log/slog"
	"slices"
	"testing"

	"pgregory.net/rapid"

	"github.com/StevenACoffman/toerr/errors"
)

// opTree is a recipe for an error tree in which every error with an attribute uses the
// same key, "op", with a value unique to that error.
type opTree struct {
	kind     string // "new", "foreign", "wrap", "mark", "seal", "join"
	op       int    // 0 for no attribute
	children []opTree
}

// TestLogValueKeepsEveryAttributeOnce checks that LogValue writes each error's
// attribute exactly once and never writes a key twice in one JSON object, however the
// errors that share the key are arranged: siblings in a join, layers of a chain, or
// either beneath a seal or a mark.
//
// A repeated key is the failure LogValue's shape exists to prevent: Cloud Logging and
// most JSON decoders keep one value of a repeated key and drop the rest silently.
func TestLogValueKeepsEveryAttributeOnce(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		var next int
		want, err := genOpTree(3, &next).Draw(t, "tree").build()
		data, _ := errors.LogValue(err).Any().(json.Marshaler).MarshalJSON()

		if dupErr := checkNoRepeatedKey(json.NewDecoder(bytes.NewReader(data))); dupErr != nil {
			t.Fatalf("%v in %s", dupErr, data)
		}
		var tree any
		if jsonErr := json.Unmarshal(data, &tree); jsonErr != nil {
			t.Fatalf("invalid JSON: %v in %s", jsonErr, data)
		}
		got := ops(tree)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("op values %v, want %v, in %s", got, want, data)
		}
	})
}

func genOpTree(depth int, next *int) *rapid.Generator[opTree] {
	return rapid.Custom(func(t *rapid.T) opTree {
		kinds := []string{"new", "foreign"}
		if depth > 0 {
			kinds = append(kinds, "wrap", "mark", "seal", "join")
		}
		n := opTree{kind: rapid.SampledFrom(kinds).Draw(t, "kind")}
		switch n.kind {
		case "join":
			n.children = rapid.SliceOfN(genOpTree(depth-1, next), 1, 3).Draw(t, "children")
		case "mark", "seal":
			n.children = []opTree{genOpTree(depth-1, next).Draw(t, "child")}
		case "wrap":
			n.children = []opTree{genOpTree(depth-1, next).Draw(t, "child")}
			n.op = drawOp(t, next)
		default:
			n.op = drawOp(t, next)
		}
		return n
	})
}

// drawOp returns a fresh op value, or 0 for an error without the attribute.
func drawOp(t *rapid.T, next *int) int {
	if !rapid.Bool().Draw(t, "hasOp") {
		return 0
	}
	*next++
	return *next
}

// build returns the op values the tree carries, and the tree.
func (n opTree) build() (want []int, err error) {
	var attrs []slog.Attr
	if n.op != 0 {
		want = []int{n.op}
		attrs = []slog.Attr{slog.Int("op", n.op)}
	}
	var children []error
	for _, c := range n.children {
		ops, e := c.build()
		want = append(want, ops...)
		children = append(children, e)
	}
	switch n.kind {
	case "new":
		return want, errors.New("leaf", attrs...)
	case "foreign":
		return want, &foreignError{attrs: attrs}
	case "wrap":
		return want, errors.Wrap(children[0], attrs...)
	case "mark":
		return want, errors.Mark(children[0], stderrors.New("marker"))
	case "seal":
		return want, errors.Seal(children[0])
	default: // join
		return want, errors.Join(children...)
	}
}

// checkNoRepeatedKey reads one JSON value and reports the first object that repeats a
// key, which a decoder into a map would hide.
func checkNoRepeatedKey(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("read token: %w", err)
	}
	switch tok {
	case json.Delim('{'):
		err = checkMembers(dec)
	case json.Delim('['):
		for err == nil && dec.More() {
			err = checkNoRepeatedKey(dec)
		}
	default:
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = dec.Token(); err != nil { // the closing delimiter.
		return fmt.Errorf("read closing delimiter: %w", err)
	}
	return nil
}

// checkMembers reads the members of an object whose "{" has been read.
func checkMembers(dec *json.Decoder) error {
	seen := map[string]bool{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return fmt.Errorf("read key: %w", err)
		}
		key, _ := tok.(string)
		if seen[key] {
			return fmt.Errorf("repeated key %q", key)
		}
		seen[key] = true
		if err := checkNoRepeatedKey(dec); err != nil {
			return err
		}
	}
	return nil
}

// ops returns every "op" value under an "attrs" object anywhere in a decoded tree.
func ops(v any) []int {
	var found []int
	switch x := v.(type) {
	case map[string]any:
		if attrs, ok := x["attrs"].(map[string]any); ok {
			if op, ok := attrs["op"].(float64); ok {
				found = append(found, int(op))
			}
		}
		for _, child := range x {
			found = append(found, ops(child)...)
		}
	case []any:
		for _, child := range x {
			found = append(found, ops(child)...)
		}
	}
	return found
}
