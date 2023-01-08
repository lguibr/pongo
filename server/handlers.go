package server

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

func (s *Server) HandleSubscribe(g *game.Game) func(ws *websocket.Conn) {
	return func(ws *websocket.Conn) {

		fmt.Println("New Connection from client: ", ws.RemoteAddr())
		fmt.Println(g)

		s.conns[ws] = true

		unsubscribePlayer, player := g.SubscribePlayer()

		//INFO Reading the direction from the client
		go func() { s.readLoop(ws, player.Paddle.SetDirection, unsubscribePlayer) }()

		//INFO Writing the game state to the client
		for {
			payload := g.ToJson()
			ws.Write(payload)
			time.Sleep(utils.Period)
		}

	}
}

func (s *Server) HandleGetSit(g *game.Game) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, string(g.ToJson()))
	}
}
