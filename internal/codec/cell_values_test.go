package codec

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
	"github.com/google/go-cmp/cmp"
)

func tabularRows(t *testing.T, format models.Format, input string) []map[string]interface{} {
	t.Helper()
	jsonDecoder, _ := GetCodec(models.FormatJSON)
	data, err := jsonDecoder.Decode([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	codec, _ := GetCodec(format)
	if err := codec.Encode(&buf, data); err != nil {
		t.Fatal(err)
	}
	rows, err := codec.Decode(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return rows.([]map[string]interface{})
}

func TestTabularFormatsKeepLiteralDottedKeys(t *testing.T) {
	const input = `[{"a.b":1,"plain":"x","nested":{"c":2,"d.e":3}},{"a.b":4,"nested":{"c":5}}]`
	want := []map[string]interface{}{
		{"a.b": "1", "plain": "x", "nested.c": "2", "nested.d.e": "3"},
		{"a.b": "4", "nested.c": "5"},
	}
	for _, format := range []models.Format{models.FormatCSV, models.FormatTSV, models.FormatXLSX} {
		t.Run(string(format), func(t *testing.T) {
			if diff := cmp.Diff(want, tabularRows(t, format, input)); diff != "" {
				t.Errorf("rows (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTabularFormatsStillFlattenNestedObjects(t *testing.T) {
	const input = `[{"user":{"name":"Ada","address":{"city":"London"}}}]`
	want := []map[string]interface{}{{"user.name": "Ada", "user.address.city": "London"}}
	for _, format := range []models.Format{models.FormatCSV, models.FormatXLSX} {
		if diff := cmp.Diff(want, tabularRows(t, format, input)); diff != "" {
			t.Errorf("%s rows (-want +got):\n%s", format, diff)
		}
	}
}

func TestTabularFormatsWriteArraysAsJSON(t *testing.T) {
	const input = `[{"tags":["a","b"],"nums":[1,2.5],"objs":[{"x":1},{"y":"z"}],"empty":[],"name":"n"}]`
	want := []map[string]interface{}{{
		"tags":  `["a","b"]`,
		"nums":  `[1,2.5]`,
		"objs":  `[{"x":1},{"y":"z"}]`,
		"empty": `[]`,
		"name":  "n",
	}}
	for _, format := range []models.Format{models.FormatCSV, models.FormatTSV, models.FormatXLSX} {
		t.Run(string(format), func(t *testing.T) {
			if diff := cmp.Diff(want, tabularRows(t, format, input)); diff != "" {
				t.Errorf("rows (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCSVArraysAreQuotedJSON(t *testing.T) {
	out := convert(t, models.FormatJSON, models.FormatCSV, `[{"tags":["a","b"],"html":["<&>"]}]`)
	records, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if got := records[1]; got[0] != `["<&>"]` || got[1] != `["a","b"]` {
		t.Errorf("row = %q", got)
	}
}
