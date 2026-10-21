package document

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"testing"
)

const letterDOCX = `{"actions":[
	{"type":"AddParagraph"},
	{"type":"AddText","txt":"My Title","size":24,"bold":true,"color":"0000FF"},
	{"type":"AddParagraph"},
	{"type":"AddText","txt":"Regular text, "},
	{"type":"AddText","txt":"emphasis","italic":true,"color":"#ff0000"},
	{"type":"AddText","txt":" & <markup> \"quoted\"","size":11,"bold":true,"italic":true,"color":"auto"}
]}`

func TestRenderDOCXMatchesGolden(t *testing.T) {
	out, err := RenderDOCX(mustJSON[DocxRequest](t, letterDOCX), testLimits)
	if err != nil {
		t.Fatalf("RenderDOCX: %v", err)
	}
	doc := zipEntry(t, out, "word/document.xml")
	golden(t, "letter_document.xml", doc)
	assertWellFormed(t, doc)
}

func TestRenderDOCXPackageParts(t *testing.T) {
	out, err := RenderDOCX(DocxRequest{}, testLimits)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/_rels/document.xml.rels", "word/styles.xml"} {
		assertWellFormed(t, zipEntry(t, out, part))
	}
}

func TestRenderDOCXRunProperties(t *testing.T) {
	tests := []struct {
		action string
		want   string
	}{
		{`{"type":"AddText","txt":"b","bold":true}`, `<w:rPr><w:b></w:b></w:rPr>`},
		{`{"type":"AddText","txt":"i","italic":true}`, `<w:rPr><w:i></w:i></w:rPr>`},
		{`{"type":"AddText","txt":"c","color":"#A1b2C3"}`, `<w:rPr><w:color w:val="A1b2C3"></w:color></w:rPr>`},
		{`{"type":"AddText","txt":"s","size":12}`, `<w:rPr><w:sz w:val="24"></w:sz></w:rPr>`},
		{`{"type":"AddText","txt":"all","size":1638,"bold":true,"italic":true,"color":"auto"}`, `<w:rPr><w:b></w:b><w:i></w:i><w:color w:val="auto"></w:color><w:sz w:val="3276"></w:sz></w:rPr>`},
		{`{"type":"AddText","txt":"plain"}`, `<w:rPr></w:rPr>`},
	}
	for _, tt := range tests {
		out, err := RenderDOCX(mustJSON[DocxRequest](t, `{"actions":[`+tt.action+`]}`), testLimits)
		if err != nil {
			t.Errorf("%s: %v", tt.action, err)
			continue
		}
		if doc := zipEntry(t, out, "word/document.xml"); !bytes.Contains(doc, []byte(tt.want)) {
			t.Errorf("%s: document lacks %s\n%s", tt.action, tt.want, doc)
		}
	}
}

func TestRenderDOCXEscapesText(t *testing.T) {
	text := `</w:t></w:r><w:r><w:t>injected` + "\x00\x01￾" + `]]>&amp;`
	out, err := RenderDOCX(DocxRequest{Actions: []DocxAction{{Type: "AddText", Txt: text}}}, testLimits)
	if err != nil {
		t.Fatal(err)
	}
	doc := zipEntry(t, out, "word/document.xml")
	assertWellFormed(t, doc)
	if bytes.Count(doc, []byte("<w:r>")) != 1 {
		t.Errorf("text broke out of its run:\n%s", doc)
	}
}

func TestRenderDOCXRejectsInvalidRuns(t *testing.T) {
	tests := []struct {
		raw   string
		index int
		typ   string
	}{
		{`{"actions":[{"type":"AddText","txt":"x","color":"red"}]}`, 0, "AddText"},
		{`{"actions":[{"type":"AddText","txt":"x","color":"12345"}]}`, 0, "AddText"},
		{`{"actions":[{"type":"AddText","txt":"x","color":"1234567"}]}`, 0, "AddText"},
		{`{"actions":[{"type":"AddText","txt":"x","color":"GG0000"}]}`, 0, "AddText"},
		{`{"actions":[{"type":"AddText","txt":"x","color":"\"/><w:b/>"}]}`, 0, "AddText"},
		{`{"actions":[{"type":"AddParagraph"},{"type":"AddText","txt":"x","size":-1}]}`, 1, "AddText"},
		{`{"actions":[{"type":"AddText","txt":"x","size":1639}]}`, 0, "AddText"},
		{`{"actions":[{"type":"AddParagraph"},{"type":"AddTable"}]}`, 1, "AddTable"},
		{`{"actions":[{"type":"addtext"}]}`, 0, "addtext"},
	}
	for _, tt := range tests {
		_, err := RenderDOCX(mustJSON[DocxRequest](t, tt.raw), testLimits)
		assertActionError(t, err, tt.index, tt.typ)
	}
}

func TestRenderDOCXTextWithoutParagraphStartsOne(t *testing.T) {
	out, err := RenderDOCX(mustJSON[DocxRequest](t, `{"actions":[{"type":"AddText","txt":"a"},{"type":"AddText","txt":"b"},{"type":"AddParagraph"},{"type":"AddText","txt":"c"}]}`), testLimits)
	if err != nil {
		t.Fatal(err)
	}
	doc := zipEntry(t, out, "word/document.xml")
	if n := bytes.Count(doc, []byte("<w:p>")); n != 2 {
		t.Errorf("paragraphs = %d, want 2", n)
	}
}

func TestRenderDOCXActionLimit(t *testing.T) {
	req := DocxRequest{Actions: make([]DocxAction, testLimits.MaxActions+1)}
	if _, err := RenderDOCX(req, testLimits); !errors.Is(err, ErrLimit) {
		t.Fatalf("err = %v, want ErrLimit", err)
	}
}

func assertWellFormed(t *testing.T, doc []byte) {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(doc))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("malformed XML: %v\n%.300s", err, doc)
		}
	}
}

func FuzzRenderDOCX(f *testing.F) {
	f.Add([]byte(letterDOCX))
	f.Fuzz(func(t *testing.T, data []byte) {
		req, ok := fuzzRequest[DocxRequest](data)
		if !ok {
			return
		}
		out, err := RenderDOCX(req, Limits{MaxActions: 50, MaxXLSXCells: 1000})
		if err != nil {
			return
		}
		assertWellFormed(t, zipEntry(t, out, "word/document.xml"))
	})
}

func BenchmarkRenderDOCX(b *testing.B) {
	req := mustJSON[DocxRequest](b, letterDOCX)
	for range 50 {
		req.Actions = append(req.Actions, DocxAction{Type: "AddText", Txt: strings.Repeat("lorem ipsum ", 20), Bold: true})
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := RenderDOCX(req, testLimits); err != nil {
			b.Fatal(err)
		}
	}
}

func TestRenderDOCXPreservesSurroundingWhitespace(t *testing.T) {
	out, err := RenderDOCX(mustJSON[DocxRequest](t, `{"actions":[{"type":"AddText","txt":"Total: "},{"type":"AddText","txt":"42","bold":true},{"type":"AddText","txt":"word"}]}`), testLimits)
	if err != nil {
		t.Fatal(err)
	}
	doc := zipEntry(t, out, "word/document.xml")
	if !bytes.Contains(doc, []byte(`<w:t xml:space="preserve">Total: </w:t>`)) {
		t.Errorf("trailing space will be dropped by Word:\n%s", doc)
	}
	if !bytes.Contains(doc, []byte(`<w:t>word</w:t>`)) {
		t.Errorf("text without surrounding whitespace should stay unannotated:\n%s", doc)
	}
}
