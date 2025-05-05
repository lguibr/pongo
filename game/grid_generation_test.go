// File: game/grid_generation_test.go
package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
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

	// Check that all cells have non-nil Empty Data initially
	for i := range grid {
		for j := range grid[i] {
			if grid[i][j].Data == nil {
				t.Errorf("Expected cell at position (%d, %d) to have non-nil Data initially, but got nil", i, j)
			} else if grid[i][j].Data.Type != utils.Cells.Empty {
				t.Errorf("Expected cell at position (%d, %d) to have Empty type initially, but got %v", i, j, grid[i][j].Data.Type)
			}
		}
	}
}

// TestGrid_FillSymmetrical tests the new symmetrical fill method.
func TestGrid_FillSymmetrical(t *testing.T) {
	type FillSymmetricalTestCase struct {
		name              string
		gridSize          int
		density           float64
		centerClearRadius int
		wallClearDist     int
		brickMinLife      int // Add life config
		brickMaxLife      int
		expectPanic       bool
		expectBricks      bool // Whether bricks are expected (depends on params)
	}

	testCases := []FillSymmetricalTestCase{
		{"Size10_Density0.5_Clear1_Wall2_Life1_3", 10, 0.5, 1, 2, 1, 3, false, true}, // Expect some bricks
		{"Size6_Density0.8_Clear1_Wall1_Life1_1", 6, 0.8, 1, 1, 1, 1, false, true},   // Expect some bricks
		{"Size6_Density0.1_Clear1_Wall1_Life1_1", 6, 0.1, 1, 1, 1, 1, false, false},  // Might be empty
		{"Size16_Density0.3_Clear2_Wall3_Life2_5", 16, 0.3, 2, 3, 2, 5, false, true},
		{"Size10_Density1.0_Clear0_Wall0_Life1_1", 10, 1.0, 0, 0, 1, 1, false, true}, // Fill almost everything
		{"Size10_Density0.0_Clear1_Wall1_Life1_1", 10, 0.0, 1, 1, 1, 1, false, false}, // Should be empty
		{"Size4_Density1.0_Clear1_Wall1_Life1_1", 4, 1.0, 1, 1, 1, 1, false, false},   // Clear zones leave no space
		{"OddSize", 9, 0.5, 1, 1, 1, 1, true, false},                                  // Should panic (NewGrid)
		{"ZeroSize", 0, 0.5, 1, 1, 1, 1, true, false},                                 // Should panic (NewGrid)
		{"InvalidLifeRange", 10, 0.5, 1, 1, 3, 1, false, true},                        // MaxLife < MinLife, should be corrected internally
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			didPanic, panicMsg := utils.AssertPanics(t, func() {
				grid := NewGrid(tc.gridSize) // This might panic for invalid sizes

				// Create a config based on test case parameters
				cfg := utils.DefaultConfig()
				cfg.GridSize = tc.gridSize // Use test case grid size
				// Ensure CanvasSize is compatible if GridSize changed significantly
				// For simplicity, assume default CanvasSize is okay or adjust if needed
				// if cfg.CanvasSize % cfg.GridSize != 0 { cfg.CanvasSize = cfg.GridSize * (cfg.CanvasSize / utils.DefaultConfig().GridSize) }
				cfg.GridFillDensity = tc.density
				cfg.GridClearCenterRadius = tc.centerClearRadius
				cfg.GridClearWallDistance = tc.wallClearDist
				cfg.GridBrickMinLife = tc.brickMinLife
				cfg.GridBrickMaxLife = tc.brickMaxLife

				// Call FillSymmetrical with the constructed config
				grid.FillSymmetrical(cfg)

				if !tc.expectPanic {
					center := float64(tc.gridSize) / 2.0 // Use float for center calculation
					brickCount := 0
					symmetryOk := true // Assume true initially
					minLifeFound := 999
					maxLifeFound := 0

					// Check counts and clear zones
					for r := 0; r < tc.gridSize; r++ {
						for c := 0; c < tc.gridSize; c++ {
							cellData := grid[r][c].Data
							isBrick := cellData != nil && cellData.Type == utils.Cells.Brick

							if isBrick {
								brickCount++
								// Track min/max life found
								if cellData.Life < minLifeFound {
									minLifeFound = cellData.Life
								}
								if cellData.Life > maxLifeFound {
									maxLifeFound = cellData.Life
								}

								// Check center clear zone
								cellCenterX := float64(c) + 0.5
								cellCenterY := float64(r) + 0.5
								distFromCenterSq := (cellCenterX-center)*(cellCenterX-center) + (cellCenterY-center)*(cellCenterY-center)
								assert.GreaterOrEqual(t, distFromCenterSq, float64(tc.centerClearRadius*tc.centerClearRadius), "Brick found inside center clear radius at (%d, %d)", r, c)

								// Check wall clear zone
								assert.GreaterOrEqual(t, r, tc.wallClearDist, "Brick found too close to left wall at (%d, %d)", r, c)
								assert.Less(t, r, tc.gridSize-tc.wallClearDist, "Brick found too close to right wall at (%d, %d)", r, c)
								assert.GreaterOrEqual(t, c, tc.wallClearDist, "Brick found too close to top wall at (%d, %d)", r, c)
								assert.Less(t, c, tc.gridSize-tc.wallClearDist, "Brick found too close to bottom wall at (%d, %d)", r, c)
							}

							// --- Simplified Symmetry Check (180 degrees) ---
							r180 := tc.gridSize - 1 - r
							c180 := tc.gridSize - 1 - c
							cell180Data := grid[r180][c180].Data
							isBrick180 := cell180Data != nil && cell180Data.Type == utils.Cells.Brick

							if isBrick != isBrick180 {
								symmetryOk = false
							}
							// Check life symmetry if needed
							if isBrick && isBrick180 && cellData.Life != cell180Data.Life {
								symmetryOk = false
								t.Logf("Symmetry Life Fail: Cell (%d, %d) life=%d vs Cell (%d, %d) life=%d", r, c, cellData.Life, r180, c180, cell180Data.Life)
							}
						}
					}

					// Assertions based on expectations
					if tc.expectBricks {
						assert.Positive(t, brickCount, "Expected at least one brick when expectBricks is true")
						// Check if found life is within expected range (corrected for invalid input)
						expectedMinLife := tc.brickMinLife
						if expectedMinLife < 1 {
							expectedMinLife = 1
						}
						expectedMaxLife := tc.brickMaxLife
						if expectedMaxLife < expectedMinLife {
							expectedMaxLife = expectedMinLife
						}
						assert.GreaterOrEqual(t, minLifeFound, expectedMinLife, "Min life found (%d) is less than expected min (%d)", minLifeFound, expectedMinLife)
						assert.LessOrEqual(t, maxLifeFound, expectedMaxLife, "Max life found (%d) is greater than expected max (%d)", maxLifeFound, expectedMaxLife)

					} else {
						// Relaxed assertion for low density / small grids
						assert.LessOrEqual(t, brickCount, 10, "Expected very few or no bricks when expectBricks is false")
					}

					assert.True(t, symmetryOk, "Grid generation should be symmetrical (180 deg)")
				}

			}, "")

			if didPanic != tc.expectPanic {
				t.Errorf("Panic expectation mismatch: Expected panic=%t, Got panic=%t. Msg: %s", tc.expectPanic, didPanic, panicMsg)
			}
		})
	}
}