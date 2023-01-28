package server

import (
	"fmt"
	"io"
	"net/http"

	"github.com/lguibr/pongo/game"

	"golang.org/x/net/websocket"
)

func (s *Server) HandleSubscribe(g *game.Game) func(ws *websocket.Conn) {
	return func(ws *websocket.Conn) {
		fmt.Println("New Connection from client: ", ws.RemoteAddr())
		fmt.Println(g)

		s.OpenConnection(ws)

		if !g.HasPlayer() {
			g.Canvas.Grid.Fill(0, 0, 0, 0)
		}

		index := g.GetNextIndex()
		ioChannel := make(chan game.PlayerMessage)
		player := game.NewPlayer(g.Canvas, index, ioChannel)

		go func() {
			for message := range ioChannel {
				switch payload := message.(type) {
				case game.PlayerConnectMessage:
					g.Players[index] = payload.PlayerPayload
					player.Subscribe()
					fmt.Println("Player connected: ", payload)
				case game.PlayerDisconnectMessage:
					g.Players[index] = nil
					fmt.Println("Player Disconnected: ")
					s.CloseConnection(ws)
				default:
					continue
				}
			}
		}()

		//TODO Should be removed and pass to inside playerSubscribe
		for _, ball := range player.Balls {
			if ball == nil {
				continue
			}
			go ball.Engine(g)
		}

		// //INFO Reading the input from the client
		// go player.ReadInput(ws)
		//INFO Writing the game state to the client
		g.WriteGameState(ws)
	}
}

func (s *Server) HandleGetSit(g *game.Game) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := io.WriteString(w, string(g.ToJson()))
		if err != nil {
			fmt.Println("Error writing to client: ", err)
		}
	}
}
