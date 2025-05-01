// File: game/grid_fill_test.go
package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestGrid_FillGridWithQuarterGrids(t *testing.T) {
	type FillGridWithQuarterGridsTestCase struct {
		name           string
		q1, q2, q3, q4 Grid
		expectedGrid   Grid
		panics         bool
	}

	// Simple 1x1 quarter grids
	q1_1x1_b1 := Grid{{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1, Level: 1}}}}
	q2_1x1_e0 := Grid{{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0, Level: 0}}}}
	q3_1x1_b2 := Grid{{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 2, Level: 2}}}}
	q4_1x1_e0 := Grid{{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0, Level: 0}}}}

	// Expected 2x2 grid
	expected_2x2 := Grid{
		{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1, Level: 1}}, Cell{X: 0, Y: 1, Data: &BrickData{Type: utils.Cells.Empty, Life: 0, Level: 0}}},
		{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 2, Level: 2}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Empty, Life: 0, Level: 0}}},
	}

	testCases := []FillGridWithQuarterGridsTestCase{
		{
			name:         "Valid 2x2",
			q1:           q1_1x1_b1,
			q2:           q2_1x1_e0,
			q3:           q3_1x1_b2,
			q4:           q4_1x1_e0,
			expectedGrid: expected_2x2,
			panics:       false,
		},
		{
			name:   "Mismatched Quarter Sizes",
			q1:     Grid{{Cell{}}},
			q2:     Grid{{Cell{}}, {Cell{}}}, // Different size
			q3:     Grid{{Cell{}}},
			q4:     Grid{{Cell{}}},
			panics: true,
		},
		{
			name:   "Empty Quarter Grids",
			q1:     Grid{},
			q2:     Grid{},
			q3:     Grid{},
			q4:     Grid{},
			panics: true, // Panics because len(q1) == 0
		},
		{
			name:         "Main Grid Wrong Size",
			q1:           q1_1x1_b1,
			q2:           q2_1x1_e0,
			q3:           q3_1x1_b2,
			q4:           q4_1x1_e0,
			expectedGrid: NewGrid(3), // Main grid size 3 != 2 * 1
			panics:       true,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			mainGridSize := 0
			if !test.panics {
				mainGridSize = len(test.q1) * 2
			} else if test.name == "Main Grid Wrong Size" {
				mainGridSize = 3 // Specific size for this panic case
			} else {
				mainGridSize = 2 // Default size for other panic cases
			}
			grid := NewGrid(mainGridSize)

			didPanic, _ := utils.AssertPanics(t, func() {
				grid.FillGridWithQuarterGrids(test.q1, test.q2, test.q3, test.q4)
			}, "")

			if didPanic != test.panics {
				t.Errorf("Panic expectation mismatch: Expected panic=%t, Got panic=%t", test.panics, didPanic)
			}

			if !test.panics {
				match := grid.Compare(test.expectedGrid)
				if !match {
					// Print grids for easier debugging if comparison fails
					t.Errorf("Grid comparison failed.\nExpected:\n%v\nGot:\n%v", test.expectedGrid, grid)
				}
			}
		})
	}

}

func TestGrid_Fill(t *testing.T) {
	type FillTestCase struct {
		name              string
		gridSize          int
		numberOfVectors   int
		maxVectorSize     int
		randomWalkers     int
		randomSteps       int
		expectedMaxBricks int // Estimate max possible bricks
		panics            bool
	}

	testCases := []FillTestCase{
		{
			name:            "10x10 Grid",
			gridSize:        10,
			numberOfVectors: 2,
			maxVectorSize:   2,
			randomWalkers:   2,
			randomSteps:     2,
			// Rough estimate: (vectors * size + walkers * steps) * 4 quarters
			expectedMaxBricks: (2*2 + 2*2) * 4,
			panics:            false,
		},
		{
			name:              "6x6 Grid Min",
			gridSize:          6,
			numberOfVectors:   1,
			maxVectorSize:     1,
			randomWalkers:     1,
			randomSteps:       1,
			expectedMaxBricks: (1*1 + 1*1) * 4,
			panics:            false,
		},
		{
			name:     "Odd Grid Size",
			gridSize: 9,
			panics:   true,
		},
		{
			name:     "Zero Grid Size",
			gridSize: 0,
			panics:   true,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			didPanic, _ := utils.AssertPanics(t, func() {
				grid := NewGrid(test.gridSize)
				// Use defaults by passing 0, let Fill use config values
				grid.Fill(0, 0, 0, 0)

				// Check if grid is filled (at least one brick) if not panicking
				if !test.panics {
					hasBrick := false
					for i := range grid {
						for j := range grid[i] {
							if grid[i][j].Data != nil && grid[i][j].Data.Type == utils.Cells.Brick {
								hasBrick = true
								break
							}
						}
						if hasBrick {
							break
						}
					}
					if !hasBrick {
						t.Errorf("Expected grid to have at least one brick after Fill, but found none.")
					}
				}
			}, "")

			if didPanic != test.panics {
				t.Errorf("Panic expectation mismatch: Expected panic=%t, Got panic=%t", test.panics, didPanic)
			}
		})
	}
}
