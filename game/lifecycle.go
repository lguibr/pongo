// File: game/lifecycle.go
package game

import (
	"fmt"
	"time"

	"github.com/lguibr/pongo/bollywood" // Import bollywood
	"golang.org/x/net/websocket"
)

// LifeCycleResult holds data needed by the connection handler after setup.
type LifeCycleResult struct {
	Player    *Player
	PaddlePID *bollywood.PID // PID of the created PaddleActor
	// PaddleChannel removed
}

// LifeCycle sets up the game logic components for a player.
// It now spawns a PaddleActor and returns its PID.
func (game *Game) LifeCycle(ws *websocket.Conn, playerIndex int, stopCh <-chan struct{}, coordinatedClose func()) (*LifeCycleResult, error) {
	fmt.Printf("Setting up LifeCycle game components for player index %d (%s)\n", playerIndex, ws.RemoteAddr())

	// Initiate a new game grid if this is the first player (still needs actor refactor for safety)
	if !game.HasPlayer() {
		fmt.Println("First player joining, initializing grid.")
		game.Canvas.Grid.Fill(0, 0, 0, 0)
	}

	// Create channels (to be replaced by PIDs where applicable)
	playerChannel := NewPlayerChannel()
	// paddleChannel := NewPaddleChannel() // Removed
	ballChannel := NewBallChannel() // Keep for Ball for now

	// Create game entities (Paddle state is now created for the actor)
	player := NewPlayer(game.Canvas, playerIndex, playerChannel)
	playerPaddleData := NewPaddle(game.Canvas.CanvasSize, playerIndex) // Create initial data struct
	initialPlayerBall := NewBall(
		ballChannel, 0, 0, 0, game.Canvas.CanvasSize, playerIndex, time.Now().Nanosecond(),
	)

	// --- Actor Spawning --- 
	// TODO: Need access to the main bollywood.Engine instance here
	// This should likely be passed into LifeCycle or accessed globally/via context.
	// engine := GetEngineInstance() // Hypothetical function to get the engine
	// TODO: Need PID of the GameActor (assuming one exists)
	// gameActorPID := GetGameActorPID() // Hypothetical

	// Spawn Paddle Actor
	// paddleProps := NewPaddleActorProducer(*playerPaddleData, gameActorPID) // Pass paddle data copy, game actor PID
	// paddlePID := engine.Spawn(paddleProps)
	// fmt.Printf("Spawned PaddleActor %s for player %d\n", paddlePID, playerIndex)
	// --- Placeholder PID until Engine integration --- 
	paddlePID := &bollywood.PID{ID: "dummy-paddle-pid-" + fmt.Sprint(playerIndex)} 
	fmt.Printf("WARNING: Using placeholder PID for PaddleActor: %s\n", paddlePID)
	// ----------------------------------------------

	// Start necessary *non-actor* goroutines
	// TODO: Ball and Player logic should also become actors eventually.
	go game.ReadBallChannel(playerIndex, initialPlayerBall) // Keep Ball logic for now
	go initialPlayerBall.Engine()                         // Keep Ball logic for now
	go game.ReadPlayerChannel(playerIndex, playerChannel, playerPaddleData, initialPlayerBall) // Keep for player score/connect/disconnect

	// Goroutines for PaddleActor are managed by the bollywood engine (Spawn starts them)
	// go playerPaddle.ReadPaddleChannel(paddleChannel) // DEPRECATED
	// go playerPaddle.Engine() // DEPRECATED

	// Start WebSocket *writer* goroutine
	go game.WriteGameState(ws, stopCh) // WriteGameState listens to stopCh

	// Send the initial connect message AFTER starting routines/actors
	player.Connect()

	fmt.Printf("LifeCycle setup complete for player %d (%s)\n", playerIndex, ws.RemoteAddr())

	// Return the necessary data to the handler
	return &LifeCycleResult{
		Player:    player,
		PaddlePID: paddlePID, // Return the PID of the spawned (or placeholder) actor
	}, nil
}
