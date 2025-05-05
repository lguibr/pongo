// File: game/grid.go
package game

import (
	"fmt"
	"math/rand"

	"github.com/lguibr/pongo/utils"
)

type Grid [][]Cell

// NewGrid initializes an empty grid of the specified size.
func NewGrid(gridSize int) Grid {
	if gridSize <= 0 {
		panic("Grid size must be positive")
	}
	grid := make(Grid, gridSize)
	for i := range grid {
		grid[i] = make([]Cell, gridSize)
		for j := range grid[i] {
			// Initialize with Empty Data using the function from cell.go
			grid[i][j] = Cell{X: i, Y: j, Data: NewBrickData(utils.Cells.Empty, 0)}
		}
	}
	return grid
}

// FillSymmetrical populates the grid with bricks ensuring rotational symmetry and clear zones.
// Uses configuration for density and life range.
func (grid Grid) FillSymmetrical(cfg utils.Config) {
	gridSize := len(grid)
	if gridSize == 0 || gridSize%2 != 0 {
		panic("Grid size must be non-zero and even for FillSymmetrical")
	}
	center := gridSize / 2
	density := cfg.GridFillDensity
	centerClearRadius := cfg.GridClearCenterRadius
	wallClearDist := cfg.GridClearWallDistance
	minLife := cfg.GridBrickMinLife // Accessing config field
	maxLife := cfg.GridBrickMaxLife // Accessing config field

	// Ensure minLife is at least 1
	if minLife < 1 {
		minLife = 1
	}
	// Ensure maxLife is at least minLife
	if maxLife < minLife {
		maxLife = minLife
	}
	lifeRange := maxLife - minLife + 1

	// 1. Define the safe quadrant boundaries (top-left quadrant example)
	minCoord := wallClearDist
	maxCoord := center - 1 // Quadrant boundary (inclusive)

	if minCoord > maxCoord {
		fmt.Printf("WARN: FillSymmetrical - wallClearDist (%d) and centerClearRadius (%d) leave no space in quadrant for grid size %d. Grid will be empty.\n", wallClearDist, centerClearRadius, gridSize)
	} else {
		// 2. Generate pattern in the top-left safe quadrant
		for r := minCoord; r <= maxCoord; r++ {
			for c := minCoord; c <= maxCoord; c++ {
				cellCenterX := float64(c) + 0.5
				cellCenterY := float64(r) + 0.5
				gridCenter := float64(gridSize) / 2.0
				distFromCenterSq := (cellCenterX-gridCenter)*(cellCenterX-gridCenter) + (cellCenterY-gridCenter)*(cellCenterY-gridCenter)

				if distFromCenterSq < float64(centerClearRadius*centerClearRadius) {
					continue // Skip cells within the center clear radius
				}

				// Place brick based on density
				if rand.Float64() < density {
					// Calculate life based on config range
					life := minLife
					if lifeRange > 1 {
						life += rand.Intn(lifeRange)
					}
					grid.setBrickWithSymmetry(r, c, life)
				}
			}
		}
	}

	// 3. Explicitly clear center and wall zones again
	grid.clearCenterZone(center, centerClearRadius)
	grid.clearWallZones(wallClearDist)
}

// setBrickWithSymmetry places a brick at (r, c) and its symmetrical counterparts.
func (grid Grid) setBrickWithSymmetry(r, c, life int) {
	gridSize := len(grid)

	positions := [4][2]int{
		{r, c},                               // Original (Top-Left Quadrant assumed)
		{c, gridSize - 1 - r},                // Rotated 90 deg clockwise (Top-Right Quadrant)
		{gridSize - 1 - r, gridSize - 1 - c}, // Rotated 180 deg (Bottom-Right Quadrant)
		{gridSize - 1 - c, r},                // Rotated 270 deg clockwise (Bottom-Left Quadrant)
	}

	for _, pos := range positions {
		row, col := pos[0], pos[1]
		if row >= 0 && row < gridSize && col >= 0 && col < gridSize {
			if grid[row][col].Data == nil || grid[row][col].Data.Type == utils.Cells.Empty {
				grid[row][col].Data = NewBrickData(utils.Cells.Brick, life)
			}
		}
	}
}

// clearCenterZone ensures the central area is empty.
func (grid Grid) clearCenterZone(center, radius int) {
	gridSize := len(grid)
	gridCenter := float64(gridSize) / 2.0
	for r := 0; r < gridSize; r++ {
		for c := 0; c < gridSize; c++ {
			cellCenterX := float64(c) + 0.5
			cellCenterY := float64(r) + 0.5
			distFromCenterSq := (cellCenterX-gridCenter)*(cellCenterX-gridCenter) + (cellCenterY-gridCenter)*(cellCenterY-gridCenter)

			if distFromCenterSq < float64(radius*radius) {
				if grid[r][c].Data == nil {
					grid[r][c].Data = NewBrickData(utils.Cells.Empty, 0)
				} else {
					grid[r][c].Data.Type = utils.Cells.Empty
					grid[r][c].Data.Life = 0
					grid[r][c].Data.Level = 0
				}
			}
		}
	}
}

// clearWallZones ensures areas near the walls are empty.
func (grid Grid) clearWallZones(distance int) {
	gridSize := len(grid)
	if distance <= 0 {
		return
	}
	for i := 0; i < gridSize; i++ {
		for d := 0; d < distance; d++ {
			if d < gridSize {
				if grid[i][d].Data == nil {
					grid[i][d].Data = NewBrickData(utils.Cells.Empty, 0)
				} else {
					grid[i][d].Data.Type = utils.Cells.Empty
					grid[i][d].Data.Life = 0
					grid[i][d].Data.Level = 0
				}
			}
			if gridSize-1-d >= 0 {
				if grid[i][gridSize-1-d].Data == nil {
					grid[i][gridSize-1-d].Data = NewBrickData(utils.Cells.Empty, 0)
				} else {
					grid[i][gridSize-1-d].Data.Type = utils.Cells.Empty
					grid[i][gridSize-1-d].Data.Life = 0
					grid[i][gridSize-1-d].Data.Level = 0
				}
			}
			if d < gridSize {
				if grid[d][i].Data == nil {
					grid[d][i].Data = NewBrickData(utils.Cells.Empty, 0)
				} else {
					grid[d][i].Data.Type = utils.Cells.Empty
					grid[d][i].Data.Life = 0
					grid[d][i].Data.Level = 0
				}
			}
			if gridSize-1-d >= 0 {
				if grid[gridSize-1-d][i].Data == nil {
					grid[gridSize-1-d][i].Data = NewBrickData(utils.Cells.Empty, 0)
				} else {
					grid[gridSize-1-d][i].Data.Type = utils.Cells.Empty
					grid[gridSize-1-d][i].Data.Life = 0
					grid[gridSize-1-d][i].Data.Level = 0
				}
			}
		}
	}
}

// Compare checks if two grids are identical in size and cell content.
// Handles nil grids correctly.
func (grid Grid) Compare(comparedGrid Grid) bool {
	if grid == nil && comparedGrid == nil {
		return true
	}
	if grid == nil || comparedGrid == nil {
		return false
	}
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