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
		//INFO Open WebSocket connection
		s.OpenConnection(ws)
		//INFO Start Game lifecycle
		go g.LifeCycle(ws, func() { s.CloseConnection(ws) })
		//INFO Keep WebSocket connection open
		s.KeepConnection(ws)
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
