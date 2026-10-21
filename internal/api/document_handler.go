package api

import (
	"encoding/json"
	"net/http"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/document"
)

const (
	contentTypePDF  = "application/pdf"
	contentTypeDOCX = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	contentTypeXLSX = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
)

type renderer[T any] func(T, document.Limits) ([]byte, error)

func documentHandler[T any](render renderer[T], limits document.Limits, contentType, filename string) bodyHandler {
	return func(w http.ResponseWriter, r *http.Request, body []byte) {
		var req T
		if err := json.Unmarshal(body, &req); err != nil {
			clientError(w, "Invalid JSON", err)
			return
		}

		out, err := render(req, limits)
		if err != nil {
			renderError(w, err)
			return
		}

		w.Header().Set("Content-Disposition", "attachment; filename="+filename)
		writeBody(w, contentType, out)
	}
}
