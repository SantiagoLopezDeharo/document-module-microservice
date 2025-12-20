package codec

import (
	"io"

	"github.com/vmihailenco/msgpack/v5"
)

type msgpackCodec struct{}

func (c *msgpackCodec) Decode(r io.Reader) (interface{}, error) {
	var data interface{}
	if err := msgpack.NewDecoder(r).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *msgpackCodec) Encode(w io.Writer, data interface{}) error {
	return msgpack.NewEncoder(w).Encode(data)
}
