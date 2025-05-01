// File: game/grid_test.go
package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

// Helper to create cells for testing
func newTestCell(x, y int, life int) Cell {
	if life > 0 {
		return NewCell(x, y, life, utils.Cells.Brick)
	}
	return NewCell(x, y, 0, utils.Cells.Empty)
}

func TestGrid_Compare(t *testing.T) {
	testCases := []struct {
		name   string
		grid   Grid
		grid2  Grid
		result bool
	}{
		{
			name: "Grids are the same",
			grid: Grid{
				{newTestCell(0, 0, 1), newTestCell(0, 1, 2)},
				{newTestCell(1, 0, 0), newTestCell(1, 1, 1)},
			},
			grid2: Grid{
				{newTestCell(0, 0, 1), newTestCell(0, 1, 2)},
				{newTestCell(1, 0, 0), newTestCell(1, 1, 1)},
			},
			result: true,
		},
		{
			name: "Grids have different size (rows)",
			grid: Grid{
				{newTestCell(0, 0, 1), newTestCell(0, 1, 2)},
				{newTestCell(1, 0, 0), newTestCell(1, 1, 1)},
			},
			grid2: Grid{
				{newTestCell(0, 0, 1)},
			},
			result: false,
		},
		{
			name: "Grids have different size (cols)",
			grid: Grid{
				{newTestCell(0, 0, 1), newTestCell(0, 1, 2)},
				{newTestCell(1, 0, 0), newTestCell(1, 1, 1)},
			},
			grid2: Grid{
				{newTestCell(0, 0, 1)},
				{newTestCell(1, 0, 0)},
			},
			result: false,
		},
		{
			name: "Grids have different data (life)",
			grid: Grid{
				{newTestCell(0, 0, 1), newTestCell(0, 1, 2)},
				{newTestCell(1, 0, 0), newTestCell(1, 1, 1)},
			},
			grid2: Grid{
				{newTestCell(0, 0, 1), newTestCell(0, 1, 2)},
				{newTestCell(1, 0, 0), newTestCell(1, 1, 99)}, // Different life
			},
			result: false,
		},
		{
			name: "Grids have different data (type)",
			grid: Grid{
				{newTestCell(0, 0, 1), newTestCell(0, 1, 2)},
				{newTestCell(1, 0, 0), newTestCell(1, 1, 1)}, // Brick
			},
			grid2: Grid{
				{newTestCell(0, 0, 1), newTestCell(0, 1, 2)},
				{newTestCell(1, 0, 0), newTestCell(1, 1, 0)}, // Empty
			},
			result: false,
		},
		{
			name:   "Both grids empty",
			grid:   Grid{},
			grid2:  Grid{},
			result: true,
		},
		{
			name:   "One grid nil, the other empty",
			grid:   nil,
			grid2:  Grid{},
			result: false, // Corrected expectation
		},
		{
			name:   "Both grids nil",
			grid:   nil,
			grid2:  nil,
			result: true, // Corrected expectation
		},
		{
			name:   "One element grid and nil grid",
			grid:   Grid{{newTestCell(0, 0, 1)}},
			grid2:  nil,
			result: false,
		},
		{
			name: "Grids with nil Data cells",
			grid: Grid{
				{Cell{X: 0, Y: 0, Data: nil}, newTestCell(0, 1, 1)},
			},
			grid2: Grid{
				{Cell{X: 0, Y: 0, Data: nil}, newTestCell(0, 1, 1)},
			},
			result: true,
		},
		{
			name: "Grids with different nil Data cells",
			grid: Grid{
				{Cell{X: 0, Y: 0, Data: nil}, newTestCell(0, 1, 1)},
			},
			grid2: Grid{
				{newTestCell(0, 0, 0), newTestCell(0, 1, 1)}, // Cell 0,0 has non-nil Data
			},
			result: false,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			// Call the Compare method from game/grid.go
			result := test.grid.Compare(test.grid2)
			if result != test.result {
				t.Errorf("Test case '%s' failed: expected %t, got %t", test.name, test.result, result)
			}
		})
	}
}
