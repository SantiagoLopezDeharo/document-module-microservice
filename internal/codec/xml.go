package codec

import (
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"strings"
	"unicode"

	"github.com/clbanning/mxj/v2"
)

type xmlCodec struct{}

var xmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")

// prepareXML validates element and attribute names and escapes string values
// for mxj, which writes both verbatim: unescaped values produced malformed
// XML for "&" and let "</a><admin>true</admin>" inject markup. Containers are
// copied only along the paths to values that need escaping, so the caller's
// data is never modified and clean data costs no allocations. "-name" keys
// are attributes and "#text" is element text, mirroring how mxj decodes XML.
func prepareXML(v interface{}) (interface{}, bool, error) {
	switch val := v.(type) {
	case string:
		if strings.ContainsAny(val, `&<>"'`) {
			return xmlEscaper.Replace(val), true, nil
		}
	case map[string]interface{}:
		return prepareXMLMap(val)
	case mxj.Map:
		return prepareXMLMap(val)
	case []interface{}:
		return prepareXMLList(val, func(i int) interface{} { return val[i] })
	case []map[string]interface{}:
		return prepareXMLList(val, func(i int) interface{} { return val[i] })
	}
	return v, false, nil
}

func prepareXMLList[T any](list []T, item func(int) interface{}) (interface{}, bool, error) {
	var out []interface{}
	for i := range list {
		escaped, changed, err := prepareXML(item(i))
		if err != nil {
			return nil, false, err
		}
		if !changed {
			continue
		}
		if out == nil {
			out = make([]interface{}, len(list))
			for j := range list {
				out[j] = item(j)
			}
		}
		out[i] = escaped
	}
	if out != nil {
		return out, true, nil
	}
	return list, false, nil
}

func prepareXMLMap(m map[string]interface{}) (interface{}, bool, error) {
	var out map[string]interface{}
	for key, child := range m {
		if key != "#text" && !isXMLName(strings.TrimPrefix(key, "-")) {
			return nil, false, fmt.Errorf("key %q is not a valid XML name", key)
		}
		escaped, changed, err := prepareXML(child)
		if err != nil {
			return nil, false, err
		}
		if changed {
			if out == nil {
				out = maps.Clone(m)
			}
			out[key] = escaped
		}
	}
	if out != nil {
		return out, true, nil
	}
	return m, false, nil
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

func (c *xmlCodec) Decode(b []byte) (interface{}, error) {
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
	data, _, err := prepareXML(data)
	if err != nil {
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
