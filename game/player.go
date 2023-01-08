package game

import (
	"github.com/lguibr/pongo/utils"
)

type Player struct {
	Index  int     `json:"index"`
	Id     string  `json:"id"`
	Canvas *Canvas `json:"canvas"`
	Color  [3]int  `json:"color"`
	Paddle *Paddle `json:"paddle"`
	Balls  []*Ball `json:"balls"`
}

func NewPlayer(canvas *Canvas, index int, id string) *Player {

	return &Player{
		Index:  index,
		Id:     id,
		Canvas: canvas,
		Color:  utils.NewRandomColor(),
		Paddle: NewPaddle(canvas, index),
		Balls: []*Ball{
			NewBall(canvas, 0, 0, 0, index),
		},
	}
}
