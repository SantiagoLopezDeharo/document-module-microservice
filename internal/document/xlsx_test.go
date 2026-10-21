package document

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/xuri/excelize/v2"
)

const salesXLSX = `{"actions":[
	{"type":"NewSheet","sheet":"Summary"},
	{"type":"SetActiveSheet","sheet":"Summary"},
	{"type":"SetCellValue","sheet":"Sheet1","axis":"A1","value":"Product"},
	{"type":"SetCellValue","sheet":"Sheet1","axis":"B1","value":"Units"},
	{"type":"SetCellValue","sheet":"Sheet1","axis":"A2","value":"Widget"},
	{"type":"SetCellValue","sheet":"Sheet1","axis":"B2","value":42},
	{"type":"SetCellValue","sheet":"Sheet1","axis":"C2","value":true},
	{"type":"SetCellValue","sheet":"Summary","axis":"A1","value":"Total"},
	{"type":"SetColWidth","sheet":"Sheet1","start_col":"A","end_col":"B","width":25},
	{"type":"SetRowHeight","sheet":"Sheet1","row":1,"height":30},
	{"type":"MergeCell","sheet":"Summary","h_cell":"A1","v_cell":"C1"},
	{"type":"NewStyle","style_ref":"header","style":{"font":{"bold":true,"size":14},"fill":{"type":"pattern","color":["#CCCCCC"],"pattern":1}}},
	{"type":"SetCellStyle","sheet":"Sheet1","h_cell":"A1","v_cell":"B1","style_ref":"header"}
]}`

func renderAndOpen(t *testing.T, raw string) *excelize.File {
	t.Helper()
	out, err := RenderXLSX(mustJSON[XLSXRequest](t, raw), testLimits)
	if err != nil {
		t.Fatalf("RenderXLSX: %v", err)
	}
	f, err := excelize.OpenReader(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("output is not a workbook: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestRenderXLSXWorkbook(t *testing.T) {
	f := renderAndOpen(t, salesXLSX)

	if got := f.GetSheetList(); len(got) != 2 || got[0] != "Sheet1" || got[1] != "Summary" {
		t.Errorf("sheets = %v", got)
	}
	if got := f.GetSheetName(f.GetActiveSheetIndex()); got != "Summary" {
		t.Errorf("active sheet = %q", got)
	}
	cells := map[string]string{"Sheet1!A1": "Product", "Sheet1!B1": "Units", "Sheet1!A2": "Widget", "Sheet1!B2": "42", "Sheet1!C2": "TRUE", "Summary!A1": "Total"}
	for ref, want := range cells {
		var sheet, axis string
		fmt.Sscanf(ref, "%6s", &sheet)
		sheet, axis = splitRef(ref)
		if got, _ := f.GetCellValue(sheet, axis); got != want {
			t.Errorf("%s = %q, want %q", ref, got, want)
		}
	}
	if w, _ := f.GetColWidth("Sheet1", "B"); w != 25 {
		t.Errorf("column width = %v", w)
	}
	if h, _ := f.GetRowHeight("Sheet1", 1); h != 30 {
		t.Errorf("row height = %v", h)
	}
	merges, _ := f.GetMergeCells("Summary")
	if len(merges) != 1 || merges[0].GetStartAxis() != "A1" || merges[0].GetEndAxis() != "C1" {
		t.Errorf("merges = %v", merges)
	}
	styleID, _ := f.GetCellStyle("Sheet1", "B1")
	style, err := f.GetStyle(styleID)
	if err != nil || style.Font == nil || !style.Font.Bold || style.Font.Size != 14 {
		t.Errorf("header style = %+v, %v", style, err)
	}
	if id, _ := f.GetCellStyle("Sheet1", "A2"); id != 0 {
		t.Errorf("unstyled cell has style %d", id)
	}
}

func splitRef(ref string) (string, string) {
	for i := range ref {
		if ref[i] == '!' {
			return ref[:i], ref[i+1:]
		}
	}
	return "", ref
}

func TestRenderXLSXStyleByID(t *testing.T) {
	f := renderAndOpen(t, `{"actions":[
		{"type":"NewStyle","style":{"font":{"italic":true}}},
		{"type":"SetCellStyle","sheet":"Sheet1","h_cell":"A1","v_cell":"A1","style_id":1},
		{"type":"SetActiveSheet","index":0}
	]}`)
	if id, _ := f.GetCellStyle("Sheet1", "A1"); id != 1 {
		t.Errorf("style id = %d", id)
	}
}

func TestRenderXLSXReportsFailingAction(t *testing.T) {
	tests := []struct {
		raw   string
		index int
		typ   string
	}{
		{`{"actions":[{"type":"Chart"}]}`, 0, "Chart"},
		{`{"actions":[{"type":"SetCellValue","sheet":"Sheet1","axis":"A0","value":1}]}`, 0, "SetCellValue"},
		{`{"actions":[{"type":"SetCellValue","sheet":"Sheet1","axis":"1A","value":1}]}`, 0, "SetCellValue"},
		{`{"actions":[{"type":"SetCellValue","sheet":"Missing","axis":"A1","value":1}]}`, 0, "SetCellValue"},
		{`{"actions":[{"type":"NewSheet","sheet":"bad/name"}]}`, 0, "NewSheet"},
		{`{"actions":[{"type":"NewSheet","sheet":"0123456789012345678901234567890123"}]}`, 0, "NewSheet"},
		{`{"actions":[{"type":"SetActiveSheet","sheet":"Missing"}]}`, 0, "SetActiveSheet"},
		{`{"actions":[{"type":"SetActiveSheet","index":5}]}`, 0, "SetActiveSheet"},
		{`{"actions":[{"type":"SetActiveSheet","index":-1}]}`, 0, "SetActiveSheet"},
		{`{"actions":[{"type":"SetColWidth","sheet":"Sheet1","start_col":"A","end_col":"B","width":300}]}`, 0, "SetColWidth"},
		{`{"actions":[{"type":"SetColWidth","sheet":"Sheet1","start_col":"1","end_col":"B","width":3}]}`, 0, "SetColWidth"},
		{`{"actions":[{"type":"SetRowHeight","sheet":"Sheet1","row":0,"height":10}]}`, 0, "SetRowHeight"},
		{`{"actions":[{"type":"SetRowHeight","sheet":"Sheet1","row":1,"height":1000}]}`, 0, "SetRowHeight"},
		{`{"actions":[{"type":"MergeCell","sheet":"Sheet1","h_cell":"A1","v_cell":""}]}`, 0, "MergeCell"},
		{`{"actions":[{"type":"NewStyle","style_ref":"x"}]}`, 0, "NewStyle"},
		{`{"actions":[{"type":"NewStyle","style":{"font":{"size":1000}}}]}`, 0, "NewStyle"},
		{`{"actions":[{"type":"SetCellStyle","sheet":"Sheet1","h_cell":"A1","v_cell":"A1"}]}`, 0, "SetCellStyle"},
		{`{"actions":[{"type":"SetCellStyle","sheet":"Sheet1","h_cell":"A1","v_cell":"A1","style_ref":"nope"}]}`, 0, "SetCellStyle"},
		{`{"actions":[{"type":"SetCellStyle","sheet":"Sheet1","h_cell":"A1","v_cell":"A1","style_id":99}]}`, 0, "SetCellStyle"},
	}
	for _, tt := range tests {
		_, err := RenderXLSX(mustJSON[XLSXRequest](t, tt.raw), testLimits)
		assertActionError(t, err, tt.index, tt.typ)
	}
}

func TestRenderXLSXCellBudget(t *testing.T) {
	over := []string{
		`{"type":"SetCellValue","sheet":"Sheet1","axis":"XFD1048576","value":1}`,
		`{"type":"SetCellValue","sheet":"Sheet1","axis":"A20000","value":1}`,
		`{"type":"SetRowHeight","sheet":"Sheet1","row":1048576,"height":10}`,
		`{"type":"MergeCell","sheet":"Sheet1","h_cell":"A1","v_cell":"XFD1048576"}`,
		`{"type":"MergeCell","sheet":"Sheet1","h_cell":"XFD1048576","v_cell":"A1"}`,
		`{"type":"SetCellStyle","sheet":"Sheet1","h_cell":"A1","v_cell":"Z200000","style_id":1}`,
	}
	for _, action := range over {
		raw := `{"actions":[{"type":"NewStyle","style":{"font":{"bold":true}}},` + action + `]}`
		_, err := RenderXLSX(mustJSON[XLSXRequest](t, raw), testLimits)
		if !errors.Is(err, ErrLimit) {
			t.Errorf("%s: err = %v, want ErrLimit", action, err)
		}
	}

	var rows bytes.Buffer
	rows.WriteString(`{"actions":[`)
	for r := 1; r <= testLimits.MaxActions; r++ {
		if r > 1 {
			rows.WriteString(",")
		}
		fmt.Fprintf(&rows, `{"type":"SetCellValue","sheet":"Sheet1","axis":"C%d","value":%d}`, r, r)
	}
	rows.WriteString("]}")
	f := renderAndOpen(t, rows.String())
	if got, _ := f.GetCellValue("Sheet1", fmt.Sprintf("C%d", testLimits.MaxActions)); got != fmt.Sprint(testLimits.MaxActions) {
		t.Errorf("last row = %q", got)
	}
}

func TestRenderXLSXActionLimit(t *testing.T) {
	req := XLSXRequest{Actions: make([]XLSXAction, testLimits.MaxActions+1)}
	if _, err := RenderXLSX(req, testLimits); !errors.Is(err, ErrLimit) {
		t.Fatalf("err = %v, want ErrLimit", err)
	}
}

func FuzzRenderXLSX(f *testing.F) {
	f.Add([]byte(salesXLSX))
	f.Fuzz(func(t *testing.T, data []byte) {
		req, ok := fuzzRequest[XLSXRequest](data)
		if !ok {
			return
		}
		out, err := RenderXLSX(req, Limits{MaxActions: 30, MaxXLSXCells: 2000})
		if err != nil {
			return
		}
		f, err := excelize.OpenReader(bytes.NewReader(out))
		if err != nil {
			t.Fatalf("rendered workbook cannot be reopened: %v", err)
		}
		_ = f.Close()
	})
}

func BenchmarkRenderXLSX(b *testing.B) {
	req := mustJSON[XLSXRequest](b, salesXLSX)
	for r := 3; r < 90; r++ {
		req.Actions = append(req.Actions, XLSXAction{Type: "SetCellValue", Sheet: "Sheet1", Axis: fmt.Sprintf("A%d", r), Value: r})
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := RenderXLSX(req, testLimits); err != nil {
			b.Fatal(err)
		}
	}
}
