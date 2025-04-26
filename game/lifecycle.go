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
	BallPID   *bollywood.PID // PID of the created BallActor
	PaddlePID *bollywood.PID // PID of the created PaddleActor
	// PaddleChannel removed
}

// LifeCycle sets up the game logic components for a player.
// It now spawns PaddleActor and BallActor instances (using placeholders).
func (game *Game) LifeCycle(ws *websocket.Conn, playerIndex int, stopCh <-chan struct{}, coordinatedClose func()) (*LifeCycleResult, error) {
	fmt.Printf("Setting up LifeCycle game components for player index %d (%s)\n", playerIndex, ws.RemoteAddr())

	// Initiate a new game grid if this is the first player (still needs actor refactor for safety)
	if !game.HasPlayer() {
		fmt.Println("First player joining, initializing grid.")
		game.Canvas.Grid.Fill(0, 0, 0, 0)
	}

	// Create channels (to be replaced by PIDs where applicable)
	playerChannel := NewPlayerChannel() // Keep for player connect/disconnect/score for now
	// paddleChannel := NewPaddleChannel() // Removed
	// ballChannel := NewBallChannel() // Removed

	// Create game entities (Data structs for actors)
	player := NewPlayer(game.Canvas, playerIndex, playerChannel)
	playerPaddleData := NewPaddle(game.Canvas.CanvasSize, playerIndex) // Create initial paddle data struct
	initialPlayerBall := NewBall( // Create initial ball data struct
		0, 0, 0, game.Canvas.CanvasSize, playerIndex, time.Now().Nanosecond(),
	)

	// --- Actor Spawning ---
	// TODO: Need access to the main bollywood.Engine instance here
	// This should likely be passed into LifeCycle or accessed globally/via context.
	// engine := GetEngineInstance() // Hypothetical function to get the engine
	// TODO: Need PID of the GameActor (assuming one exists)
	// gameActorPID := GetGameActorPID() // Hypothetical

	// --- Placeholder PIDs until Engine integration ---
	gameActorPID := &bollywood.PID{ID: "dummy-game-actor-pid"} // Placeholder!
	fmt.Printf("WARNING: Using placeholder GameActor PID: %s\n", gameActorPID)

	// Spawn Paddle Actor
	// paddleProps := NewPaddleActorProducer(*playerPaddleData, gameActorPID)
	// paddlePID := engine.Spawn(paddleProps)
	// fmt.Printf("Spawned PaddleActor %s for player %d\n", paddlePID, playerIndex)
	paddlePID := &bollywood.PID{ID: "dummy-paddle-pid-" + fmt.Sprint(playerIndex)} // Placeholder!
	fmt.Printf("WARNING: Using placeholder PID for PaddleActor: %s\n", paddlePID)

	// Spawn Ball Actor
	// ballProps := NewBallActorProducer(*initialPlayerBall, gameActorPID)
	// ballPID := engine.Spawn(ballProps)
	// fmt.Printf("Spawned BallActor %s for ball %d\n", ballPID, initialPlayerBall.Id)
	ballPID := &bollywood.PID{ID: "dummy-ball-pid-" + fmt.Sprint(initialPlayerBall.Id)} // Placeholder!
	fmt.Printf("WARNING: Using placeholder PID for BallActor: %s\n", ballPID)
	// ----------------------------------------------


	// Start necessary *non-actor* goroutines
	// TODO: Player logic should also become an actor eventually.
	go game.ReadPlayerChannel(playerIndex, playerChannel, playerPaddleData, initialPlayerBall) // Keep for player score/connect/disconnect

	// Goroutines for BallActor and PaddleActor are managed by the bollywood engine (Spawn starts them)
	// go game.ReadBallChannel(playerIndex, initialPlayerBall) // DEPRECATED
	// go initialPlayerBall.Engine()                         // DEPRECATED
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
		BallPID:   ballPID,   // Return Ball Actor PID
		PaddlePID: paddlePID, // Return Paddle Actor PID
	}, nil
}
