package main

import (
	"fmt"
	"net/http"

	"github.com/lguibr/pongo/game"
	"github.com/lguibr/pongo/server"
	"golang.org/x/net/websocket"
)

func main() {
	websocketServer := server.New()

	game := game.StartGame()

	fmt.Println("Game started:")
	fmt.Println(game)

	http.HandleFunc("/", websocketServer.HandleGetSit(game))

	http.Handle("/subscribe", websocket.Handler(websocketServer.HandleSubscribe(game)))

	panic(http.ListenAndServe(":3001", nil))

}
