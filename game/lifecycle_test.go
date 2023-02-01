package game

import (
	"testing"

	"golang.org/x/net/websocket"
)

func TestGame_LifeCycle(t *testing.T) {

	game := StartGame()
	ws := new(websocket.Conn)
	close := func() { ws.Close() }

	game.LifeCycle(ws, close)

	if game.Balls[0] == nil {
		t.Errorf("Expected 1 ball, got %d", len(game.Balls))
	}
	if game.Players[0] == nil {
		t.Errorf("Expected 1 player, got %d", len(game.Players))
	}

}
