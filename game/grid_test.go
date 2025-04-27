// File: game/grid_compare_test.go
package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

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
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, Cell{X: 0, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
			},
			grid2: Grid{
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, Cell{X: 0, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
			},
			result: true,
		},
		{
			name: "Grids have different size",
			grid: Grid{
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, Cell{X: 0, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
			},
			grid2: Grid{
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
			},
			result: false,
		},
		{
			name: "Grids have different column size",
			grid: Grid{
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, Cell{X: 0, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
			},
			grid2: Grid{
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, Cell{X: 0, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
			},

			result: false,
		},
		{
			name: "Grids have different row size",
			grid: Grid{
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, Cell{X: 0, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
			},
			grid2: Grid{
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}},
			},

			result: false,
		},
		{
			name: "Grids have different data",
			grid: Grid{
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, Cell{X: 0, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}},
			},
			grid2: Grid{
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}, Cell{X: 0, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}},
			},
			result: false,
		},
		{
			name:   "Grids is empty",
			grid:   Grid{},
			grid2:  Grid{},
			result: true,
		},
		{
			name:   "One grid is nil, the other is empty",
			grid:   nil,
			grid2:  Grid{},
			result: true,
		},
		{
			name:   "Both grids are nil",
			grid:   nil,
			grid2:  nil,
			result: true,
		},
		{
			name:   "One element grid and nil grid",
			grid:   Grid{{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}}},
			grid2:  nil,
			result: false,
		},
	}
	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			result := test.grid.Compare(test.grid2)
			if result != test.result {
				t.Errorf("Test case '%s' failed: expected %v, got %v", test.name, test.result, result)
			}
		})
	}
}
