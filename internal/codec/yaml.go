package codec

import (
	"io"

	"gopkg.in/yaml.v3"
)

type yamlCodec struct{}

func (c *yamlCodec) Decode(r io.Reader) (interface{}, error) {
	var data interface{}
	if err := yaml.NewDecoder(r).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *yamlCodec) Encode(w io.Writer, data interface{}) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(data)
}
