package document

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

// workFactor bounds the cells actions may visit in total, so restyling the
// same range repeatedly cannot burn CPU without growing the worksheet.
const workFactor = 4

// cellBudget approximates the worksheet memory excelize allocates: every row
// up to the highest one referenced, and in each row every cell up to the
// highest column referenced there. A single cell at XFD1048576 would otherwise
// allocate about 200 MB, and styling a large range several gigabytes.
type cellBudget struct {
	limit   int
	cells   int
	work    int
	maxRow  int
	rowCols map[int]int
}

func newCellBudget(limit int) *cellBudget {
	return &cellBudget{limit: limit, rowCols: make(map[int]int)}
}

func (b *cellBudget) cell(axis string) error {
	col, row, err := excelize.CellNameToCoordinates(axis)
	if err != nil {
		return err
	}
	return b.touch(row, col, row, col)
}

func (b *cellBudget) area(from, to string) error {
	c1, r1, err := excelize.CellNameToCoordinates(from)
	if err != nil {
		return err
	}
	c2, r2, err := excelize.CellNameToCoordinates(to)
	if err != nil {
		return err
	}
	return b.touch(min(r1, r2), min(c1, c2), max(r1, r2), max(c1, c2))
}

func (b *cellBudget) row(row int) error {
	if row < 1 {
		return fmt.Errorf("invalid row number %d", row)
	}
	return b.touch(row, 0, row, 0)
}

func (b *cellBudget) touch(r1, c1, r2, c2 int) error {
	b.work += (r2 - r1 + 1) * max(1, c2-c1+1)
	if b.work > workFactor*b.limit {
		return b.exceeded()
	}
	if r2 > b.maxRow {
		b.cells += r2 - b.maxRow
		b.maxRow = r2
	}
	if c2 > 0 {
		for r := r1; r <= r2; r++ {
			if prev := b.rowCols[r]; c2 > prev {
				b.cells += c2 - prev
				b.rowCols[r] = c2
			}
		}
	}
	if b.cells > b.limit {
		return b.exceeded()
	}
	return nil
}

func (b *cellBudget) exceeded() error {
	return fmt.Errorf("%w: worksheet would exceed %d cells", ErrLimit, b.limit)
}
