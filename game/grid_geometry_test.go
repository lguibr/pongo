package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
)

func TestGrid_LineIntersectedCellIndices(t *testing.T) {

	type LineIntersectedCellIndicesCase struct {
		name            string
		GridSize        int
		Line            [2][2]int
		ExpectedIndices [][2]int
	}

	cases := []LineIntersectedCellIndicesCase{
		{
			name:            "Diagonal 2x2",
			GridSize:        2,
			Line:            [2][2]int{{0, 0}, {1, 1}},
			ExpectedIndices: [][2]int{{0, 0}, {0, 1}, {1, 0}, {1, 1}},
		},
		{
			name:            "Horizontal Line 2x2",
			GridSize:        2,
			Line:            [2][2]int{{0, 0}, {1, 0}},
			ExpectedIndices: [][2]int{{0, 0}, {1, 0}},
		},
		{
			name:            "Vertical Line 2x2",
			GridSize:        2,
			Line:            [2][2]int{{0, 0}, {0, 1}},
			ExpectedIndices: [][2]int{{0, 0}, {0, 1}},
		},
		{
			name:            "Single Point 2x2",
			GridSize:        2,
			Line:            [2][2]int{{0, 0}, {0, 0}},
			ExpectedIndices: [][2]int{{0, 0}},
		},
		{
			name:            "Diagonal 3x3",
			GridSize:        3,
			Line:            [2][2]int{{0, 0}, {2, 2}},
			ExpectedIndices: [][2]int{{0, 0}, {0, 1}, {0, 2}, {1, 0}, {1, 1}, {1, 2}, {2, 0}, {2, 1}, {2, 2}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			grid := NewGrid(tc.GridSize)
			indices := grid.LineIntersectedCellIndices(10, tc.Line) // Cell size doesn't matter for this logic

			// Use assert.ElementsMatch for order-independent comparison
			assert.ElementsMatch(t, tc.ExpectedIndices, indices)
		})
	}

}

func TestGrid_Rotate(t *testing.T) {

	type RotateTestCase struct {
		name     string
		grid     Grid
		expected Grid
	}

	// Helper to create a cell quickly
	newTestCell := func(x, y int, life int) Cell {
		if life > 0 {
			return NewCell(x, y, life, utils.Cells.Brick)
		}
		return NewCell(x, y, 0, utils.Cells.Empty)
	}

	testCases := []RotateTestCase{
		{
			name: "2x2 Empty",
			grid: Grid{
				{newTestCell(0, 0, 0), newTestCell(0, 1, 0)},
				{newTestCell(1, 0, 0), newTestCell(1, 1, 0)},
			},
			expected: Grid{
				{newTestCell(0, 0, 0), newTestCell(0, 1, 0)}, // Expected coords updated
				{newTestCell(1, 0, 0), newTestCell(1, 1, 0)},
			},
		},
		{
			name: "2x2 With Data",
			grid: Grid{
				{newTestCell(0, 0, 1), newTestCell(0, 1, 0)},
				{newTestCell(1, 0, 0), newTestCell(1, 1, 2)},
			},
			// Expected grid after 90-degree clockwise rotation
			expected: Grid{
				{newTestCell(0, 0, 0), newTestCell(0, 1, 1)}, // (1,0)->(0,0), (0,0)->(0,1)
				{newTestCell(1, 0, 2), newTestCell(1, 1, 0)}, // (1,1)->(1,0), (0,1)->(1,1)
			},
		},
		{
			name: "3x3 Identity",
			grid: Grid{
				{newTestCell(0, 0, 1), newTestCell(0, 1, 2), newTestCell(0, 2, 3)},
				{newTestCell(1, 0, 4), newTestCell(1, 1, 5), newTestCell(1, 2, 6)},
				{newTestCell(2, 0, 7), newTestCell(2, 1, 8), newTestCell(2, 2, 9)},
			},
			expected: Grid{
				{newTestCell(0, 0, 7), newTestCell(0, 1, 4), newTestCell(0, 2, 1)},
				{newTestCell(1, 0, 8), newTestCell(1, 1, 5), newTestCell(1, 2, 2)},
				{newTestCell(2, 0, 9), newTestCell(2, 1, 6), newTestCell(2, 2, 3)},
			},
		},
		{
			name: "1x1",
			grid: Grid{
				{newTestCell(0, 0, 5)},
			},
			expected: Grid{
				{newTestCell(0, 0, 5)},
			},
		},
		{
			name:     "Empty Grid",
			grid:     Grid{},
			expected: Grid{},
		},
		{
			name:     "Nil Grid",
			grid:     nil,
			expected: nil,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			result := test.grid.Rotate()
			if !result.Compare(test.expected) {
				t.Errorf("Grid comparison failed.\nExpected:\n%v\nGot:\n%v", test.expected, result)
			}
		})
	}
}
