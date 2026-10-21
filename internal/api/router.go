package api

import (
	"net/http"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/config"
	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/document"
)

func NewRouter(cfg *config.Config) http.Handler {
	limits := document.Limits{MaxActions: cfg.MaxActions, MaxXLSXCells: cfg.MaxXLSXCells}
	guard := guarded(cfg.MaxBodyBytes, cfg.MaxConcurrentRequests)

	mux := http.NewServeMux()
	mux.Handle("POST /convert", guard(http.HandlerFunc(convertHandler)))
	mux.Handle("POST /convert/pdf", guard(documentHandler(document.RenderPDF, limits, contentTypePDF, "output.pdf")))
	mux.Handle("POST /convert/docx", guard(documentHandler(document.RenderDOCX, limits, contentTypeDOCX, "output.docx")))
	mux.Handle("POST /convert/xlsx", guard(documentHandler(document.RenderXLSX, limits, contentTypeXLSX, "output.xlsx")))
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	return mux
}

// guarded caps request bodies and the number of requests processed at once;
// conversions are CPU and memory bound, so excess requests wait their turn.
func guarded(maxBodyBytes int64, maxConcurrent int) func(http.Handler) http.Handler {
	slots := make(chan struct{}, maxConcurrent)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			case <-r.Context().Done():
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			next.ServeHTTP(w, r)
		})
	}
}
