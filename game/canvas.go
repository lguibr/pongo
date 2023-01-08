package game

import (
	"encoding/json"
	"fmt"

	"github.com/lguibr/pongo/utils"
)

type Canvas struct {
	Grid       Grid `json:"grid"`
	Width      int  `json:"width"`
	Height     int  `json:"height"`
	GridSize   int  `json:"gridSize"`
	CanvasSize int  `json:"canvasSize"`
}

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
	}
}

func (c *Canvas) ToJson() []byte {
	canvas, err := json.Marshal(c)
	if err != nil {
		fmt.Println(err)
		return []byte{}
	}
	return canvas
}
