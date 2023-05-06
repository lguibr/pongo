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
	newGame := game.NewGame()
	go newGame.ReadChannel()

	websocketServer := server.New()
	fmt.Println("Server started on port", port)
	http.HandleFunc("/", websocketServer.HandleGetSit(newGame))
	http.Handle("/subscribe", websocket.Handler(websocketServer.HandleSubscribe(newGame)))

	panic(http.ListenAndServe(port, nil))
}
