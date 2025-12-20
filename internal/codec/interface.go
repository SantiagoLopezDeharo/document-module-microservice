package codec

import "io"

type Codec interface {
	Decode(r io.Reader) (interface{}, error)
	Encode(w io.Writer, data interface{}) error
}
