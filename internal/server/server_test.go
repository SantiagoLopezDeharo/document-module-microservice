package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewSetsTimeouts(t *testing.T) {
	srv := New(":1", http.NotFoundHandler())
	if srv.Addr != ":1" || srv.ReadHeaderTimeout <= 0 || srv.ReadTimeout <= 0 || srv.WriteTimeout <= 0 || srv.IdleTimeout <= 0 {
		t.Errorf("timeouts not set: %+v", srv)
	}
	if srv.ReadHeaderTimeout >= srv.ReadTimeout {
		t.Error("header timeout should be much shorter than the body timeout")
	}
	if srv.MaxHeaderBytes <= 0 || srv.MaxHeaderBytes > 1<<20 {
		t.Errorf("MaxHeaderBytes = %d", srv.MaxHeaderBytes)
	}
}

func TestPprof(t *testing.T) {
	ts := httptest.NewServer(Pprof("127.0.0.1:0").Handler)
	defer ts.Close()
	for path, want := range map[string]int{
		"/debug/pprof/":                  http.StatusOK,
		"/debug/pprof/goroutine?debug=1": http.StatusOK,
		"/debug/pprof/heap?debug=1":      http.StatusOK,
		"/debug/pprof/cmdline":           http.StatusOK,
		"/debug/pprof/symbol":            http.StatusOK,
		"/health":                        http.StatusNotFound,
	} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("%s: %d, want %d", path, resp.StatusCode, want)
		}
	}
}
