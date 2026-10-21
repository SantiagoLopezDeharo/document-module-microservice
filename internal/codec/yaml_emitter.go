package codec

import (
	"bufio"
	"encoding/base64"
	"io"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// yamlEmitter writes the plain data model produced by the decoders (maps,
// slices, strings, numbers, booleans and null) directly as block YAML.
// yaml.v3 keeps every emitter event of a document queued until it finishes,
// allocating about 400 bytes per input byte on large lists; this writer
// streams with no per-value allocations beyond key sorting.
type yamlEmitter struct {
	w *bufio.Writer
}

type yamlContext int

const (
	yamlDocument yamlContext = iota
	yamlAfterKey
	yamlAfterDash
)

// emitPlainYAML reports false without writing when data holds a type it does
// not handle, so the caller can fall back to yaml.v3.
func emitPlainYAML(w io.Writer, data interface{}) (bool, error) {
	if !plainYAMLData(data) {
		return false, nil
	}
	e := yamlEmitter{w: bufio.NewWriterSize(w, 32<<10)}
	e.node(data, yamlDocument, 0)
	return true, e.w.Flush()
}

func plainYAMLData(v interface{}) bool {
	switch val := v.(type) {
	case nil, string, bool, float64, float32, int, int64, uint64:
		return true
	case map[string]interface{}:
		for key, child := range val {
			if !utf8.ValidString(key) || !plainYAMLData(child) {
				return false
			}
		}
		return true
	case []interface{}:
		for _, child := range val {
			if !plainYAMLData(child) {
				return false
			}
		}
		return true
	case []map[string]interface{}:
		for _, child := range val {
			if !plainYAMLData(child) {
				return false
			}
		}
		return true
	}
	return false
}

func (e *yamlEmitter) node(v interface{}, ctx yamlContext, indent int) {
	switch val := v.(type) {
	case map[string]interface{}:
		e.mapping(val, ctx, indent)
	case []interface{}:
		e.sequence(len(val), func(i int) interface{} { return val[i] }, ctx, indent)
	case []map[string]interface{}:
		e.sequence(len(val), func(i int) interface{} { return val[i] }, ctx, indent)
	default:
		e.scalar(val, ctx)
	}
}

func (e *yamlEmitter) mapping(m map[string]interface{}, ctx yamlContext, indent int) {
	if len(m) == 0 {
		e.inline("{}", ctx)
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for i, k := range keys {
		e.startLine(i, ctx, indent)
		if len(k) > maxImplicitKeyBytes {
			e.w.WriteString("? ")
			e.str(k)
			e.w.WriteByte('\n')
			e.pad(indent)
		} else {
			e.str(k)
		}
		e.w.WriteByte(':')
		e.node(m[k], yamlAfterKey, indent+2)
	}
}

// YAML limits implicit keys to 1024 characters and escaping can grow a key
// sixfold, so longer keys use the explicit "? key" form.
const maxImplicitKeyBytes = 128

func (e *yamlEmitter) sequence(n int, item func(int) interface{}, ctx yamlContext, indent int) {
	if n == 0 {
		e.inline("[]", ctx)
		return
	}
	for i := range n {
		e.startLine(i, ctx, indent)
		e.w.WriteString("- ")
		e.node(item(i), yamlAfterDash, indent+2)
	}
}

// startLine begins the i-th entry of a collection. After a dash the first
// entry continues on the dash's line; after a key every entry gets its own.
func (e *yamlEmitter) startLine(i int, ctx yamlContext, indent int) {
	if i == 0 && ctx != yamlAfterKey {
		return
	}
	if i == 0 {
		e.w.WriteByte('\n')
	}
	e.pad(indent)
}

func (e *yamlEmitter) pad(indent int) {
	for range indent {
		e.w.WriteByte(' ')
	}
}

func (e *yamlEmitter) inline(text string, ctx yamlContext) {
	if ctx == yamlAfterKey {
		e.w.WriteByte(' ')
	}
	e.w.WriteString(text)
	e.w.WriteByte('\n')
}

func (e *yamlEmitter) scalar(v interface{}, ctx yamlContext) {
	if ctx == yamlAfterKey {
		e.w.WriteByte(' ')
	}
	var buf [32]byte
	switch val := v.(type) {
	case nil:
		e.w.WriteString("null")
	case bool:
		e.w.Write(strconv.AppendBool(buf[:0], val))
	case int:
		e.w.Write(strconv.AppendInt(buf[:0], int64(val), 10))
	case int64:
		e.w.Write(strconv.AppendInt(buf[:0], val, 10))
	case uint64:
		e.w.Write(strconv.AppendUint(buf[:0], val, 10))
	case float32:
		e.float(float64(val), 32)
	case float64:
		e.float(val, 64)
	case string:
		e.str(val)
	}
	e.w.WriteByte('\n')
}

// float matches yaml.v3's formatting so numbers decode to the same values,
// except negative zero, which "-0" would turn into the integer 0.
func (e *yamlEmitter) float(f float64, bits int) {
	if f == 0 && math.Signbit(f) {
		e.w.WriteString("-0.0")
		return
	}
	var buf [32]byte
	s := strconv.AppendFloat(buf[:0], f, 'g', -1, bits)
	switch string(s) {
	case "+Inf":
		e.w.WriteString(".inf")
	case "-Inf":
		e.w.WriteString("-.inf")
	case "NaN":
		e.w.WriteString(".nan")
	default:
		e.w.Write(s)
	}
}

// yamlPlainSafe accepts strings that every YAML 1.1 and 1.2 parser reads back
// as the same string when unquoted. Everything else is double-quoted.
var yamlPlainSafe = regexp.MustCompile(`^[A-Za-z_](?:[A-Za-z0-9_ ./@-]*[A-Za-z0-9_./@])?$`)

var yamlReserved = map[string]bool{
	"true": true, "false": true, "yes": true, "no": true, "on": true, "off": true, "y": true, "n": true,
	"null": true, "nan": true, "inf": true,
}

func (e *yamlEmitter) str(s string) {
	if !utf8.ValidString(s) {
		e.w.WriteString("!!binary ")
		e.w.WriteString(base64.StdEncoding.EncodeToString([]byte(s)))
		return
	}
	if len(s) <= 64 && yamlPlainSafe.MatchString(s) && !yamlReserved[strings.ToLower(s)] {
		e.w.WriteString(s)
		return
	}
	e.quoted(s)
}

// yamlMustEscape reports runes outside YAML's printable set, plus the line
// and BOM characters that parsers would fold or strip.
func yamlMustEscape(r rune) bool {
	switch {
	case r < 0x20, r == 0x7f, r >= 0x80 && r < 0xa0:
		return true
	case r == 0x2028, r == 0x2029, r == 0xfeff, r == 0xfffe, r == 0xffff:
		return true
	case r >= 0xd800 && r <= 0xdfff:
		return true
	}
	return false
}

func (e *yamlEmitter) quoted(s string) {
	const hex = "0123456789ABCDEF"
	e.w.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			e.w.WriteByte('\\')
			e.w.WriteRune(r)
		case r == '\n':
			e.w.WriteString(`\n`)
		case r == '\t':
			e.w.WriteString(`\t`)
		case r == '\r':
			e.w.WriteString(`\r`)
		case yamlMustEscape(r):
			e.w.WriteString(`\u`)
			for shift := 12; shift >= 0; shift -= 4 {
				e.w.WriteByte(hex[(r>>shift)&0xf])
			}
		default:
			e.w.WriteRune(r)
		}
	}
	e.w.WriteByte('"')
}
