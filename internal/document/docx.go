package document

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/SantiagoLopezDeharo/document-module-microservice/internal/docx"
)

type DocxAction struct {
	Type string `json:"type"`

	Txt    string `json:"txt,omitempty"`
	Size   int    `json:"size,omitempty"`
	Color  string `json:"color,omitempty"`
	Bold   bool   `json:"bold,omitempty"`
	Italic bool   `json:"italic,omitempty"`
}

type DocxRequest struct {
	Actions []DocxAction `json:"actions"`
}

// maxFontSize is the largest size Word accepts, in points.
const maxFontSize = 1638

// Word refuses to open documents whose run colors are not hex triplets.
var docxColor = regexp.MustCompile(`^(?:#?[0-9A-Fa-f]{6}|auto)$`)

func RenderDOCX(req DocxRequest, limits Limits) ([]byte, error) {
	if err := checkActionCount(len(req.Actions), limits); err != nil {
		return nil, err
	}

	f := docx.NewFile()
	var paragraph *docx.Paragraph
	for i, action := range req.Actions {
		switch action.Type {
		case "AddParagraph":
			paragraph = f.AddParagraph()
		case "AddText":
			if err := validateText(action); err != nil {
				return nil, &ActionError{Index: i, Type: action.Type, Err: err}
			}
			if paragraph == nil {
				paragraph = f.AddParagraph()
			}
			addText(paragraph, action)
		default:
			return nil, &ActionError{Index: i, Type: action.Type, Err: errUnknownAction}
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func validateText(a DocxAction) error {
	if a.Size < 0 || a.Size > maxFontSize {
		return fmt.Errorf("size must be between 1 and %d", maxFontSize)
	}
	if a.Color != "" && !docxColor.MatchString(a.Color) {
		return fmt.Errorf("color %q must be a 6-digit hex value or auto", a.Color)
	}
	return nil
}

func addText(p *docx.Paragraph, a DocxAction) {
	run := p.AddText(a.Txt)
	if a.Size > 0 {
		run.Size(a.Size)
	}
	if a.Color != "" {
		run.Color(strings.TrimPrefix(a.Color, "#"))
	}
	if a.Bold {
		run.Bold()
	}
	if a.Italic {
		run.Italic()
	}
}
