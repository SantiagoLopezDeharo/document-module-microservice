package codec

import (
	"bytes"
	"fmt"
	"io"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/utils"
	"github.com/clbanning/mxj/v2"
	"github.com/xuri/excelize/v2"
)

type xlsxCodec struct{}

// Workbooks are zip archives, so a small upload can inflate enormously. These
// limits bound the decompressed size and keep every part in memory instead of
// spilling large XML parts to temporary files.
const (
	maxUnzipBytes    = 256 << 20
	maxUnzipXMLBytes = maxUnzipBytes
)

func (c *xlsxCodec) Decode(data []byte) (interface{}, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit:    maxUnzipBytes,
		UnzipXMLSizeLimit: maxUnzipXMLBytes,
	})
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Assume data is in the first sheet
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, err
	}

	if len(rows) < 1 {
		return []map[string]interface{}{}, nil
	}

	headers := rows[0]
	var result []map[string]interface{}

	for i := 1; i < len(rows); i++ {
		row := rows[i]
		record := make(map[string]interface{})
		for j, cell := range row {
			if j < len(headers) {
				if cell == "" {
					continue
				}
				record[headers[j]] = cell
			}
		}
		result = append(result, record)
	}

	return result, nil
}

func (c *xlsxCodec) Encode(w io.Writer, data interface{}) error {
	f := excelize.NewFile()
	defer f.Close()

	// Normalize data to []map[string]interface{}
	var rows []map[string]interface{}
	switch v := data.(type) {
	case []map[string]interface{}:
		rows = v
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				rows = append(rows, m)
			} else {
				// Try to convert if possible, or skip/error
				// For simplicity, error if not object
				return fmt.Errorf("xlsx requires array of objects")
			}
		}
	case map[string]interface{}:
		rows = append(rows, v)
	case mxj.Map:
		rows = append(rows, map[string]interface{}(v))
	default:
		return fmt.Errorf("xlsx requires array of objects, got %T", data)
	}

	if len(rows) == 0 {
		// Write empty file
		return f.Write(w)
	}

	headers := utils.CollectHeaders(rows)

	// Write Headers
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Sheet1", cell, h)
	}

	// Write Data
	for rIdx, row := range rows {
		for cIdx, h := range headers {
			val, ok := utils.GetValue(row, h)
			cell, _ := excelize.CoordinatesToCellName(cIdx+1, rIdx+2)
			if !ok {
				f.SetCellValue("Sheet1", cell, "")
				continue
			}
			f.SetCellValue("Sheet1", cell, val)
		}
	}

	return f.Write(w)
}
