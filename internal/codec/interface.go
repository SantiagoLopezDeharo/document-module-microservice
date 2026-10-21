package codec

import "io"

type Codec interface {
	Decode(data []byte) (interface{}, error)
	Encode(w io.Writer, data interface{}) error
}
