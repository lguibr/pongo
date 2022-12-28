package server

import (
	"fmt"
	"time"

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

func (s *Server) CreateSubscriptionGame(g *game.Game) func(ws *websocket.Conn) {

	return func(ws *websocket.Conn) {

		fmt.Println("New Connection from client:", ws.RemoteAddr(), '\n')
		s.conns[ws] = true
		currentIndex := g.GetNextIndex()
		unsubscribePlayer := g.SubscribePlayer()

		//INFO Reading the direction from the client
		fmt.Println(string(g.ToJson()), '\n')

		//INFO Reading the direction from the client
		go func() { s.readLoop(ws, g.Paddles[currentIndex].SetDirection, unsubscribePlayer) }()

		//INFO Writing the game state to the client
		for {
			payload := g.ToJson()
			ws.Write(payload)
			time.Sleep(utils.Period)
		}

	}
}
