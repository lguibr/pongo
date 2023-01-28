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
		//INFO Initiate the player channel
		playerChannel := make(chan game.PlayerMessage)
		//INFO Initiate the ball channel
		ballChannel := make(chan game.BallMessage)

		initialBall := game.NewBall(ballChannel, 0, 0, 0, g.Canvas.CanvasSize, index)

		player := game.NewPlayer(g.Canvas, index, playerChannel, initialBall)

		fmt.Println("Start reading from player channel")
		//INFO Reading Player messages
		go func() {
			for message := range playerChannel {
				switch payload := message.(type) {
				case game.PlayerConnectMessage:
					g.Players[index] = player
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
		//INFO Subscribe the player
		player.Subscribe()

		fmt.Println("Start reading from ball channel")
		//INFO Reading Ball messages
		go func() {
			for message := range ballChannel {
				switch payload := message.(type) {
				case game.BallPositionMessage:
					ball := payload.Ball
					ball.CollidePaddles(g.Players)
					ball.CollideCells(g.Canvas.Grid, g.Canvas.CellSize)
					ball.CollideWalls()
				default:
					continue
				}
			}
		}()

		g.WriteGameState(ws)
		player.ReadInput(ws)

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
