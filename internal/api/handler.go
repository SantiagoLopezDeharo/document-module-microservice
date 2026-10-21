package api

import (
	"bytes"
	"net/http"
	"strings"
	"sync"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/codec"
	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/models"
	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/utils"
)

func convertHandler(w http.ResponseWriter, r *http.Request, body []byte) {
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

	data, err := decoder.Decode(body)
	if err != nil {
		clientError(w, "Decode error", err)
		return
	}

	out := getBuffer(len(body))
	defer putBuffer(out)
	if err := encoder.Encode(out, data); err != nil {
		http.Error(w, "Encode error: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}
	writeBody(w, utils.GetContentType(outputFormat), out.Bytes())
}

// maxPooledBuffer keeps a few large conversions from pinning memory in the
// pool; bigger buffers are left to the garbage collector.
const maxPooledBuffer = 4 << 20

var buffers = sync.Pool{New: func() any { return new(bytes.Buffer) }}

// getBuffer returns an empty buffer that can hold at least sizeHint bytes,
// since output is usually about as large as the input.
func getBuffer(sizeHint int) *bytes.Buffer {
	buf := buffers.Get().(*bytes.Buffer)
	buf.Reset()
	buf.Grow(sizeHint)
	return buf
}

func putBuffer(buf *bytes.Buffer) {
	if buf.Cap() <= maxPooledBuffer {
		buffers.Put(buf)
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
