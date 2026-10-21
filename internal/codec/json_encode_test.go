package codec

import (
	"bytes"
	"encoding/json"
	"math"
	"testing"
)

func encodeJSONV1(t testing.TB, v interface{}) string {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestJSONEncodeMatchesEncodingJSON(t *testing.T) {
	values := []interface{}{
		nil, true, 0.0, -0.0, 1.5, 1e21, 1e-7, math.MaxFloat64, int64(math.MaxInt64), uint64(math.MaxUint64),
		"", "<b>&amp;</b>", "line\nbreak\ttab ", "\x00\x1f", "\xff invalid", "日本 🚀",
		map[string]interface{}{}, []interface{}{}, []map[string]interface{}{}, []map[string]interface{}(nil), []interface{}(nil), map[string]interface{}(nil),
		map[string]interface{}{"b": 1, "a": []interface{}{map[string]interface{}{"z": nil, "y": []interface{}{}}}, "c": map[string]interface{}{}},
		yamlRowsForTest(30),
	}
	for _, v := range values {
		if got, want := encodeString(t, "JSON", v), encodeJSONV1(t, v); got != want {
			t.Errorf("value %#v\n got %q\nwant %q", v, got, want)
		}
	}
}

func FuzzJSONEncodeMatchesEncodingJSON(f *testing.F) {
	f.Add([]byte(`{"a":[1,2.5,{"b":null,"c":"x<y"}],"d":{}}`))
	f.Add([]byte(`[9007199254740993,-0.0,1e400,"\ud800"]`))
	c := &jsonCodec{}
	f.Fuzz(func(t *testing.T, data []byte) {
		v, err := c.Decode(data)
		if err != nil {
			return
		}
		if got, want := encodeString(t, "JSON", v), encodeJSONV1(t, v); got != want {
			t.Fatalf("input %q\n got %q\nwant %q", data, got, want)
		}
	})
}

func BenchmarkJSONEncodeRows(b *testing.B) {
	rows := yamlRowsForTest(20_000)
	b.Run("v2-indented", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_ = (&jsonCodec{}).Encode(&bytes.Buffer{}, rows)
		}
	})
	b.Run("v1-encoder", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			encodeJSONV1(b, rows)
		}
	})
}
