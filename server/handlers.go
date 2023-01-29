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
		//INFO Start the WebSocket connection
		s.OpenConnection(ws)
		close := func() { s.CloseConnection(ws) }
		//INFO Initiate a new game if there is no player
		if !g.HasPlayer() {
			g.Canvas.Grid.Fill(0, 0, 0, 0)
		}
		//INFO Initiating channels
		playerChannel := make(chan game.PlayerMessage)
		ballChannel := make(chan game.BallMessage)
		paddleChannel := make(chan game.PaddleMessage)
		// INFO Initiate the player and player's dependencies
		playerIndex := g.GetNextIndex()
		currentBallIndex := len(g.Balls)
		if currentBallIndex < 0 {
			currentBallIndex = 0
		}
		player := game.NewPlayer(g.Canvas, playerIndex, playerChannel)
		playerPaddle := game.NewPaddle(paddleChannel, g.Canvas.CanvasSize, playerIndex)
		initialPlayerBall := game.NewBall(ballChannel, 0, 0, 0, g.Canvas.CanvasSize, playerIndex, currentBallIndex)
		//INFO Start reading from game's entities channels
		go g.ReadPlayerChannel(playerIndex, playerChannel, playerPaddle, initialPlayerBall, close)
		go g.ReadBallChannel(playerIndex, ballChannel)
		go playerPaddle.ReadPaddleChannel(paddleChannel)
		//INFO Connect the player
		player.Connect()
		//INFO Start reading input from player and writing game state to player
		go player.ReadInput(ws, paddleChannel)
		go g.WriteGameState(ws)
		//INFO Wait for player to disconnect
		g.Players[playerIndex].WaitDisconnection()
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
