package game

import (
	"github.com/lguibr/pongo/utils"
)

type Player struct {
	Index  int     `json:"index"`
	Id     string  `json:"ownerId"`
	Canvas *Canvas `json:"canvas"`
	Color  [3]int  `json:"color"`
}

func CreatePlayer(canvas *Canvas, index int, id string) *Player {

	return &Player{
		Index:  index,
		Id:     id,
		Canvas: canvas,
		Color:  utils.CreateRandomColor(),
	}
}
