package docx

import (
	"bytes"
	"encoding/xml"
	"testing"
)

func marshalRun(t *testing.T, run *Run) string {
	t.Helper()
	out, err := xml.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func TestRunPropertiesFollowSchemaOrder(t *testing.T) {
	p := NewFile().AddParagraph()
	run := p.AddText("x").Size(10).Color("FF0000").Italic().Bold()
	want := `<w:r><w:rPr><w:b></w:b><w:i></w:i><w:color w:val="FF0000"></w:color><w:sz w:val="20"></w:sz></w:rPr><w:t>x</w:t></w:r>`
	if got := marshalRun(t, run); got != want {
		t.Errorf("run =\n%s\nwant\n%s", got, want)
	}
}

func TestAddTextPreservesSurroundingWhitespace(t *testing.T) {
	p := NewFile().AddParagraph()
	tests := map[string]string{
		"plain":     `<w:t>plain</w:t>`,
		" leading":  `<w:t xml:space="preserve"> leading</w:t>`,
		"trailing ": `<w:t xml:space="preserve">trailing </w:t>`,
		"\ttab":     `<w:t xml:space="preserve">&#x9;tab</w:t>`,
		"in side":   `<w:t>in side</w:t>`,
		"":          `<w:t></w:t>`,
	}
	for text, want := range tests {
		if got := marshalRun(t, p.AddText(text)); !bytes.Contains([]byte(got), []byte(want)) {
			t.Errorf("%q => %s, want %s", text, got, want)
		}
	}
}

func TestFileWriteProducesArchive(t *testing.T) {
	f := NewFile()
	f.AddParagraph().AddText("hello")
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("PK\x03\x04")) {
		t.Fatal("output is not a zip archive")
	}
}
