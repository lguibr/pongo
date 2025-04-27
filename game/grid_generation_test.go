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

	// Check that all cells are empty
	for i := range grid {
		for j := range grid[i] {
			if grid[i][j].Data.Type != utils.Cells.Empty {
				t.Errorf("Expected cell at position (%d, %d) to be empty, but got %s", i, j, grid[i][j].Data.Type)
			}
			if grid[i][j].Data.Life != 0 {
				t.Errorf("Expected cell at position (%d, %d) to have life 0, but got %d", i, j, grid[i][j].Data.Life)
			}
		}
	}
}

func TestCreateQuarterGridSeed(t *testing.T) {
	type TestCreateQuarterGridSeedTestCase struct {
		gridSize                int
		numberOfVectors         int
		maxVectorSize           int
		expectedBrickCellsCount float64
	}

	testCases := []TestCreateQuarterGridSeedTestCase{
		{10, 5, 5, float64(5 * 5)},
		{20, 10, 8, float64(10 * 8)},
		{30, 15, 12, float64(15 * 12)},
	}

	for _, test := range testCases {
		t.Run("QuarterGridSeed", func(t *testing.T) {
			for i := 0; i < 100; i++ { // Run multiple times due to randomness

				// set up test grid
				grid := Grid{}
				for i := 0; i < test.gridSize; i++ {
					row := []Cell{}
					for j := 0; j < test.gridSize; j++ {
						cell := Cell{X: i, Y: j, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}
						row = append(row, cell)
					}
					grid = append(grid, row)
				}

				grid.CreateQuarterGridSeed(test.numberOfVectors, test.maxVectorSize)

				// check that the correct number of cells have been modified
				count := 0
				for i := range grid {
					for j := range grid[i] {
						if grid[i][j].Data.Type == utils.Cells.Brick {
							count += grid[i][j].Data.Life
						}
					}
				}
				if float64(count) > test.expectedBrickCellsCount {
					t.Errorf("Expected %f Brick cells, got %d", test.expectedBrickCellsCount, count)
				}

				// check that all modified cells are in the top-left quarter of the grid
				for i := range grid {
					for j := range grid[i] {
						if grid[i][j].Data.Type == utils.Cells.Brick {
							if i > (test.gridSize/2-1) || j > (test.gridSize/2-1) {
								t.Errorf("Brick cell at (%d, %d) is not in the top-left quarter of the grid", i, j)
							}
						}
					}
				}
			}
		})
	}
}

func TestGrid_RandomWalker(t *testing.T) {
	type RandomWalkerTestCase struct {
		grid        Grid
		steps       int
		totalBricks int
	}

	testCases := []RandomWalkerTestCase{
		{
			grid:        NewGrid(10),
			steps:       10,
			totalBricks: 10,
		},
		{
			grid:        NewGrid(10),
			steps:       100,
			totalBricks: 100,
		},
		{
			grid:        NewGrid(10),
			steps:       1000,
			totalBricks: 1000,
		},
	}

	for _, test := range testCases {
		t.Run("RandomWalker", func(t *testing.T) {
			test.grid.RandomWalker(test.steps)
			totalBricks := 0
			for i := range test.grid {
				for j := range test.grid[i] {
					if test.grid[i][j].Data.Type == utils.Cells.Brick {
						totalBricks += test.grid[i][j].Data.Life
					}
				}
			}
			if totalBricks != test.totalBricks {
				t.Errorf("Expected %d bricks after %d steps, got %d", test.totalBricks, test.steps, totalBricks)
			}
		})
	}
}
