package codec

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/magiconair/properties"
)

const (
	propertiesEmptyMapSentinel  = "__properties_empty_map__"
	propertiesEmptyListSentinel = "__properties_empty_list__"
)

type propertiesCodec struct{}

func (c *propertiesCodec) Decode(buf []byte) (interface{}, error) {
	// Expansion stays disabled: ${...} is data to convert, and expanding it made
	// a circular reference call log.Fatal and terminate the server.
	loader := properties.Loader{Encoding: properties.UTF8, DisableExpansion: true}
	p, err := loader.LoadBytes(buf)
	if err != nil {
		return nil, err
	}

	flat := p.Map()
	for key := range flat {
		if err := checkKeyDepth(key); err != nil {
			return nil, err
		}
	}
	// Convert to map[string]interface{}
	// We unflatten keys with dots to support nested structures
	return unflattenProperties(flat), nil
}

func unflattenProperties(flat map[string]string) map[string]interface{} {
	root := make(map[string]interface{})
	for k, v := range flat {
		// Convert "data[0].x" -> "data.0.x"
		// We replace [ with . and remove ]
		normalizedKey := strings.ReplaceAll(k, "[", ".")
		normalizedKey = strings.ReplaceAll(normalizedKey, "]", "")

		parts := strings.Split(normalizedKey, ".")

		current := root
		for i := 0; i < len(parts)-1; i++ {
			part := parts[i]
			if _, exists := current[part]; !exists {
				current[part] = make(map[string]interface{})
			}

			if nextMap, ok := current[part].(map[string]interface{}); ok {
				current = nextMap
			} else {
				// Conflict: trying to treat a value as a map
				// This can happen if keys are ambiguous, e.g. "a" and "a.b"
				// We'll overwrite the value with a map to support the deeper key
				// But this means we lose the leaf value "a".
				// For this exercise, we assume consistent input.
				newMap := make(map[string]interface{})
				current[part] = newMap
				current = newMap
			}
		}

		last := parts[len(parts)-1]
		// Try to convert value to appropriate type (int, float, bool)
		// Properties are strings, but we want typed values if possible for the matrix test
		current[last] = parseValue(v)
	}

	processed := restorePropertySentinels(convertMapsToSlices(root))
	if m, ok := processed.(map[string]interface{}); ok {
		return m
	}
	// Should not happen for root unless root itself is a slice (which properties doesn't support well)
	return root
}

func parseValue(v string) interface{} {
	if i, err := strconv.Atoi(v); err == nil {
		return float64(i) // Use float64 to match JSON/YAML default decoding for numbers
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	return v
}

func convertMapsToSlices(v interface{}) interface{} {
	m, ok := v.(map[string]interface{})
	if !ok {
		return v
	}

	// Recursively convert children first
	for k, val := range m {
		m[k] = convertMapsToSlices(val)
	}

	// Check if keys are all integers 0..N-1
	if len(m) == 0 {
		return m
	}

	type idxValue struct {
		idx int
		val interface{}
	}

	entries := make([]idxValue, 0, len(m))
	for k, val := range m {
		idx, err := strconv.Atoi(k)
		if err != nil || idx < 0 {
			return m
		}
		entries = append(entries, idxValue{idx: idx, val: val})
	}

	if len(entries) == 0 {
		return m
	}

	maxIdx := entries[0].idx
	for _, entry := range entries[1:] {
		if entry.idx > maxIdx {
			maxIdx = entry.idx
		}
	}

	// Require contiguous indexes starting at zero
	if maxIdx != len(entries)-1 {
		return m
	}

	slice := make([]interface{}, len(entries))
	for _, entry := range entries {
		if entry.idx >= len(slice) || slice[entry.idx] != nil {
			return m
		}
		slice[entry.idx] = entry.val
	}

	for _, item := range slice {
		if item == nil {
			return m
		}
	}

	return slice
}

func (c *propertiesCodec) Encode(w io.Writer, data interface{}) error {
	// Properties format is essentially flat Key=Value.
	// We need to flatten the input data if it's nested.

	m, ok := data.(map[string]interface{})
	if !ok {
		// If it's a list, we can't really represent it well in .properties without index keys
		// e.g. item.0.key=value
		// Let's try to flatten everything.
		return fmt.Errorf("properties format requires a map object, got %T", data)
	}

	flat := make(map[string]string)
	flattenToProperties(m, "", flat)

	// Sort keys for deterministic output
	keys := make([]string, 0, len(flat))
	for k := range flat {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if _, err := fmt.Fprintf(w, "%s=%s\n", k, flat[k]); err != nil {
			return err
		}
	}
	return nil
}

func flattenToProperties(data map[string]interface{}, prefix string, result map[string]string) {
	for k, v := range data {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			if len(val) == 0 {
				result[fullKey] = escapePropertyValue(propertiesEmptyMapSentinel)
				continue
			}
			flattenToProperties(val, fullKey, result)
		case []interface{}:
			if len(val) == 0 {
				result[fullKey] = escapePropertyValue(propertiesEmptyListSentinel)
				continue
			}
			for i, item := range val {
				itemKey := fmt.Sprintf("%s[%d]", fullKey, i)
				if subMap, ok := item.(map[string]interface{}); ok {
					flattenToProperties(subMap, itemKey, result)
				} else {
					result[itemKey] = escapePropertyValue(fmt.Sprintf("%v", item))
				}
			}
		default:
			result[fullKey] = escapePropertyValue(fmt.Sprintf("%v", val))
		}
	}
}

var propertyValueEscaper = strings.NewReplacer(
	"\\", "\\\\",
	"\n", "\\n",
	"\r", "\\r",
	"\t", "\\t",
	"\f", "\\f",
	"=", "\\=",
	":", "\\:",
	"#", "\\#",
	"!", "\\!",
)

func escapePropertyValue(val string) string {
	escaped := propertyValueEscaper.Replace(val)
	if escaped == "" {
		return escaped
	}
	first := escaped[0]
	if first == ' ' || first == '\t' {
		escaped = "\\" + escaped
	}
	return escaped
}

func restorePropertySentinels(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			val[k] = restorePropertySentinels(child)
		}
		return val
	case []interface{}:
		for i, child := range val {
			val[i] = restorePropertySentinels(child)
		}
		return val
	case string:
		switch val {
		case propertiesEmptyMapSentinel:
			return map[string]interface{}{}
		case propertiesEmptyListSentinel:
			return []interface{}{}
		}
		return val
	default:
		return v
	}
}
