package utils

import (
	"slices"
	"testing"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
)

func TestMimeToFormat(t *testing.T) {
	tests := map[string]models.Format{
		"application/json":                models.FormatJSON,
		"application/json; charset=utf-8": models.FormatJSON,
		"application/xml":                 models.FormatXML,
		"text/xml":                        models.FormatXML,
		"text/csv":                        models.FormatCSV,
		"application/toml":                models.FormatTOML,
		"text/tab-separated-values":       models.FormatTSV,
		"application/x-yaml":              models.FormatYAML,
		"text/yaml":                       models.FormatYAML,
		"application/msgpack":             models.FormatMsgPack,
		"application/x-msgpack":           models.FormatMsgPack,
		"text/x-java-properties":          models.FormatProperties,
		"text/properties":                 models.FormatProperties,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": models.FormatXLSX,
		"":           "",
		"*/*":        "",
		"text/plain": "",
	}
	for mime, want := range tests {
		if got := MimeToFormat(mime); got != want {
			t.Errorf("MimeToFormat(%q) = %q, want %q", mime, got, want)
		}
	}
}

func TestGetContentTypeRoundTrips(t *testing.T) {
	for _, f := range []models.Format{models.FormatJSON, models.FormatXML, models.FormatCSV, models.FormatTSV, models.FormatTOML, models.FormatYAML, models.FormatMsgPack, models.FormatXLSX, models.FormatProperties} {
		if got := MimeToFormat(GetContentType(f)); got != f {
			t.Errorf("%s -> %q -> %q", f, GetContentType(f), got)
		}
	}
	if got := GetContentType("NOPE"); got != "text/plain" {
		t.Errorf("unknown format content type = %q", got)
	}
}

func TestCollectHeadersFlattensAndSorts(t *testing.T) {
	rows := []map[string]interface{}{
		{"b": 1, "a": map[string]interface{}{"z": 1, "y": map[string]interface{}{"x": 2}}},
		{"c": nil, "b": 2},
		{},
	}
	want := []string{"a.y.x", "a.z", "b", "c"}
	if got := CollectHeaders(rows); !slices.Equal(got, want) {
		t.Errorf("CollectHeaders = %v, want %v", got, want)
	}
	if got := CollectHeaders(nil); len(got) != 0 {
		t.Errorf("CollectHeaders(nil) = %v", got)
	}
}

func TestGetValue(t *testing.T) {
	row := map[string]interface{}{
		"a":    map[string]interface{}{"b": map[string]interface{}{"c": 3}},
		"n":    nil,
		"list": []interface{}{1, 2},
		"f":    1.5,
		"t":    true,
	}
	tests := []struct {
		key  string
		want string
		ok   bool
	}{
		{"a.b.c", "3", true},
		{"n", "", true},
		{"list", "[1 2]", true},
		{"f", "1.5", true},
		{"t", "true", true},
		{"a.b", "map[c:3]", true},
		{"a.x", "", false},
		{"a.b.c.d", "", false},
		{"missing", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		got, ok := GetValue(row, tt.key)
		if got != tt.want || ok != tt.ok {
			t.Errorf("GetValue(%q) = %q, %v; want %q, %v", tt.key, got, ok, tt.want, tt.ok)
		}
	}
}

func BenchmarkCollectHeaders(b *testing.B) {
	rows := make([]map[string]interface{}, 1000)
	for i := range rows {
		rows[i] = map[string]interface{}{"id": i, "name": "x", "meta": map[string]interface{}{"a": 1, "b": 2}}
	}
	b.ReportAllocs()
	for b.Loop() {
		CollectHeaders(rows)
	}
}
