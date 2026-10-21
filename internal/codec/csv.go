package codec

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/utils"
	"github.com/clbanning/mxj/v2"
)

type csvCodec struct {
	comma rune
}

func (c *csvCodec) Decode(data []byte) (interface{}, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = c.comma
	reader.ReuseRecord = true

	headers, err := reader.Read()
	if err != nil {
		return nil, err
	}

	// csv.Reader reuses the same backing array when ReuseRecord is enabled,
	// so keep a stable copy of the header row before the next read mutates it.
	copiedHeaders := make([]string, len(headers))
	copy(copiedHeaders, headers)
	headers = copiedHeaders

	var result []map[string]interface{}
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		row := make(map[string]interface{}, len(headers))
		for i, h := range headers {
			if i < len(record) {
				value := record[i]
				if value == "" {
					continue
				}
				row[h] = value
			}
		}
		result = append(result, row)
	}
	return result, nil
}

func (c *csvCodec) Encode(w io.Writer, data interface{}) error {
	writer := csv.NewWriter(w)
	writer.Comma = c.comma
	defer writer.Flush()

	var rows []map[string]interface{}

	switch v := data.(type) {
	case []map[string]interface{}:
		rows = v
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				rows = append(rows, m)
			} else {
				return fmt.Errorf("csv/tsv requires an array of objects")
			}
		}
	case map[string]interface{}:
		rows = append(rows, v)
	case mxj.Map:
		rows = append(rows, map[string]interface{}(v))
	default:
		return fmt.Errorf("csv/tsv requires an array of objects or a single object, got %T", data)
	}

	if len(rows) == 0 {
		return nil
	}

	headers := utils.CollectHeaders(rows)

	if err := writer.Write(headers); err != nil {
		return err
	}

	record := make([]string, len(headers))
	for _, row := range rows {
		for i := range record {
			record[i] = ""
		}
		for idx, h := range headers {
			if val, ok := utils.GetValue(row, h); ok {
				record[idx] = val
			}
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}
	return nil
}
