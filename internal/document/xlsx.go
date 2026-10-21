package document

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/xuri/excelize/v2"
)

type XLSXAction struct {
	Type string `json:"type"`

	Sheet    string      `json:"sheet,omitempty"`
	Index    int         `json:"index,omitempty"`
	Axis     string      `json:"axis,omitempty"`
	Value    interface{} `json:"value,omitempty"`
	StartCol string      `json:"start_col,omitempty"`
	EndCol   string      `json:"end_col,omitempty"`
	Width    float64     `json:"width,omitempty"`
	Row      int         `json:"row,omitempty"`
	Height   float64     `json:"height,omitempty"`
	HCell    string      `json:"h_cell,omitempty"`
	VCell    string      `json:"v_cell,omitempty"`

	Style    *excelize.Style `json:"style,omitempty"`
	StyleID  int             `json:"style_id,omitempty"`
	StyleRef string          `json:"style_ref,omitempty"`
}

type XLSXRequest struct {
	Actions []XLSXAction `json:"actions"`
}

var (
	errMissingStyle = errors.New("style is required")
	errNoStyle      = errors.New("style_id or style_ref is required")
)

type workbook struct {
	file   *excelize.File
	styles map[string]int
	budget *cellBudget
}

func RenderXLSX(req XLSXRequest, limits Limits) ([]byte, error) {
	if err := checkActionCount(len(req.Actions), limits); err != nil {
		return nil, err
	}

	wb := &workbook{
		file:   excelize.NewFile(),
		styles: make(map[string]int),
		budget: newCellBudget(limits.MaxXLSXCells),
	}
	defer wb.file.Close()

	for i, action := range req.Actions {
		if err := wb.apply(action); err != nil {
			return nil, &ActionError{Index: i, Type: action.Type, Err: err}
		}
	}

	var buf bytes.Buffer
	if err := wb.file.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (wb *workbook) apply(a XLSXAction) error {
	f := wb.file
	switch a.Type {
	case "NewSheet":
		_, err := f.NewSheet(a.Sheet)
		return err
	case "SetActiveSheet":
		return wb.setActiveSheet(a)
	case "SetCellValue":
		if err := wb.budget.cell(a.Axis); err != nil {
			return err
		}
		return f.SetCellValue(a.Sheet, a.Axis, a.Value)
	case "SetColWidth":
		return f.SetColWidth(a.Sheet, a.StartCol, a.EndCol, a.Width)
	case "SetRowHeight":
		if err := wb.budget.row(a.Row); err != nil {
			return err
		}
		return f.SetRowHeight(a.Sheet, a.Row, a.Height)
	case "MergeCell":
		if err := wb.budget.area(a.HCell, a.VCell); err != nil {
			return err
		}
		return f.MergeCell(a.Sheet, a.HCell, a.VCell)
	case "NewStyle":
		if a.Style == nil {
			return errMissingStyle
		}
		id, err := f.NewStyle(a.Style)
		if err == nil && a.StyleRef != "" {
			wb.styles[a.StyleRef] = id
		}
		return err
	case "SetCellStyle":
		id, err := wb.styleID(a)
		if err != nil {
			return err
		}
		if err := wb.budget.area(a.HCell, a.VCell); err != nil {
			return err
		}
		return f.SetCellStyle(a.Sheet, a.HCell, a.VCell, id)
	default:
		return errUnknownAction
	}
}

func (wb *workbook) setActiveSheet(a XLSXAction) error {
	index := a.Index
	if a.Sheet != "" {
		var err error
		if index, err = wb.file.GetSheetIndex(a.Sheet); err != nil {
			return err
		}
	}
	if index < 0 || index >= wb.file.SheetCount {
		return fmt.Errorf("sheet %q (index %d) does not exist", a.Sheet, index)
	}
	wb.file.SetActiveSheet(index)
	return nil
}

func (wb *workbook) styleID(a XLSXAction) (int, error) {
	if a.StyleID != 0 {
		return a.StyleID, nil
	}
	if a.StyleRef == "" {
		return 0, errNoStyle
	}
	id, ok := wb.styles[a.StyleRef]
	if !ok {
		return 0, fmt.Errorf("unknown style_ref %q", a.StyleRef)
	}
	return id, nil
}
