package document

import (
	"bytes"
	"fmt"

	"github.com/go-pdf/fpdf"
)

type PDFAction struct {
	Type string `json:"type"`

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
	Orientation string      `json:"orientation"`
	Unit        string      `json:"unit"`
	Size        string      `json:"size"`
	Actions     []PDFAction `json:"actions"`
}

func RenderPDF(req PDFRequest, limits Limits) ([]byte, error) {
	if err := checkActionCount(len(req.Actions), limits); err != nil {
		return nil, err
	}

	pdf := fpdf.New(valueOr(req.Orientation, "P"), valueOr(req.Unit, "mm"), valueOr(req.Size, "A4"), "")
	if err := pdf.Error(); err != nil {
		return nil, fmt.Errorf("%w: page setup: %w", ErrInvalid, err)
	}

	for i, action := range req.Actions {
		if err := applyPDFAction(pdf, action); err != nil {
			return nil, &ActionError{Index: i, Type: action.Type, Err: err}
		}
		if err := pdf.Error(); err != nil {
			return nil, &ActionError{Index: i, Type: action.Type, Err: err}
		}
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func applyPDFAction(pdf *fpdf.Fpdf, a PDFAction) error {
	switch a.Type {
	case "AddPage":
		pdf.AddPage()
	case "SetFont":
		pdf.SetFont(a.Family, a.Style, a.Size)
	case "Cell":
		pdf.CellFormat(a.W, a.H, a.Txt, a.Border, a.Ln, a.Align, false, 0, "")
	case "MultiCell":
		pdf.MultiCell(a.W, a.H, a.Txt, a.Border, a.Align, false)
	case "Ln":
		pdf.Ln(a.H)
	case "SetXY":
		pdf.SetXY(a.X, a.Y)
	case "SetTextColor":
		pdf.SetTextColor(a.R, a.G, a.B)
	case "SetDrawColor":
		pdf.SetDrawColor(a.R, a.G, a.B)
	case "SetFillColor":
		pdf.SetFillColor(a.R, a.G, a.B)
	default:
		return errUnknownAction
	}
	return nil
}
