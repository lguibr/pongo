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

func TestCreateQuarterGridSeed(t *testing.T) {
	type TestCreateQuarterGridSeedTestCase struct {
		name                    string
		gridSize                int
		numberOfVectors         int
		maxVectorSize           int
		expectedMaxBrickLifeSum int // Max possible sum of life points
	}

	testCases := []TestCreateQuarterGridSeedTestCase{
		{"Size10_Vec5_Len5", 10, 5, 5, 5 * 5}, // Max life sum is roughly vectors * size
		{"Size20_Vec10_Len8", 20, 10, 8, 10 * 8},
		{"Size6_Vec2_Len2", 6, 2, 2, 2 * 2},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			totalLifeSum := 0
			runs := 10 // Run multiple times due to randomness
			for i := 0; i < runs; i++ {
				grid := NewGrid(test.gridSize) // Create fresh grid each run
				grid.CreateQuarterGridSeed(test.numberOfVectors, test.maxVectorSize)

				currentLifeSum := 0
				for r := range grid {
					for c := range grid[r] {
						if grid[r][c].Data != nil && grid[r][c].Data.Type == utils.Cells.Brick {
							currentLifeSum += grid[r][c].Data.Life
						}
					}
				}
				totalLifeSum += currentLifeSum
			}
			averageLifeSum := float64(totalLifeSum) / float64(runs)

			// Check average is plausible, not strictly bounded due to overlaps
			// Allow some buffer over the simple max estimate
			plausibleMax := float64(test.expectedMaxBrickLifeSum) * 1.5
			if averageLifeSum > plausibleMax {
				t.Errorf("Average brick life sum (%.2f) seems too high, expected roughly <= %.2f", averageLifeSum, plausibleMax)
			}
			if averageLifeSum <= 0 && test.numberOfVectors > 0 && test.maxVectorSize > 0 { // Only expect bricks if generation params > 0
				t.Errorf("Expected some bricks to be generated, but average life sum was %.2f", averageLifeSum)
			}
		})
	}
}

func TestGrid_RandomWalker(t *testing.T) {
	type RandomWalkerTestCase struct {
		name        string
		gridSize    int
		steps       int
		expectPanic bool
	}

	testCases := []RandomWalkerTestCase{
		{"Size10_Steps10", 10, 10, false},
		{"Size10_Steps100", 10, 100, false},
		{"Size6_Steps50", 6, 50, false},
		{"Size0_Steps10", 0, 10, false}, // Should not panic, RandomWalker handles size 0
		{"Size1_Steps10", 1, 10, false}, // Should not panic, RandomWalker handles size 1
		{"Size2_Steps10", 2, 10, false}, // Start at [1][1] is valid
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			didPanic, panicMsg := utils.AssertPanics(t, func() {
				grid := NewGrid(test.gridSize)
				grid.RandomWalker(test.steps) // Call walker regardless of size (it should handle 0/1)

				// If not panicking, check if start cell was modified (only if size >= 2)
				if !test.expectPanic && test.gridSize >= 2 {
					startR, startC := test.gridSize/2, test.gridSize/2
					if grid[startR][startC].Data == nil || grid[startR][startC].Data.Type != utils.Cells.Brick {
						t.Errorf("Expected start cell (%d, %d) to be a brick after RandomWalker", startR, startC)
					}
				}
			}, "")

			if didPanic != test.expectPanic {
				t.Errorf("Panic expectation mismatch: Expected panic=%t, Got panic=%t. Panic message: %s", test.expectPanic, didPanic, panicMsg)
			}
		})
	}
}
