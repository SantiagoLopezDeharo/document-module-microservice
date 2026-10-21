package codec

import (
	"bytes"
	"strings"
	"testing"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
	"github.com/google/go-cmp/cmp"
)

func decodeString(t *testing.T, format models.Format, input string) (interface{}, error) {
	t.Helper()
	c, err := GetCodec(format)
	if err != nil {
		t.Fatal(err)
	}
	return c.Decode([]byte(input))
}

func encodeString(t *testing.T, format models.Format, data interface{}) string {
	t.Helper()
	c, _ := GetCodec(format)
	var buf bytes.Buffer
	if err := c.Encode(&buf, data); err != nil {
		t.Fatalf("encode %s: %v", format, err)
	}
	return buf.String()
}

func TestJSONPreservesLargeIntegers(t *testing.T) {
	input := `{"small":42,"neg":-7,"float":1.5,"exp":1e3,"limit":9007199254740992,"over":9007199254740993,"under":-9007199254740993,"max":9223372036854775807,"umax":18446744073709551615,"huge":123456789012345678901234567890}`
	got, err := decodeString(t, models.FormatJSON, input)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"small": int64(42),
		"neg":   int64(-7),
		"float": 1.5,
		"exp":   float64(1000),
		"limit": int64(9007199254740992),
		"over":  int64(9007199254740993),
		"under": int64(-9007199254740993),
		"max":   int64(9223372036854775807),
		"umax":  uint64(18446744073709551615),
		"huge":  1.2345678901234568e29,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("decoded numbers (-want +got):\n%s", diff)
	}

	for _, format := range []models.Format{models.FormatJSON, models.FormatYAML, models.FormatTOML, models.FormatXML, models.FormatProperties, models.FormatCSV} {
		out := encodeString(t, format, map[string]interface{}{"id": got.(map[string]interface{})["over"]})
		if !strings.Contains(out, "9007199254740993") {
			t.Errorf("%s output lost precision: %q", format, out)
		}
	}
}

func TestJSONRejectsNumbersOutsideFloat64(t *testing.T) {
	for _, input := range []string{`1e400`, `[-1e400]`, `{"a":{"b":123e999}}`, `1e309`} {
		if _, err := decodeString(t, models.FormatJSON, input); err == nil {
			t.Errorf("%s accepted", input)
		}
	}
}

func TestJSONNormalizesNestedNumbers(t *testing.T) {
	got, err := decodeString(t, models.FormatJSON, `[{"a":[1,{"b":9007199254740995}]}]`)
	if err != nil {
		t.Fatal(err)
	}
	want := []interface{}{map[string]interface{}{"a": []interface{}{int64(1), map[string]interface{}{"b": int64(9007199254740995)}}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestJSONRejectsTrailingData(t *testing.T) {
	for _, input := range []string{`{"a":1}{"b":2}`, `[1] [2]`, `{"a":1} x`, `1 2`, `{"a":1}]`} {
		if _, err := decodeString(t, models.FormatJSON, input); err == nil {
			t.Errorf("%q accepted", input)
		}
	}
	for _, input := range []string{"{\"a\":1}\n", " [1] \t\r\n", "null"} {
		if _, err := decodeString(t, models.FormatJSON, input); err != nil {
			t.Errorf("%q rejected: %v", input, err)
		}
	}
}

func TestPropertiesNeverExpandsOrExits(t *testing.T) {
	inputs := map[string]map[string]interface{}{
		"a=${b}\nb=${a}":      {"a": "${b}", "b": "${a}"},
		"self=${self}":        {"self": "${self}"},
		"home=${HOME}":        {"home": "${HOME}"},
		"path=${PATH}/bin":    {"path": "${PATH}/bin"},
		"open=${unterminated": {"open": "${unterminated"},
	}
	for input, want := range inputs {
		got, err := decodeString(t, models.FormatProperties, input)
		if err != nil {
			t.Errorf("%q: %v", input, err)
			continue
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("%q (-want +got):\n%s", input, diff)
		}
	}
}

func TestPropertiesRejectsMalformedEscapes(t *testing.T) {
	for _, input := range []string{`a=\u12`, `a=\uZZZZ`} {
		if _, err := decodeString(t, models.FormatProperties, input); err == nil {
			t.Errorf("%q accepted", input)
		}
	}
}

func TestDecodersRejectMalformedInput(t *testing.T) {
	inputs := map[models.Format][]string{
		models.FormatJSON:       {"", "{", `{"a":}`, "nul"},
		models.FormatXML:        {"", "<a>", "<a></b>", "not xml"},
		models.FormatTOML:       {"a =", "[a", "a = [1", `a = "x`},
		models.FormatYAML:       {"a: [1", "\t- bad", "a: 'x"},
		models.FormatMsgPack:    {"", "\xc1", "\x92\x01", "\xdb\xff\xff\xff\xffa"},
		models.FormatCSV:        {"", "a,b\n\"unterminated"},
		models.FormatTSV:        {"", "a\tb\n1\t2\t3"},
		models.FormatXLSX:       {"", "PK\x03\x04garbage", "not a workbook"},
		models.FormatProperties: {`a=\u12`},
	}
	for format, cases := range inputs {
		for _, input := range cases {
			if _, err := decodeString(t, format, input); err == nil {
				t.Errorf("%s accepted %q", format, input)
			}
		}
	}
}

func fuzzDecoder(f *testing.F, format models.Format, seeds ...string) {
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	c, _ := GetCodec(format)
	json, _ := GetCodec(models.FormatJSON)
	f.Fuzz(func(t *testing.T, data []byte) {
		decoded, err := c.Decode(data)
		if err != nil {
			return
		}
		var buf bytes.Buffer
		_ = json.Encode(&buf, decoded)
		_ = c.Encode(&bytes.Buffer{}, decoded)
	})
}

func FuzzDecodeJSON(f *testing.F) {
	fuzzDecoder(f, models.FormatJSON, `{"a":[1,2,{"b":null}]}`, `[1e400]`, `"\ud800"`)
}

func FuzzDecodeXML(f *testing.F) {
	fuzzDecoder(f, models.FormatXML, `<doc><a x="1">t</a><List><b>1</b></List></doc>`, `<?xml version="1.0"?><!DOCTYPE x [<!ENTITY a "b">]><x>&a;</x>`)
}

func FuzzDecodeTOML(f *testing.F) {
	fuzzDecoder(f, models.FormatTOML, "a = 1\n[t]\nb = [1, {c = 2}]", "[[root]]\nx = 1")
}

func FuzzDecodeYAML(f *testing.F) {
	fuzzDecoder(f, models.FormatYAML, "a: [1, {b: 2}]\n", "&a [*a]", "- !!binary aGVsbG8=")
}

func FuzzDecodeMsgPack(f *testing.F) {
	fuzzDecoder(f, models.FormatMsgPack, "\x82\xa1a\x01\xa1b\x92\x01\xc0", "\xdd\xff\xff\xff\xff")
}

func FuzzDecodeCSV(f *testing.F) {
	fuzzDecoder(f, models.FormatCSV, "a,b\n1,2\n", "\"a\"\"b\",c\n\"x\ny\",z")
}

func FuzzDecodeProperties(f *testing.F) {
	fuzzDecoder(f, models.FormatProperties, "a.b[0]=1\na.b[1]=2\n", "k\\=x=v\\u0041", "a=${b}\nb=${a}")
}
