//go:build e2e

package e2e

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

var tableFormats = []string{"json", "yaml", "xml", "csv", "tsv", "toml", "msgpack", "xlsx"}

var trickyNotes = []string{
	"plain",
	"comma, inside",
	`quote "double" and 'single'`,
	"line one\nline two",
	"tab\tseparated",
	"unicode ✓ 日本語 émoji 🚀",
	"<xml> & entities &amp;",
	"equals = colon : hash # bang !",
	"backslash \\ path C:\\temp",
	"0042",
	"1e5",
}

func orders(n int) []map[string]any {
	rows := make([]map[string]any, n)
	for i := range rows {
		rows[i] = map[string]any{
			"id":     i + 1,
			"name":   fmt.Sprintf("Customer %d", i+1),
			"price":  float64(i)*10.25 + 0.5,
			"active": i%2 == 0,
			"note":   trickyNotes[i%len(trickyNotes)],
		}
	}
	return rows
}

func mustMarshal(t testing.TB, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func asRecords(t testing.TB, data []byte) []map[string]any {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err != nil {
		t.Fatalf("not a JSON array of objects: %v\n%.300s", err, data)
	}
	return rows
}

func sameRecords(t *testing.T, want []map[string]any, got []map[string]any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("records = %d, want %d", len(got), len(want))
	}
	index := map[string]map[string]any{}
	for _, row := range got {
		index[fmt.Sprint(row["id"])] = row
	}
	for _, w := range want {
		g, ok := index[fmt.Sprint(w["id"])]
		if !ok {
			t.Fatalf("record %v missing", w["id"])
		}
		for k, v := range w {
			if fmt.Sprint(g[k]) != fmt.Sprint(v) {
				t.Errorf("record %v field %s = %q, want %q", w["id"], k, fmt.Sprint(g[k]), fmt.Sprint(v))
			}
		}
	}
}

func TestHealth(t *testing.T) {
	s := startServer(t)
	res := s.do(t, http.MethodGet, "/health", nil, nil)
	if res.status != http.StatusOK || res.text() != "OK" {
		t.Fatalf("health = %d %q", res.status, res.body)
	}
}

func TestEveryFormatPairPreservesRecords(t *testing.T) {
	s := startServer(t)
	want := orders(len(trickyNotes) * 2)
	canonical := mustMarshal(t, want)

	inputs := map[string][]byte{}
	for _, f := range tableFormats {
		inputs[f] = s.mustConvert(t, "json", f, canonical)
	}
	for _, in := range tableFormats {
		for _, out := range tableFormats {
			t.Run(in+"_to_"+out, func(t *testing.T) {
				converted := s.mustConvert(t, in, out, inputs[in])
				sameRecords(t, want, asRecords(t, s.mustConvert(t, out, "json", converted)))
			})
		}
	}
}

func TestNestedDocumentsRoundTrip(t *testing.T) {
	s := startServer(t)
	doc := map[string]any{
		"service": map[string]any{
			"name":     "api",
			"replicas": 3,
			"tags":     []any{"blue", "green"},
			"limits":   map[string]any{"cpu": 0.5, "memory": 2048},
			"routes":   []any{map[string]any{"path": "/a", "weight": 1}, map[string]any{"path": "/b", "weight": 2}},
		},
		"enabled":   true,
		"threshold": -12.75,
	}
	canonical := mustMarshal(t, doc)
	for _, f := range []string{"yaml", "toml", "msgpack", "properties"} {
		t.Run(f, func(t *testing.T) {
			back := s.mustConvert(t, f, "json", s.mustConvert(t, "json", f, canonical))
			var got map[string]any
			if err := json.Unmarshal(back, &got); err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(got) != fmt.Sprint(decodeJSON(t, canonical)) {
				t.Errorf("round trip changed the document:\n got %v\nwant %v", got, decodeJSON(t, canonical))
			}
		})
	}
	xml := s.mustConvert(t, "json", "xml", canonical)
	for _, want := range []string{"<name>api</name>", "<path>/b</path>", "<threshold>-12.75</threshold>"} {
		if !bytes.Contains(xml, []byte(want)) {
			t.Errorf("XML lacks %s:\n%s", want, xml)
		}
	}
}

func decodeJSON(t testing.TB, data []byte) map[string]any {
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestLargeIntegersSurviveEveryFormat(t *testing.T) {
	s := startServer(t)
	body := []byte(`[{"id":9007199254740993,"order":18446744073709551615}]`)
	if res := s.convert(t, "json", "toml", body); res.status != http.StatusUnprocessableEntity || !strings.Contains(res.text(), "MaxInt64") {
		t.Errorf("TOML integers are 64-bit signed; uint64 overflow should be a 422, got %d %s", res.status, res.body)
	}
	for _, f := range tableFormats {
		body := body
		if f == "toml" {
			body = []byte(`[{"id":9007199254740993,"order":9223372036854775807}]`)
		}
		back := s.mustConvert(t, f, "json", s.mustConvert(t, "json", f, body))
		if !bytes.Contains(back, []byte("9007199254740993")) || !bytes.Contains(back, bytes.TrimSuffix(bytes.SplitAfter(body, []byte(`"order":`))[1], []byte("}]"))) {
			t.Errorf("%s lost integer precision: %s", f, back)
		}
	}
}

func TestIntegersAreNotWrittenAsFloats(t *testing.T) {
	s := startServer(t)
	toml := s.mustConvert(t, "json", "toml", []byte(`{"port":8080,"ratio":0.5}`))
	if !bytes.Contains(toml, []byte("port = 8080\n")) || !bytes.Contains(toml, []byte("ratio = 0.5\n")) {
		t.Errorf("TOML = %s", toml)
	}
	pack := s.mustConvert(t, "json", "msgpack", []byte(`{"port":8080}`))
	if bytes.Contains(pack, []byte{0xcb}) {
		t.Errorf("MessagePack encodes the integer as a float64: %x", pack)
	}
}

func TestTabularOutputKeepsDottedKeysAndWritesArraysAsJSON(t *testing.T) {
	s := startServer(t)
	csv := s.mustConvert(t, "json", "csv", []byte(`[{"a.b":1,"tags":["x","y"],"user":{"name":"Ada"}}]`))
	want := "a.b,tags,user.name\n1,\"[\"\"x\"\",\"\"y\"\"]\",Ada\n"
	if string(csv) != want {
		t.Errorf("CSV = %q, want %q", csv, want)
	}
}

func TestFormatNegotiationByHeaders(t *testing.T) {
	s := startServer(t)
	mimes := map[string]string{
		"json":    "application/json",
		"xml":     "application/xml",
		"csv":     "text/csv",
		"tsv":     "text/tab-separated-values",
		"yaml":    "application/x-yaml",
		"toml":    "application/toml",
		"msgpack": "application/msgpack",
		"xlsx":    "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	}
	canonical := mustMarshal(t, orders(3))
	for in, inMime := range mimes {
		body := s.mustConvert(t, "json", in, canonical)
		for _, outMime := range mimes {
			res := s.do(t, http.MethodPost, "/convert", http.Header{"Content-Type": {inMime}, "Accept": {outMime}}, body)
			if res.status != http.StatusOK {
				t.Errorf("%s -> %s: %d %s", inMime, outMime, res.status, res.body)
				continue
			}
			if got := res.header.Get("Content-Type"); got != outMime {
				t.Errorf("%s -> %s: Content-Type %q", inMime, outMime, got)
			}
		}
	}
}

func TestLargePayloads(t *testing.T) {
	s := startServer(t)
	var csv bytes.Buffer
	csv.WriteString("id,name,email,amount,country\n")
	rows := 0
	for csv.Len() < 20<<20 {
		rows++
		fmt.Fprintf(&csv, "%d,User %d,user%d@example.com,%d.%02d,UY\n", rows, rows, rows, rows*3, rows%100)
	}
	res := s.convert(t, "csv", "json", csv.Bytes())
	if res.status != http.StatusOK {
		t.Fatalf("20 MiB CSV: %d %.200s", res.status, res.body)
	}
	if got := len(asRecords(t, res.body)); got != rows {
		t.Fatalf("records = %d, want %d", got, rows)
	}
	t.Logf("converted %d MiB CSV (%d rows) to JSON in %s", csv.Len()>>20, rows, res.latency)

	over := bytes.Repeat([]byte("a"), 33<<20)
	if res := s.convert(t, "csv", "json", over); res.status != http.StatusRequestEntityTooLarge {
		t.Fatalf("33 MiB body: status %d", res.status)
	}
	s.healthy(t)
}

// TestLargeXLSXOutput produces a worksheet beyond excelize's 16 MiB stream
// chunk, which spills to a temporary file and so needs a writable temp dir.
func TestLargeXLSXOutput(t *testing.T) {
	s := startServer(t)
	var csv bytes.Buffer
	csv.WriteString("id,name,email,amount,country\n")
	rows := 0
	for csv.Len() < 12<<20 {
		rows++
		fmt.Fprintf(&csv, "%d,User %d,user%d@example.com,%d.5,UY\n", rows, rows, rows, rows)
	}
	xlsx := s.mustConvert(t, "csv", "xlsx", csv.Bytes())
	back := asRecords(t, s.mustConvert(t, "xlsx", "json", xlsx))
	if len(back) != rows {
		t.Fatalf("records = %d, want %d", len(back), rows)
	}
}

func TestGeneratedPDF(t *testing.T) {
	s := startServer(t)
	var actions []string
	for page := 1; page <= 3; page++ {
		actions = append(actions,
			`{"type":"AddPage"}`,
			`{"type":"SetFont","family":"Arial","style":"B","size":16}`,
			fmt.Sprintf(`{"type":"Cell","w":0,"h":10,"txt":"Page %d","ln":1}`, page),
			`{"type":"SetFont","family":"Times","size":11}`,
			`{"type":"MultiCell","w":0,"h":6,"txt":"Body text that wraps across the width of the page several times to exercise line breaking."}`,
		)
	}
	res := s.generate(t, "pdf", `{"orientation":"L","size":"Letter","actions":[`+strings.Join(actions, ",")+`]}`)
	if res.status != http.StatusOK {
		t.Fatalf("status %d %s", res.status, res.body)
	}
	if !bytes.HasPrefix(res.body, []byte("%PDF-")) || !bytes.Contains(res.body, []byte("%%EOF")) {
		t.Fatal("not a complete PDF")
	}
	if pages := len(regexp.MustCompile(`/Type /Page\b[^s]`).FindAll(res.body, -1)); pages != 3 {
		t.Errorf("pages = %d, want 3", pages)
	}
	if !bytes.Contains(res.body, []byte("/MediaBox [0 0 792.00 612.00]")) {
		t.Error("landscape Letter page size not applied")
	}
	if res.header.Get("Content-Disposition") != "attachment; filename=output.pdf" {
		t.Errorf("Content-Disposition %q", res.header.Get("Content-Disposition"))
	}
}

func TestGeneratedPDFRendersUTF8Text(t *testing.T) {
	pdftotext, err := exec.LookPath("pdftotext")
	if err != nil {
		t.Skip("pdftotext not installed")
	}
	s := startServer(t)
	res := s.generate(t, "pdf", `{"actions":[{"type":"AddPage"},{"type":"SetFont","family":"Arial","size":12},{"type":"SetFillColor","r":255,"g":0,"b":0},{"type":"Cell","w":60,"h":10,"txt":"Café €","fill":true}]}`)
	if res.status != http.StatusOK {
		t.Fatalf("status %d %s", res.status, res.body)
	}
	cmd := exec.Command(pdftotext, "-", "-")
	cmd.Stdin = bytes.NewReader(res.body)
	text, err := cmd.Output()
	if err != nil || !strings.Contains(string(text), "Café €") {
		t.Errorf("extracted text %q, err %v", text, err)
	}
}

func TestGeneratedDOCX(t *testing.T) {
	s := startServer(t)
	res := s.generate(t, "docx", `{"actions":[
		{"type":"AddParagraph"},
		{"type":"AddText","txt":"Invoice ","size":20,"bold":true},
		{"type":"AddText","txt":"#2026-10","italic":true,"color":"FF0000"},
		{"type":"AddParagraph"},
		{"type":"AddText","txt":"Thanks & regards <team>"}
	]}`)
	if res.status != http.StatusOK {
		t.Fatalf("status %d %s", res.status, res.body)
	}
	doc := zipFile(t, res.body, "word/document.xml")
	for _, want := range []string{`<w:b></w:b>`, `<w:i></w:i>`, `<w:color w:val="FF0000">`, `<w:t xml:space="preserve">Invoice </w:t>`, `Thanks &amp; regards &lt;team&gt;`} {
		if !bytes.Contains(doc, []byte(want)) {
			t.Errorf("document.xml lacks %s", want)
		}
	}
}

func TestGeneratedXLSX(t *testing.T) {
	s := startServer(t)
	var actions []string
	actions = append(actions, `{"type":"NewSheet","sheet":"Data"}`, `{"type":"SetActiveSheet","sheet":"Data"}`,
		`{"type":"NewStyle","style_ref":"h","style":{"font":{"bold":true}}}`)
	for r := 1; r <= 500; r++ {
		actions = append(actions, fmt.Sprintf(`{"type":"SetCellValue","sheet":"Data","axis":"A%d","value":%d}`, r, r*r))
	}
	actions = append(actions, `{"type":"SetCellStyle","sheet":"Data","h_cell":"A1","v_cell":"A1","style_ref":"h"}`,
		`{"type":"MergeCell","sheet":"Data","h_cell":"B1","v_cell":"D1"}`)
	res := s.generate(t, "xlsx", `{"actions":[`+strings.Join(actions, ",")+`]}`)
	if res.status != http.StatusOK {
		t.Fatalf("status %d %s", res.status, res.body)
	}
	f, err := excelize.OpenReader(bytes.NewReader(res.body))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if v, _ := f.GetCellValue("Data", "A500"); v != "250000" {
		t.Errorf("A500 = %q", v)
	}
	if f.GetSheetName(f.GetActiveSheetIndex()) != "Data" {
		t.Error("active sheet not applied")
	}
	if merges, _ := f.GetMergeCells("Data"); len(merges) != 1 {
		t.Errorf("merges = %v", merges)
	}
}

func zipFile(t *testing.T, archive []byte, name string) []byte {
	t.Helper()
	data, err := readZipEntry(archive, name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readZipEntry(archive []byte, name string) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("not a zip archive: %w", err)
	}
	for _, f := range r.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("archive lacks %s", name)
}

func TestGracefulShutdownFinishesInFlightRequests(t *testing.T) {
	s := startServer(t)
	var csv bytes.Buffer
	csv.WriteString("a,b,c\n")
	for csv.Len() < 8<<20 {
		csv.WriteString("1,two,three\n")
	}
	done := make(chan response, 1)
	go func() {
		res, err := s.try(http.MethodPost, "/convert?input=csv&output=xml", nil, csv.Bytes())
		if err != nil {
			res.status = -1
		}
		done <- res
	}()
	time.Sleep(300 * time.Millisecond)
	if err := s.stop(60 * time.Second); err != nil {
		t.Fatalf("exit: %v", err)
	}
	if res := <-done; res.status != http.StatusOK {
		t.Fatalf("in-flight request was cut off: status %d", res.status)
	}
	if conn, err := net.DialTimeout("tcp", s.addr, time.Second); err == nil {
		conn.Close()
		t.Error("still accepting connections after shutdown")
	}
	if !strings.Contains(s.logs.String(), "shutting down") {
		t.Error("shutdown not logged")
	}
}

func TestStartupRejectsInvalidConfiguration(t *testing.T) {
	for _, kv := range [][2]string{{"MAX_BODY_BYTES", "0"}, {"MAX_ACTIONS", "-1"}, {"MAX_XLSX_CELLS", "many"}, {"MAX_CONCURRENT_REQUESTS", "1.5"}, {"SERVER_ADDRESS", "not-an-address"}} {
		t.Run(kv[0]+"="+kv[1], func(t *testing.T) {
			s := launch(t, raceBinary, withEnv(kv[0], kv[1]))
			select {
			case <-s.exited:
			case <-time.After(15 * time.Second):
				_ = s.cmd.Process.Kill()
				t.Fatal("server started with invalid configuration")
			}
			var exitErr *exec.ExitError
			if !errors.As(s.exitErr, &exitErr) || exitErr.ExitCode() != 1 {
				t.Fatalf("exit = %v, want status 1", s.exitErr)
			}
			if !strings.Contains(s.logs.String(), "server stopped") {
				t.Errorf("no error logged:\n%s", s.logs)
			}
		})
	}
}

func TestStartupFailsWhenPortIsTaken(t *testing.T) {
	if image != "" {
		t.Skip("the container port is always free")
	}
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	s := launch(t, raceBinary, withEnv("SERVER_ADDRESS", l.Addr().String()))
	select {
	case <-s.exited:
	case <-time.After(15 * time.Second):
		_ = s.cmd.Process.Kill()
		t.Fatal("server kept running with its port taken")
	}
	if s.exitErr == nil || !strings.Contains(s.logs.String(), "address already in use") {
		t.Errorf("exit = %v logs:\n%s", s.exitErr, s.logs)
	}
}
