package game

import (
	"encoding/json"
	"fmt"
)

type Cell struct {
	X    int                    `json:"x"`
	Y    int                    `json:"y"`
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

type Grid [][]Cell

func CreateGrid(grid_size int) *Grid {
	grid := make(Grid, grid_size)
	for i := range grid {
		grid[i] = make([]Cell, grid_size)
	}
	for i, row := range grid {
		for j := range row {
			data := make(map[string]interface{})
			grid[i][j] = Cell{X: i, Y: j, Type: "empty", Data: data}
		}
	}
	return &grid
}

func (g *Grid) ToString() string {
	return fmt.Sprintf("%v", g)
}

func (g *Grid) ToJson() []byte {
	grid, err := json.Marshal(g)
	if err != nil {
		fmt.Println(err)
		return []byte{}
	}
	return grid
}
