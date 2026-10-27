package trace

import (
	"bytes"
	"encoding/json"
	"log/slog"
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

// JSONTree is an error rendered as the JSON object LogValue documents: the full
// message, the chain of layers down to a join or the root cause, and one such tree per
// branch of the join. It is built when it is marshaled. It is deliberately neither an
// error nor a slog.LogValuer, either of which a handler would turn into something else
// before it reached MarshalJSON.
type JSONTree struct {
	err error
}

// object writes a JSON object's members, separating them with commas.
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

// NewJSONTree returns err's tree. err must not be nil.
func NewJSONTree(err error) JSONTree { return JSONTree{err: err} }

// MarshalJSON implements json.Marshaler. It never fails: a value that cannot be
// encoded is written as a string beginning "!ERROR:", as slog's JSON handler does.
func (t JSONTree) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	o := object{b: &b}
	o.key("msg")
	writeString(&b, t.err.Error())
	writeBranch(&o, buildTree(t.err))
	o.close()
	return b.Bytes(), nil
}

// MarshalText implements encoding.TextMarshaler, so a text handler writes the same JSON.
func (t JSONTree) MarshalText() ([]byte, error) { return t.MarshalJSON() }

// String returns the JSON, for fmt and for slog.Value.String.
func (t JSONTree) String() string {
	b, _ := t.MarshalJSON()
	return string(b)
}

// writeBranch writes t's "chain" and "errors" members into o.
func writeBranch(o *object, t tree) {
	layers := t.chain
	if len(t.children) > 0 && isEmptyJoin(layers[len(layers)-1]) {
		layers = layers[:len(layers)-1] // a plain join adds nothing but its branches.
	}
	if len(layers) > 0 {
		o.key("chain")
		writeChain(o.b, layers, t.chain)
	}
	if len(t.children) > 0 {
		o.key("errors")
		writeBranches(o.b, t.children)
	}
}

// writeChain writes the layers as a JSON array. chain is the whole chain the layers
// begin, which gives each layer its cause.
func writeChain(b *bytes.Buffer, layers, chain []error) {
	b.WriteByte('[')
	for i, err := range layers {
		if i > 0 {
			b.WriteByte(',')
		}
		var cause error
		if i+1 < len(chain) {
			cause = chain[i+1]
		}
		writeLayer(b, err, cause)
	}
	b.WriteByte(']')
}

// writeBranches writes a join's branches as a JSON array of trees without a message.
func writeBranches(b *bytes.Buffer, children []tree) {
	b.WriteByte('[')
	for i, child := range children {
		if i > 0 {
			b.WriteByte(',')
		}
		o := object{b: b}
		writeBranch(&o, child)
		o.close()
	}
	b.WriteByte(']')
}

// isEmptyJoin reports whether a multi-error has nothing of its own to show: no
// description, no attributes, and no call site, as errors.Join's has none.
func isEmptyJoin(err error) bool {
	_, described := err.(Describer)
	_, attrs := err.(interface{ Attrs() []slog.Attr })
	_, located := frame(err)
	return !described && !attrs && !located
}

// writeLayer writes err's layer as a JSON object. cause is the next error in the
// chain, or nil.
func writeLayer(b *bytes.Buffer, err, cause error) {
	l := describe(err, cause)
	o := object{b: b}
	for _, field := range []struct{ key, value string }{
		{"msg", l.Msg}, {"code", l.Code}, {"class", l.Class}, {"mark", l.Mark},
	} {
		if field.value != "" {
			o.key(field.key)
			writeString(b, field.value)
		}
	}
	if l.Sealed {
		o.key("sealed")
		b.WriteString("true")
	}
	if f, ok := frame(err); ok {
		o.key("at")
		writeString(b, f.File+":"+strconv.Itoa(f.Line))
	}
	if attrs := collect(l.Attrs); len(attrs) > 0 {
		o.key("attrs")
		writeMembers(b, attrs)
	}
	o.close()
}

// describe returns err's layer: its own, if it is one of this module's errors, and
// otherwise one built from its methods. A multi-error's text is its branches', so it
// has no message of its own.
func describe(err, cause error) Layer {
	if d, ok := err.(Describer); ok {
		return d.DescribeLayer(Token{})
	}
	var l Layer
	if a, ok := err.(interface{ Attrs() []slog.Attr }); ok {
		l.Attrs = a.Attrs()
	}
	if _, multi := err.(interface{ Unwrap() []error }); !multi {
		l.Msg = ownMessage(err.Error(), cause)
	}
	return l
}

// ownMessage returns the part of msg that err adds to its cause: msg with the cause's
// text and the ": " before it cut off the end. If msg does not end that way, the
// whole of msg is err's own. This is cockroachdb/errors' rule for a wrapper it has no
// encoder for.
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

func writeMembers(b *bytes.Buffer, list []member) {
	b.WriteByte('{')
	for i, m := range list {
		if i > 0 {
			b.WriteByte(',')
		}
		writeString(b, m.key)
		b.WriteByte(':')
		writeValue(b, m.value)
	}
	b.WriteByte('}')
}

// writeValue writes a resolved value as slog's JSON handler writes it.
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
		writeTime(b, v.Time())
	case slog.KindGroup:
		writeMembers(b, collect(v.Group()))
	case slog.KindFloat64:
		writeJSON(b, v.Float64())
	case slog.KindAny, slog.KindLogValuer: // Resolve leaves no LogValuer; listed for exhaustive.
		writeAny(b, v.Any())
	}
}

// writeTime writes t in RFC 3339 with nanoseconds, or an error for a year RFC 3339
// cannot hold, as slog's JSON handler does.
func writeTime(b *bytes.Buffer, t time.Time) {
	if y := t.Year(); y < 0 || y >= 10000 {
		writeString(b, "!ERROR:time.Time year outside of range [0,9999]")
		return
	}
	writeString(b, t.Format(time.RFC3339Nano))
}

// writeAny writes an error without its own JSON encoding as its text, and anything
// else as encoding/json writes it.
func writeAny(b *bytes.Buffer, x any) {
	_, marshaler := x.(json.Marshaler)
	if err, ok := x.(error); ok && !marshaler {
		writeString(b, err.Error())
		return
	}
	writeJSON(b, x)
}

// writeJSON writes x as encoding/json does without HTML escaping, which a log line
// does not need, or, if x cannot be encoded, the error as slog's JSON handler writes it.
func writeJSON(b *bytes.Buffer, x any) {
	var data bytes.Buffer
	enc := json.NewEncoder(&data)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(x); err != nil {
		writeString(b, "!ERROR:"+err.Error())
		return
	}
	b.Write(bytes.TrimSuffix(data.Bytes(), []byte("\n")))
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
		case r == ' ' || r == ' ':
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

// key writes the next member's key, after a comma or the object's opening brace.
func (o *object) key(k string) {
	if o.open {
		o.b.WriteByte(',')
	} else {
		o.b.WriteByte('{')
		o.open = true
	}
	writeString(o.b, k)
	o.b.WriteByte(':')
}

// close ends the object, writing {} if it had no members.
func (o *object) close() {
	if !o.open {
		o.b.WriteByte('{')
	}
	o.b.WriteByte('}')
}
