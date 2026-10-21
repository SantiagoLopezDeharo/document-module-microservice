package codec

import (
	"bytes"
	"io"

	"gopkg.in/yaml.v3"
)

type yamlCodec struct{}

func (c *yamlCodec) Decode(data []byte) (interface{}, error) {
	var value interface{}
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func (c *yamlCodec) Encode(w io.Writer, data interface{}) error {
	if ok, err := emitPlainYAML(w, data); ok {
		return err
	}
	return encodeYAMLV3(w, data)
}

func encodeYAMLV3(w io.Writer, data interface{}) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(data)
}
