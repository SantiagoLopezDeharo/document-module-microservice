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
