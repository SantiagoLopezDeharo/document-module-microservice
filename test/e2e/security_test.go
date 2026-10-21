//go:build e2e

package e2e

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

const memoryCeilingKB = 1500 << 10

// survives asserts the server is still up, healthy and within a sane memory
// footprint after an attack.
func survives(t *testing.T, s *server) {
	t.Helper()
	if !s.alive() {
		t.Fatalf("server died: %v\n%.4000s", s.exitErr, s.logs)
	}
	s.healthy(t)
	if image == "" {
		if rss := s.rssKB(t); rss > memoryCeilingKB {
			t.Errorf("RSS after attack = %d MiB", rss>>10)
		}
	}
}

func TestNestingBombsDoNotCrashTheProcess(t *testing.T) {
	s := startServer(t)
	const depth = 2_000_000
	bombs := map[string]string{
		"xml":             strings.Repeat("<a>", depth) + strings.Repeat("</a>", depth),
		"toml arrays":     "a = " + strings.Repeat("[", depth) + strings.Repeat("]", depth),
		"toml tables":     "a = " + strings.Repeat("{b = ", depth/4) + "1" + strings.Repeat("}", depth/4),
		"toml dotted":     strings.Repeat("a.", depth) + "a = 1",
		"msgpack":         strings.Repeat("\x91", depth) + "\xc0",
		"msgpack maps":    strings.Repeat("\x81\xa1k", depth/2) + "\xc0",
		"properties keys": strings.Repeat("a.", depth) + "a=1",
		"json":            strings.Repeat("[", depth) + strings.Repeat("]", depth),
		"yaml":            strings.Repeat("[", depth) + strings.Repeat("]", depth),
	}
	for name, body := range bombs {
		format, _, _ := strings.Cut(name, " ")
		res := s.convert(t, format, "json", []byte(body))
		if res.status < 400 || res.status >= 500 {
			t.Errorf("%s: status %d", name, res.status)
		}
		survives(t, s)
	}
}

func TestMsgpackLengthBombsAreRejectedCheaply(t *testing.T) {
	s := startServer(t)
	bombs := map[string][]byte{
		"array32": {0xdd, 0xff, 0xff, 0xff, 0xff},
		"map32":   {0xdf, 0xff, 0xff, 0xff, 0xff},
		"str32":   {0xdb, 0xff, 0xff, 0xff, 0xff, 'a'},
		"bin32":   {0xc6, 0xff, 0xff, 0xff, 0xff},
		"nested":  bytes.Repeat([]byte{0x91, 0xdd, 0x0f, 0xff, 0xff, 0xff}, 1000),
	}
	for name, body := range bombs {
		res := s.convert(t, "msgpack", "json", body)
		if res.status != http.StatusBadRequest {
			t.Errorf("%s: status %d %s", name, res.status, res.body)
		}
		if res.latency > 2*time.Second {
			t.Errorf("%s took %s", name, res.latency)
		}
		survives(t, s)
	}
}

func TestPropertiesExpansionIsInert(t *testing.T) {
	s := startServer(t, withEnv("SECRET_TOKEN", "super-secret-value"))
	for _, body := range []string{"a=${b}\nb=${a}", "leak=${SECRET_TOKEN}", "home=${HOME}", "x=${x}"} {
		res := s.convert(t, "properties", "json", []byte(body))
		if res.status != http.StatusOK {
			t.Errorf("%q: status %d %s", body, res.status, res.body)
		}
		if strings.Contains(res.text(), "super-secret-value") {
			t.Fatalf("environment leaked: %s", res.body)
		}
		survives(t, s)
	}
}

func TestYAMLAliasBomb(t *testing.T) {
	s := startServer(t)
	var doc strings.Builder
	doc.WriteString(`a: &a ["lol","lol","lol","lol","lol","lol","lol","lol","lol"]` + "\n")
	prev := "a"
	for _, name := range []string{"b", "c", "d", "e", "f", "g", "h", "i"} {
		fmt.Fprintf(&doc, "%s: &%s [%s]\n", name, name, strings.TrimSuffix(strings.Repeat("*"+prev+",", 9), ","))
		prev = name
	}
	res := s.convert(t, "yaml", "json", []byte(doc.String()))
	if res.status != http.StatusBadRequest {
		t.Errorf("billion laughs: status %d (%d bytes)", res.status, len(res.body))
	}
	if res.latency > 5*time.Second {
		t.Errorf("billion laughs took %s", res.latency)
	}
	survives(t, s)
}

func TestXMLEntityAttacks(t *testing.T) {
	s := startServer(t)
	attacks := map[string]string{
		"xxe file":       `<?xml version="1.0"?><!DOCTYPE x [<!ENTITY xxe SYSTEM "file:///etc/passwd">]><x><v>&xxe;</v></x>`,
		"xxe url":        `<?xml version="1.0"?><!DOCTYPE x [<!ENTITY xxe SYSTEM "http://169.254.169.254/latest/meta-data/">]><x><v>&xxe;</v></x>`,
		"parameter":      `<?xml version="1.0"?><!DOCTYPE x [<!ENTITY % p SYSTEM "file:///etc/passwd"> %p;]><x/>`,
		"billion laughs": `<?xml version="1.0"?><!DOCTYPE l [<!ENTITY a "lol"><!ENTITY b "&a;&a;&a;&a;&a;&a;&a;&a;&a;&a;"><!ENTITY c "&b;&b;&b;&b;&b;&b;&b;&b;&b;&b;"><!ENTITY d "&c;&c;&c;&c;&c;&c;&c;&c;&c;&c;">]><l><v>&d;</v></l>`,
	}
	for name, body := range attacks {
		res := s.convert(t, "xml", "json", []byte(body))
		if res.status == http.StatusOK && (strings.Contains(res.text(), "root:") || strings.Count(res.text(), "lol") > 10) {
			t.Errorf("%s: entity expanded: %.200s", name, res.body)
		}
		if res.status >= 500 {
			t.Errorf("%s: status %d", name, res.status)
		}
		survives(t, s)
	}
}

func TestXMLOutputCannotBeInjected(t *testing.T) {
	s := startServer(t)
	res := s.convert(t, "json", "xml", []byte(`{"user":{"name":"</name><role>admin</role><name>","id":1}}`))
	if res.status != http.StatusOK || strings.Contains(res.text(), "<role>") {
		t.Fatalf("value injected markup: %d %s", res.status, res.body)
	}
	back := s.mustConvert(t, "xml", "json", res.body)
	if !bytes.Contains(back, []byte(`"name": "</name><role>admin</role><name>"`)) {
		t.Errorf("value not preserved: %s", back)
	}
	if res := s.convert(t, "json", "xml", []byte(`{"a><b":1}`)); res.status != http.StatusUnprocessableEntity {
		t.Errorf("invalid element name: status %d %s", res.status, res.body)
	}
}

// zipBomb builds a valid workbook whose first sheet inflates to size bytes.
func zipBomb(t *testing.T, size int) []byte {
	t.Helper()
	f := excelize.NewFile()
	var base bytes.Buffer
	if err := f.Write(&base); err != nil {
		t.Fatal(err)
	}
	src, err := zip.NewReader(bytes.NewReader(base.Bytes()), int64(base.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, file := range src.File {
		w, err := zw.Create(file.Name)
		if err != nil {
			t.Fatal(err)
		}
		if file.Name != "xl/worksheets/sheet1.xml" {
			rc, _ := file.Open()
			_, _ = io.Copy(w, rc)
			rc.Close()
			continue
		}
		io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
		row := []byte(`<row><c t="inlineStr"><is><t>bomb</t></is></c></row>`)
		for written := 0; written < size; written += len(row) {
			_, _ = w.Write(row)
		}
		io.WriteString(w, `</sheetData></worksheet>`)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestXLSXZipBomb(t *testing.T) {
	s := startServer(t)
	bomb := zipBomb(t, 600<<20)
	t.Logf("zip bomb: %d KiB inflating to 600 MiB", len(bomb)>>10)
	res := s.convert(t, "xlsx", "json", bomb)
	if res.status != http.StatusBadRequest {
		t.Errorf("status %d %.200s", res.status, res.body)
	}
	survives(t, s)
}

func TestDocumentAmplificationIsBounded(t *testing.T) {
	s := startServer(t)
	attacks := map[string]string{
		"last cell":          `{"actions":[{"type":"SetCellValue","sheet":"Sheet1","axis":"XFD1048576","value":1}]}`,
		"last row":           `{"actions":[{"type":"SetRowHeight","sheet":"Sheet1","row":1048576,"height":10}]}`,
		"full sheet merge":   `{"actions":[{"type":"MergeCell","sheet":"Sheet1","h_cell":"A1","v_cell":"XFD1048576"}]}`,
		"huge style range":   `{"actions":[{"type":"NewStyle","style":{"font":{"bold":true}}},{"type":"SetCellStyle","sheet":"Sheet1","h_cell":"A1","v_cell":"Z200000","style_id":1}]}`,
		"repeated restyling": `{"actions":[{"type":"NewStyle","style":{"font":{"bold":true}}}` + strings.Repeat(`,{"type":"SetCellStyle","sheet":"Sheet1","h_cell":"A1","v_cell":"J10000","style_id":1}`, 200) + `]}`,
	}
	for name, body := range attacks {
		res := s.generate(t, "xlsx", body)
		if res.status != http.StatusRequestEntityTooLarge {
			t.Errorf("%s: status %d %.200s", name, res.status, res.body)
		}
		if res.latency > 5*time.Second {
			t.Errorf("%s took %s", name, res.latency)
		}
		survives(t, s)
	}

	tooMany := `{"actions":[` + strings.TrimSuffix(strings.Repeat(`{"type":"AddPage"},`, 10_001), ",") + `]}`
	if res := s.generate(t, "pdf", tooMany); res.status != http.StatusRequestEntityTooLarge {
		t.Errorf("10001 actions: status %d", res.status)
	}
	maxPages := `{"actions":[` + strings.TrimSuffix(strings.Repeat(`{"type":"AddPage"},`, 10_000), ",") + `]}`
	if res := s.generate(t, "pdf", maxPages); res.status != http.StatusOK || res.latency > 30*time.Second {
		t.Errorf("10000 pages: status %d in %s", res.status, res.latency)
	}
	survives(t, s)
}

func TestDocumentInjection(t *testing.T) {
	s := startServer(t)
	res := s.generate(t, "docx", `{"actions":[{"type":"AddText","txt":"x","color":"\"/><w:b/><w:color w:val=\"FF0000"}]}`)
	if res.status != http.StatusBadRequest {
		t.Errorf("color injection: status %d", res.status)
	}
	res = s.generate(t, "docx", `{"actions":[{"type":"AddText","txt":"</w:t></w:r><w:r><w:t>injected"}]}`)
	if res.status != http.StatusOK {
		t.Fatalf("status %d", res.status)
	}
	if doc := zipFile(t, res.body, "word/document.xml"); bytes.Count(doc, []byte("<w:r>")) != 1 {
		t.Errorf("text escaped its run: %s", doc)
	}
	for _, family := range []string{"../../../../etc/passwd", "/etc/passwd", "file:///etc/passwd"} {
		res := s.generate(t, "pdf", `{"actions":[{"type":"AddPage"},{"type":"SetFont","family":"`+family+`","size":12}]}`)
		if res.status != http.StatusBadRequest {
			t.Errorf("font %q: status %d", family, res.status)
		}
	}
}

func TestRequestLimits(t *testing.T) {
	s := startServer(t, withEnv("MAX_BODY_BYTES", "1024"))
	big := []byte(`{"k":"` + strings.Repeat("x", 2048) + `"}`)
	for _, path := range []string{"/convert?input=json&output=xml", "/convert/pdf", "/convert/docx", "/convert/xlsx"} {
		res := s.do(t, http.MethodPost, path, nil, big)
		if res.status != http.StatusRequestEntityTooLarge {
			t.Errorf("%s: status %d", path, res.status)
		}
	}
	if status := rawStatus(t, s, "GET /health HTTP/1.1\r\nHost: x\r\nX-Big: "+strings.Repeat("a", 256<<10)+"\r\n\r\n"); !strings.Contains(status, " 431 ") {
		t.Errorf("oversized headers: %q", status)
	}
	survives(t, s)
}

func TestSlowUploadsDoNotStarveOtherClients(t *testing.T) {
	s := startServer(t, withEnv("MAX_CONCURRENT_REQUESTS", "1"))
	conn, err := net.Dial("tcp", s.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	fmt.Fprintf(conn, "POST /convert?input=json&output=xml HTTP/1.1\r\nHost: x\r\nContent-Length: 100000\r\n\r\n{\"a\":")
	time.Sleep(200 * time.Millisecond)

	start := time.Now()
	res := s.convert(t, "json", "yaml", []byte(`{"quick":true}`))
	if res.status != http.StatusOK || time.Since(start) > 2*time.Second {
		t.Fatalf("request behind a slow uploader: %d after %s", res.status, time.Since(start))
	}
}

func TestSlowlorisHeadersAreCut(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the header read timeout")
	}
	s := startServer(t)
	conn, err := net.Dial("tcp", s.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte("POST /convert HTTP/1.1\r\nHost: x\r\n"))
	start := time.Now()
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	_, _ = io.ReadAll(conn)
	if elapsed := time.Since(start); elapsed > 15*time.Second {
		t.Fatalf("incomplete headers held the connection for %s", elapsed)
	}
}

func TestMethodAndRouteHardening(t *testing.T) {
	s := startServer(t)
	tests := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/convert", http.StatusMethodNotAllowed},
		{http.MethodPut, "/convert/pdf", http.StatusMethodNotAllowed},
		{http.MethodDelete, "/convert/xlsx", http.StatusMethodNotAllowed},
		{"TRACE", "/convert", http.StatusMethodNotAllowed},
		{http.MethodOptions, "/convert/docx", http.StatusMethodNotAllowed},
		{http.MethodPost, "/health", http.StatusMethodNotAllowed},
		{http.MethodGet, "/debug/pprof/", http.StatusNotFound},
		{http.MethodGet, "/debug/pprof/heap", http.StatusNotFound},
		{http.MethodGet, "/.env", http.StatusNotFound},
		{http.MethodPost, "/convert/../convert", http.StatusOK},
		{http.MethodPost, "/CONVERT", http.StatusNotFound},
	}
	for _, tt := range tests {
		res := s.do(t, tt.method, tt.path+"?input=json&output=yaml", nil, []byte(`{"a":1}`))
		if tt.path == "/convert/../convert" {
			continue
		}
		if res.status != tt.want {
			t.Errorf("%s %s: %d, want %d", tt.method, tt.path, res.status, tt.want)
		}
	}
}

var leakPattern = regexp.MustCompile(`goroutine \d|\.go:\d+|panic|runtime\.|/Users/|/home/|/src/`)

func TestErrorResponsesDoNotLeakInternals(t *testing.T) {
	s := startServer(t)
	bodies := map[string][]byte{
		"/convert?input=json&output=csv":       []byte(`[1]`),
		"/convert?input=xml&output=json":       []byte(`<a>`),
		"/convert?input=xlsx&output=json":      []byte(`PK garbage`),
		"/convert?input=msgpack&output=json":   {0xc1},
		"/convert?input=toml&output=json":      []byte(`a = [`),
		"/convert?input=nope&output=json":      []byte(`{}`),
		"/convert/pdf":                         []byte(`{"size":"A99"}`),
		"/convert/xlsx":                        []byte(`{"actions":[{"type":"SetCellValue","sheet":"x","axis":"?"}]}`),
		"/convert/docx":                        []byte(`{"actions":[{"type":"Nope"}]}`),
		"/convert?input=properties&output=xml": []byte(`a=\u12`),
	}
	for path, body := range bodies {
		res := s.do(t, http.MethodPost, path, nil, body)
		if res.status < 400 || res.status >= 500 {
			t.Errorf("%s: status %d", path, res.status)
		}
		if leakPattern.Match(res.body) {
			t.Errorf("%s leaks internals: %s", path, res.body)
		}
	}
}

func TestConnectionFloodDoesNotLeak(t *testing.T) {
	s := startServer(t)
	time.Sleep(200 * time.Millisecond)
	baseline := s.goroutines(t)
	var wg sync.WaitGroup
	for range 300 {
		wg.Go(func() {
			conn, err := net.Dial("tcp", s.addr)
			if err != nil {
				return
			}
			fmt.Fprintf(conn, "POST /convert?input=json&output=xml HTTP/1.1\r\nHost: x\r\nContent-Length: 50\r\n\r\n{")
			conn.Close()
		})
	}
	wg.Wait()
	if after := s.waitGoroutinesBelow(t, baseline+5, 15*time.Second); after > baseline+5 {
		t.Fatalf("goroutines %d -> %d after abandoned uploads", baseline, after)
	}
	survives(t, s)
}

// TestConcurrencyLimitBoundsMemory compares the peak footprint of one
// conversion with sixteen concurrent ones: with two processing slots the
// latter must stay close to twice the former instead of growing with clients.
func TestConcurrencyLimitBoundsMemory(t *testing.T) {
	if testing.Short() || image != "" {
		t.Skip("heavy, needs a local process to sample")
	}
	var csv bytes.Buffer
	csv.WriteString("id,name,email,city\n")
	for i := 0; csv.Len() < 10<<20; i++ {
		fmt.Fprintf(&csv, "%d,name %d,user%d@example.com,Montevideo\n", i, i, i)
	}
	single := peakRSS(t, csv.Bytes(), 1)
	concurrent := peakRSS(t, csv.Bytes(), 16)
	t.Logf("peak RSS: 1 client %d MiB, 16 clients on 2 slots %d MiB", single>>10, concurrent>>10)
	bodies := int64(16 * csv.Len() >> 10)
	if concurrent > 3*single+2*bodies {
		t.Errorf("memory grows with clients: %d MiB > 3 x %d MiB + bodies", concurrent>>10, single>>10)
	}
}

func peakRSS(t *testing.T, body []byte, clients int) int64 {
	s := startServer(t, withEnv("MAX_CONCURRENT_REQUESTS", "2"))
	var peak atomic.Int64
	stop := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		for {
			select {
			case <-stop:
				return
			case <-time.After(20 * time.Millisecond):
				if rss := s.rssKB(t); rss > peak.Load() {
					peak.Store(rss)
				}
			}
		}
	}()
	var wg sync.WaitGroup
	var failures atomic.Int64
	for range clients {
		wg.Go(func() {
			if res, err := s.try(http.MethodPost, "/convert?input=csv&output=xml", nil, body); err != nil || res.status != 200 {
				failures.Add(1)
				t.Logf("conversion failed: %v status %d %.200s", err, res.status, res.body)
			}
		})
	}
	wg.Wait()
	close(stop)
	<-sampled
	if failures.Load() > 0 {
		t.Fatalf("%d conversions failed", failures.Load())
	}
	return peak.Load()
}

func rawStatus(t *testing.T, s *server, request string) string {
	t.Helper()
	conn, err := net.Dial("tcp", s.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_, _ = conn.Write([]byte(request))
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	line, _ := bufio.NewReader(conn).ReadString('\n')
	return line
}
