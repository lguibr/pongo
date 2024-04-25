package main

import (
	"fmt"
	"net/http"

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/server"
	"golang.org/x/net/websocket"
)

var port = ":3001"

func main() {

	var games []*game.Game
	games = append(games, game.StartGame())
	websocketServer := server.New()
	fmt.Println("Server started on port", port)
	http.Handle("/subscribe", websocket.Handler(websocketServer.HandleSubscribe(games)))
	panic(http.ListenAndServe(port, nil))
}
