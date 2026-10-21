package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/codec"
	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/document"
)

func writeBody(w http.ResponseWriter, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if _, err := w.Write(body); err != nil {
		slog.Debug("write response", "error", err)
	}
}

// clientError reports a problem with the request: 413 when it exceeds a
// resource limit, 400 otherwise.
func clientError(w http.ResponseWriter, prefix string, err error) {
	status := http.StatusBadRequest
	if isLimit(err) {
		status = http.StatusRequestEntityTooLarge
	}
	http.Error(w, prefix+": "+err.Error(), status)
}

// renderError hides failures that the request did not cause.
func renderError(w http.ResponseWriter, err error) {
	if errors.Is(err, document.ErrInvalid) || isLimit(err) {
		clientError(w, "Invalid document", err)
		return
	}
	slog.Error("document rendering failed", "error", err)
	http.Error(w, "Failed to generate document", http.StatusInternalServerError)
}

func isLimit(err error) bool {
	_, tooLarge := errors.AsType[*http.MaxBytesError](err)
	return tooLarge || errors.Is(err, codec.ErrTooDeep) || errors.Is(err, document.ErrLimit)
}
