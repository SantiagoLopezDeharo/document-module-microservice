package codec

import (
	"io"

	"github.com/vmihailenco/msgpack/v5"
)

type msgpackCodec struct{}

func (c *msgpackCodec) Decode(r io.Reader) (interface{}, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err := checkMsgpackDepth(b); err != nil {
		return nil, err
	}
	var data interface{}
	if err := msgpack.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *msgpackCodec) Encode(w io.Writer, data interface{}) error {
	return msgpack.NewEncoder(w).Encode(data)
}
