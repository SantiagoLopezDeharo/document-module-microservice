package api

import (
	"bytes"
	"net/http"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/config"
	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/document"
)

func NewRouter(cfg *config.Config) http.Handler {
	limits := document.Limits{MaxActions: cfg.MaxActions, MaxXLSXCells: cfg.MaxXLSXCells}
	guard := guarded(cfg.MaxBodyBytes, cfg.MaxConcurrentRequests)

	mux := http.NewServeMux()
	mux.Handle("POST /convert", guard(convertHandler))
	mux.Handle("POST /convert/pdf", guard(documentHandler(document.RenderPDF, limits, contentTypePDF, "output.pdf")))
	mux.Handle("POST /convert/docx", guard(documentHandler(document.RenderDOCX, limits, contentTypeDOCX, "output.docx")))
	mux.Handle("POST /convert/xlsx", guard(documentHandler(document.RenderXLSX, limits, contentTypeXLSX, "output.xlsx")))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	return mux
}

// bodyHandler receives the request body already read and size-checked.
type bodyHandler func(w http.ResponseWriter, r *http.Request, body []byte)

// guarded caps request bodies and the number of requests processed at once.
// Bodies are received before taking a slot, so clients that upload slowly
// cannot occupy the slots that bound CPU and memory spent on conversions.
func guarded(maxBodyBytes int64, maxConcurrent int) func(bodyHandler) http.Handler {
	slots := make(chan struct{}, maxConcurrent)
	return func(next bodyHandler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := readBody(w, r, maxBodyBytes)
			if err != nil {
				clientError(w, "Error reading body", err)
				return
			}
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-r.Context().Done():
				return
			}
			next(w, r, body)
		})
	}
}

// readBody sizes its buffer from Content-Length so large uploads are read
// into a single allocation instead of a chain of doubling copies.
func readBody(w http.ResponseWriter, r *http.Request, maxBodyBytes int64) ([]byte, error) {
	var buf bytes.Buffer
	if r.ContentLength > 0 && r.ContentLength <= maxBodyBytes {
		buf.Grow(int(r.ContentLength) + bytes.MinRead)
	}
	_, err := buf.ReadFrom(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	return buf.Bytes(), err
}
