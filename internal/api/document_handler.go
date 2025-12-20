package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gingfrederik/docx"
	"github.com/go-pdf/fpdf"
	"github.com/xuri/excelize/v2"
)

type PDFAction struct {
	Type string `json:"type"` // AddPage, SetFont, Cell, MultiCell, Ln, SetTextColor, Image

	// Parameters for various methods
	Family string  `json:"family,omitempty"`
	Style  string  `json:"style,omitempty"`
	Size   float64 `json:"size,omitempty"`

	W      float64 `json:"w,omitempty"`
	H      float64 `json:"h,omitempty"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Txt    string  `json:"txt,omitempty"`
	Border string  `json:"border,omitempty"`
	Ln     int     `json:"ln,omitempty"`
	Align  string  `json:"align,omitempty"`

	R int `json:"r,omitempty"`
	G int `json:"g,omitempty"`
	B int `json:"b,omitempty"`
}

type PDFRequest struct {
	Orientation string      `json:"orientation"` // P or L
	Unit        string      `json:"unit"`        // pt, mm, cm, in
	Size        string      `json:"size"`        // A3, A4, A5, Letter, Legal
	Actions     []PDFAction `json:"actions"`
}

type DocxAction struct {
	Type string `json:"type"` // AddParagraph, AddText

	// Parameters
	Txt    string `json:"txt,omitempty"`
	Size   int    `json:"size,omitempty"`
	Color  string `json:"color,omitempty"` // Hex color like "FF0000"
	Bold   bool   `json:"bold,omitempty"`
	Italic bool   `json:"italic,omitempty"`
}

type DocxRequest struct {
	Actions []DocxAction `json:"actions"`
}

type XLSXAction struct {
	Type string `json:"type"` // NewSheet, SetActiveSheet, SetCellValue, SetColWidth, SetRowHeight, MergeCell, NewStyle, SetCellStyle

	// Parameters
	Sheet    string      `json:"sheet,omitempty"`
	Index    int         `json:"index,omitempty"` // For SetActiveSheet
	Axis     string      `json:"axis,omitempty"`  // Cell coordinate e.g. "A1"
	Value    interface{} `json:"value,omitempty"`
	StartCol string      `json:"start_col,omitempty"`
	EndCol   string      `json:"end_col,omitempty"`
	Width    float64     `json:"width,omitempty"`
	Row      int         `json:"row,omitempty"`
	Height   float64     `json:"height,omitempty"`
	HCell    string      `json:"h_cell,omitempty"` // Top-left for merge/style
	VCell    string      `json:"v_cell,omitempty"` // Bottom-right for merge/style

	// For Style
	Style    *excelize.Style `json:"style,omitempty"`
	StyleID  int             `json:"style_id,omitempty"`
	StyleRef string          `json:"style_ref,omitempty"` // To reference a style created in a previous action
}

type XLSXRequest struct {
	Actions []XLSXAction `json:"actions"`
}

func pdfHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PDFRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Defaults
	if req.Orientation == "" {
		req.Orientation = "P"
	}
	if req.Unit == "" {
		req.Unit = "mm"
	}
	if req.Size == "" {
		req.Size = "A4"
	}

	pdf := fpdf.New(req.Orientation, req.Unit, req.Size, "")

	for _, action := range req.Actions {
		switch action.Type {
		case "AddPage":
			pdf.AddPage()
		case "SetFont":
			pdf.SetFont(action.Family, action.Style, action.Size)
		case "Cell":
			pdf.CellFormat(action.W, action.H, action.Txt, action.Border, action.Ln, action.Align, false, 0, "")
		case "MultiCell":
			pdf.MultiCell(action.W, action.H, action.Txt, action.Border, action.Align, false)
		case "Ln":
			pdf.Ln(action.H)
		case "SetXY":
			pdf.SetXY(action.X, action.Y)
		case "SetTextColor":
			pdf.SetTextColor(action.R, action.G, action.B)
		case "SetDrawColor":
			pdf.SetDrawColor(action.R, action.G, action.B)
		case "SetFillColor":
			pdf.SetFillColor(action.R, action.G, action.B)
		}
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=output.pdf")

	if err := pdf.Output(w); err != nil {
		fmt.Printf("PDF generation error: %v\n", err)
	}
}

func docxHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req DocxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	fmt.Printf("Received DOCX request with %d actions\n", len(req.Actions))

	f := docx.NewFile()
	var currentPara *docx.Paragraph

	for i, action := range req.Actions {
		fmt.Printf("Processing action %d: %s\n", i, action.Type)
		switch action.Type {
		case "AddParagraph":
			currentPara = f.AddParagraph()
		case "AddText":
			if currentPara == nil {
				currentPara = f.AddParagraph()
			}
			run := currentPara.AddText(action.Txt)
			if action.Size > 0 {
				run.Size(action.Size)
			}
			if action.Color != "" {
				run.Color(action.Color)
			}
			// Note: Bold and Italic are not supported by the gingfrederik/docx package
			// These properties are ignored
		}
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", "attachment; filename=output.docx")

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		fmt.Printf("DOCX generation error: %v\n", err)
		http.Error(w, "Failed to generate DOCX", http.StatusInternalServerError)
		return
	}

	fmt.Printf("Generated DOCX size: %d bytes\n", buf.Len())
	if _, err := w.Write(buf.Bytes()); err != nil {
		fmt.Printf("Error writing response: %v\n", err)
	}
}

func xlsxHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req XLSXRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	f := excelize.NewFile()
	styleMap := make(map[string]int)

	for _, action := range req.Actions {
		var err error
		switch action.Type {
		case "NewSheet":
			f.NewSheet(action.Sheet)
		case "SetActiveSheet":
			if action.Sheet != "" {
				idx, err := f.GetSheetIndex(action.Sheet)
				if err == nil {
					f.SetActiveSheet(idx)
				}
			} else {
				f.SetActiveSheet(action.Index)
			}
		case "SetCellValue":
			err = f.SetCellValue(action.Sheet, action.Axis, action.Value)
		case "SetColWidth":
			err = f.SetColWidth(action.Sheet, action.StartCol, action.EndCol, action.Width)
		case "SetRowHeight":
			err = f.SetRowHeight(action.Sheet, action.Row, action.Height)
		case "MergeCell":
			err = f.MergeCell(action.Sheet, action.HCell, action.VCell)
		case "NewStyle":
			if action.Style != nil {
				id, err := f.NewStyle(action.Style)
				if err == nil {
					if action.StyleRef != "" {
						styleMap[action.StyleRef] = id
					}
				} else {
					fmt.Printf("Error creating style: %v\n", err)
				}
			}
		case "SetCellStyle":
			styleID := action.StyleID
			if styleID == 0 && action.StyleRef != "" {
				styleID = styleMap[action.StyleRef]
			}
			if styleID != 0 {
				err = f.SetCellStyle(action.Sheet, action.HCell, action.VCell, styleID)
			}
		}
		if err != nil {
			fmt.Printf("Error processing action %s: %v\n", action.Type, err)
		}
	}

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=output.xlsx")

	if err := f.Write(w); err != nil {
		fmt.Printf("XLSX generation error: %v\n", err)
	}
}
