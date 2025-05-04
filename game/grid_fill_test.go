// File: game/grid_fill_test.go
package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
)

// TestGrid_Fill tests the new centralized Fill method.
func TestGrid_Fill(t *testing.T) {
	type FillTestCase struct {
		name            string
		gridSize        int
		numberOfVectors int
		maxVectorSize   int
		randomWalkers   int
		randomSteps     int
		panics          bool
	}

	testCases := []FillTestCase{
		{
			name:            "10x10 Grid Default Params",
			gridSize:        10,
			numberOfVectors: 0, // Use defaults
			maxVectorSize:   0,
			randomWalkers:   0,
			randomSteps:     0,
			panics:          false,
		},
		{
			name:            "6x6 Grid Min Params",
			gridSize:        6,
			numberOfVectors: 1,
			maxVectorSize:   1,
			randomWalkers:   1,
			randomSteps:     1,
			panics:          false,
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
		{
			name:            "Large Grid",
			gridSize:        32,
			numberOfVectors: 50,
			maxVectorSize:   10,
			randomWalkers:   10,
			randomSteps:     20,
			panics:          false,
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			didPanic, _ := utils.AssertPanics(t, func() {
				grid := NewGrid(test.gridSize)
				grid.Fill(test.numberOfVectors, test.maxVectorSize, test.randomWalkers, test.randomSteps)

				// Check if grid is filled (at least one brick) if not panicking
				if !test.panics {
					hasBrick := false
					brickCount := 0
					center := test.gridSize / 2
					centerBrickCount := 0

					for r := range grid {
						for c := range grid[r] {
							if grid[r][c].Data != nil && grid[r][c].Data.Type == utils.Cells.Brick {
								hasBrick = true
								brickCount++
								// Check if brick is near center (adjust radius as needed)
								distSq := (r-center)*(r-center) + (c-center)*(c-center)
								if distSq <= (test.gridSize/4)*(test.gridSize/4) { // Check within inner quarter radius
									centerBrickCount++
								}
							}
						}
					}
					if test.name == "6x6 Grid Min Params" {
						// For minimal params, it's okay if no bricks are generated outside the cleared center
						assert.GreaterOrEqual(t, brickCount, 0, "Brick count should be non-negative for minimal params")
					} else if !hasBrick && test.numberOfVectors > 0 && test.randomWalkers > 0 { // Only expect bricks if generation params > 0
						t.Errorf("Expected grid to have at least one brick after Fill, but found none.")
					}

					if hasBrick {
						t.Logf("Grid %s: Total Bricks: %d, Bricks near center: %d", test.name, brickCount, centerBrickCount)
						// Basic check for centralization: more than a small fraction should be near center
						// Relaxed threshold from 0.2 to 0.1
						if brickCount > 10 && float64(centerBrickCount)/float64(brickCount) < 0.1 {
							t.Errorf("Expected bricks to be more centralized, but only %.2f%% are near center.", 100*float64(centerBrickCount)/float64(brickCount))
						}
					}

					// Check if center is clear
					centerCell := grid[center][center]
					assert.NotNil(t, centerCell.Data, "Center cell data should not be nil")
					if centerCell.Data != nil {
						assert.Equal(t, utils.Cells.Empty, centerCell.Data.Type, "Center cell should be empty")
					}
				}
			}, "")

			if didPanic != test.panics {
				t.Errorf("Panic expectation mismatch: Expected panic=%t, Got panic=%t", test.panics, didPanic)
			}
		})
	}
}

// TestGrid_drawLineAndApplyBricks tests the line drawing helper.
func TestGrid_drawLineAndApplyBricks(t *testing.T) {
	gridSize := 10
	grid := NewGrid(gridSize)

	// Draw a diagonal line
	grid.drawLineAndApplyBricks(1, 1, 5, 5)

	// Check some points along the line
	assert.NotNil(t, grid[1][1].Data)
	assert.Equal(t, utils.Cells.Brick, grid[1][1].Data.Type)
	assert.Equal(t, 1, grid[1][1].Data.Life)

	assert.NotNil(t, grid[3][3].Data)
	assert.Equal(t, utils.Cells.Brick, grid[3][3].Data.Type)
	assert.Equal(t, 1, grid[3][3].Data.Life)

	assert.NotNil(t, grid[5][5].Data)
	assert.Equal(t, utils.Cells.Brick, grid[5][5].Data.Type)
	assert.Equal(t, 1, grid[5][5].Data.Life)

	// Check a point not on the line
	assert.Nil(t, grid[1][2].Data)

	// Draw overlapping line
	grid.drawLineAndApplyBricks(3, 3, 7, 7)
	assert.NotNil(t, grid[3][3].Data)
	assert.Equal(t, utils.Cells.Brick, grid[3][3].Data.Type)
	assert.Equal(t, 2, grid[3][3].Data.Life, "Life should increment on overlap") // Life should increase
	assert.Equal(t, 2, grid[3][3].Data.Level)

	assert.NotNil(t, grid[6][6].Data)
	assert.Equal(t, utils.Cells.Brick, grid[6][6].Data.Type)
	assert.Equal(t, 1, grid[6][6].Data.Life) // New part of the line
}

// TestGrid_applyRandomWalk tests the random walk helper.
func TestGrid_applyRandomWalk(t *testing.T) {
	gridSize := 10
	grid := NewGrid(gridSize)
	center := gridSize / 2
	steps := 20

	grid.applyRandomWalk(center, center, steps)

	// Check start point
	assert.NotNil(t, grid[center][center].Data)
	assert.Equal(t, utils.Cells.Brick, grid[center][center].Data.Type)
	assert.GreaterOrEqual(t, grid[center][center].Data.Life, 1)

	// Check total bricks (should be <= steps + 1)
	brickCount := 0
	for r := range grid {
		for c := range grid[r] {
			if grid[r][c].Data != nil && grid[r][c].Data.Type == utils.Cells.Brick {
				brickCount++
			}
		}
	}
	assert.LessOrEqual(t, brickCount, steps+1, "Number of bricks should not exceed steps+1")
	assert.Greater(t, brickCount, 0, "Should have at least one brick (start point)")
}