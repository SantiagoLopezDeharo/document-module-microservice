package config

import (
	"runtime"
	"strings"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(nil))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Config{
		Address:               DefaultAddress,
		MaxBodyBytes:          DefaultMaxBodyBytes,
		MaxActions:            DefaultMaxActions,
		MaxXLSXCells:          DefaultMaxXLSXCells,
		MaxConcurrentRequests: 2 * runtime.GOMAXPROCS(0),
	}
	if *cfg != want {
		t.Errorf("Load() = %+v, want %+v", *cfg, want)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"SERVER_ADDRESS":          "127.0.0.1:9000",
		"MAX_BODY_BYTES":          " 1024 ",
		"MAX_ACTIONS":             "5",
		"MAX_XLSX_CELLS":          "100",
		"MAX_CONCURRENT_REQUESTS": "3",
		"PPROF_ADDR":              "127.0.0.1:6060",
	}))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := Config{
		Address:               "127.0.0.1:9000",
		MaxBodyBytes:          1024,
		MaxActions:            5,
		MaxXLSXCells:          100,
		MaxConcurrentRequests: 3,
		PprofAddr:             "127.0.0.1:6060",
	}
	if *cfg != want {
		t.Errorf("Load() = %+v, want %+v", *cfg, want)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	keys := []string{"MAX_BODY_BYTES", "MAX_ACTIONS", "MAX_XLSX_CELLS", "MAX_CONCURRENT_REQUESTS"}
	values := map[string]string{
		"0":                    "must be >= 1",
		"-1":                   "must be >= 1",
		"ten":                  "must be an integer",
		"1.5":                  "must be an integer",
		"1e3":                  "must be an integer",
		"99999999999999999999": "must be an integer",
	}
	for _, key := range keys {
		for value, want := range values {
			cfg, err := Load(env(map[string]string{key: value}))
			if err == nil || cfg != nil {
				t.Errorf("%s=%s accepted: %+v", key, value, cfg)
				continue
			}
			if !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), want) {
				t.Errorf("%s=%s: error %q should mention %q", key, value, err, want)
			}
		}
	}
}

func TestLoadReportsEveryProblem(t *testing.T) {
	_, err := Load(env(map[string]string{
		"MAX_BODY_BYTES":          "0",
		"MAX_ACTIONS":             "x",
		"MAX_XLSX_CELLS":          "-5",
		"MAX_CONCURRENT_REQUESTS": "none",
	}))
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, key := range []string{"MAX_BODY_BYTES", "MAX_ACTIONS", "MAX_XLSX_CELLS", "MAX_CONCURRENT_REQUESTS"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error %q does not mention %s", err, key)
		}
	}
}
