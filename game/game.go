package game

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

type Game struct {
	Canvas  *Canvas    `json:"canvas"`
	Players [4]*Player `json:"players"`
	Paddles [4]*Paddle `json:"paddles"`
	Balls   []*Ball    `json:"balls"`
}

func StartGame() *Game {
	rand.Seed(time.Now().UnixNano())

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
		fmt.Println("Error Marshaling the game state", err)
		return []byte{}
	}
	return gameBytes
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

func (game *Game) WriteGameState(ws *websocket.Conn) {
	for {
		_, err := ws.Write(game.ToJson())
		if err != nil {
			fmt.Println("Error writing to client: ", err)
			return
		}
		time.Sleep(utils.Period)
	}
}

func (game *Game) RemovePlayer(playerIndex int) {
	game.Players[playerIndex] = nil
	game.Paddles[playerIndex] = nil
	for i, ball := range game.Balls {
		if ball.OwnerIndex == playerIndex {
			game.Balls = append(game.Balls[:i], game.Balls[i+1:]...)
		}
	}
}

func (g *Game) AddPlayer(index int, player *Player, playerPaddle *Paddle, initialPlayerBall *Ball) {
	g.Players[index] = player
	g.Paddles[index] = playerPaddle
	go playerPaddle.Engine()
	g.Balls = append(g.Balls, initialPlayerBall)
	go initialPlayerBall.Engine()

}
