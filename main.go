package main

import (
	"net/http"

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/server"
	"golang.org/x/net/websocket"
)

func main() {
	wsServer := server.NewServer()
	game := game.StartGame()

	http.Handle("/pongo", websocket.Handler(wsServer.CreateSubscriptionGame(game)))

	http.ListenAndServe(":3001", nil)

}
