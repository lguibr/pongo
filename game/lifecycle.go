// File: game/lifecycle.go
package game

import (
	"fmt"
	"time"

	"golang.org/x/net/websocket" // Keep for ws type hint if needed, but ws is managed by handler now
)

// LifeCycleResult holds data needed by the connection handler after setup.
type LifeCycleResult struct {
	Player        *Player
	PaddleChannel chan PaddleMessage // Channel to send direction updates *to* the paddle logic
}

// LifeCycle sets up the game logic components for a player.
// It no longer manages the WebSocket read loop directly.
// It now returns channels/data needed by the handler.
func (game *Game) LifeCycle(ws *websocket.Conn, playerIndex int, stopCh <-chan struct{}, coordinatedClose func()) (*LifeCycleResult, error) {
	fmt.Printf("Setting up LifeCycle game components for player index %d (%s)\n", playerIndex, ws.RemoteAddr())

	// Initiate a new game grid if this is the first player (still needs actor refactor for safety)
	if !game.HasPlayer() {
		fmt.Println("First player joining, initializing grid.")
		game.Canvas.Grid.Fill(0, 0, 0, 0)
	}

	// Create channels (to be replaced by PIDs)
	playerChannel := NewPlayerChannel()
	paddleChannel := NewPaddleChannel() // This is needed by the handler's read loop
	ballChannel := NewBallChannel()

	// Create game entities (to be refactored into actors)
	player := NewPlayer(game.Canvas, playerIndex, playerChannel)
	playerPaddle := NewPaddle(paddleChannel, game.Canvas.CanvasSize, playerIndex)
	initialPlayerBall := NewBall(
		ballChannel, 0, 0, 0, game.Canvas.CanvasSize, playerIndex, time.Now().Nanosecond(),
	)

	// Start entity logic goroutines (to be replaced by actor spawns)
	go game.ReadPlayerChannel(playerIndex, playerChannel, playerPaddle, initialPlayerBall)
	go playerPaddle.ReadPaddleChannel(paddleChannel) // Listens for messages sent by the handler
	go game.ReadBallChannel(playerIndex, initialPlayerBall)
	go playerPaddle.Engine() // Needs stop mechanism
	go initialPlayerBall.Engine() // Needs stop mechanism

	// Start WebSocket *writer* goroutine
	go game.WriteGameState(ws, stopCh) // WriteGameState listens to stopCh

	// Send the initial connect message AFTER starting routines
	player.Connect()

	fmt.Printf("LifeCycle setup complete for player %d (%s)\n", playerIndex, ws.RemoteAddr())

	// Return the necessary data to the handler
	return &LifeCycleResult{
		Player:        player,
		PaddleChannel: paddleChannel,
	}, nil
}