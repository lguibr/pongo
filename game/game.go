package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/lguibr/pongo/utils"
)

type Game struct {
	Canvas  *Canvas    `json:"canvas"`
	Players [4]*Player `json:"players"`
}

func StartGame() *Game {
	canvas := NewCanvas(0, 0)
	canvas.Grid.Fill(0, 0, 0, 0)

	players := [4]*Player{}

	game := Game{
		Canvas:  canvas,
		Players: players,
	}

	return &game
}

func (game *Game) ToJson() []byte {
	gameBytes, err := json.Marshal(game)
	if err != nil {
		fmt.Println(err)
		return []byte{}
	}
	return gameBytes
}

func (game *Game) SubscribeBall(ball *Ball) {
	go func() {

		for {

			if game.Players[ball.Index] == nil {
				fmt.Println("player ball", ball.Index, "disconnected")
				return
			}

			ball.Move()
			ball.CollidePaddles(game.Players)
			ball.CollideCells()
			ball.CollideWalls()

			time.Sleep(utils.Period)
		}
	}()
}

func (game *Game) SubscribePaddle(paddle *Paddle) {
	go func() {
		for {

			if game.Players[paddle.Index] == nil {
				fmt.Println("player paddle", paddle.Index, "disconnected")
				return
			}

			paddle.Move()
			time.Sleep(utils.Period)
		}
	}()
}

func (game *Game) SubscribePlayer() (func(), *Player) {
	rand.Seed(time.Now().UnixNano())
	if !game.HasPlayer() {
		game.Canvas.Grid.Fill(0, 0, 0, 0)
	}
	index := game.GetNextIndex()
	playerId := "player" + fmt.Sprint(index)

	player := NewPlayer(game.Canvas, index, playerId)

	game.Players[index] = player

	for _, ball := range player.Balls {
		if ball == nil {
			continue
		}
		game.SubscribeBall(ball)
	}
	game.SubscribePaddle(player.Paddle)

	return func() { game.UnSubscribePlayer(index) }, player
}

func (game *Game) UnSubscribePlayer(index int) {
	fmt.Println("UnSubscribePlayer of index: ", index)
	game.Players[index].Balls = nil
	game.Players[index].Paddle = nil
	game.Players[index] = nil
}

func (game *Game) GetNextIndex() int {
	for i, player := range game.Players {
		if player == nil {
			return i
		}
	}
	return 0
}

func (game *Game) HasPlayer() bool {
	for _, player := range game.Players {
		if player != nil {
			return true
		}
	}
	return false
}
