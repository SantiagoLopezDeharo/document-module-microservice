package document

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

const reportPDF = `{
  "orientation": "P", "unit": "mm", "size": "A4",
  "actions": [
    {"type": "AddPage"},
    {"type": "SetTextColor", "r": 200, "g": 200, "b": 200},
    {"type": "SetFont", "family": "Arial", "style": "B", "size": 50},
    {"type": "SetXY", "x": 80, "y": 10},
    {"type": "Cell", "w": 0, "h": 20, "txt": "DRAFT", "ln": 1, "align": "R"},
    {"type": "SetTextColor", "r": 0, "g": 0, "b": 0},
    {"type": "SetFont", "family": "Helvetica", "style": "", "size": 12},
    {"type": "MultiCell", "w": 0, "h": 6, "txt": "Quarterly report body text", "align": "L"},
    {"type": "Ln", "h": 10},
    {"type": "SetFillColor", "r": 240, "g": 240, "b": 240},
    {"type": "SetDrawColor", "r": 10, "g": 20, "b": 30},
    {"type": "Cell", "w": 40, "h": 10, "txt": "ID", "border": "1", "ln": 1, "align": "C"},
    {"type": "SetFont", "family": "Courier", "style": "I", "size": 10},
    {"type": "Cell", "w": 40, "h": 10, "txt": "Item (Alpha)", "border": "LR"},
    {"type": "SetFont", "family": "Symbol", "style": "", "size": 16},
    {"type": "Cell", "w": 0, "h": 10, "txt": "S x", "ln": 1},
    {"type": "AddPage"},
    {"type": "SetFont", "family": "Times", "style": "BI", "size": 14},
    {"type": "Cell", "w": 0, "h": 10, "txt": "Second page"}
  ]
}`

func TestRenderPDFReport(t *testing.T) {
	out, err := RenderPDF(mustJSON[PDFRequest](t, reportPDF), testLimits)
	if err != nil {
		t.Fatalf("RenderPDF: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-1.")) || !bytes.HasSuffix(bytes.TrimSpace(out), []byte("%%EOF")) {
		t.Fatalf("output is not a complete PDF: %.40q", out)
	}
	if pages := len(pdfPage.FindAll(out, -1)); pages != 2 {
		t.Errorf("pages = %d, want 2", pages)
	}
	text := pdfText(t, out)
	for _, want := range []string{"(DRAFT)", "(Quarterly report body text)", "(ID)", `(Item \(Alpha\))`, "(Second page)"} {
		if !strings.Contains(text, want) {
			t.Errorf("rendered PDF lacks %s", want)
		}
	}
	// fpdf treats Symbol as an alias of ZapfDingbats.
	for _, font := range []string{"Helvetica-Bold", "Helvetica", "Courier-Oblique", "ZapfDingbats", "Times-BoldItalic"} {
		if !bytes.Contains(out, []byte("/BaseFont /"+font)) {
			t.Errorf("font %s not embedded", font)
		}
	}
}

func TestRenderPDFFillsCellsWithTheFillColor(t *testing.T) {
	raw := `{"actions":[
		{"type":"AddPage"},
		{"type":"SetFont","family":"Arial","size":12},
		{"type":"SetFillColor","r":255,"g":0,"b":0},
		{"type":"Cell","w":40,"h":10,"txt":"filled","fill":true,"ln":1},
		{"type":"SetFillColor","r":0,"g":0,"b":255},
		{"type":"MultiCell","w":40,"h":10,"txt":"multi filled","fill":true},
		{"type":"SetFillColor","r":0,"g":255,"b":0},
		{"type":"Cell","w":40,"h":10,"txt":"plain"}
	]}`
	out, err := RenderPDF(mustJSON[PDFRequest](t, raw), testLimits)
	if err != nil {
		t.Fatal(err)
	}
	text := pdfText(t, out)
	if !strings.Contains(text, "1.000 0.000 0.000 rg") || !strings.Contains(text, "0.000 0.000 1.000 rg") {
		t.Errorf("fill colors are not applied:\n%s", text)
	}
	if got := strings.Count(text, " re f"); got != 2 {
		t.Errorf("filled rectangles = %d, want 2 (only cells with fill:true)", got)
	}
}

func TestRenderPDFEncodesUTF8ForCoreFonts(t *testing.T) {
	raw := `{"actions":[
		{"type":"AddPage"},
		{"type":"SetFont","family":"Arial","size":12},
		{"type":"Cell","w":0,"h":10,"txt":"Café €","ln":1},
		{"type":"MultiCell","w":0,"h":6,"txt":"Ñandú señor über ½"}
	]}`
	out, err := RenderPDF(mustJSON[PDFRequest](t, raw), testLimits)
	if err != nil {
		t.Fatal(err)
	}
	text := pdfText(t, out)
	for _, want := range []string{"(Caf\xe9 \x80)", "(\xd1and\xfa se\xf1or \xfcber \xbd)"} {
		if !strings.Contains(text, want) {
			t.Errorf("content stream lacks %q:\n%q", want, text)
		}
	}
	if strings.Contains(text, "\xc3") {
		t.Errorf("UTF-8 bytes leaked into the content stream: %q", text)
	}
}

func TestRenderPDFTextIsExtractable(t *testing.T) {
	pdftotext, err := exec.LookPath("pdftotext")
	if err != nil {
		t.Skip("pdftotext not installed")
	}
	raw := `{"actions":[
		{"type":"AddPage"},
		{"type":"SetFont","family":"Arial","size":12},
		{"type":"Cell","w":0,"h":10,"txt":"Café € Ñandú","ln":1}
	]}`
	out, err := RenderPDF(mustJSON[PDFRequest](t, raw), testLimits)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(pdftotext, "-", "-")
	cmd.Stdin = bytes.NewReader(out)
	text, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "Café € Ñandú") {
		t.Errorf("extracted text = %q", text)
	}
}

func TestRenderPDFDefaultsAndPageSetup(t *testing.T) {
	tests := map[string]string{
		`{"actions":[{"type":"AddPage"}]}`:                               "/MediaBox [0 0 595.28 841.89]",
		`{"orientation":"L","actions":[{"type":"AddPage"}]}`:             "/MediaBox [0 0 841.89 595.28]",
		`{"size":"Letter","unit":"in","actions":[{"type":"AddPage"}]}`:   "/MediaBox [0 0 612.00 792.00]",
		`{"size":"A3","unit":"pt","actions":[{"type":"AddPage"}]}`:       "/MediaBox [0 0 841.89 1190.55]",
		`{"orientation":"landscape","actions":[{"type":"AddPage"}]}`:     "/MediaBox [0 0 841.89 595.28]",
		`{"size":"Legal","unit":"cm","actions":[{"type":"AddPage"}]}`:    "/MediaBox [0 0 612.00 1008.00]",
		`{"size":"A5","orientation":"P","actions":[{"type":"AddPage"}]}`: "/MediaBox [0 0 420.94 595.28]",
	}
	for raw, want := range tests {
		out, err := RenderPDF(mustJSON[PDFRequest](t, raw), testLimits)
		if err != nil {
			t.Errorf("%s: %v", raw, err)
			continue
		}
		if !bytes.Contains(out, []byte(want)) {
			t.Errorf("%s: output lacks %s", raw, want)
		}
	}
}

func TestRenderPDFWithoutActions(t *testing.T) {
	out, err := RenderPDF(PDFRequest{}, testLimits)
	if err != nil || !bytes.HasPrefix(out, []byte("%PDF")) {
		t.Fatalf("empty request: %v", err)
	}
}

func TestRenderPDFRejectsInvalidPageSetup(t *testing.T) {
	for _, raw := range []string{`{"orientation":"X"}`, `{"unit":"furlong"}`, `{"size":"A99"}`} {
		_, err := RenderPDF(mustJSON[PDFRequest](t, raw), testLimits)
		if !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", raw, err)
		}
	}
}

func TestRenderPDFReportsFailingAction(t *testing.T) {
	tests := []struct {
		raw   string
		index int
		typ   string
	}{
		{`{"actions":[{"type":"AddPage"},{"type":"Explode"}]}`, 1, "Explode"},
		{`{"actions":[{"type":""}]}`, 0, ""},
		{`{"actions":[{"type":"addpage"}]}`, 0, "addpage"},
		{`{"actions":[{"type":"AddPage"},{"type":"SetFont","family":"Comic Sans","size":12}]}`, 1, "SetFont"},
		{`{"actions":[{"type":"AddPage"},{"type":"SetFont","family":"../../etc/passwd","size":12}]}`, 1, "SetFont"},
		{`{"actions":[{"type":"AddPage"},{"type":"Cell","w":10,"h":10,"txt":"no font"}]}`, 1, "Cell"},
	}
	for _, tt := range tests {
		_, err := RenderPDF(mustJSON[PDFRequest](t, tt.raw), testLimits)
		assertActionError(t, err, tt.index, tt.typ)
	}
}

func TestRenderPDFActionLimit(t *testing.T) {
	req := PDFRequest{Actions: make([]PDFAction, testLimits.MaxActions+1)}
	if _, err := RenderPDF(req, testLimits); !errors.Is(err, ErrLimit) {
		t.Fatalf("err = %v, want ErrLimit", err)
	}
	req.Actions = req.Actions[:testLimits.MaxActions]
	for i := range req.Actions {
		req.Actions[i].Type = "AddPage"
	}
	if _, err := RenderPDF(req, testLimits); err != nil {
		t.Fatalf("request at the action limit rejected: %v", err)
	}
}

func TestRenderPDFExtremeGeometry(t *testing.T) {
	raw := `{"actions":[
		{"type":"AddPage"},
		{"type":"SetFont","family":"Arial","size":1e300},
		{"type":"MultiCell","w":-1e9,"h":5,"txt":"` + strings.Repeat("ab ", 500) + `"},
		{"type":"SetFont","family":"Arial","size":12},
		{"type":"MultiCell","w":0.0001,"h":0.0001,"txt":"` + strings.Repeat("ab ", 500) + `"},
		{"type":"SetXY","x":-1e300,"y":1e300},
		{"type":"Ln","h":1e308},
		{"type":"Cell","w":1e300,"h":1e300,"txt":"x"}
	]}`
	if _, err := RenderPDF(mustJSON[PDFRequest](t, raw), testLimits); err != nil {
		t.Fatalf("extreme geometry: %v", err)
	}
}

func FuzzRenderPDF(f *testing.F) {
	f.Add([]byte(reportPDF))
	f.Add([]byte(`{"orientation":"L","actions":[{"type":"AddPage"},{"type":"SetFont","family":"Arial","size":9},{"type":"MultiCell","w":5,"h":1,"txt":"x y z"}]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		req, ok := fuzzRequest[PDFRequest](data)
		if !ok {
			return
		}
		_, _ = RenderPDF(req, Limits{MaxActions: 20, MaxXLSXCells: 1000})
	})
}

func fuzzRequest[T any](data []byte) (T, bool) {
	var req T
	return req, json.Unmarshal(data, &req) == nil
}

func BenchmarkRenderPDF(b *testing.B) {
	req := mustJSON[PDFRequest](b, reportPDF)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := RenderPDF(req, testLimits); err != nil {
			b.Fatal(err)
		}
	}
}
