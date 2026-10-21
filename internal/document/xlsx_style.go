package document

import (
	"encoding/json"
	"strings"

	"github.com/xuri/excelize/v2"
)

// xlsxStyle is an excelize.Style that also accepts snake_case keys such as
// num_fmt or wrap_text, at any depth. excelize matches the Go field names
// case-insensitively, so removing the underscores is all it takes; the
// original spellings (numfmt, NumFmt) keep working.
type xlsxStyle excelize.Style

func (s *xlsxStyle) UnmarshalJSON(data []byte) error {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	normalized, err := json.Marshal(withoutKeyUnderscores(raw))
	if err != nil {
		return err
	}
	return json.Unmarshal(normalized, (*excelize.Style)(s))
}

func withoutKeyUnderscores(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, child := range val {
			out[strings.ReplaceAll(k, "_", "")] = withoutKeyUnderscores(child)
		}
		return out
	case []interface{}:
		for i, child := range val {
			val[i] = withoutKeyUnderscores(child)
		}
	}
	return v
}
