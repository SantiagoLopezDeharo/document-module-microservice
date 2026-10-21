package codec

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

// MaxDepth matches the nesting limit of encoding/json and yaml.v3. The XML,
// TOML and MessagePack decoders recurse without a limit, so a few megabytes of
// nesting would overflow the goroutine stack and crash the whole process.
const MaxDepth = 10_000

var ErrTooDeep = fmt.Errorf("document nesting exceeds %d levels", MaxDepth)

// checkXMLDepth stops at the first syntax error and lets the real decoder
// report it.
func checkXMLDepth(data []byte) error {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	depth := 0
	for {
		tok, err := dec.RawToken()
		if err != nil {
			return nil
		}
		switch tok.(type) {
		case xml.StartElement:
			if depth++; depth > MaxDepth {
				return ErrTooDeep
			}
		case xml.EndElement:
			depth--
		}
	}
}

// checkTOMLDepth over-approximates nesting as open arrays and inline tables
// plus the dotted segments of the key being read, skipping strings and
// comments. Malformed input is left for the real decoder to reject.
func checkTOMLDepth(data []byte) error {
	var open []byte
	inKey, keyDots := true, 0
	for i := 0; i < len(data); i++ {
		switch c := data[i]; c {
		case '#':
			if end := bytes.IndexByte(data[i:], '\n'); end >= 0 {
				i += end - 1
			} else {
				return nil
			}
		case '"', '\'':
			i = skipTOMLString(data, i)
		case '\n':
			if len(open) == 0 {
				inKey, keyDots = true, 0
			}
		case '=':
			inKey, keyDots = false, 0
		case '.':
			if inKey {
				keyDots++
			}
		case '[':
			if !(inKey && len(open) == 0) {
				open = append(open, c)
			}
		case '{':
			open = append(open, c)
			inKey, keyDots = true, 0
		case ',':
			if len(open) > 0 && open[len(open)-1] == '{' {
				inKey, keyDots = true, 0
			}
		case ']', '}':
			if len(open) > 0 {
				open = open[:len(open)-1]
				inKey = false
			}
		}
		if len(open)+keyDots > MaxDepth {
			return ErrTooDeep
		}
	}
	return nil
}

// skipTOMLString returns the index of the quote closing the string opened at i.
func skipTOMLString(data []byte, i int) int {
	quote := data[i]
	escapes := quote == '"'
	if bytes.HasPrefix(data[i:], []byte{quote, quote, quote}) {
		end := bytes.Index(data[i+3:], []byte{quote, quote, quote})
		if !escapes {
			if end < 0 {
				return len(data)
			}
			return i + 3 + end + 2
		}
		for j := i + 3; j < len(data); j++ {
			switch data[j] {
			case '\\':
				j++
			case quote:
				if bytes.HasPrefix(data[j:], []byte{quote, quote, quote}) {
					return j + 2
				}
			}
		}
		return len(data)
	}
	for j := i + 1; j < len(data); j++ {
		switch data[j] {
		case '\\':
			if escapes {
				j++
			}
		case quote, '\n':
			return j
		}
	}
	return len(data)
}

var errTruncated = errors.New("truncated")

// checkMsgpackDepth walks the encoding iteratively, tracking how many items
// each open array or map still expects.
func checkMsgpackDepth(data []byte) error {
	var pending []uint64
	for i := 0; i < len(data); {
		items, next, err := msgpackItem(data, i)
		if err != nil {
			return nil
		}
		i = next
		if items > 0 {
			if pending = append(pending, items); len(pending) > MaxDepth {
				return ErrTooDeep
			}
			continue
		}
		for len(pending) > 0 {
			pending[len(pending)-1]--
			if pending[len(pending)-1] > 0 {
				break
			}
			pending = pending[:len(pending)-1]
		}
	}
	return nil
}

// msgpackItem decodes the header at i and returns how many nested items it
// opens and the offset of the next header.
func msgpackItem(data []byte, i int) (items uint64, next int, err error) {
	c := data[i]
	i++
	length := func(width int) (uint64, error) {
		if i+width > len(data) {
			return 0, errTruncated
		}
		var v uint64
		switch width {
		case 1:
			v = uint64(data[i])
		case 2:
			v = uint64(binary.BigEndian.Uint16(data[i:]))
		case 4:
			v = uint64(binary.BigEndian.Uint32(data[i:]))
		}
		i += width
		return v, nil
	}
	skip := func(n uint64) (uint64, int, error) {
		if n > uint64(len(data)-i) {
			return 0, 0, errTruncated
		}
		return 0, i + int(n), nil
	}
	payload := func(width int, extra uint64) (uint64, int, error) {
		n, err := length(width)
		if err != nil {
			return 0, 0, err
		}
		return skip(n + extra)
	}
	container := func(width int, perItem uint64) (uint64, int, error) {
		n, err := length(width)
		return n * perItem, i, err
	}

	switch {
	case c <= 0x7f, c >= 0xe0, c == 0xc0, c == 0xc2, c == 0xc3:
		return 0, i, nil
	case c <= 0x8f:
		return 2 * uint64(c&0x0f), i, nil
	case c <= 0x9f:
		return uint64(c & 0x0f), i, nil
	case c <= 0xbf:
		return skip(uint64(c & 0x1f))
	}

	switch c {
	case 0xc4, 0xd9:
		return payload(1, 0)
	case 0xc5, 0xda:
		return payload(2, 0)
	case 0xc6, 0xdb:
		return payload(4, 0)
	case 0xc7:
		return payload(1, 1)
	case 0xc8:
		return payload(2, 1)
	case 0xc9:
		return payload(4, 1)
	case 0xcc, 0xd0:
		return skip(1)
	case 0xcd, 0xd1:
		return skip(2)
	case 0xca, 0xce, 0xd2:
		return skip(4)
	case 0xcb, 0xcf, 0xd3:
		return skip(8)
	case 0xd4, 0xd5, 0xd6, 0xd7, 0xd8:
		return skip(1 + 1<<(c-0xd4))
	case 0xdc:
		return container(2, 1)
	case 0xdd:
		return container(4, 1)
	case 0xde:
		return container(2, 2)
	case 0xdf:
		return container(4, 2)
	}
	return 0, 0, fmt.Errorf("invalid msgpack byte 0x%x", c)
}

func checkKeyDepth(key string) error {
	if strings.Count(key, ".")+strings.Count(key, "[") >= MaxDepth {
		return ErrTooDeep
	}
	return nil
}
