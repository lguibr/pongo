package server

import (
	"github.com/lguibr/pongo/game"

	"golang.org/x/net/websocket"
)

func (s *Server) HandleSubscribe(games []*game.Game) func(ws *websocket.Conn) {
	return func(ws *websocket.Conn) {
		var currentGameIndex int = -1
		var currentGame *game.Game

		for _, game := range games {
			if game != nil && game.GetNextIndex() != -1 {
				currentGame = game
				break
			}
		}
		noAvailableGame := currentGameIndex == -1

		if noAvailableGame {
			currentGame := game.StartGame()
			games = append(games, currentGame)
		} else {
			currentGame = games[currentGameIndex]
		}

		go currentGame.ReadGameChannel()

		//INFO Open WebSocket connection
		s.OpenConnection(ws)
		//INFO Start Game lifecycle
		go currentGame.LifeCycle(ws, func() {
			s.CloseConnection(ws)
		})
		//INFO Keep WebSocket connection open
		s.KeepConnection(ws)
	}
}
