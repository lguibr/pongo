// File: game/grid.go
package game

import (
	"fmt"
	// "math" // Removed unused import
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
			// Initialize with nil Data, Fill will populate it
			grid[i][j] = Cell{X: i, Y: j, Data: nil}
		}
	}
	return grid
}

// Fill populates the grid with bricks using a centralized generation approach.
// It uses symmetrical vector drawing from the center and random walks starting near the center.
func (grid Grid) Fill(numberOfVectors, maxVectorSize, randomWalkers, randomSteps int) {
	gridSize := len(grid)
	if gridSize == 0 || gridSize%2 != 0 {
		panic("Grid size must be non-zero and even for Fill")
	}
	center := gridSize / 2 // Center index (e.g., for 16x16, center is 8)

	// Use default config values if parameters are zero
	cfg := utils.DefaultConfig()
	if numberOfVectors == 0 {
		numberOfVectors = cfg.GridFillVectors
	}
	if maxVectorSize == 0 {
		maxVectorSize = cfg.GridFillVectorSize
	}
	if randomWalkers == 0 {
		randomWalkers = cfg.GridFillWalkers
	}
	if randomSteps == 0 {
		randomSteps = cfg.GridFillSteps
	}

	// --- Phase 1: Symmetrical Vector Drawing from Center ---
	for i := 0; i < numberOfVectors; i++ {
		// Generate a random vector (can be positive or negative components)
		vecX := rand.Intn(maxVectorSize*2) - maxVectorSize
		vecY := rand.Intn(maxVectorSize*2) - maxVectorSize

		// Ensure vector has some length
		if vecX == 0 && vecY == 0 {
			if rand.Intn(2) == 0 {
				vecX = utils.RandomNumberN(maxVectorSize) // Get non-zero random number
			} else {
				vecY = utils.RandomNumberN(maxVectorSize)
			}
		}

		// Draw 4 symmetrical lines from the center
		grid.drawLineAndApplyBricks(center, center, center+vecX, center+vecY)
		grid.drawLineAndApplyBricks(center, center, center-vecX, center+vecY)
		grid.drawLineAndApplyBricks(center, center, center+vecX, center-vecY)
		grid.drawLineAndApplyBricks(center, center, center-vecX, center-vecY)
	}

	// --- Phase 2: Random Walkers from Center ---
	for i := 0; i < randomWalkers; i++ {
		grid.applyRandomWalk(center, center, randomSteps)
	}

	// --- Phase 3: Ensure Center is Clear (Optional but good for gameplay) ---
	// Clear a small area around the exact center to prevent immediate ball spawns inside bricks
	clearRadius := 1 // Clear center cell + immediate neighbors
	for r := center - clearRadius; r <= center+clearRadius; r++ {
		for c := center - clearRadius; c <= center+clearRadius; c++ {
			if r >= 0 && r < gridSize && c >= 0 && c < gridSize {
				// Set to empty, ensuring Data is not nil
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

// drawLineAndApplyBricks uses Bresenham's line algorithm (or similar) to mark cells along a line.
// Modifies the grid cells directly.
func (grid Grid) drawLineAndApplyBricks(x0, y0, x1, y1 int) {
	gridSize := len(grid)
	dx := utils.Abs(x1 - x0)
	dy := -utils.Abs(y1 - y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy // error value e_xy

	for { // loop
		// Apply brick logic to current point (x0, y0)
		if x0 >= 0 && x0 < gridSize && y0 >= 0 && y0 < gridSize {
			cell := &grid[x0][y0] // Get pointer to modify
			if cell.Data == nil {
				cell.Data = NewBrickData(utils.Cells.Brick, 1) // Start with life 1
			} else if cell.Data.Type == utils.Cells.Brick {
				cell.Data.Life++ // Increase life if already a brick
				cell.Data.Level = cell.Data.Life
			} else { // If empty or block, turn into brick
				cell.Data.Type = utils.Cells.Brick
				cell.Data.Life = 1
				cell.Data.Level = 1
			}
		}

		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy { // e_xy+e_x > 0
			err += dy
			x0 += sx
		}
		if e2 <= dx { // e_xy+e_y < 0
			err += dx
			y0 += sy
		}
	}
}

// applyRandomWalk performs a random walk starting from (startX, startY).
// Modifies the grid cells directly.
func (grid Grid) applyRandomWalk(startX, startY, numberOfSteps int) {
	gridSize := len(grid)
	if gridSize == 0 {
		return
	}

	// Ensure start point is valid (should be center, but check anyway)
	if startX < 0 || startX >= gridSize || startY < 0 || startY >= gridSize {
		fmt.Printf("WARN: Random walk start point (%d, %d) out of bounds for grid size %d\n", startX, startY, gridSize)
		return // Invalid start point
	}

	currentX, currentY := startX, startY

	// Apply brick to the starting cell of the walk
	startCell := &grid[currentX][currentY]
	if startCell.Data == nil {
		startCell.Data = NewBrickData(utils.Cells.Brick, 1)
	} else if startCell.Data.Type == utils.Cells.Brick {
		startCell.Data.Life++
		startCell.Data.Level = startCell.Data.Life
	} else {
		startCell.Data.Type = utils.Cells.Brick
		startCell.Data.Life = 1
		startCell.Data.Level = 1
	}

	for i := 0; i < numberOfSteps; i++ {
		// Generate potential next step (relative offset: N, S, E, W)
		dx, dy := 0, 0
		move := rand.Intn(4)
		switch move {
		case 0: // North
			dy = -1
		case 1: // South
			dy = 1
		case 2: // East
			dx = 1
		case 3: // West
			dx = -1
		}

		nextX, nextY := currentX+dx, currentY+dy

		// Check bounds
		if nextX < 0 || nextX >= gridSize || nextY < 0 || nextY >= gridSize {
			// If out of bounds, stay put for this step
			continue
		}

		// Update the cell at the next point
		nextCell := &grid[nextX][nextY] // Get pointer to modify
		if nextCell.Data == nil {
			nextCell.Data = NewBrickData(utils.Cells.Brick, 1)
		} else if nextCell.Data.Type == utils.Cells.Brick {
			nextCell.Data.Life++
			nextCell.Data.Level = nextCell.Data.Life // Update level
		} else {
			nextCell.Data.Type = utils.Cells.Brick
			nextCell.Data.Life = 1
			nextCell.Data.Level = 1
		}
		currentX, currentY = nextX, nextY // Update current position
	}
}

// Compare checks if two grids are identical in size and cell content.
// Handles nil grids correctly.
func (grid Grid) Compare(comparedGrid Grid) bool {
	// Case 1: Both nil
	if grid == nil && comparedGrid == nil {
		return true
	}
	// Case 2: One nil, one not nil
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