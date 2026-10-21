package codec

import (
	"io"

	"github.com/vmihailenco/msgpack/v5"
)

type msgpackCodec struct{}

func (c *msgpackCodec) Decode(data []byte) (interface{}, error) {
	if err := checkMsgpack(data); err != nil {
		return nil, err
	}
	var value interface{}
	if err := msgpack.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func (c *msgpackCodec) Encode(w io.Writer, data interface{}) error {
	return msgpack.NewEncoder(w).Encode(data)
}
