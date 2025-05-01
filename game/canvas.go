package game

import (
	"fmt"

	"github.com/lguibr/asciiring/types"
	"github.com/lguibr/pongo/utils"
)

type Canvas struct {
	Grid       Grid `json:"grid"`
	Width      int  `json:"width"`
	Height     int  `json:"height"`
	GridSize   int  `json:"gridSize"`
	CanvasSize int  `json:"canvasSize"`
	CellSize   int  `json:"cellSize"`
}

func (c *Canvas) GetGrid() [][]Cell  { return c.Grid }
func (c *Canvas) GetCanvasSize() int { return c.CanvasSize }
func (c *Canvas) GetCellSize() int   { return c.CellSize }

func NewCanvas(size, gridSize int) *Canvas {

	if size == 0 {
		size = utils.DefaultConfig().CanvasSize // Use default if 0
	}
	if gridSize == 0 {
		gridSize = utils.DefaultConfig().GridSize // Use default if 0
	}
	if size%gridSize != 0 {
		panic(fmt.Sprintf("Canvas size (%d) must be a multiple of grid size (%d)", size, gridSize))
	}

	if gridSize < 6 {
		panic("GridSize must be greater or equal than 6")
	}

	return &Canvas{
		Grid:       NewGrid(gridSize), // Grid starts empty, filled by GameActor later
		Width:      size,
		Height:     size,
		GridSize:   gridSize,
		CanvasSize: size,
		CellSize:   size / gridSize,
	}
}

func (canvas *Canvas) DrawGameOnRGBGrid(paddles [4]*Paddle, balls []*Ball) [][]types.RGBPixel {

	// Initialize empty RGB grid
	grid := make([][]types.RGBPixel, canvas.GetCanvasSize())
	for i := range grid {
		grid[i] = make([]types.RGBPixel, canvas.GetCanvasSize())
	}

	// Define colors for different game objects
	paddleColor := types.RGBPixel{R: 0, G: 255, B: 0} // white
	brickColor := types.RGBPixel{R: 255, G: 0, B: 0}  // red
	ballColor := types.RGBPixel{R: 0, G: 0, B: 255}   // blue

	// Draw the bricks on the RGB grid
	for _, row := range canvas.GetGrid() {
		for _, cell := range row {

			if cell.Data != nil && cell.Data.Life >= 1 { // Check cell.Data is not nil
				x, y := cell.GetX()*canvas.GetCellSize(), cell.GetY()*canvas.GetCellSize()
				for i := 0; i < canvas.GetCellSize(); i++ {
					for j := 0; j < canvas.GetCellSize(); j++ {
						// Boundary check for drawing pixels
						pixelX, pixelY := x+i, y+j
						if pixelX >= 0 && pixelX < canvas.GetCanvasSize() && pixelY >= 0 && pixelY < canvas.GetCanvasSize() {
							grid[pixelX][pixelY] = brickColor
						}
					}
				}
			}
		}
	}

	// Draw the paddles on the RGB grid
	for _, paddle := range paddles {
		if paddle == nil {
			continue
		}
		for i := paddle.GetX(); i < paddle.GetX()+paddle.GetWidth(); i++ {
			for j := paddle.GetY(); j < paddle.GetY()+paddle.GetHeight(); j++ {
				// Boundary check for drawing pixels
				if i >= 0 && i < canvas.GetCanvasSize() && j >= 0 && j < canvas.GetCanvasSize() {
					grid[i][j] = paddleColor
				}
			}
		}
	}

	// Draw the balls on the RGB grid
	for _, ball := range balls {
		if ball == nil {
			continue
		}

		startX := ball.GetX() - ball.GetRadius()
		startY := ball.GetY() - ball.GetRadius()
		endX := ball.GetX() + ball.GetRadius()
		endY := ball.GetY() + ball.GetRadius()

		// Clamp drawing bounds to canvas limits
		startX = utils.MaxInt(0, startX)
		startY = utils.MaxInt(0, startY)
		endX = utils.MinInt(canvas.GetCanvasSize()-1, endX)
		endY = utils.MinInt(canvas.GetCanvasSize()-1, endY)

		for i := startX; i <= endX; i++ {
			for j := startY; j <= endY; j++ {
				// Check if the pixel lies inside the ball using the equation of a circle
				if (i-ball.GetX())*(i-ball.GetX())+(j-ball.GetY())*(j-ball.GetY()) <= ball.GetRadius()*ball.GetRadius() {
					grid[i][j] = ballColor
				}
			}
		}
	}

	return grid
}
