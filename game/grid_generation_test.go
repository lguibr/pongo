// File: game/grid_generation_test.go
package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestGrid_NewGrid(t *testing.T) {
	gridSize := 5
	grid := NewGrid(gridSize)

	// Check the grid size
	if len(grid) != gridSize {
		t.Errorf("Expected grid to have length %d, but got %d", gridSize, len(grid))
	}
	if len(grid[0]) != gridSize {
		t.Errorf("Expected grid to have width %d, but got %d", gridSize, len(grid[0]))
	}

	// Check that all cells have nil Data initially
	for i := range grid {
		for j := range grid[i] {
			if grid[i][j].Data != nil {
				t.Errorf("Expected cell at position (%d, %d) to have nil Data initially, but got %v", i, j, grid[i][j].Data)
			}
		}
	}
}

// TestCreateQuarterGridSeed is removed as the function is removed.

func TestGrid_RandomWalker(t *testing.T) {
	type RandomWalkerTestCase struct {
		name        string
		gridSize    int
		startX      int // Use specific start points for testing walker logic
		startY      int
		steps       int
		expectPanic bool
	}

	testCases := []RandomWalkerTestCase{
		{"Size10_Steps10_Center", 10, 5, 5, 10, false},
		{"Size10_Steps100_Center", 10, 5, 5, 100, false},
		{"Size6_Steps50_Center", 6, 3, 3, 50, false},
		{"Size0_Steps10", 0, 0, 0, 10, true}, // Corrected: NewGrid(0) panics
		{"Size1_Steps10", 1, 0, 0, 10, false},
		{"Size2_Steps10_Start00", 2, 0, 0, 10, false},
		{"Size10_StartOutOfBounds", 10, 11, 5, 10, false}, // Should not panic, applyRandomWalk handles invalid start
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			didPanic, panicMsg := utils.AssertPanics(t, func() {
				grid := NewGrid(test.gridSize) // This line will panic for gridSize 0
				// Use applyRandomWalk directly for testing
				grid.applyRandomWalk(test.startX, test.startY, test.steps)

				// If not panicking and start is valid, check if start cell was modified
				if !test.expectPanic && test.gridSize > 0 && test.startX >= 0 && test.startX < test.gridSize && test.startY >= 0 && test.startY < test.gridSize {
					startCell := grid[test.startX][test.startY]
					if startCell.Data == nil || startCell.Data.Type != utils.Cells.Brick {
						t.Errorf("Expected start cell (%d, %d) to be a brick after RandomWalker", test.startX, test.startY)
					}
				}
			}, "")

			if didPanic != test.expectPanic {
				t.Errorf("Panic expectation mismatch: Expected panic=%t, Got panic=%t. Panic message: %s", test.expectPanic, didPanic, panicMsg)
			}
		})
	}
}