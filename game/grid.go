// File: game/grid.go
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
			// Initialize with nil Data, GameActor will fill it
			grid[i][j] = Cell{X: i, Y: j, Data: nil}
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
		// Use default cell size from config for intersection check if needed,
		// though the logic here doesn't strictly depend on it.
		indexes = append(indexes, grid.LineIntersectedCellIndices(utils.DefaultConfig().CellSize, line)...)
	}

	for _, index := range indexes {
		// Check bounds before accessing grid element
		if index[0] < 0 || index[0] >= len(grid) || index[1] < 0 || index[1] >= len(grid[0]) {
			continue
		}

		// Initialize Data if nil
		if grid[index[0]][index[1]].Data == nil {
			// Use local NewBrickData
			grid[index[0]][index[1]].Data = NewBrickData(utils.Cells.Empty, 0)
		}

		if grid[index[0]][index[1]].Data.Type == utils.Cells.Brick {
			grid[index[0]][index[1]].Data.Life = grid[index[0]][index[1]].Data.Life + 1
			grid[index[0]][index[1]].Data.Level = grid[index[0]][index[1]].Data.Life // Update level too
			continue
		}

		// Change type to Brick and set life/level
		grid[index[0]][index[1]].Data.Type = utils.Cells.Brick
		grid[index[0]][index[1]].Data.Life = 1
		grid[index[0]][index[1]].Data.Level = 1

	}

}

func (grid Grid) FillGridWithQuarterGrids(q1, q2, q3, q4 Grid) {
	if len(q1) == 0 || len(q1) != len(q2) || len(q1) != len(q3) || len(q1) != len(q4) {
		panic("Quarter grids must be of the same non-zero size")
	}
	if len(grid) == 0 || len(grid) != 2*len(q1) || len(grid[0]) != 2*len(q1[0]) {
		panic("Main grid must be twice the size of the quarter grids")
	}

	n := len(grid)
	m := len(grid[0])
	qn := len(q1)
	qm := len(q1[0])

	for i := 0; i < qn; i++ {
		for j := 0; j < qm; j++ {
			// Deep copy cell data to avoid sharing pointers across quarters
			// Use local BrickData type
			copyData := func(data *BrickData) *BrickData {
				if data == nil {
					return nil
				}
				newData := *data
				return &newData
			}

			// Filling quarter one (top-left)
			grid[i][j] = q1[i][j]
			grid[i][j].X = i
			grid[i][j].Y = j
			grid[i][j].Data = copyData(q1[i][j].Data)

			// Filling quarter two (top-right)
			grid[i][m-1-j] = q2[i][j] // Use q2 data
			grid[i][m-1-j].X = i
			grid[i][m-1-j].Y = m - 1 - j
			grid[i][m-1-j].Data = copyData(q2[i][j].Data)

			// Filling quarter three (bottom-left)
			grid[n-1-i][j] = q3[i][j] // Use q3 data
			grid[n-1-i][j].X = n - 1 - i
			grid[n-1-i][j].Y = j
			grid[n-1-i][j].Data = copyData(q3[i][j].Data)

			// Filling quarter four (bottom-right)
			grid[n-1-i][m-1-j] = q4[i][j] // Use q4 data
			grid[n-1-i][m-1-j].X = n - 1 - i
			grid[n-1-i][m-1-j].Y = m - 1 - j
			grid[n-1-i][m-1-j].Data = copyData(q4[i][j].Data)
		}
	}
}

func (grid Grid) Rotate() Grid {
	if len(grid) == 0 || len(grid[0]) == 0 {
		return grid // Return empty or invalid grid as is
	}
	rows := len(grid)
	cols := len(grid[0])
	result := make([][]Cell, cols) // New grid dimensions are swapped
	for i := range result {
		result[i] = make([]Cell, rows)
	}
	for i, row := range grid {
		for j, cell := range row {
			result[j][rows-i-1] = cell // Assign original cell to rotated position
			// Update X, Y coordinates in the rotated cell
			result[j][rows-i-1].X = j
			result[j][rows-i-1].Y = rows - i - 1
		}
	}
	return result
}

func (grid Grid) RandomWalker(numberOfSteps int) {
	gridSize := len(grid)
	if gridSize == 0 {
		return
	} // Cannot walk on empty grid

	// Ensure start point is valid
	startX := gridSize / 2
	startY := gridSize / 2
	if startX < 0 || startX >= gridSize || startY < 0 || startY >= gridSize {
		return // Invalid start point
	}

	// Initialize start cell data if nil
	if grid[startX][startY].Data == nil {
		// Use local NewBrickData
		grid[startX][startY].Data = NewBrickData(utils.Cells.Empty, 0)
	}

	grid[startX][startY].Data.Type = utils.Cells.Brick
	grid[startX][startY].Data.Life = 1
	grid[startX][startY].Data.Level = 1

	currentPoint := [2]int{startX, startY}

	for i := 0; i < numberOfSteps; i++ {
		// Generate potential next step (relative offset)
		dx := utils.RandomNumberN(1) // -1 or 1
		dy := utils.RandomNumberN(1) // -1 or 1

		// Randomly choose to step horizontally or vertically
		nextPoint := currentPoint
		if rand.Intn(2) == 0 { // Step horizontally
			nextPoint[0] += dx
		} else { // Step vertically
			nextPoint[1] += dy
		}

		// Check bounds
		if nextPoint[0] < 0 || nextPoint[0] >= gridSize || nextPoint[1] < 0 || nextPoint[1] >= gridSize {
			// If out of bounds, stay put for this step (or try another direction?)
			// Staying put is simpler.
			continue
		}

		// Update the cell at the next point
		nextCell := &grid[nextPoint[0]][nextPoint[1]] // Get pointer to modify
		if nextCell.Data == nil {
			// Use local NewBrickData
			nextCell.Data = NewBrickData(utils.Cells.Empty, 0)
		}

		if nextCell.Data.Type == utils.Cells.Brick {
			nextCell.Data.Life++
			nextCell.Data.Level = nextCell.Data.Life // Update level
		} else {
			nextCell.Data.Type = utils.Cells.Brick
			nextCell.Data.Life = 1
			nextCell.Data.Level = 1
		}
		currentPoint = nextPoint // Update current position
	}
}

// Compare checks if two grids are identical in size and cell content.
// Handles nil grids correctly.
func (grid Grid) Compare(comparedGrid Grid) bool {
	// Case 1: Both nil
	if grid == nil && comparedGrid == nil {
		return true
	}
	// Case 2: One nil, one not nil - THIS IS THE CRITICAL CHECK
	if grid == nil || comparedGrid == nil {
		return false
	}
	// Case 3: Both non-nil
	// Check lengths first
	if len(grid) != len(comparedGrid) {
		return false
	}
	// Check elements (only if lengths match)
	for i := range grid {
		if len(grid[i]) != len(comparedGrid[i]) {
			return false
		}
		for j := range grid[i] {
			// Use the Cell.Compare method which handles nil Data pointers
			match := grid[i][j].Compare(comparedGrid[i][j])
			if !match {
				return false
			}
		}
	}
	// If all checks pass, they are equal
	return true
}

func (grid Grid) Fill(numberOfVectors, maxVectorSize, randomWalkers, randomSteps int) {
	if len(grid) == 0 || len(grid)%2 != 0 {
		panic("Grid size must be non-zero and even for Fill")
	}
	gridSize := len(grid)
	halfGridSize := gridSize / 2

	// Use default config values if parameters are zero
	cfg := utils.DefaultConfig()
	if numberOfVectors == 0 {
		numberOfVectors = cfg.GridFillVectors
	}
	if maxVectorSize == 0 {
		maxVectorSize = cfg.GridFillVectorSize
	}
	if randomWalkers == 0 {
		randomWalkers = cfg.GridFillWalkers // Correct field name
	}
	if randomSteps == 0 {
		randomSteps = cfg.GridFillSteps // Correct field name
	}

	quarters := [4]Grid{}

	for i := 0; i < 4; i++ {
		gridSeed := NewGrid(halfGridSize)
		gridSeed.CreateQuarterGridSeed(numberOfVectors, maxVectorSize)
		for j := 0; j < randomWalkers; j++ {
			gridSeed.RandomWalker(randomSteps)
		}
		quarters[i] = gridSeed // Store the generated quarter
	}

	// Rotate quarters appropriately before filling
	// Q1: Top-Left (no rotation needed)
	// Q2: Top-Right (needs 1 rotation from its seed generation perspective)
	// Q3: Bottom-Left (needs 3 rotations)
	// Q4: Bottom-Right (needs 2 rotations)
	grid.FillGridWithQuarterGrids(
		quarters[0],
		quarters[1].Rotate(),
		quarters[3].Rotate().Rotate().Rotate(), // Q3 needs 3 rotations total
		quarters[2].Rotate().Rotate(),          // Q4 needs 2 rotations total
	)
}
