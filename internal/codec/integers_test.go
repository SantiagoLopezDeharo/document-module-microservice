package codec

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
	"github.com/vmihailenco/msgpack/v5"
)

func convert(t *testing.T, in, out models.Format, input string) string {
	t.Helper()
	decoder, err := GetCodec(in)
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := GetCodec(out)
	if err != nil {
		t.Fatal(err)
	}
	data, err := decoder.Decode([]byte(input))
	if err != nil {
		t.Fatalf("decode %s: %v", in, err)
	}
	var buf bytes.Buffer
	if err := encoder.Encode(&buf, data); err != nil {
		t.Fatalf("encode %s: %v", out, err)
	}
	return buf.String()
}

const integerJSON = `{"count":1,"neg":-7,"f":1.5,"whole":2.0,"big":9007199254740993,"list":[1,2]}`

func TestJSONIntegersStayIntegersInOutputFormats(t *testing.T) {
	tests := []struct {
		format models.Format
		want   []string
		reject []string
	}{
		{models.FormatTOML, []string{"count = 1\n", "neg = -7\n", "f = 1.5\n", "whole = 2.0\n", "big = 9007199254740993\n", "list = [1, 2]\n"}, []string{"count = 1.0"}},
		{models.FormatJSON, []string{`"count": 1,`, `"neg": -7,`, `"f": 1.5,`, `"big": 9007199254740993,`}, []string{`1.0`}},
		{models.FormatYAML, []string{"count: 1\n", "neg: -7\n", "f: 1.5\n"}, []string{"count: 1.0"}},
		{models.FormatXML, []string{"<count>1</count>", "<neg>-7</neg>", "<f>1.5</f>"}, []string{"1.0"}},
		{models.FormatProperties, []string{"count=1\n", "neg=-7\n", "f=1.5\n"}, []string{"count=1.0"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			out := convert(t, models.FormatJSON, tt.format, integerJSON)
			for _, want := range tt.want {
				if !strings.Contains(out, want) {
					t.Errorf("output lacks %q:\n%s", want, out)
				}
			}
			for _, reject := range tt.reject {
				if strings.Contains(out, reject) {
					t.Errorf("output contains %q:\n%s", reject, out)
				}
			}
		})
	}
}

func TestJSONIntegersStayIntegersInMsgPack(t *testing.T) {
	out := convert(t, models.FormatJSON, models.FormatMsgPack, integerJSON)
	var decoded map[string]interface{}
	if err := msgpack.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"count", "neg", "big"} {
		switch reflect.TypeOf(decoded[key]).Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		default:
			t.Errorf("%s encoded as %T, want an integer", key, decoded[key])
		}
	}
	if _, ok := decoded["f"].(float64); !ok {
		t.Errorf("f encoded as %T, want float64", decoded["f"])
	}
	for _, item := range decoded["list"].([]interface{}) {
		if _, isFloat := item.(float64); isFloat {
			t.Errorf("list item encoded as float: %v", item)
		}
	}
}

func TestPropertiesIntegersStayIntegersInTOML(t *testing.T) {
	out := convert(t, models.FormatProperties, models.FormatTOML, "port=8080\nratio=0.5\n")
	if !strings.Contains(out, "port = 8080\n") || !strings.Contains(out, "ratio = 0.5\n") {
		t.Errorf("unexpected TOML:\n%s", out)
	}
}
