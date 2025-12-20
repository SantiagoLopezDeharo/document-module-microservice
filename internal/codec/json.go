package codec

import (
	"encoding/json"
	"io"
)

type jsonCodec struct{}

func (c *jsonCodec) Decode(r io.Reader) (interface{}, error) {
	var data interface{}
	if err := json.NewDecoder(r).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *jsonCodec) Encode(w io.Writer, data interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}
