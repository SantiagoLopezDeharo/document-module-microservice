package codec

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type jsonCodec struct{}

var errTrailingJSON = errors.New("unexpected data after the JSON document")

func (c *jsonCodec) Decode(data []byte) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var value interface{}
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errTrailingJSON
	}
	return normalizeNumbers(value)
}

// jsonEncodeOptions reproduce encoding/json's indented Encoder output. The v1
// Encoder marshals compactly and then reformats into a second buffer; v2
// indents while writing.
var jsonEncodeOptions = jsonv2.JoinOptions(
	json.DefaultOptionsV1(),
	jsontext.WithIndent("  "),
	jsontext.EscapeForHTML(false),
)

func (c *jsonCodec) Encode(w io.Writer, data interface{}) error {
	if err := jsonv2.MarshalWrite(w, data, jsonEncodeOptions); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\n")
	return err
}

// normalizeNumbers turns integer literals into int64 (uint64 above the int64
// range) so they stay integers in formats that distinguish them, such as TOML
// and MsgPack, and identifiers such as 9007199254740993 survive conversion.
// Other numbers become float64. Numbers outside the float64 range are
// rejected, as encoding/json does.
func normalizeNumbers(v interface{}) (interface{}, error) {
	var err error
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			if val[k], err = normalizeNumbers(child); err != nil {
				return nil, err
			}
		}
	case []interface{}:
		for i, child := range val {
			if val[i], err = normalizeNumbers(child); err != nil {
				return nil, err
			}
		}
	case json.Number:
		return jsonNumber(string(val))
	}
	return v, nil
}

func jsonNumber(s string) (interface{}, error) {
	if !strings.ContainsAny(s, ".eE") {
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return i, nil
		}
		if u, err := strconv.ParseUint(s, 10, 64); err == nil {
			return u, nil
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("number %s is out of range", s)
	}
	return f, nil
}
