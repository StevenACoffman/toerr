package errors_test

import (
	"bytes"
	"encoding"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"

	errors "github.com/StevenACoffman/toerr/errors"
	"github.com/StevenACoffman/toerr/errors/sentinel"
)

// panicText is what LogValue writes for a typed-nil *nilDerefError, as slog does.
const panicText = "!PANIC: runtime error: invalid memory address or nil pointer dereference"

// atPath matches the directory and line of an "at" member, which vary by machine and
// by edit, leaving the file name.
var atPath = regexp.MustCompile(`"at":"[^"]*/([^/"]+):\d+"`)

// redacted logs as a fixed string, as a type holding a secret would.
type redacted string

// rawJSON encodes itself, as a dependency's type might.
type rawJSON struct{}

// silentMarkerError is a typed marker as Mark's documentation shows one: its embedded
// error is nil, so its Error method panics.
type silentMarkerError struct{ error }

// emptyMarkerError is a marker whose text is empty.
type emptyMarkerError struct{}

// nilDerefError's Error dereferences its receiver, so a typed nil of it panics.
type nilDerefError struct{ msg string }

// batchError is a multi-error from another package that reports attributes of its own.
type batchError struct{ errs []error }

func (redacted) LogValue() slog.Value { return slog.StringValue("REDACTED") }

func (rawJSON) MarshalJSON() ([]byte, error) { return []byte(`{"raw":true}`), nil }

func (e *nilDerefError) Error() string { return e.msg }

func (emptyMarkerError) Error() string { return "" }

func (e *batchError) Error() string      { return stderrors.Join(e.errs...).Error() }
func (e *batchError) Unwrap() []error    { return e.errs }
func (e *batchError) Attrs() []slog.Attr { return []slog.Attr{slog.Int("batch", 7)} }

// logJSON returns LogValue's JSON for err, with each "at" cut to its file name. It
// calls MarshalJSON as slog's handlers do; json.Marshal would also escape HTML.
func logJSON(t *testing.T, err error) string {
	t.Helper()
	m, ok := errors.LogValue(err).Any().(json.Marshaler)
	if !ok {
		t.Fatalf("LogValue(%v) is not a json.Marshaler", err)
	}
	data, marshalErr := m.MarshalJSON()
	if marshalErr != nil {
		t.Fatalf("marshal: %v", marshalErr)
	}
	return atPath.ReplaceAllString(string(data), `"at":"$1"`)
}

func TestLogValueShape(t *testing.T) {
	marker := stderrors.New("marker")
	var typedNil *nilDerefError
	cases := map[string]struct {
		err  error
		want string
	}{
		"leaf": {
			errors.New("disk full", slog.String("device", "sda1")),
			`{"msg":"disk full","chain":[` +
				`{"msg":"disk full","at":"logvalue_test.go","attrs":{"device":"sda1"}}]}`,
		},
		"chain, outermost first": {
			errors.WrapWithMessage(errors.New("d", slog.String("op", "d")), "c",
				slog.String("op", "c")),
			`{"msg":"c: d","chain":[` +
				`{"msg":"c","at":"logvalue_test.go","attrs":{"op":"c"}},` +
				`{"msg":"d","at":"logvalue_test.go","attrs":{"op":"d"}}]}`,
		},
		"siblings each keep their key": {
			errors.WrapWithMessage(errors.Join(
				errors.New("b failed", slog.String("op", "b")),
				errors.WrapWithMessage(errors.New("d failed", slog.String("op", "d")),
					"c failed", slog.String("op", "c")),
			), "a failed", slog.String("op", "a")),
			`{"msg":"a failed: b failed\nc failed: d failed",` +
				`"chain":[{"msg":"a failed","at":"logvalue_test.go","attrs":{"op":"a"}}],` +
				`"errors":[` +
				`{"chain":[{"msg":"b failed","at":"logvalue_test.go","attrs":{"op":"b"}}]},` +
				`{"chain":[{"msg":"c failed","at":"logvalue_test.go","attrs":{"op":"c"}},` +
				`{"msg":"d failed","at":"logvalue_test.go","attrs":{"op":"d"}}]}]}`,
		},
		"join at the root": {
			errors.Join(errors.New("b"), errors.New("c")),
			`{"msg":"b\nc","errors":[` +
				`{"chain":[{"msg":"b","at":"logvalue_test.go"}]},` +
				`{"chain":[{"msg":"c","at":"logvalue_test.go"}]}]}`,
		},
		"join of one branch": {
			errors.Join(errors.New("b")),
			`{"msg":"b","errors":[{"chain":[{"msg":"b","at":"logvalue_test.go"}]}]}`,
		},
		"wrap without a message": {
			errors.Wrap(errors.New("x"), slog.String("op", "a")),
			`{"msg":"x","chain":[{"at":"logvalue_test.go","attrs":{"op":"a"}},` +
				`{"msg":"x","at":"logvalue_test.go"}]}`,
		},
		"seal shows what it hides": {
			errors.Wrap(errors.Seal(errors.New("x", slog.String("op", "x")))),
			`{"msg":"x","chain":[{"at":"logvalue_test.go"},{"sealed":true},` +
				`{"msg":"x","at":"logvalue_test.go","attrs":{"op":"x"}}]}`,
		},
		"sealed join keeps its siblings apart": {
			errors.Seal(errors.Join(
				errors.New("b", slog.String("op", "b")),
				errors.New("c", slog.String("op", "c")),
			)),
			`{"msg":"b\nc","chain":[{"sealed":true}],"errors":[` +
				`{"chain":[{"msg":"b","at":"logvalue_test.go","attrs":{"op":"b"}}]},` +
				`{"chain":[{"msg":"c","at":"logvalue_test.go","attrs":{"op":"c"}}]}]}`,
		},
		"mark": {
			errors.Mark(errors.New("x"), marker),
			`{"msg":"x","chain":[{"mark":"marker"},{"msg":"x","at":"logvalue_test.go"}]}`,
		},
		"mark on a foreign error": {
			errors.Mark(stderrors.New("x"), marker),
			`{"msg":"x","chain":[{"mark":"marker"},{"at":"logvalue_test.go"},` +
				`{"msg":"x","type":"*errors.errorString"}]}`,
		},
		"foreign wrapper with a prefix": {
			fmt.Errorf("load config: %w", errors.New("x")),
			`{"msg":"load config: x","chain":[{"msg":"load config","type":"*fmt.wrapError"},` +
				`{"msg":"x","at":"logvalue_test.go"}]}`,
		},
		"foreign wrapper without a prefix keeps its whole text": {
			fmt.Errorf("%w (retrying)", stderrors.New("x")),
			`{"msg":"x (retrying)","chain":[{"msg":"x (retrying)","type":"*fmt.wrapError"},` +
				`{"msg":"x","type":"*errors.errorString"}]}`,
		},
		"foreign wrapper that adds nothing": {
			fmt.Errorf("%w", stderrors.New("x")),
			`{"msg":"x","chain":[{"type":"*fmt.wrapError"},` +
				`{"msg":"x","type":"*errors.errorString"}]}`,
		},
		"typed marker that says nothing names its type": {
			errors.Mark(errors.New("x"), &silentMarkerError{}),
			`{"msg":"x","chain":[{"mark":"*errors_test.silentMarkerError"},` +
				`{"msg":"x","at":"logvalue_test.go"}]}`,
		},
		"marker with empty text names its type": {
			errors.Mark(errors.New("x"), emptyMarkerError{}),
			`{"msg":"x","chain":[{"mark":"errors_test.emptyMarkerError"},` +
				`{"msg":"x","at":"logvalue_test.go"}]}`,
		},
		"multi-error with attributes of its own": {
			&batchError{errs: []error{stderrors.New("a"), stderrors.New("b")}},
			`{"msg":"a\nb","chain":[{"type":"*errors_test.batchError","attrs":{"batch":7}}],` +
				`"errors":[{"chain":[{"msg":"a","type":"*errors.errorString"}]},` +
				`{"chain":[{"msg":"b","type":"*errors.errorString"}]}]}`,
		},
		"typed nil": {
			error(typedNil),
			`{"msg":"` + panicText + `","chain":[` +
				`{"msg":"` + panicText + `","type":"*errors_test.nilDerefError"}]}`,
		},
		"typed nil beneath a wrap keeps the wrap": {
			errors.Wrap(typedNil, slog.String("op", "a")),
			`{"msg":"` + panicText + `","chain":[` +
				`{"at":"logvalue_test.go","attrs":{"op":"a"}},` +
				`{"msg":"` + panicText + `","type":"*errors_test.nilDerefError"}]}`,
		},
		"a sentinel is one of toerr's own, without a type": {
			errors.Wrap(sentinel.New("not found")),
			`{"msg":"not found","chain":[{"at":"logvalue_test.go"},{"msg":"not found"}]}`,
		},
		"foreign error with attributes": {
			&sqlFieldError{column: "email"},
			`{"msg":"bad column email","chain":[` +
				`{"msg":"bad column email","type":"*errors_test.sqlFieldError",` +
				`"attrs":{"column":"email"}}]}`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := logJSON(t, tc.err); got != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

// Attribute values follow slog's rules and its JSON handler's encodings.
func TestLogValueAttrs(t *testing.T) {
	cases := map[string]struct {
		attrs []slog.Attr
		want  string
	}{
		"a repeated key keeps its first position and takes its last value": {
			[]slog.Attr{slog.String("op", "a"), slog.Int("id", 1), slog.String("op", "b")},
			`{"op":"b","id":1}`,
		},
		"zero attribute and empty group dropped, empty key inlined": {
			[]slog.Attr{
				{},
				slog.Group("empty"), slog.Group("", slog.Int("x", 1)),
				slog.Group("db", slog.Int("rows", 2)),
			},
			`{"x":1,"db":{"rows":2}}`,
		},
		"an error value whose Error panics": {
			[]slog.Attr{slog.Any("cause", error((*nilDerefError)(nil)))},
			`{"cause":"` + panicText + `"}`,
		},
		"a redacting LogValuer still redacts": {
			[]slog.Attr{slog.Any("token", redacted("hunter2"))},
			`{"token":"REDACTED"}`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := logJSON(t, errors.New("x", tc.attrs...))
			want := `{"msg":"x","chain":[{"msg":"x","at":"logvalue_test.go","attrs":` +
				tc.want + `}]}`
			if got != want {
				t.Errorf("got\n%s\nwant\n%s", got, want)
			}
		})
	}
}

// %+v survives a typed nil in the chain: the panic text replaces its message, and the
// frames still print.
func TestTraceSurvivesATypedNil(t *testing.T) {
	cases := map[string]error{
		"wrapped": errors.Wrap((*nilDerefError)(nil)),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			got := fmt.Sprintf("%+v", err)
			if !strings.HasPrefix(got, panicText+"\n") ||
				!strings.Contains(got, "logvalue_test.go:") {
				t.Errorf("%%+v =\n%s", got)
			}
		})
	}
}

// LogValue encodes each kind of value as slog's JSON handler does, including the text
// it writes for a value it cannot encode, and drops or keeps each attribute as slog does.
func TestLogValueEncodesValuesAsSlogDoes(t *testing.T) {
	cases := map[string][]slog.Attr{
		"bool":                {slog.Bool("v", true)},
		"int":                 {slog.Int64("v", -1234567)},
		"uint":                {slog.Uint64("v", math.MaxUint64)},
		"float":               {slog.Float64("v", 1.5)},
		"NaN":                 {slog.Float64("v", math.NaN())},
		"duration":            {slog.Duration("v", 1500*time.Millisecond)},
		"time":                {slog.Time("v", time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC))},
		"time in year 0":      {slog.Time("v", time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC))},
		"time in year 9999":   {slog.Time("v", time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC))},
		"time in year 10000":  {slog.Time("v", time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC))},
		"time in year -1":     {slog.Time("v", time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC))},
		"string":              {slog.String("v", "<b> & \"quoted\"\n\u2028")},
		"group":               {slog.Group("v", slog.Int("rows", 2))},
		"slice":               {slog.Any("v", []int{1, 2})},
		"map":                 {slog.Any("v", map[string]int{"b": 1, "a": 2})},
		"HTML in an encoding": {slog.Any("v", map[string]string{"h": "<a> & <b>"})},
		"nil":                 {slog.Any("v", nil)},
		"error":               {slog.Any("v", stderrors.New("boom"))},
		"self-encoding":       {slog.Any("v", rawJSON{})},
		"unencodable":         {slog.Any("v", make(chan int))},
		"the zero attr":       {slog.Attr{}, slog.Int("v", 1)},
		"an empty key":        {slog.Any("", map[string]int{"x": 1}), slog.Int("v", 1)},
		"an empty group":      {slog.Group("g"), slog.Int("v", 1)},
	}
	for name, attrs := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
				ReplaceAttr: func(groups []string, b slog.Attr) slog.Attr {
					if len(groups) == 0 &&
						(b.Key == slog.TimeKey || b.Key == slog.LevelKey || b.Key == slog.MessageKey) {
						return slog.Attr{}
					}
					return b
				},
			})).LogAttrs(t.Context(), slog.LevelInfo, "", attrs...)
			want := strings.TrimSuffix(buf.String(), "\n")

			got := logJSON(t, errors.New("x", attrs...))
			_, gotAttrs, _ := strings.Cut(got, `"attrs":`)
			if gotAttrs = strings.TrimSuffix(gotAttrs, "}]}"); gotAttrs != want {
				t.Errorf("LogValue wrote %s, slog wrote %s", gotAttrs, want)
			}
		})
	}
}

// Outside slog, the value inside LogValue's result is what encodes the tree; the
// slog.Value itself encodes as an empty object.
func TestLogValueOutsideSlog(t *testing.T) {
	err := errors.New("disk full")
	cases := map[string]struct {
		value      any
		wantPrefix string
	}{
		"the value inside": {errors.LogValue(err).Any(), `{"msg":"disk full","chain":[`},
		"the slog.Value":   {errors.LogValue(err), `{}`},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			data, marshalErr := json.Marshal(tc.value)
			if marshalErr != nil || !strings.HasPrefix(string(data), tc.wantPrefix) {
				t.Errorf("json.Marshal = %s, %v; want prefix %s", data, marshalErr, tc.wantPrefix)
			}
		})
	}
}

// fmt and a text handler write the same JSON as a JSON handler.
func TestLogValueTextIsItsJSON(t *testing.T) {
	tree := errors.LogValue(errors.New("disk full")).Any()
	data, marshalErr := tree.(json.Marshaler).MarshalJSON()
	if marshalErr != nil {
		t.Fatalf("MarshalJSON: %v", marshalErr)
	}
	text, textErr := tree.(encoding.TextMarshaler).MarshalText()
	if textErr != nil || !bytes.Equal(text, data) {
		t.Errorf("MarshalText = %s, %v; want %s", text, textErr, data)
	}
	if got := fmt.Sprint(tree); got != string(data) {
		t.Errorf("fmt.Sprint = %s, want %s", got, data)
	}
}

// "at" is the location %+v prints for the same error, in full.
func TestLogValueAtMatchesTheTrace(t *testing.T) {
	err := errors.New("x")
	frame := regexp.MustCompile(`\t(\S+:\d+)\n`).FindStringSubmatch(fmt.Sprintf("%+v", err))
	if frame == nil {
		t.Fatalf("no frame in %+v", err)
	}
	data, _ := json.Marshal(errors.LogValue(err).Any())
	if want := fmt.Sprintf(`"at":%q`, frame[1]); !strings.Contains(string(data), want) {
		t.Errorf("%s does not contain %s", data, want)
	}
}

func TestLogValueNil(t *testing.T) {
	if v := errors.LogValue(nil); !v.Equal(slog.Value{}) {
		t.Errorf("LogValue(nil) = %v, want the zero Value", v)
	}
}

// Handlers encode the value with MarshalJSON: a JSON handler nests the object, and a
// text handler writes the same JSON as a string.
func TestLogValueInHandlers(t *testing.T) {
	err := errors.New("disk full", slog.String("device", "sda1"))
	cases := map[string]struct {
		handler func(*bytes.Buffer) slog.Handler
		want    string
	}{
		"json": {
			func(b *bytes.Buffer) slog.Handler { return slog.NewJSONHandler(b, nil) },
			`"error":{"msg":"disk full","chain":[{"msg":"disk full","at":`,
		},
		"text": {
			func(b *bytes.Buffer) slog.Handler { return slog.NewTextHandler(b, nil) },
			`error="{\"msg\":\"disk full\",\"chain\":[{\"msg\":\"disk full\",\"at\":`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			slog.New(tc.handler(&buf)).
				ErrorContext(t.Context(), "failed", slog.Any("error", errors.LogValue(err)))
			if !strings.Contains(buf.String(), tc.want) {
				t.Errorf("%s does not contain %s", buf.String(), tc.want)
			}
		})
	}
}
