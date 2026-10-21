package codec

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
)

type jsonCodec struct{}

var errTrailingJSON = errors.New("unexpected data after the JSON document")

func (c *jsonCodec) Decode(r io.Reader) (interface{}, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	var data interface{}
	if err := dec.Decode(&data); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errTrailingJSON
	}
	return normalizeNumbers(data), nil
}

func (c *jsonCodec) Encode(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// normalizeNumbers keeps numbers as float64, like the default decoder, except
// integers that float64 cannot represent exactly, which become int64 or uint64
// so identifiers such as 9007199254740993 survive conversion.
func normalizeNumbers(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			val[k] = normalizeNumbers(child)
		}
	case []interface{}:
		for i, child := range val {
			val[i] = normalizeNumbers(child)
		}
	case json.Number:
		return jsonNumber(string(val))
	}
	return v
}

const maxExactFloatInt = 1 << 53

func jsonNumber(s string) interface{} {
	if !strings.ContainsAny(s, ".eE") {
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			if i > maxExactFloatInt || i < -maxExactFloatInt {
				return i
			}
			return float64(i)
		}
		if u, err := strconv.ParseUint(s, 10, 64); err == nil {
			return u
		}
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
