//go:build e2e

package e2e

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

type response struct {
	status  int
	header  http.Header
	body    []byte
	latency time.Duration
}

func (r response) text() string { return string(r.body) }

func (s *server) do(t testing.TB, method, path string, header http.Header, body []byte) response {
	t.Helper()
	res, err := s.try(method, path, header, body)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return res
}

func (s *server) try(method, path string, header http.Header, body []byte) (response, error) {
	req, err := http.NewRequest(method, s.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return response{}, err
	}
	for k, v := range header {
		req.Header[k] = v
	}
	start := time.Now()
	resp, err := httpClient.Do(req)
	if err != nil {
		return response{}, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return response{resp.StatusCode, resp.Header, data, time.Since(start)}, err
}

func (s *server) convert(t testing.TB, input, output string, body []byte) response {
	t.Helper()
	return s.do(t, http.MethodPost, "/convert?"+url.Values{"input": {input}, "output": {output}}.Encode(), nil, body)
}

func (s *server) generate(t testing.TB, kind string, body string) response {
	t.Helper()
	return s.do(t, http.MethodPost, "/convert/"+kind, http.Header{"Content-Type": {"application/json"}}, []byte(body))
}

func (s *server) mustConvert(t testing.TB, input, output string, body []byte) []byte {
	t.Helper()
	res := s.convert(t, input, output, body)
	if res.status != http.StatusOK {
		t.Fatalf("convert %s->%s: %d %s", input, output, res.status, res.body)
	}
	return res.body
}

func (s *server) healthy(t testing.TB) {
	t.Helper()
	if res := s.do(t, http.MethodGet, "/health", nil, nil); res.status != http.StatusOK {
		t.Fatalf("health: %d", res.status)
	}
}
