// File: game/grid_geometry_test.go
package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestGrid_LineIntersectedCellIndices(t *testing.T) {

	type LineIntersectedCellIndicesCase struct {
		GridSize        int
		Line            [2][2]int
		ExpectedIndices [][2]int
	}

	cases := []LineIntersectedCellIndicesCase{
		{
			GridSize:        2,
			Line:            [2][2]int{{0, 0}, {1, 1}},
			ExpectedIndices: [][2]int{{0, 0}, {0, 1}, {1, 0}, {1, 1}},
		},
		{
			GridSize:        2,
			Line:            [2][2]int{{0, 0}, {1, 0}},
			ExpectedIndices: [][2]int{{0, 0}, {1, 0}},
		},
		{
			GridSize:        2,
			Line:            [2][2]int{{0, 0}, {0, 1}},
			ExpectedIndices: [][2]int{{0, 0}, {0, 1}},
		},
		{
			GridSize:        2,
			Line:            [2][2]int{{0, 0}, {0, 0}},
			ExpectedIndices: [][2]int{{0, 0}},
		},
	}

	for i, tc := range cases {
		t.Run("LineIntersect", func(t *testing.T) {
			grid := NewGrid(tc.GridSize)
			indices := grid.LineIntersectedCellIndices(10, tc.Line) // Cell size doesn't matter for this logic

			if len(indices) != len(tc.ExpectedIndices) {
				t.Errorf("Case %d: Expected %d indices, got %d", i, len(tc.ExpectedIndices), len(indices))
			}

			// Simple comparison assuming order doesn't matter for this test's purpose
			// A more robust check would sort both slices or use a map/set.
			matchCount := 0
			for _, expected := range tc.ExpectedIndices {
				for _, actual := range indices {
					if expected == actual {
						matchCount++
						break
					}
				}
			}
			if matchCount != len(tc.ExpectedIndices) {
				t.Errorf("Case %d: Expected indices %v, got %v", i, tc.ExpectedIndices, indices)
			}
		})
	}

}

func TestGrid_Rotate(t *testing.T) {

	type RotateTestCase struct {
		grid     Grid
		expected Grid
	}

	testCases := []RotateTestCase{
		{
			grid: Grid{
				{NewCell(0, 0, 0, utils.Cells.Empty), NewCell(0, 1, 0, utils.Cells.Empty)},
				{NewCell(1, 0, 0, utils.Cells.Empty), NewCell(1, 1, 0, utils.Cells.Empty)},
			},
			expected: Grid{
				{NewCell(1, 0, 0, utils.Cells.Empty), NewCell(0, 0, 0, utils.Cells.Empty)},
				{NewCell(1, 1, 0, utils.Cells.Empty), NewCell(0, 1, 0, utils.Cells.Empty)},
			},
		},
		{
			grid: Grid{
				{NewCell(0, 0, 0, utils.Cells.Empty), NewCell(0, 1, 0, utils.Cells.Empty), NewCell(0, 2, 0, utils.Cells.Empty)},
				{NewCell(1, 0, 0, utils.Cells.Empty), NewCell(1, 1, 0, utils.Cells.Empty), NewCell(1, 2, 0, utils.Cells.Empty)},
				{NewCell(2, 0, 0, utils.Cells.Empty), NewCell(2, 1, 0, utils.Cells.Empty), NewCell(2, 2, 0, utils.Cells.Empty)},
			},
			expected: Grid{
				{NewCell(2, 0, 0, utils.Cells.Empty), NewCell(1, 0, 0, utils.Cells.Empty), NewCell(0, 0, 0, utils.Cells.Empty)},
				{NewCell(2, 1, 0, utils.Cells.Empty), NewCell(1, 1, 0, utils.Cells.Empty), NewCell(0, 1, 0, utils.Cells.Empty)},
				{NewCell(2, 2, 0, utils.Cells.Empty), NewCell(1, 2, 0, utils.Cells.Empty), NewCell(0, 2, 0, utils.Cells.Empty)},
			},
		},
		{
			grid: Grid{
				{NewCell(0, 0, 0, utils.Cells.Empty)},
			},
			expected: Grid{
				{NewCell(0, 0, 0, utils.Cells.Empty)},
			},
		},
	}

	for _, test := range testCases {
		t.Run("Rotate", func(t *testing.T) {
			result := test.grid.Rotate()
			if !result.Compare(test.expected) {
				t.Errorf("Expected %v, got %v", test.expected, result)
			}
		})
	}
}
