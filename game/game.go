package game

import (
	"encoding/json"
	"fmt"
)

type Game struct {
	Canvas  *Canvas    `json:"canvas"`
	Grid    *Grid      `json:"grid"`
	Paddles [4]*Paddle `json:"paddles"`
	Balls   [4]*Ball   `json:"balls"`
}

func StartGame() *Game {
	grid_size := 20

	canvas := CreateCanvas(300)

	grid := CreateGrid(grid_size)
	balls := [4]*Ball{}
	paddles := [4]*Paddle{}

	game := Game{canvas, grid, paddles, balls}
	return &game
}

func (g *Game) ToJson() []byte {
	game, err := json.Marshal(g)
	if err != nil {
		fmt.Println(err)
		return []byte{}
	}
	return game
}
