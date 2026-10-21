package api

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/codec"
	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/utils"
)

func convertHandler(w http.ResponseWriter, r *http.Request) {
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

	data, err := decoder.Decode(r.Body)
	if err != nil {
		clientError(w, "Decode error", err)
		return
	}

	var out bytes.Buffer
	if err := encoder.Encode(&out, data); err != nil {
		http.Error(w, "Encode error: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeBody(w, utils.GetContentType(outputFormat), out.Bytes())
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
