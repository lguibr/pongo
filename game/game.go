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
		fmt.Println(err)
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
