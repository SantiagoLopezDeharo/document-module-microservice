package codec

import (
	"bytes"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

var yamlTrickyStrings = []string{
	"", " ", "plain", "two words", "trailing ", " leading", "Ada Lovelace", "user@example.com", "path/to/file", "a.b.c", "snake_case", "kebab-case",
	"true", "True", "TRUE", "false", "yes", "Yes", "no", "on", "Off", "y", "N", "null", "Null", "~", "nan", "NaN", ".nan", "inf", ".inf", "-.inf",
	"0", "007", "-1", "+1", ".5", "1e3", "1_000", "0x1F", "0o17", "0b101", "1:30", "12:30:00", "2026-10-21", "2026-10-21T10:00:00Z",
	"-", "- item", "?", "? key", ":", ": value", "a: b", "a:b", "a #b", "#comment", "@at", "`tick", "!tag", "&anchor", "*alias", "|literal", ">folded",
	"%directive", "{flow}", "[flow]", "'single'", `"double"`, "back\\slash", "line\nbreak", "cr\rhere", "tab\there", "nul\x00byte", "esc\x1b[0m",
	"bell\a", "del\x7f", "nel\u0085", "ls\u2028ps\u2029", "bom\ufeff", "é accent", "日本語", "emoji 🚀", "mixed ✓ ascii", "---", "...", "--- doc",
	strings.Repeat("long", 40), "\xff\xfe invalid utf8", "C1\u0090control", "nonchar\uffff\ufffe", "private\ue000", "astral\U0001F600",
}

func reencodeWithYAMLV3(t testing.TB, v interface{}) interface{} {
	t.Helper()
	out, err := yaml.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var back interface{}
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	return back
}

func viaPlainEmitter(t testing.TB, v interface{}) (interface{}, string) {
	t.Helper()
	var buf bytes.Buffer
	ok, err := emitPlainYAML(&buf, v)
	if err != nil || !ok {
		t.Fatalf("emitPlainYAML = %v, %v", ok, err)
	}
	var back interface{}
	if err := yaml.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("emitted YAML does not parse: %v\n%s", err, buf.String())
	}
	return back, buf.String()
}

// canonicalYAML maps a value onto what YAML can distinguish: numbers compare
// by their shortest decimal form, since yaml.v3 decodes integral numbers as
// int and JSON produces float64.
func canonicalYAML(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, child := range val {
			out[k] = canonicalYAML(child)
		}
		return out
	case []map[string]interface{}:
		out := make([]interface{}, len(val))
		for i, child := range val {
			out[i] = canonicalYAML(child)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(val))
		for i, child := range val {
			out[i] = canonicalYAML(child)
		}
		return out
	case int:
		return strconv.FormatInt(int64(val), 10)
	case int64:
		return strconv.FormatInt(val, 10)
	case uint64:
		return strconv.FormatUint(val, 10)
	case float32:
		return strconv.FormatFloat(float64(val), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(val, 'g', -1, 64)
	}
	return v
}

// assertRoundTrips checks that yaml.v3 reads the emitted document back as
// the original value.
func assertRoundTrips(t testing.TB, v interface{}) {
	t.Helper()
	got, doc := viaPlainEmitter(t, v)
	if want := canonicalYAML(v); !reflect.DeepEqual(canonicalYAML(got), want) {
		t.Errorf("value %#v\nemitted:\n%s\ndecodes to %#v", v, doc, got)
	}
}

func TestPlainYAMLStrings(t *testing.T) {
	for _, s := range yamlTrickyStrings {
		assertRoundTrips(t, s)
		assertRoundTrips(t, map[string]interface{}{"k": s})
		if utf8.ValidString(s) {
			assertRoundTrips(t, map[string]interface{}{s: "v"})
		} else if plainYAMLData(map[string]interface{}{s: "v"}) {
			t.Errorf("invalid UTF-8 key %q should fall back to yaml.v3", s)
		}
		assertRoundTrips(t, []interface{}{s, s})
	}
}

func TestPlainYAMLScalars(t *testing.T) {
	values := []interface{}{
		nil, true, false, 0, -7, 42, int64(math.MaxInt64), int64(math.MinInt64), uint64(math.MaxUint64),
		0.0, math.Copysign(0, -1), 1.5, -2.25, 1e300, 5e-324, 123456789.123, float64(9007199254740993), math.Inf(1), math.Inf(-1), float32(0.1),
	}
	for _, v := range values {
		assertRoundTrips(t, v)
		assertRoundTrips(t, map[string]interface{}{"v": v})
		assertRoundTrips(t, []interface{}{v})
	}
	got, _ := viaPlainEmitter(t, math.NaN())
	if f, ok := got.(float64); !ok || !math.IsNaN(f) {
		t.Errorf("NaN decoded as %#v", got)
	}
}

func TestPlainYAMLStructures(t *testing.T) {
	structures := []interface{}{
		map[string]interface{}{},
		[]interface{}{},
		[]map[string]interface{}{},
		[]map[string]interface{}{{"a": 1}, {}},
		map[string]interface{}{"empty map": map[string]interface{}{}, "empty list": []interface{}{}},
		[]interface{}{[]interface{}{1, 2}, []interface{}{}, []interface{}{[]interface{}{"deep"}}},
		[]interface{}{map[string]interface{}{"a": []interface{}{map[string]interface{}{"b": []interface{}{1, map[string]interface{}{}}}}}},
		map[string]interface{}{"list": []interface{}{map[string]interface{}{"x": 1, "y": []interface{}{"p", "q"}}, "scalar", nil}},
		yamlRowsForTest(50),
	}
	for _, v := range structures {
		assertRoundTrips(t, v)
	}
}

// yaml.v3 encodes a value of "\n" as a block scalar that it reads back as an
// empty string; the plain emitter keeps it.
func TestPlainYAMLKeepsWhatYAMLV3Loses(t *testing.T) {
	value := map[string]interface{}{"0": "\n"}
	if lossy := reencodeWithYAMLV3(t, value); reflect.DeepEqual(lossy, value) {
		t.Log("yaml.v3 no longer loses this value")
	}
	assertRoundTrips(t, value)
	assertRoundTrips(t, []interface{}{"\n", "\n\n", " \n", "\n "})
}

func TestPlainYAMLLongKeys(t *testing.T) {
	for _, n := range []int{maxImplicitKeyBytes, maxImplicitKeyBytes + 1, 2000} {
		long := strings.Repeat("k", n)
		assertRoundTrips(t, map[string]interface{}{long: "v", "short": 1})
		assertRoundTrips(t, map[string]interface{}{long: map[string]interface{}{"nested": []interface{}{1, 2}}})
		assertRoundTrips(t, []interface{}{map[string]interface{}{long: []interface{}{"x"}, "a": 1}})
		assertRoundTrips(t, map[string]interface{}{strings.Repeat("\x01", n): "control"})
	}
}

func TestPlainYAMLLayout(t *testing.T) {
	data := []interface{}{
		map[string]interface{}{"id": 1, "name": "Ada", "tags": []interface{}{"math", "engines"}, "meta": map[string]interface{}{"x": 1}},
		[]interface{}{"a", "b"},
		"scalar",
	}
	want := `- id: 1
  meta:
    x: 1
  name: Ada
  tags:
    - math
    - engines
- - a
  - b
- scalar
`
	if _, got := viaPlainEmitter(t, data); got != want {
		t.Errorf("layout:\n%s\nwant:\n%s", got, want)
	}
}

func TestYAMLCodecFallsBackForOtherTypes(t *testing.T) {
	when := time.Date(2026, 10, 21, 10, 0, 0, 0, time.UTC)
	out := encodeString(t, "YAML", map[string]interface{}{"at": when, "n": 1})
	if !strings.Contains(out, "2026-10-21T10:00:00Z") {
		t.Errorf("time value not encoded by yaml.v3: %s", out)
	}
	if ok, _ := emitPlainYAML(&bytes.Buffer{}, []interface{}{struct{}{}}); ok {
		t.Error("unsupported type accepted by the plain emitter")
	}
}

func yamlRowsForTest(n int) []interface{} {
	rows := make([]interface{}, n)
	for i := range rows {
		rows[i] = map[string]interface{}{
			"id":   float64(i),
			"name": fmt.Sprintf("User %d", i),
			"note": yamlTrickyStrings[i%len(yamlTrickyStrings)],
			"tags": []interface{}{"a", map[string]interface{}{"nested": i}},
		}
	}
	return rows
}

func FuzzPlainYAMLAgainstYAMLV3(f *testing.F) {
	for _, s := range yamlTrickyStrings {
		f.Add([]byte(fmt.Sprintf(`{"k":%q,"l":[%q,{"m":%q}]}`, s, s, s)))
	}
	f.Add([]byte(`[1,2.5,-3e10,true,null,{"a":{}},[]]`))
	jsonCodec := &jsonCodec{}
	f.Fuzz(func(t *testing.T, data []byte) {
		value, err := jsonCodec.Decode(data)
		if err != nil {
			return
		}
		var buf bytes.Buffer
		if ok, err := emitPlainYAML(&buf, value); !ok || err != nil {
			t.Fatalf("plain emitter refused JSON data: %v %v", ok, err)
		}
		var got interface{}
		if err := yaml.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("emitted YAML does not parse: %v\n%s", err, buf.String())
		}
		if !reflect.DeepEqual(canonicalYAML(got), canonicalYAML(value)) {
			t.Fatalf("mismatch for %s\nemitted:\n%s\ngot  %#v\nwant %#v", data, buf.String(), got, value)
		}
	})
}

func BenchmarkYAMLEncodeRows(b *testing.B) {
	rows := yamlRowsForTest(20_000)
	b.Run("plain-emitter", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := (&yamlCodec{}).Encode(&bytes.Buffer{}, rows); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("yaml.v3", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := encodeYAMLV3(&bytes.Buffer{}, rows); err != nil {
				b.Fatal(err)
			}
		}
	})
}
