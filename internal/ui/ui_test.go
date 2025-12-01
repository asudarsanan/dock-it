package ui

import (
	"testing"

	"github.com/rivo/tview"
)

func TestRestoreSelection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		selectedRow int
		total       int
		wantRow     int
	}{
		{"empty", 5, 0, 0},
		{"valid", 3, 5, 3},
		{"lessThanOne", 0, 4, 1},
		{"greaterThanTotal", 10, 2, 1},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			testUI := &UI{table: tview.NewTable()}
			testUI.table.Select(9, 0)
			testUI.restoreSelection(tt.selectedRow, tt.total)
			row, _ := testUI.table.GetSelection()
			if row != tt.wantRow {
				t.Fatalf("restoreSelection row = %d, want %d", row, tt.wantRow)
			}
		})
	}
}
