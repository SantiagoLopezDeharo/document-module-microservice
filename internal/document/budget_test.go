package document

import (
	"errors"
	"testing"
)

func TestCellBudgetModel(t *testing.T) {
	b := newCellBudget(100)
	steps := []struct {
		op        func() error
		wantCells int
	}{
		{func() error { return b.cell("C1") }, 1 + 3},
		{func() error { return b.cell("A1") }, 4},
		{func() error { return b.cell("E1") }, 6},
		{func() error { return b.cell("B3") }, 3 + 5 + 2},
		{func() error { return b.row(10) }, 17},
		{func() error { return b.area("B2", "A1") }, 17 + 2},
		{func() error { return b.area("A4", "B5") }, 19 + 4},
	}
	for i, s := range steps {
		if err := s.op(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
		if b.cells != s.wantCells {
			t.Fatalf("step %d: cells = %d, want %d", i, b.cells, s.wantCells)
		}
	}
	if err := b.area("A1", "Z10"); !errors.Is(err, ErrLimit) {
		t.Fatalf("over budget: err = %v", err)
	}
}

func TestCellBudgetBoundsRepeatedWork(t *testing.T) {
	b := newCellBudget(100)
	var err error
	for i := 0; i < workFactor*2 && err == nil; i++ {
		err = b.area("A1", "J10")
	}
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("restyling the same range forever was allowed (cells %d, work %d)", b.cells, b.work)
	}
}

func TestCellBudgetRejectsInvalidReferences(t *testing.T) {
	b := newCellBudget(100)
	for _, err := range []error{b.cell(""), b.cell("A0"), b.cell("ZZZZ1"), b.area("A1", "?"), b.area("", "A1"), b.row(0), b.row(-3)} {
		if err == nil || errors.Is(err, ErrLimit) {
			t.Errorf("err = %v, want a reference error", err)
		}
	}
}
