package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConvertHandler(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		queryInput     string
		queryOutput    string
		contentType    string
		accept         string
		body           string
		expectedStatus int
	}{
		{
			name:           "json to xml via query params",
			method:         "POST",
			queryInput:     "json",
			queryOutput:    "xml",
			body:           `{"a": 1, "b": 2}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "csv to json via query params",
			method:         "POST",
			queryInput:     "csv",
			queryOutput:    "json",
			body:           "a,b\n1,2\n3,4\n",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "json to toml via headers",
			method:         "POST",
			contentType:    "application/json",
			accept:         "application/toml",
			body:           `{"key": "value"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "tsv to xml",
			method:         "POST",
			queryInput:     "tsv",
			queryOutput:    "xml",
			body:           "name\tage\nAlice\t30\nBob\t25\n",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "method not allowed",
			method:         "GET",
			queryInput:     "json",
			queryOutput:    "xml",
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:           "missing input format",
			method:         "POST",
			queryOutput:    "json",
			body:           `{}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "missing output format",
			method:         "POST",
			queryInput:     "json",
			body:           `{}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid input format",
			method:         "POST",
			queryInput:     "invalid",
			queryOutput:    "json",
			body:           `{}`,
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/convert"
			if tt.queryInput != "" || tt.queryOutput != "" {
				url += "?"
				if tt.queryInput != "" {
					url += "input=" + tt.queryInput
				}
				if tt.queryOutput != "" {
					if tt.queryInput != "" {
						url += "&"
					}
					url += "output=" + tt.queryOutput
				}
			}

			req := httptest.NewRequest(tt.method, url, bytes.NewBufferString(tt.body))
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}

			w := httptest.NewRecorder()
			convertHandler(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}
		})
	}
}

func TestHealthEndpoint(t *testing.T) {
	router := NewRouter()
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Body.String() != "OK" {
		t.Errorf("Expected body 'OK', got '%s'", w.Body.String())
	}
}

func TestComplexConversion(t *testing.T) {
	jsonInput := `[
		{"id": 1, "name": "Alice", "score": 95.5},
		{"id": 2, "name": "Bob", "score": 87.3}
	]`

	req := httptest.NewRequest("POST", "/convert?input=json&output=csv", bytes.NewBufferString(jsonInput))
	w := httptest.NewRecorder()
	convertHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	output := w.Body.String()
	if output == "" {
		t.Error("Expected non-empty CSV output")
	}
}
