package codec

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
	"github.com/vmihailenco/msgpack/v5"
)

func nested(format models.Format, depth int) string {
	switch format {
	case models.FormatJSON, models.FormatYAML:
		return strings.Repeat("[", depth) + strings.Repeat("]", depth)
	case models.FormatXML:
		return strings.Repeat("<a>", depth) + strings.Repeat("</a>", depth)
	case models.FormatTOML:
		return "a = " + strings.Repeat("[", depth) + strings.Repeat("]", depth)
	case models.FormatMsgPack:
		return strings.Repeat("\x91", depth-1) + "\x90"
	case models.FormatProperties:
		return strings.Repeat("a.", depth-1) + "a = 1"
	}
	panic("no nesting for " + format)
}

var nestableFormats = []models.Format{
	models.FormatJSON, models.FormatYAML, models.FormatXML, models.FormatTOML, models.FormatMsgPack, models.FormatProperties,
}

// The XML, TOML and MessagePack decoders used to recurse until the goroutine
// stack overflowed, which is a fatal error that kills the whole process.
func TestDecodeRejectsPathologicalNesting(t *testing.T) {
	for _, format := range nestableFormats {
		t.Run(string(format), func(t *testing.T) {
			c, err := GetCodec(format)
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.Decode(strings.NewReader(nested(format, 1_000_000)))
			if err == nil {
				t.Fatal("million-level nesting accepted")
			}
			if format != models.FormatJSON && format != models.FormatYAML && !errors.Is(err, ErrTooDeep) {
				t.Errorf("error = %v, want ErrTooDeep", err)
			}
		})
	}
}

func TestDecodeAcceptsReasonableNesting(t *testing.T) {
	for _, format := range nestableFormats {
		t.Run(string(format), func(t *testing.T) {
			c, _ := GetCodec(format)
			if _, err := c.Decode(strings.NewReader(nested(format, 500))); err != nil {
				t.Fatalf("500 levels rejected: %v", err)
			}
		})
	}
}

func TestTOMLNestingVariants(t *testing.T) {
	deep := MaxDepth + 1
	rejected := map[string]string{
		"arrays":        "a = " + strings.Repeat("[", deep) + strings.Repeat("]", deep),
		"inline tables": "a = " + strings.Repeat("{b = ", deep) + "1" + strings.Repeat("}", deep),
		"dotted keys":   strings.Repeat("a.", deep) + "a = 1",
		"table header":  "[" + strings.Repeat("a.", deep) + "a]\nb = 1",
		"mixed":         "a = " + strings.Repeat("[{b = ", deep/2+1) + "1" + strings.Repeat("}]", deep/2+1),
	}
	for name, doc := range rejected {
		if err := checkTOMLDepth([]byte(doc)); !errors.Is(err, ErrTooDeep) {
			t.Errorf("%s: err = %v, want ErrTooDeep", name, err)
		}
	}

	accepted := map[string]string{
		"brackets in basic string":     `a = "` + strings.Repeat("[", deep) + `"`,
		"brackets in literal string":   `a = '` + strings.Repeat("{", deep) + `'`,
		"brackets in multiline":        "a = \"\"\"\n" + strings.Repeat("[", deep) + "\n\"\"\"",
		"brackets in literal multi":    "a = '''\n" + strings.Repeat("[", deep) + "\n'''",
		"escaped quote keeps string":   `a = "\"` + strings.Repeat("[", deep) + `"`,
		"brackets in comment":          "# " + strings.Repeat("[", deep) + "\na = 1",
		"dots in values":               "a = [" + strings.Repeat("1.5, ", deep) + "1.5]",
		"dots in quoted keys":          `"` + strings.Repeat("a.", deep) + `" = 1`,
		"many sequential arrays":       strings.Repeat("a = [[1]]\n", deep),
		"many sequential tables":       strings.Repeat("[[t]]\nx = 1\n", deep),
		"dates and times":              "d = 1979-05-27T07:32:00.999999Z",
		"unterminated string":          `a = "` + strings.Repeat("[", 10),
		"unterminated comment at EOF":  "a = 1 # " + strings.Repeat("[", deep),
		"unterminated multiline basic": "a = \"\"\"" + strings.Repeat("[", deep),
		"unbalanced closers":           strings.Repeat("]", deep),
	}
	for name, doc := range accepted {
		if err := checkTOMLDepth([]byte(doc)); err != nil {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestXMLNestingIgnoresMarkupInText(t *testing.T) {
	deep := MaxDepth + 1
	accepted := map[string]string{
		"cdata":        "<r><![CDATA[" + strings.Repeat("<a>", deep) + "]]></r>",
		"comment":      "<r><!--" + strings.Repeat("<a>", deep) + "--></r>",
		"siblings":     "<r>" + strings.Repeat("<a/>", deep) + "</r>",
		"broken input": "<r><a",
	}
	for name, doc := range accepted {
		if err := checkXMLDepth([]byte(doc)); err != nil {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if err := checkXMLDepth([]byte(strings.Repeat("<a x='1'>", deep))); !errors.Is(err, ErrTooDeep) {
		t.Errorf("unclosed deep elements: err = %v", err)
	}
}

func TestMsgpackScannerHandlesEveryType(t *testing.T) {
	values := []interface{}{
		nil, true, false,
		0, 127, -1, -32, -33, 128, 255, 256, 65535, 65536, int64(1) << 40, int64(-1) << 40, uint64(1) << 63,
		float32(1.5), 2.5,
		"", "short", strings.Repeat("s", 40), strings.Repeat("m", 300), strings.Repeat("l", 70000),
		[]byte{}, []byte("bin"), bytes.Repeat([]byte{1}, 300), bytes.Repeat([]byte{2}, 70000),
		[]interface{}{}, make([]interface{}, 20), make([]interface{}, 70000),
		map[string]interface{}{}, map[string]interface{}{"k": 1},
		bigMap(20), bigMap(70000),
		map[string]interface{}{"nested": []interface{}{map[string]interface{}{"x": []interface{}{1, "two", nil}}}},
	}
	for i, v := range values {
		data, err := msgpack.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := checkMsgpack(data); err != nil {
			t.Errorf("value %d (%T) rejected: %v", i, v, err)
		}
		pending, end := scanMsgpack(data)
		if pending != 0 || end != len(data) {
			t.Errorf("value %d (%T): scanner left %d open containers at offset %d/%d", i, v, pending, end, len(data))
		}
	}

	exts := map[string][]byte{
		"fixext1":  {0xd4, 1, 0},
		"fixext2":  {0xd5, 1, 0, 0},
		"fixext4":  {0xd6, 1, 0, 0, 0, 0},
		"fixext8":  append([]byte{0xd7, 1}, make([]byte, 8)...),
		"fixext16": append([]byte{0xd8, 1}, make([]byte, 16)...),
		"ext8":     {0xc7, 2, 1, 0, 0},
		"ext16":    {0xc8, 0, 2, 1, 0, 0},
		"ext32":    {0xc9, 0, 0, 0, 2, 1, 0, 0},
	}
	for name, data := range exts {
		if pending, end := scanMsgpack(append([]byte{0x91}, data...)); pending != 0 || end != len(data)+1 {
			t.Errorf("%s: pending %d end %d", name, pending, end)
		}
	}
}

// scanMsgpack mirrors checkMsgpack and reports where the walk ended.
func scanMsgpack(data []byte) (open, end int) {
	var pending []uint64
	i := 0
	for i < len(data) {
		items, next, err := msgpackItem(data, i)
		if err != nil {
			return len(pending), i
		}
		i = next
		if items > 0 {
			pending = append(pending, items)
			continue
		}
		for len(pending) > 0 {
			pending[len(pending)-1]--
			if pending[len(pending)-1] > 0 {
				break
			}
			pending = pending[:len(pending)-1]
		}
	}
	return len(pending), i
}

func bigMap(n int) map[string]interface{} {
	m := make(map[string]interface{}, n)
	for i := range n {
		m[strings.Repeat("k", 1+i%5)+string(rune('a'+i%26))+strings.Repeat("x", i/130)] = i
	}
	return m
}

func TestMsgpackRejectsMalformedStructure(t *testing.T) {
	if err := checkMsgpack(nil); err != nil {
		t.Errorf("empty input: %v", err)
	}
	inputs := [][]byte{
		{0xc1},
		{0xdc},
		{0xdd, 0xff, 0xff, 0xff, 0xff},
		{0xdb, 0xff, 0xff, 0xff, 0xff, 'a'},
		{0xc7, 0x05},
		{0xd8, 1},
		{0xcb, 0, 0},
		{0x92, 0x01},
		{0x81, 0xa1, 'k'},
		{0xdf, 0xff, 0xff, 0xff, 0xff},
		{0xc6, 0xff, 0xff, 0xff, 0xff},
		{0xdc, 0x00, 0x03, 0x01, 0x02},
	}
	for _, in := range inputs {
		if err := checkMsgpack(in); err == nil {
			t.Errorf("% x accepted", in)
		}
	}
	deepMaps := bytes.Repeat([]byte{0x81, 0xa1, 'k'}, MaxDepth+1)
	if err := checkMsgpack(deepMaps); !errors.Is(err, ErrTooDeep) {
		t.Errorf("deep maps: err = %v", err)
	}
	deepArray32 := append(bytes.Repeat([]byte{0xdd, 0, 0, 0, 1}, MaxDepth+1), 0xc0)
	if err := checkMsgpack(deepArray32); !errors.Is(err, ErrTooDeep) {
		t.Errorf("deep array32: err = %v", err)
	}
}

func TestCheckKeyDepth(t *testing.T) {
	if err := checkKeyDepth(strings.Repeat("a.", MaxDepth-1) + "a"); err != nil {
		t.Errorf("key at the limit rejected: %v", err)
	}
	if err := checkKeyDepth(strings.Repeat("a[0]", MaxDepth)); !errors.Is(err, ErrTooDeep) {
		t.Errorf("deep indexed key: err = %v", err)
	}
}

func FuzzCheckTOMLDepth(f *testing.F) {
	for _, seed := range []string{"a = [1, [2]]", `a = "[["`, "[t.u]\nx = {y = [1]}", "a = '''[[['''", "# [[\n"} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = checkTOMLDepth(data)
	})
}

func FuzzCheckMsgpack(f *testing.F) {
	for _, v := range []interface{}{nil, []interface{}{1, "a"}, map[string]interface{}{"k": []byte("v")}} {
		data, _ := msgpack.Marshal(v)
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = checkMsgpack(data)
	})
}
