// File: game/grid_fill_test.go
package game

import (
	"testing"

	"github.com/lguibr/pongo/utils"
)

func TestGrid_FillGridWithQuarterGrids(t *testing.T) {
	type FillGridWithQuarterGridsTestCase struct {
		q1, q2, q3, q4 Grid
		expectedGrid   Grid
	}

	testCases := []FillGridWithQuarterGridsTestCase{

		{
			// Test case 2: q1, q2, q3, and q4 have different values
			q1: Grid{{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}}},
			q2: Grid{{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}}},
			q3: Grid{{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}}},
			q4: Grid{{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}}},
			expectedGrid: Grid{
				{Cell{X: 0, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 1}}, Cell{X: 0, Y: 1, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}},
				{Cell{X: 1, Y: 0, Data: &BrickData{Type: utils.Cells.Brick, Life: 2}}, Cell{X: 1, Y: 1, Data: &BrickData{Type: utils.Cells.Empty, Life: 0}}},
			},
		},
	}

	for _, test := range testCases {
		t.Run("FillWithQuarters", func(t *testing.T) {
			grid := NewGrid(len(test.q1) * 2)
			grid.FillGridWithQuarterGrids(test.q1, test.q2, test.q3, test.q4)
			match := grid.Compare(test.expectedGrid)
			if !match {
				t.Errorf("Expected %v, got %v", test.expectedGrid, grid)
			}
		})
	}

}

func TestGrid_Fill(t *testing.T) {
	type FillTestCase struct {
		grid            Grid
		numberOfVectors int
		maxVectorSize   int
		randomSteps     int
		randomWalkers   int
		totalBricks     int
	}

	testCases := []FillTestCase{
		{
			grid:            NewGrid(10),
			numberOfVectors: 2,
			maxVectorSize:   2,
			randomSteps:     2,
			randomWalkers:   2,
			totalBricks:     (2 * 2) + (2*2)*4, //INFO ( (numberOfVectors * maxVectorSize + ) + (randomWalkers * randomSteps)) * 4
		},
	}

	for _, test := range testCases {
		t.Run("FillGrid", func(t *testing.T) {
			for i := 0; i < 100; i++ { // Run multiple times due to randomness

				test.grid.Fill(test.numberOfVectors, test.maxVectorSize, test.randomSteps, test.randomWalkers)
				totalBricks := 0
				for i := range test.grid {
					for j := range test.grid[i] {
						if test.grid[i][j].Data.Type == utils.Cells.Brick {
							totalBricks += test.grid[i][j].Data.Life
						}
					}
				}
				if totalBricks > test.totalBricks {
					t.Errorf("Expected max of %d bricks after %d steps, got %d", test.totalBricks, test.randomSteps, totalBricks)
				}
			}
		})
	}
}
