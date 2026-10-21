package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/document"
	"github.com/xuri/excelize/v2"
)

type call struct {
	method, target string
	header         map[string]string
	body           []byte
}

func serve(t testing.TB, h http.Handler, c call) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(c.method, c.target, bytes.NewReader(c.body))
	for k, v := range c.header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRoutesAndMethods(t *testing.T) {
	router := testRouter(t)
	tests := []struct {
		method, path string
		want         int
	}{
		{http.MethodGet, "/health", http.StatusOK},
		{http.MethodHead, "/health", http.StatusOK},
		{http.MethodPost, "/health", http.StatusMethodNotAllowed},
		{http.MethodGet, "/convert", http.StatusMethodNotAllowed},
		{http.MethodPut, "/convert", http.StatusMethodNotAllowed},
		{http.MethodDelete, "/convert/pdf", http.StatusMethodNotAllowed},
		{http.MethodGet, "/convert/docx", http.StatusMethodNotAllowed},
		{http.MethodPatch, "/convert/xlsx", http.StatusMethodNotAllowed},
		{http.MethodGet, "/", http.StatusNotFound},
		{http.MethodPost, "/convert/html", http.StatusNotFound},
		{http.MethodPost, "/convert/pdf/", http.StatusNotFound},
		{http.MethodGet, "/debug/pprof/", http.StatusNotFound},
	}
	for _, tt := range tests {
		if rec := serve(t, router, call{method: tt.method, target: tt.path}); rec.Code != tt.want {
			t.Errorf("%s %s = %d, want %d", tt.method, tt.path, rec.Code, tt.want)
		}
	}
}

var conversionFixtures = map[string]struct {
	body     string
	contains []string
}{
	"json":       {`[{"id":1,"name":"Ada"},{"id":2,"name":"Grace"}]`, []string{"Ada", "Grace"}},
	"yaml":       {"- id: 1\n  name: Ada\n- id: 2\n  name: Grace\n", []string{"Ada", "Grace"}},
	"xml":        {`<doc><List><id>1</id><name>Ada</name></List><List><id>2</id><name>Grace</name></List></doc>`, []string{"Ada", "Grace"}},
	"csv":        {"id,name\n1,Ada\n2,Grace\n", []string{"Ada", "Grace"}},
	"tsv":        {"id\tname\n1\tAda\n2\tGrace\n", []string{"Ada", "Grace"}},
	"toml":       {"[[root]]\nid = 1\nname = \"Ada\"\n[[root]]\nid = 2\nname = \"Grace\"\n", []string{"Ada", "Grace"}},
	"properties": {"root[0].name=Ada\nroot[1].name=Grace\n", []string{"Ada", "Grace"}},
}

var outputFormats = []string{"json", "yaml", "xml", "csv", "tsv", "toml", "msgpack", "xlsx"}

func TestConvertEveryPair(t *testing.T) {
	router := testRouter(t)
	for input, fx := range conversionFixtures {
		for _, output := range outputFormats {
			if input == "properties" && (output == "csv" || output == "tsv" || output == "xlsx") {
				continue
			}
			t.Run(input+"_to_"+output, func(t *testing.T) {
				rec := serve(t, router, call{method: http.MethodPost, target: "/convert?input=" + input + "&output=" + output, body: []byte(fx.body)})
				if rec.Code != http.StatusOK {
					t.Fatalf("status %d: %s", rec.Code, rec.Body)
				}
				if rec.Header().Get("Content-Length") != fmt.Sprint(rec.Body.Len()) {
					t.Errorf("Content-Length %q for %d bytes", rec.Header().Get("Content-Length"), rec.Body.Len())
				}
				back := serve(t, router, call{method: http.MethodPost, target: "/convert?input=" + output + "&output=json", body: rec.Body.Bytes()})
				if back.Code != http.StatusOK {
					t.Fatalf("round trip to json: %d %s", back.Code, back.Body)
				}
				for _, want := range fx.contains {
					if !strings.Contains(back.Body.String(), want) {
						t.Errorf("round trip lost %q: %s", want, back.Body)
					}
				}
			})
		}
	}
}

func TestConvertFormatNegotiation(t *testing.T) {
	router := testRouter(t)
	tests := []struct {
		name        string
		target      string
		header      map[string]string
		body        string
		wantStatus  int
		wantContent string
	}{
		{"query params", "/convert?input=json&output=yaml", nil, `{"a":1}`, 200, "application/x-yaml"},
		{"query is case insensitive", "/convert?input=JsOn&output=XmL", nil, `{"a":1}`, 200, "application/xml"},
		{"headers", "/convert", map[string]string{"Content-Type": "application/json; charset=utf-8", "Accept": "text/csv"}, `[{"a":1}]`, 200, "text/csv"},
		{"query wins over headers", "/convert?output=toml", map[string]string{"Content-Type": "application/json", "Accept": "text/csv"}, `{"a":1}`, 200, "application/toml"},
		{"xlsx accept header", "/convert", map[string]string{"Content-Type": "application/json", "Accept": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"}, `[{"a":1}]`, 200, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
		{"missing both", "/convert", nil, `{}`, 400, ""},
		{"unknown accept", "/convert?input=json", map[string]string{"Accept": "*/*"}, `{}`, 400, ""},
		{"unsupported input", "/convert?input=html&output=json", nil, `{}`, 400, ""},
		{"unsupported output", "/convert?input=json&output=pdf", nil, `{}`, 400, ""},
	}
	for _, tt := range tests {
		rec := serve(t, router, call{method: http.MethodPost, target: tt.target, header: tt.header, body: []byte(tt.body)})
		if rec.Code != tt.wantStatus {
			t.Errorf("%s: status %d, want %d (%s)", tt.name, rec.Code, tt.wantStatus, rec.Body)
			continue
		}
		if tt.wantContent != "" && rec.Header().Get("Content-Type") != tt.wantContent {
			t.Errorf("%s: Content-Type %q, want %q", tt.name, rec.Header().Get("Content-Type"), tt.wantContent)
		}
	}
}

func TestConvertXLSXUploadByContentType(t *testing.T) {
	f := excelize.NewFile()
	f.SetCellValue("Sheet1", "A1", "name")
	f.SetCellValue("Sheet1", "A2", "Ada")
	var wb bytes.Buffer
	if err := f.Write(&wb); err != nil {
		t.Fatal(err)
	}
	rec := serve(t, testRouter(t), call{
		method: http.MethodPost, target: "/convert",
		header: map[string]string{"Content-Type": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "Accept": "application/json"},
		body:   wb.Bytes(),
	})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name": "Ada"`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
}

func TestConvertErrorStatuses(t *testing.T) {
	router := NewRouter(testConfig(t, map[string]string{"MAX_BODY_BYTES": "4096"}))
	tests := []struct {
		name   string
		target string
		body   string
		want   int
		says   string
	}{
		{"malformed json", "/convert?input=json&output=xml", `{"a":`, 400, "Decode error"},
		{"trailing json", "/convert?input=json&output=xml", `{"a":1}{"b":2}`, 400, "unexpected data"},
		{"malformed xml", "/convert?input=xml&output=json", `<a>`, 400, "Decode error"},
		{"circular properties", "/convert?input=properties&output=json", "a=${b}\nb=${a}", 200, "${b}"},
		{"scalars to csv", "/convert?input=json&output=csv", `[1,2,3]`, 422, "Encode error"},
		{"string to xlsx", "/convert?input=json&output=xlsx", `"text"`, 422, "Encode error"},
		{"list to properties", "/convert?input=json&output=properties", `[1]`, 422, "Encode error"},
		{"body too large", "/convert?input=json&output=xml", `{"k":"` + strings.Repeat("x", 5000) + `"}`, 413, "too large"},
		{"too deep", "/convert?input=xml&output=json", strings.Repeat("<a>", 1000), 400, "Decode error"},
		{"empty body", "/convert?input=json&output=xml", ``, 400, "Decode error"},
	}
	for _, tt := range tests {
		rec := serve(t, router, call{method: http.MethodPost, target: tt.target, body: []byte(tt.body)})
		if rec.Code != tt.want || !strings.Contains(rec.Body.String(), tt.says) {
			t.Errorf("%s: %d %q, want %d containing %q", tt.name, rec.Code, rec.Body, tt.want, tt.says)
		}
		if tt.want >= 400 && rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" {
			t.Errorf("%s: error Content-Type %q", tt.name, rec.Header().Get("Content-Type"))
		}
	}
}

func TestConvertRejectsExcessiveNestingWith413(t *testing.T) {
	deep := strings.Repeat("<a>", 20_000) + strings.Repeat("</a>", 20_000)
	rec := serve(t, testRouter(t), call{method: http.MethodPost, target: "/convert?input=xml&output=json", body: []byte(deep)})
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
}

func TestDocumentEndpoints(t *testing.T) {
	router := testRouter(t)
	tests := []struct {
		path, body, contentType, filename, magic string
	}{
		{"/convert/pdf", `{"actions":[{"type":"AddPage"},{"type":"SetFont","family":"Arial","size":12},{"type":"Cell","w":40,"h":10,"txt":"Hi"}]}`, contentTypePDF, "output.pdf", "%PDF-"},
		{"/convert/docx", `{"actions":[{"type":"AddText","txt":"Hi","bold":true}]}`, contentTypeDOCX, "output.docx", "PK\x03\x04"},
		{"/convert/xlsx", `{"actions":[{"type":"SetCellValue","sheet":"Sheet1","axis":"A1","value":"Hi"}]}`, contentTypeXLSX, "output.xlsx", "PK\x03\x04"},
	}
	for _, tt := range tests {
		rec := serve(t, router, call{method: http.MethodPost, target: tt.path, body: []byte(tt.body)})
		if rec.Code != http.StatusOK {
			t.Errorf("%s: status %d %s", tt.path, rec.Code, rec.Body)
			continue
		}
		if got := rec.Header().Get("Content-Type"); got != tt.contentType {
			t.Errorf("%s: Content-Type %q", tt.path, got)
		}
		if got := rec.Header().Get("Content-Disposition"); got != "attachment; filename="+tt.filename {
			t.Errorf("%s: Content-Disposition %q", tt.path, got)
		}
		if !bytes.HasPrefix(rec.Body.Bytes(), []byte(tt.magic)) {
			t.Errorf("%s: body starts with %q", tt.path, rec.Body.Bytes()[:min(8, rec.Body.Len())])
		}
		if rec.Header().Get("Content-Length") != fmt.Sprint(rec.Body.Len()) {
			t.Errorf("%s: Content-Length mismatch", tt.path)
		}
	}
}

func TestDocumentErrorStatuses(t *testing.T) {
	router := NewRouter(testConfig(t, map[string]string{"MAX_ACTIONS": "3", "MAX_XLSX_CELLS": "100", "MAX_BODY_BYTES": "2048"}))
	tests := []struct {
		path, body string
		want       int
		says       string
	}{
		{"/convert/pdf", `not json`, 400, "Invalid JSON"},
		{"/convert/pdf", `{"actions":"nope"}`, 400, "Invalid JSON"},
		{"/convert/pdf", `{"size":"A99"}`, 400, "page setup"},
		{"/convert/pdf", `{"actions":[{"type":"AddPage"},{"type":"Boom"}]}`, 400, "action 1 (Boom)"},
		{"/convert/pdf", `{"actions":[{},{},{},{}]}`, 413, "at most 3"},
		{"/convert/docx", `{"actions":[{"type":"AddText","color":"red"}]}`, 400, "action 0 (AddText)"},
		{"/convert/xlsx", `{"actions":[{"type":"SetCellValue","sheet":"Sheet1","axis":"A500","value":1}]}`, 413, "exceed 100 cells"},
		{"/convert/xlsx", `{"actions":[{"type":"SetCellValue","sheet":"Nope","axis":"A1","value":1}]}`, 400, "action 0 (SetCellValue)"},
		{"/convert/docx", `{"actions":[{"type":"AddText","txt":"` + strings.Repeat("x", 3000) + `"}]}`, 413, "too large"},
	}
	for _, tt := range tests {
		rec := serve(t, router, call{method: http.MethodPost, target: tt.path, body: []byte(tt.body)})
		if rec.Code != tt.want || !strings.Contains(rec.Body.String(), tt.says) {
			t.Errorf("%s %s: %d %q, want %d containing %q", tt.path, tt.body[:min(40, len(tt.body))], rec.Code, rec.Body, tt.want, tt.says)
		}
		if rec.Header().Get("Content-Disposition") != "" {
			t.Errorf("%s: error response advertises an attachment", tt.path)
		}
	}
}

func TestRenderErrorHidesInternalFailures(t *testing.T) {
	failing := func(document.PDFRequest, document.Limits) ([]byte, error) {
		return nil, errors.New("disk /var/secret exploded")
	}
	rec := serve(t, documentHandler(failing, document.Limits{}, contentTypePDF, "x.pdf"), call{method: http.MethodPost, target: "/", body: []byte(`{}`)})
	if rec.Code != http.StatusInternalServerError || strings.Contains(rec.Body.String(), "secret") {
		t.Fatalf("status %d body %q", rec.Code, rec.Body)
	}
}

func TestGuardedLimitsConcurrency(t *testing.T) {
	const limit = 3
	var active, peak atomic.Int64
	release := make(chan struct{})
	slow := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		<-release
		active.Add(-1)
	})
	h := guarded(1<<20, limit)(slow)

	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { serve(t, h, call{method: http.MethodPost, target: "/"}) })
	}
	time.Sleep(100 * time.Millisecond)
	if got := active.Load(); got != limit {
		t.Errorf("active requests = %d, want %d", got, limit)
	}
	close(release)
	wg.Wait()
	if peak.Load() != limit {
		t.Errorf("peak concurrency = %d, want %d", peak.Load(), limit)
	}
}

func TestGuardedAbandonsCancelledWaiters(t *testing.T) {
	block := make(chan struct{})
	h := guarded(1<<20, 1)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-block }))
	go serve(t, h, call{method: http.MethodPost, target: "/"})
	time.Sleep(20 * time.Millisecond)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	ctx, cancel := contextWithCancel(req)
	cancel()
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(httptest.NewRecorder(), req.WithContext(ctx))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled request kept waiting for a slot")
	}
	close(block)
}

func TestGuardedCapsBody(t *testing.T) {
	h := guarded(10, 1)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			clientError(w, "read", err)
		}
	}))
	if rec := serve(t, h, call{method: http.MethodPost, target: "/", body: []byte("01234567890")}); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status %d", rec.Code)
	}
	if rec := serve(t, h, call{method: http.MethodPost, target: "/", body: []byte("0123456789")}); rec.Code != http.StatusOK {
		t.Errorf("body at the limit: status %d", rec.Code)
	}
}

func BenchmarkConvertJSONToCSV(b *testing.B) {
	var body strings.Builder
	body.WriteString("[")
	for i := range 1000 {
		if i > 0 {
			body.WriteString(",")
		}
		fmt.Fprintf(&body, `{"id":%d,"name":"user %d","active":true,"meta":{"score":%d.5}}`, i, i, i)
	}
	body.WriteString("]")
	router := testRouter(b)
	payload := []byte(body.String())
	b.ReportAllocs()
	b.SetBytes(int64(len(payload)))
	for b.Loop() {
		if rec := serve(b, router, call{method: http.MethodPost, target: "/convert?input=json&output=csv", body: payload}); rec.Code != 200 {
			b.Fatal(rec.Code)
		}
	}
}
