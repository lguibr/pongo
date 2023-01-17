package game

import (
	"math/rand"

	"github.com/lguibr/pongo/utils"
)

type Grid [][]Cell

func (grid Grid) LineIntersectedCellIndices(cellSize int, line [2][2]int) [][2]int {
	var intersects [][2]int
	for i := range grid {
		for j := range grid[i] {
			if line[0][0] <= i && i <= line[1][0] && line[0][1] <= j && j <= line[1][1] {
				intersects = append(intersects, [2]int{i, j})
			}
		}
	}
	return intersects
}

func NewGrid(gridSize int) Grid {
	grid := make(Grid, gridSize)
	for i := range grid {
		grid[i] = make([]Cell, gridSize)
	}

	for i, row := range grid {
		for j := range row {
			data := &BrickData{Type: "Empty", Life: 0}
			grid[i][j] = Cell{X: i, Y: j, Data: data}
		}
	}
	return grid
}

func (grid Grid) CreateQuarterGridSeed(numberOfVectors, maxVectorSize int) {
	vectorZero := [2]int{0, 0}
	randomVectors := utils.NewRandomPositiveVectors(numberOfVectors, maxVectorSize)

	randomLines := [][2][2]int{}
	for _, vector := range randomVectors {
		randomLines = append(randomLines, [2][2]int{vectorZero, vector})
	}

	indexes := [][2]int{}
	for _, line := range randomLines {
		indexes = append(indexes, grid.LineIntersectedCellIndices(utils.CellSize, line)...)
	}

	for _, index := range indexes {
		if grid[index[0]][index[1]].Data.Type == "Brick" {
			grid[index[0]][index[1]].Data.Life = grid[index[0]][index[1]].Data.Life + 1
			continue
		}

		grid[index[0]][index[1]] = Cell{
			X: index[0],
			Y: index[1],
			Data: &BrickData{
				Type: "Brick",
				Life: 1,
			},
		}

	}

}

func (grid Grid) FillGridWithQuarterGrids(q1, q2, q3, q4 Grid) {

	if len(q1) != len(q2) || len(q1) != len(q3) || len(q1) != len(q4) || len(q1) == 0 {
		panic("Grids must be of the same size")
	}
	if len(grid) != 2*len(q1) || len(grid) == 0 {
		panic("Grid must be twice the size of the quarter grids")
	}

	n := len(grid)
	m := len(grid[0])

	for i := 0; i < n/2; i++ {

		for j := 0; j < m/2; j++ {

			//INFO Filling quarter one of the grid
			grid[i][j] = q1[i][j]
			grid[i][j].X = i //INFO Fixing the X value
			grid[i][j].Y = j //INFO Fixing the Y value

			//INFO Filling quarter two of the grid
			grid[i][m-1-j] = q2[i][j]
			grid[i][m-1-j].X = i
			grid[i][m-1-j].Y = m - 1 - j

			//INFO Filling quarter three of the grid
			grid[n-1-i][j] = q3[i][j]
			grid[n-1-i][j].X = n - 1 - i
			grid[n-1-i][j].Y = j

			//INFO Filling quarter four of the grid
			grid[n-1-i][m-1-j] = q4[i][j]
			grid[n-1-i][m-1-j].X = n - 1 - i
			grid[n-1-i][m-1-j].Y = m - 1 - j

		}

	}
}

func (grid Grid) Rotate() Grid {
	result := make([][]Cell, len(grid[0]))
	for i := range result {
		result[i] = make([]Cell, len(grid))
	}
	for i, row := range grid {
		for j, cell := range row {
			result[j][len(grid)-i-1] = cell
		}
	}
	return result
}

func (grid Grid) RandomWalker(numberOfSteps int) {
	gridSize := len(grid)
	startPoint := [2]int{rand.Intn(gridSize), rand.Intn(gridSize)}
	grid[startPoint[0]][startPoint[1]].Data.Type = "Brick"
	grid[startPoint[0]][startPoint[1]].Data.Life = 1
	var getNextPoint func(currentPoint [2]int) [2]int
	getNextPoint = func(currentPoint [2]int) [2]int {

		nextPoint := [2]int{currentPoint[0] + utils.RandomNumber(1), currentPoint[1] + utils.RandomNumber(1)}
		if nextPoint[0] < 0 || nextPoint[0] >= gridSize || nextPoint[1] < 0 || nextPoint[1] >= gridSize {
			return getNextPoint(currentPoint)
		}
		return nextPoint
	}

	stepsResting := numberOfSteps - 1

	for i := 0; i < stepsResting; i++ {
		nextPoint := getNextPoint(startPoint)
		nextCell := grid[nextPoint[0]][nextPoint[1]]
		if nextCell.Data.Type == "Brick" {
			nextCell.Data.Life++
		} else {
			nextCell.Data.Type = "Brick"
			nextCell.Data.Life = 1
		}
	}
}

func (grid Grid) Compare(comparedGrid Grid) bool {
	if len(grid) != len(comparedGrid) {
		return false
	}
	for i := range grid {
		if len(grid[i]) != len(comparedGrid[i]) {
			return false
		}
		for j := range grid[i] {
			match := grid[i][j].Compare(comparedGrid[i][j])
			if !match {
				return false
			}
		}
	}
	return true
}

func (grid Grid) Fill(numberOfVectors, maxVectorSize, randomWalkers, randomSteps int) {

	if numberOfVectors == 0 {
		numberOfVectors = utils.NumberOfVectors
	}
	if maxVectorSize == 0 {
		maxVectorSize = utils.MaxVectorSize
	}
	if randomWalkers == 0 {
		randomWalkers = utils.NumberOfRandomWalkers
	}
	if randomSteps == 0 {
		randomSteps = utils.NumberOfRandomSteps
	}

	gridSize := len(grid)
	halfGridSize := gridSize / 2

	quarters := [4]Grid{}

	for i := 0; i < 4; i++ {

		gridSeed := NewGrid(halfGridSize)
		gridSeed.CreateQuarterGridSeed(numberOfVectors, maxVectorSize)
		gridSeed.RandomWalker(randomSteps)
		quarters[i] = gridSeed.Rotate().Rotate()

	}

	grid.FillGridWithQuarterGrids(
		quarters[0],
		quarters[1],
		quarters[2],
		quarters[3],
	)

}
