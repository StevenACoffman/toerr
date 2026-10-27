package trace

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Layer is what one error in a chain contributes to LogValue's output. Empty fields
// are left out.
type Layer struct {
	Msg    string // this error's own message, without its cause's
	Code   string
	Class  string
	Mark   string
	Sealed bool
	Attrs  []slog.Attr
}

// Describer is implemented by this module's error types to describe their own layer.
// Like HiddenCause, it takes a Token so that only this module can implement it; any
// other error is described from its Error, Unwrap, and Attrs methods.
type Describer interface {
	DescribeLayer(Token) Layer
}

// Tree is an error rendered as JSON: the full message, the chain of layers down to a
// join or a leaf, and one such tree per branch of the join. It is built when it is
// marshaled. It is deliberately neither an error nor a slog.LogValuer, either of which
// a handler would turn into something else before it reached MarshalJSON.
type Tree struct {
	err error
}

// object writes the members of a JSON object, separating them with commas.
type object struct {
	b    *bytes.Buffer
	open bool
}

// member is one key of a JSON object and its resolved value.
type member struct {
	key   string
	value slog.Value
}

// members collects the members of one JSON object in order, without repeating a key.
type members struct {
	list  []member
	index map[string]int
}

// NewTree returns err's tree.
func NewTree(err error) Tree { return Tree{err: err} }

// MarshalJSON implements json.Marshaler. It never fails: a value that cannot be
// encoded is written as a string beginning "!ERROR:", as slog's JSON handler does.
func (t Tree) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	o := object{b: &b}
	o.member("msg", func() { writeString(&b, t.err.Error()) }, true)
	writeBranch(&o, t.err)
	o.close()
	return b.Bytes(), nil
}

// MarshalText implements encoding.TextMarshaler, so a text handler writes the same JSON.
func (t Tree) MarshalText() ([]byte, error) { return t.MarshalJSON() }

// String returns the JSON.
func (t Tree) String() string {
	b, _ := t.MarshalJSON()
	return string(b)
}

// writeBranch writes the "chain" and "errors" members of the tree rooted at err into o.
func writeBranch(o *object, err error) {
	var chain []error
	var branches []error
	for err != nil {
		if multi, ok := err.(interface{ Unwrap() []error }); ok {
			branches = multi.Unwrap()
			break
		}
		chain = append(chain, err)
		err = next(err)
	}
	o.member("chain", func() {
		o.b.WriteByte('[')
		for i, e := range chain {
			if i > 0 {
				o.b.WriteByte(',')
			}
			writeLayer(o.b, e, next(e))
		}
		o.b.WriteByte(']')
	}, len(chain) > 0)
	o.member("errors", func() {
		o.b.WriteByte('[')
		for i, e := range branches {
			if i > 0 {
				o.b.WriteByte(',')
			}
			branch := object{b: o.b}
			writeBranch(&branch, e)
			branch.close()
		}
		o.b.WriteByte(']')
	}, len(branches) > 0)
}

// next returns the error beneath err in its chain, following a seal's hidden cause.
func next(err error) (cause error) {
	switch x := err.(type) { //nolint:errorlint // deliberate single-node inspection.
	case interface{ Unwrap() error }:
		cause = x.Unwrap()
	case HiddenCause:
		cause = x.TraceCause(Token{})
	}
	return cause //nolint:wrapcheck // the next node of the walk, not an error for a caller.
}

// writeLayer writes err's layer as a JSON object.
func writeLayer(b *bytes.Buffer, err, cause error) {
	l := describe(err, cause)
	o := object{b: b}
	o.member("msg", func() { writeString(b, l.Msg) }, l.Msg != "")
	o.member("code", func() { writeString(b, l.Code) }, l.Code != "")
	o.member("class", func() { writeString(b, l.Class) }, l.Class != "")
	o.member("mark", func() { writeString(b, l.Mark) }, l.Mark != "")
	o.member("sealed", func() { b.WriteString("true") }, l.Sealed)
	if at := location(err); at != "" {
		o.member("at", func() { writeString(b, at) }, true)
	}
	if members := collect(l.Attrs); len(members) > 0 {
		o.member("attrs", func() { writeMembers(b, members) }, true)
	}
	o.close()
}

// describe returns err's layer: its own, if it is one of this module's errors, and
// otherwise one built from its methods.
func describe(err, cause error) Layer {
	if d, ok := err.(Describer); ok {
		return d.DescribeLayer(Token{})
	}
	var l Layer
	if a, ok := err.(interface{ Attrs() []slog.Attr }); ok {
		l.Attrs = a.Attrs()
	}
	l.Msg = ownMessage(err.Error(), cause)
	return l
}

// ownMessage returns the part of msg that err adds to its cause: msg with the cause's
// text and the ": " before it cut off the end. If msg does not end that way, the
// whole of msg is err's own.
func ownMessage(msg string, cause error) string {
	if cause == nil {
		return msg
	}
	causeMsg := cause.Error()
	if msg == causeMsg {
		return ""
	}
	if prefix, ok := strings.CutSuffix(msg, ": "+causeMsg); ok {
		return prefix
	}
	return msg
}

// location returns the file:line of err's call site, or "" if it recorded none.
func location(err error) string {
	e, ok := err.(interface{ TracePC() uintptr })
	if !ok || e.TracePC() == 0 {
		return ""
	}
	f, _ := runtime.CallersFrames([]uintptr{e.TracePC()}).Next()
	if f.File == "" {
		return ""
	}
	return f.File + ":" + strconv.Itoa(f.Line)
}

// collect resolves attrs into the members of one JSON object, by slog's rules: the
// zero Attr is dropped, a group with an empty key is inlined, and an empty group is
// dropped. A key that repeats keeps its first position and takes its last value.
func collect(attrs []slog.Attr) []member {
	var o members
	o.add(attrs)
	return o.list
}

func (o *members) add(attrs []slog.Attr) {
	for _, a := range attrs {
		// The zero Attr, tested as slog does: Equal would panic on a slice value.
		if a.Key == "" && a.Value.Kind() == slog.KindAny && a.Value.Any() == nil {
			continue
		}
		v := a.Value.Resolve()
		if v.Kind() == slog.KindGroup {
			if a.Key == "" {
				o.add(v.Group())
				continue
			}
			if len(v.Group()) == 0 {
				continue
			}
		}
		o.set(a.Key, v)
	}
}

func (o *members) set(key string, v slog.Value) {
	if i, ok := o.index[key]; ok {
		o.list[i].value = v
		return
	}
	if o.index == nil {
		o.index = map[string]int{}
	}
	o.index[key] = len(o.list)
	o.list = append(o.list, member{key: key, value: v})
}

func writeMembers(b *bytes.Buffer, members []member) {
	b.WriteByte('{')
	for i, m := range members {
		if i > 0 {
			b.WriteByte(',')
		}
		writeString(b, m.key)
		b.WriteByte(':')
		writeValue(b, m.value)
	}
	b.WriteByte('}')
}

// writeValue writes a resolved value with the encodings slog's JSON handler uses.
func writeValue(b *bytes.Buffer, v slog.Value) {
	switch v.Kind() {
	case slog.KindString:
		writeString(b, v.String())
	case slog.KindInt64:
		b.WriteString(strconv.FormatInt(v.Int64(), 10))
	case slog.KindUint64:
		b.WriteString(strconv.FormatUint(v.Uint64(), 10))
	case slog.KindBool:
		b.WriteString(strconv.FormatBool(v.Bool()))
	case slog.KindDuration:
		b.WriteString(strconv.FormatInt(int64(v.Duration()), 10))
	case slog.KindTime:
		writeString(b, v.Time().Format(time.RFC3339Nano))
	case slog.KindGroup:
		writeMembers(b, collect(v.Group()))
	case slog.KindFloat64:
		writeAny(b, v.Float64())
	case slog.KindAny, slog.KindLogValuer:
		writeAny(b, v.Any())
	}
}

// writeAny writes an error without its own JSON encoding as its text, and anything
// else as encoding/json writes it.
func writeAny(b *bytes.Buffer, x any) {
	_, marshaler := x.(json.Marshaler)
	if err, ok := x.(error); ok && !marshaler {
		writeString(b, err.Error())
		return
	}
	data, err := marshal(x)
	if err != nil {
		writeString(b, "!ERROR:"+err.Error())
		return
	}
	b.Write(data)
}

// writeString writes s as a JSON string, escaped as encoding/json escapes it apart
// from HTML: quotes, backslashes, control characters, U+2028 and U+2029 are escaped,
// and invalid UTF-8 becomes U+FFFD.
func writeString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	start := 0
	for i := 0; i < len(s); {
		c := s[i]
		if c < utf8.RuneSelf {
			i++
			if c >= ' ' && c != '"' && c != '\\' {
				continue
			}
			b.WriteString(s[start : i-1])
			writeEscapedByte(b, c)
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			b.WriteString(s[start:i])
			b.WriteRune(utf8.RuneError)
			start = i + size
		case r == '\u2028' || r == '\u2029':
			b.WriteString(s[start:i])
			b.WriteString(`\u`)
			b.WriteString(strconv.FormatInt(int64(r), 16))
			start = i + size
		}
		i += size
	}
	b.WriteString(s[start:])
	b.WriteByte('"')
}

// writeEscapedByte writes an ASCII byte that a JSON string must escape.
func writeEscapedByte(b *bytes.Buffer, c byte) {
	switch c {
	case '"', '\\':
		b.WriteByte('\\')
		b.WriteByte(c)
	case '\n':
		b.WriteString(`\n`)
	case '\r':
		b.WriteString(`\r`)
	case '\t':
		b.WriteString(`\t`)
	case '\b':
		b.WriteString(`\b`)
	case '\f':
		b.WriteString(`\f`)
	default:
		b.WriteString(`\u00`)
		b.WriteString(strconv.FormatInt(int64(c)|0x100, 16)[1:])
	}
}

// marshal is json.Marshal without HTML escaping, which a log line does not need.
func marshal(x any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(x); err != nil {
		return nil, fmt.Errorf("encode %T: %w", x, err)
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}

func (o *object) member(key string, value func(), present bool) {
	if !present {
		return
	}
	if o.open {
		o.b.WriteByte(',')
	} else {
		o.b.WriteByte('{')
		o.open = true
	}
	writeString(o.b, key)
	o.b.WriteByte(':')
	value()
}

func (o *object) close() {
	if !o.open {
		o.b.WriteByte('{')
	}
	o.b.WriteByte('}')
}
