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

	// Assume data is in the first sheet. Rows streams the worksheet instead of
	// materialising every row and cell up front like GetRows.
	rows, err := f.Rows(f.GetSheetName(0))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var headers []string
	result := []map[string]interface{}{}
	for rows.Next() {
		cells, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		if headers == nil {
			headers = append([]string{}, cells...)
			continue
		}
		record := make(map[string]interface{}, len(headers))
		for j, cell := range cells {
			if j < len(headers) && cell != "" {
				record[headers[j]] = cell
			}
		}
		result = append(result, record)
	}
	if err := rows.Error(); err != nil {
		return nil, err
	}
	if headers == nil || len(result) == 0 {
		return []map[string]interface{}{}, nil
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
		return f.Write(w)
	}

	// The stream writer emits rows sequentially, avoiding the in-memory cell
	// grid that SetCellValue builds for every cell.
	sw, err := f.NewStreamWriter("Sheet1")
	if err != nil {
		return err
	}
	headers := utils.CollectHeaders(rows)
	record := make([]interface{}, len(headers))
	for i, h := range headers {
		record[i] = h
	}
	if err := sw.SetRow("A1", record); err != nil {
		return err
	}
	for rIdx, row := range rows {
		for cIdx, h := range headers {
			record[cIdx] = nil
			if val, ok := utils.GetValue(row, h); ok {
				record[cIdx] = val
			}
		}
		cell, _ := excelize.CoordinatesToCellName(1, rIdx+2)
		if err := sw.SetRow(cell, record); err != nil {
			return err
		}
	}
	if err := sw.Flush(); err != nil {
		return err
	}
	return f.Write(w)
}
