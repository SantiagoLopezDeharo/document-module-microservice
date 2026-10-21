package api

import (
	"net/http"
	"testing"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/config"
)

func testConfig(t testing.TB, overrides map[string]string) *config.Config {
	t.Helper()
	cfg, err := config.Load(func(key string) string { return overrides[key] })
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func testRouter(t testing.TB) http.Handler {
	return NewRouter(testConfig(t, nil))
}
