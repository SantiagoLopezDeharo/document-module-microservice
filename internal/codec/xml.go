package codec

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/clbanning/mxj/v2"
)

type xmlCodec struct{}

// mxj writes values verbatim unless told otherwise, which produced malformed
// XML for "&" and let values such as "</a><admin>true</admin>" inject markup.
func init() {
	mxj.XMLEscapeChars(true)
}

// validateXMLNames rejects keys that cannot be element or attribute names,
// since mxj would emit them verbatim. "-name" keys are attributes and "#text"
// is element text, mirroring how mxj decodes XML.
func validateXMLNames(v interface{}) error {
	switch val := v.(type) {
	case map[string]interface{}:
		for key, child := range val {
			if key != "#text" && !isXMLName(strings.TrimPrefix(key, "-")) {
				return fmt.Errorf("key %q is not a valid XML name", key)
			}
			if err := validateXMLNames(child); err != nil {
				return err
			}
		}
	case mxj.Map:
		return validateXMLNames(map[string]interface{}(val))
	case []interface{}:
		for _, child := range val {
			if err := validateXMLNames(child); err != nil {
				return err
			}
		}
	case []map[string]interface{}:
		for _, child := range val {
			if err := validateXMLNames(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func isXMLName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_' || r == ':' || unicode.IsLetter(r):
		case i > 0 && (r == '-' || r == '.' || unicode.IsDigit(r)):
		default:
			return false
		}
	}
	return true
}

func (c *xmlCodec) Decode(r io.Reader) (interface{}, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if err := checkXMLDepth(b); err != nil {
		return nil, err
	}
	mv, err := mxj.NewMapXml(b)
	if err != nil {
		return nil, err
	}

	// Unwrap logic:
	// 1. If root has 1 key (e.g. "doc" or "root")
	// 2. And that key contains "List" which is a slice -> Return slice
	// 3. Or if that key contains the actual object data -> Return inner map
	if len(mv) == 1 {
		for _, v := range mv {
			if innerMap, ok := v.(map[string]interface{}); ok {
				// Check for List wrapper
				if list, ok := innerMap["List"]; ok {
					if slice, ok := list.([]interface{}); ok {
						return slice, nil
					}
				}
				// Check if we should unwrap the root (e.g. "doc" wrapping a simple object)
				// This is ambiguous. If I send {"a":1}, XML is <doc><a>1</a></doc>.
				// mxj gives {"doc": {"a": 1}}.
				// I want {"a": 1}.
				// But if I sent {"doc": {"a": 1}}, XML is <doc><doc><a>1</a></doc></doc> (if wrapped again)
				// or <doc><a>1</a></doc> if "doc" was the root.
				// Let's assume if the root key is "doc" or "root", we unwrap it.
				return innerMap, nil
			}
		}
	}

	return map[string]interface{}(mv), nil
}

func (c *xmlCodec) Encode(w io.Writer, data interface{}) error {
	if err := validateXMLNames(data); err != nil {
		return err
	}
	if m, ok := data.(map[string]interface{}); ok {
		// If map has multiple keys, wrap it in "doc" to ensure valid XML root
		if len(m) != 1 {
			wrapped := map[string]interface{}{
				"doc": m,
			}
			return mxj.Map(wrapped).XmlIndentWriter(w, "", "  ")
		}
		return mxj.Map(m).XmlIndentWriter(w, "", "  ")
	}
	if s, ok := data.([]map[string]interface{}); ok {
		// Convert to []interface{} for mxj
		list := make([]interface{}, len(s))
		for i, v := range s {
			list[i] = v
		}
		wrapped := map[string]interface{}{
			"List": list,
		}
		return mxj.Map(wrapped).XmlIndentWriter(w, "", "  ")
	}
	if s, ok := data.([]interface{}); ok {
		wrapped := map[string]interface{}{
			"List": s,
		}
		return mxj.Map(wrapped).XmlIndentWriter(w, "", "  ")
	}
	return xml.NewEncoder(w).Encode(data)
}
