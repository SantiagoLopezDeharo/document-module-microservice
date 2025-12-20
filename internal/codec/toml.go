package codec

import (
	"io"

	"github.com/pelletier/go-toml/v2"
)

type tomlCodec struct{}

func (c *tomlCodec) Decode(r io.Reader) (interface{}, error) {
	var data interface{}
	if err := toml.NewDecoder(r).Decode(&data); err != nil {
		return nil, err
	}

	// Unwrap "root" if it was added by Encode
	if m, ok := data.(map[string]interface{}); ok {
		if len(m) == 1 {
			if root, ok := m["root"]; ok {
				if _, isSlice := root.([]interface{}); isSlice {
					return root, nil
				}
				if _, isMapSlice := root.([]map[string]interface{}); isMapSlice {
					return root, nil
				}
			}
		}
	}

	return data, nil
}

func (c *tomlCodec) Encode(w io.Writer, data interface{}) error {
	// TOML does not support root-level arrays. We must wrap them.
	switch v := data.(type) {
	case []interface{}:
		data = map[string]interface{}{"root": v}
	case []map[string]interface{}:
		data = map[string]interface{}{"root": v}
	}
	return toml.NewEncoder(w).Encode(data)
}
