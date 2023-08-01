package game

import (
	"github.com/lguibr/pongo/types"
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
		size = utils.CanvasSize
	}
	if gridSize == 0 {
		gridSize = utils.GridSize
	}
	if size%gridSize != 0 {
		panic("Size must be a multiple of gridSize")
	}

	if gridSize < 6 {
		panic("GridSize must be greater or equal than 6")
	}

	return &Canvas{
		Grid:       NewGrid(gridSize),
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

			if cell.Data.Life >= 1 {
				x, y := cell.GetX()*canvas.GetCellSize(), cell.GetY()*canvas.GetCellSize()
				for i := 0; i < canvas.GetCellSize(); i++ {
					for j := 0; j < canvas.GetCellSize(); j++ {
						grid[x+i][y+j] = brickColor
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
				grid[i][j] = paddleColor
			}
		}
	}

	// Draw the balls on the RGB grid
	for _, ball := range balls {
		if ball == nil {
			continue
		}

		startX := ball.GetX() - ball.GetRadius()
		if startX < 0 {
			startX = 0
		}

		startY := ball.GetY() - ball.GetRadius()
		if startY < 0 {
			startY = 0
		}

		for i := startX; i <= ball.GetX()+ball.GetRadius() && i < len(grid); i++ {
			for j := startY; j <= ball.GetY()+ball.GetRadius() && j < len(grid[i]); j++ {
				// Check if the pixel lies inside the ball using the equation of a circle
				if (i-ball.GetX())*(i-ball.GetX())+(j-ball.GetY())*(j-ball.GetY()) <= ball.GetRadius()*ball.GetRadius() {
					grid[i][j] = ballColor
				}
			}
		}
	}

	return grid
}
