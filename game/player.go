package game

import (
	"fmt"

	"github.com/lguibr/pongo/utils"
)

// Player struct now primarily for holding state data used in JSON marshalling.
type Player struct {
	Index int    `json:"index"`
	Id    string `json:"id"`
	Color [3]int `json:"color"`
	Score int32  `json:"score"`
}

// NewPlayer creates the Player data struct.
func NewPlayer(canvas *Canvas, index int) *Player {
	cfg := utils.DefaultConfig() // Get default config for initial score
	return &Player{
		Index: index,
		Id:    "player" + fmt.Sprint(index),
		Color: utils.NewRandomColor(),
		Score: int32(cfg.InitialScore), // Use initial score from config
	}
}
