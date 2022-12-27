package server

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

var index = -1

type Direction struct {
	Direction string `json:"direction"`
}

func updatePaddleDirection(paddle *game.Paddle) func(buffer []byte) {
	return func(buffer []byte) {
		direction := Direction{}
		err := json.Unmarshal(buffer, &direction)
		if err != nil {
			fmt.Println("Error unmarshalling message:", err)
		}
		paddle.Direction = utils.DirectionFromString(direction.Direction)
		fmt.Println(paddle)
	}
}

func (s *Server) CreateSubscriptionGame(g *game.Game) func(ws *websocket.Conn) {

	return func(ws *websocket.Conn) {

		fmt.Println("New Connection from client:", ws.RemoteAddr())
		s.conns[ws] = true

		index++
		currentIndex := index % 4

		ownerId := "lg" + fmt.Sprint(currentIndex)

		g.Balls[currentIndex] = game.CreateBall(ownerId, g.Canvas, 0, 0, 0)
		g.Paddles[currentIndex] = game.CreatePaddle(ownerId, g.Canvas)

		g.Grid.SubscribeBall(g.Balls[currentIndex])
		g.Grid.SubscribePaddle(g.Paddles[currentIndex])

		fmt.Println("%V", g)

		go func() { s.readLoop(ws, updatePaddleDirection(g.Paddles[currentIndex])) }()

		for {
			payload := g.ToJson()
			ws.Write(payload)
			time.Sleep(utils.Period)
		}

	}
}
