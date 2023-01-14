package game

import (
	"testing"
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
	//TODO To be implemented

}
func TestGrid_CreateQuarterGridSeed(t *testing.T) {
	//TODO To be implemented

}
func TestGrid_Rotate(t *testing.T) {
	//TODO To be implemented

}

func TestGrid_FillGridWithQuarterGrids(t *testing.T) {
	grids := [4]Grid{}
	canvasSize := 8
	halfCanvasSize := canvasSize / 2

	numberOfVectors := canvasSize - canvasSize/3
	maxVectorSize := (canvasSize / 3)

	for i := 0; i < 4; i++ {

		gridSeed := NewGrid(halfCanvasSize)
		gridSeed.CreateQuarterGridSeed(numberOfVectors, maxVectorSize)

		grids[i] = gridSeed.Rotate().Rotate()

	}

	finalCanvas := NewCanvas(canvasSize, canvasSize)

	finalCanvas.Grid.FillGridWithQuarterGrids(
		grids[0],
		grids[1],
		grids[2],
		grids[3],
	)

	//TODO To be implemented

}

func TestGrid_FillGrid(t *testing.T) {
	for i := 0; i < 4; i++ {
		canvasSize := 2
		grid := NewGrid(canvasSize)
		numberOfVectors := 500
		maxVectorSize := 10
		grid.CreateQuarterGridSeed(numberOfVectors, maxVectorSize)

		if grid[0][0].Data.Life != numberOfVectors {
			t.Errorf("Expected %v, got %v", numberOfVectors, grid[0][0])
		}
	}

	//TODO To be implemented
}

func TestGrid_RandomWalker(t *testing.T) {
	//TODO To be implemented

}
