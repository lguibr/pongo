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

	for i := 0; i < len(cases); i++ {
		line := cases[i].Line
		expectedIndices := cases[i].ExpectedIndices
		gridSize := cases[i].GridSize

		canvas := NewGrid(gridSize)
		indices := canvas.LineIntersectedCellIndices(10, line)

		if len(indices) != len(expectedIndices) {
			t.Errorf("Expected %v indices, got %v", len(expectedIndices), len(indices))
		}

		for i, expected := range expectedIndices {
			currentValue := indices[i]
			if expected != currentValue {
				t.Errorf("Expected index %v, got %v", expected, currentValue)
			}
		}
	}

}

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
		for i := 0; i < 100; i++ {

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
	}
}

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
		grid := NewGrid(len(test.q1) * 2)
		grid.FillGridWithQuarterGrids(test.q1, test.q2, test.q3, test.q4)
		match := grid.Compare(test.expectedGrid)
		if !match {
			t.Errorf("Expected %v, got %v", test.expectedGrid, grid)
		}
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
		result := test.grid.Rotate()
		if !result.Compare(test.expected) {
			t.Errorf("Expected %v, got %v", test.expected, result)
		}
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
		for i := 0; i < 100; i++ {

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

	}
}

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
		result := test.grid.Compare(test.grid2)
		if result != test.result {
			t.Errorf("Test case '%s' failed: expected %v, got %v", test.name, test.result, result)
		}
	}
}
