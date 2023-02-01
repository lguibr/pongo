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
	g := game.StartGame()
	go g.ReadGameChannel()

	websocketServer := server.New()
	fmt.Println("Server started on port", port)
	http.HandleFunc("/", websocketServer.HandleGetSit(g))
	http.Handle("/subscribe", websocket.Handler(websocketServer.HandleSubscribe(g)))

	panic(http.ListenAndServe(port, nil))
}
