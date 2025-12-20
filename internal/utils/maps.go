package utils

import (
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

func GetValue(row map[string]interface{}, key string) (string, bool) {
	parts := strings.Split(key, ".")
	var current interface{} = row

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return "", false
		}

		val, exists := m[part]
		if !exists {
			return "", false
		}

		current = val
	}

	if current == nil {
		return "", true
	}

	return fmt.Sprintf("%v", current), true
}
