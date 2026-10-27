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
		// One seed per branch of the escaper, each after text it must keep: \b and \f
		// have short escapes, a lone continuation byte is invalid UTF-8, and U+2028 and
		// U+2029 are escaped.
		"bell\b feed\f", "ok\x80", "ok\xffok", "a b c",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		got, _ := trace.NewJSONTree(stderrors.New(s)).MarshalJSON()

		type layer struct {
			Msg  string `json:"msg,omitempty"`
			Type string `json:"type"`
		}
		var want bytes.Buffer
		enc := json.NewEncoder(&want)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(struct {
			Msg   string  `json:"msg"`
			Chain []layer `json:"chain"`
		}{s, []layer{{s, "*errors.errorString"}}})

		if !bytes.Equal(got, bytes.TrimSuffix(want.Bytes(), []byte("\n"))) {
			t.Errorf("got  %s\nwant %s", got, want.Bytes())
		}
	})
}
