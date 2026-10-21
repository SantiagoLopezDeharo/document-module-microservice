package codec

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"testing"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
)

func wellFormedXML(t *testing.T, doc string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(doc))
	for {
		if _, err := dec.Token(); err == io.EOF {
			return
		} else if err != nil {
			t.Fatalf("encoder produced malformed XML: %v\n%s", err, doc)
		}
	}
}

func TestXMLEncodeEscapesValues(t *testing.T) {
	values := []string{
		"Tom & Jerry",
		"a < b > c",
		`</name><admin>true</admin><name>`,
		`"quotes" and 'apostrophes'`,
		"<![CDATA[x]]>",
		"&amp; already escaped",
	}
	for _, v := range values {
		data := map[string]interface{}{"name": v, "other": 1}
		out := encodeString(t, models.FormatXML, data)
		wellFormedXML(t, out)
		if strings.Contains(out, "<admin>") {
			t.Fatalf("value injected markup:\n%s", out)
		}
		back, err := decodeString(t, models.FormatXML, out)
		if err != nil {
			t.Fatal(err)
		}
		if got := back.(map[string]interface{})["name"]; got != v {
			t.Errorf("round trip = %q, want %q", got, v)
		}
	}
}

func TestXMLEncodeDoesNotModifyItsInput(t *testing.T) {
	data := map[string]interface{}{
		"a":    "x & y",
		"list": []interface{}{"<b>", map[string]interface{}{"c": "'q'"}},
		"rows": []map[string]interface{}{{"d": "1 > 0"}},
		"keep": "clean",
	}
	for range 3 {
		out := encodeString(t, models.FormatXML, data)
		wellFormedXML(t, out)
		if strings.Contains(out, "&amp;amp;") {
			t.Fatalf("value escaped twice:\n%s", out)
		}
	}
	if data["a"] != "x & y" || data["list"].([]interface{})[0] != "<b>" ||
		data["list"].([]interface{})[1].(map[string]interface{})["c"] != "'q'" || data["rows"].([]map[string]interface{})[0]["d"] != "1 > 0" {
		t.Fatalf("Encode modified its input: %v", data)
	}
}

func TestXMLEncodeEscapesAttributesAndText(t *testing.T) {
	data := map[string]interface{}{"item": map[string]interface{}{"-id": `a"b&c`, "#text": "x<y"}}
	out := encodeString(t, models.FormatXML, data)
	wellFormedXML(t, out)
	back, err := decodeString(t, models.FormatXML, out)
	if err != nil {
		t.Fatal(err)
	}
	item := back.(map[string]interface{})
	if item["-id"] != `a"b&c` || item["#text"] != "x<y" {
		t.Errorf("round trip = %v", item)
	}
}

func BenchmarkXMLEncodeRows(b *testing.B) {
	rows := make([]interface{}, 30000)
	for i := range rows {
		rows[i] = map[string]interface{}{"id": float64(i), "name": "User", "email": "u@example.com", "note": "a & b"}
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = (&xmlCodec{}).Encode(&bytes.Buffer{}, rows)
	}
}

func TestXMLEncodeRejectsInvalidElementNames(t *testing.T) {
	c, _ := GetCodec(models.FormatXML)
	for _, key := range []string{"a><b", "first name", "1st", "", "a/b", "x=y", "<x>", "-", "a&b"} {
		err := c.Encode(&bytes.Buffer{}, map[string]interface{}{key: 1, "ok": 2})
		if err == nil {
			t.Errorf("key %q accepted", key)
		}
		nested := map[string]interface{}{"outer": []interface{}{map[string]interface{}{key: 1}}}
		if err := c.Encode(&bytes.Buffer{}, nested); err == nil {
			t.Errorf("nested key %q accepted", key)
		}
	}
	for _, key := range []string{"name", "_private", "x-y.z", "ns:tag", "日本", "-attr", "#text"} {
		out := encodeString(t, models.FormatXML, map[string]interface{}{"root": map[string]interface{}{key: "v", "k": 1}})
		wellFormedXML(t, out)
	}
}
