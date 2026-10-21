package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

func CollectHeaders(rows []map[string]interface{}) []string {
	keySet := make(map[string]bool)
	for _, row := range rows {
		FlattenKeys(row, "", keySet)
	}

	headers := make([]string, 0, len(keySet))
	for k := range keySet {
		headers = append(headers, k)
	}
	sort.Strings(headers)
	return headers
}

func FlattenKeys(data map[string]interface{}, prefix string, keySet map[string]bool) {
	for k, v := range data {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}

		if nestedMap, ok := v.(map[string]interface{}); ok {
			FlattenKeys(nestedMap, fullKey, keySet)
		} else {
			keySet[fullKey] = true
		}
	}
}

// GetValue resolves a header produced by CollectHeaders. A key may itself
// contain dots, so a literal match wins over a split into nested objects.
func GetValue(row map[string]interface{}, key string) (string, bool) {
	value, ok := lookup(row, key)
	if !ok {
		return "", false
	}
	return formatValue(value), true
}

func lookup(data map[string]interface{}, key string) (interface{}, bool) {
	if value, ok := data[key]; ok {
		return value, true
	}
	for i := 0; i < len(key); i++ {
		if key[i] != '.' {
			continue
		}
		if child, ok := data[key[:i]].(map[string]interface{}); ok {
			if value, found := lookup(child, key[i+1:]); found {
				return value, true
			}
		}
	}
	return nil, false
}

func formatValue(value interface{}) string {
	switch value.(type) {
	case nil:
		return ""
	case []interface{}, []map[string]interface{}, map[string]interface{}:
		if encoded, err := marshalJSON(value); err == nil {
			return encoded
		}
	}
	return fmt.Sprintf("%v", value)
}

func marshalJSON(value interface{}) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimSuffix(buf.String(), "\n"), nil
}
