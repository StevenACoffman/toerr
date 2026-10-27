package trace_test

import (
	"bytes"
	"encoding/json"
	stderrors "errors"
	"testing"

	"github.com/StevenACoffman/toerr/errors/internal/trace"
)

// The tree escapes a message exactly as encoding/json does without HTML escaping,
// which is how slog's JSON handler writes strings.
func FuzzTreeEscapesAsEncodingJSON(f *testing.F) {
	for _, s := range []string{
		"", "plain", `quote " and \ backslash`, "line\nbreak\r\ttab", "\x00\x1f\x7f",
		"<html> & more", "  ", "\xff invalid", "�", "héllo, 世界",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, _ := trace.NewTree(stderrors.New(s)).MarshalJSON()

		type layer struct {
			Msg string `json:"msg,omitempty"`
		}
		var want bytes.Buffer
		enc := json.NewEncoder(&want)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(struct {
			Msg   string  `json:"msg"`
			Chain []layer `json:"chain"`
		}{s, []layer{{s}}})

		if !bytes.Equal(got, bytes.TrimSuffix(want.Bytes(), []byte("\n"))) {
			t.Errorf("got  %s\nwant %s", got, want.Bytes())
		}
	})
}
