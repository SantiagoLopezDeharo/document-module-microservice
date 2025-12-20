package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/codec"
	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/utils"
)

func convertHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	inputFormat := parseInputFormat(r)
	outputFormat := parseOutputFormat(r)

	if inputFormat == "" || outputFormat == "" {
		http.Error(w, "Input or Output format not specified", http.StatusBadRequest)
		return
	}

	decoder, err := codec.GetCodec(inputFormat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	encoder, err := codec.GetCodec(outputFormat)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", utils.GetContentType(outputFormat))

	data, err := decoder.Decode(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Decode error: %v", err), http.StatusBadRequest)
		return
	}

	if err := encoder.Encode(w, data); err != nil {
		fmt.Printf("Encode error: %v\n", err)
	}
}

func parseInputFormat(r *http.Request) models.Format {
	if v := r.URL.Query().Get("input"); v != "" {
		return models.Format(strings.ToUpper(v))
	}
	ct := r.Header.Get("Content-Type")
	return utils.MimeToFormat(ct)
}

func parseOutputFormat(r *http.Request) models.Format {
	if v := r.URL.Query().Get("output"); v != "" {
		return models.Format(strings.ToUpper(v))
	}
	accept := r.Header.Get("Accept")
	return utils.MimeToFormat(accept)
}
