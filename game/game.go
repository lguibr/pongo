package game

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lguibr/pongo/utils"
)

type Game struct {
	Canvas  *Canvas    `json:"canvas"`
	Grid    *Grid      `json:"grid"`
	Paddles [4]*Paddle `json:"paddles"`
	Balls   [4]*Ball   `json:"balls"`
	Players [4]*Player `json:"players"`
}

func StartGame() *Game {
	grid := CreateGrid(utils.GridSize)
	canvas := CreateCanvas(utils.CanvasSize)
	balls := [4]*Ball{}
	paddles := [4]*Paddle{}
	players := [4]*Player{}

	game := Game{
		Canvas:  canvas,
		Grid:    grid,
		Paddles: paddles,
		Balls:   balls,
		Players: players,
	}

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

func (g *Game) SubscribeBall(ball *Ball) {
	go func() {
		for {
			ball.Move()
			ball.Bounce()
			time.Sleep(utils.Period)
		}
	}()
}

func (g *Game) SubscribePaddle(paddle *Paddle) {
	go func() {
		for {
			paddle.Move()
			time.Sleep(utils.Period)
		}
	}()
}

func (g *Game) SubscribePlayer() func() {

	index := g.GetNextIndex()
	playerId := "player" + fmt.Sprint(index)

	g.Players[index] = CreatePlayer(g.Canvas, index, playerId)
	g.Paddles[index] = CreatePaddle(playerId, g.Canvas)
	g.Balls[index] = CreateBall(playerId, g.Canvas, 0, 0, 0)

	g.SubscribeBall(g.Balls[index])
	g.SubscribePaddle(g.Paddles[index])
	currentIndex := index

	return func() {
		g.UnSubscribePlayer(currentIndex)
	}
}

func (g *Game) UnSubscribePlayer(index int) {
	fmt.Println("UnSubscribePlayer")
	fmt.Println(index)

	g.Balls[index] = nil
	g.Paddles[index] = nil
	g.Players[index] = nil

}

func (g *Game) GetNextIndex() int {
	for i, player := range g.Players {
		if player == nil {
			return i
		}
	}
	return 0
}
