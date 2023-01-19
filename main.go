package main

import (
	"fmt"
	"net/http"

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/server"
	"golang.org/x/net/websocket"
)

func main() {
	wsServer := server.NewServer()
	game := game.StartGame()

	fmt.Println("Game started:")
	fmt.Println(game)

	http.HandleFunc("/", wsServer.HandleGetSit(game))

	http.Handle("/subscribe", websocket.Handler(wsServer.HandleSubscribe(game)))

	panic(http.ListenAndServe(":3001", nil))

}
