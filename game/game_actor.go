// File: game/game_actor.go
package game

import (
	"fmt"
	"reflect" // Import reflect for logging message type
	"sync"
	"sync/atomic"
	"time"

	"github.com/lguibr/pongo/bollywood"
	"github.com/lguibr/pongo/utils"
)

// MaxPlayers is the maximum number of players allowed in the game.
const MaxPlayers = 4 // Exported constant

// GameActor manages the overall game state and coordinates child actors.
// It acts as the central authority for game logic, state, and communication.
type GameActor struct {
	canvas        *Canvas
	players       [MaxPlayers]*playerInfo    // Use exported constant
	paddles       [MaxPlayers]*Paddle        // Use exported constant
	paddleActors  [MaxPlayers]*bollywood.PID // Use exported constant
	balls         map[int]*Ball              // Live state of balls (updated by messages) - Keyed by Ball ID
	ballActors    map[int]*bollywood.PID
	engine        *bollywood.Engine // Reference to the engine
	ticker        *time.Ticker
	stopTickerCh  chan struct{}
	gameStateJSON atomic.Value   // Stores marshalled JSON for HTTP endpoint
	selfPID       *bollywood.PID // Store self PID for internal use
	mu            sync.RWMutex   // Protects shared maps/slices (players, paddles, balls, actors, connToIndex)

	// Map to find player index from connection (connection -> index)
	connToIndex map[PlayerConnection]int
}

// playerInfo holds state associated with a connected player/websocket.
type playerInfo struct {
	Index       int
	ID          string
	Score       int
	Color       [3]int
	Ws          PlayerConnection // Use the interface type
	IsConnected bool             // Tracks if the connection is considered active by GameActor
}

// NewGameActorProducer creates a producer for the GameActor.
func NewGameActorProducer(engine *bollywood.Engine) bollywood.Producer {
	return func() bollywood.Actor {
		canvas := NewCanvas(0, 0) // Default size
		canvas.Grid.Fill(0, 0, 0, 0)

		ga := &GameActor{
			canvas:       canvas,
			players:      [MaxPlayers]*playerInfo{},    // Use exported constant
			paddles:      [MaxPlayers]*Paddle{},        // Use exported constant
			paddleActors: [MaxPlayers]*bollywood.PID{}, // Use exported constant
			balls:        make(map[int]*Ball),
			ballActors:   make(map[int]*bollywood.PID),
			engine:       engine,
			stopTickerCh: make(chan struct{}),
			connToIndex:  make(map[PlayerConnection]int), // Initialize the new map
			// mu is zero-value initialized, which is fine for sync.RWMutex
		}
		ga.updateGameStateJSON() // Initialize JSON state
		return ga
	}
}

// Receive is the main message handler for the GameActor. It dispatches messages
// to specific handler methods based on the message type.
func (a *GameActor) Receive(ctx bollywood.Context) {
	actorPIDStr := "nil" // Default string if selfPID is nil
	if a.selfPID == nil {
		// Try to set selfPID immediately if it's the first message
		a.selfPID = ctx.Self()
		if a.selfPID != nil {
			actorPIDStr = a.selfPID.String()
			// fmt.Printf("GameActor %s: Self PID set on first Receive.\n", actorPIDStr) // Reduce noise
		} else {
			fmt.Println("GameActor ???: Failed to set self PID on first Receive.")
			// Cannot proceed reliably without a PID
			return
		}
	} else {
		actorPIDStr = a.selfPID.String() // Use existing PID for logging
	}

	// --- Log entry into Receive ---
	msg := ctx.Message()
	fmt.Printf("GameActor %s: Receive entered. Message Type: %s\n", actorPIDStr, reflect.TypeOf(msg))
	// --- End Log ---

	switch m := msg.(type) { // Use 'm' to avoid shadowing 'msg' variable used above
	case bollywood.Started:
		fmt.Printf("GameActor %s: Processing Started message.\n", actorPIDStr)
		// Ensure selfPID is set if it wasn't already (should be redundant now)
		if a.selfPID == nil {
			a.selfPID = ctx.Self()
			if a.selfPID != nil {
				actorPIDStr = a.selfPID.String()
				fmt.Printf("GameActor %s: Self PID set during Started.\n", actorPIDStr)
			} else {
				fmt.Printf("GameActor ???: Failed to set self PID during Started.\n")
				return // Cannot start ticker without PID
			}
		}
		a.ticker = time.NewTicker(utils.Period)
		go a.runTickerLoop()

	case *GameTick:
		// fmt.Printf("GameActor %s: Processing GameTick.\n", actorPIDStr) // Reduce noise unless debugging ticks specifically
		// Lock needed for collision detection which modifies state
		a.mu.Lock()
		// fmt.Printf("GameActor %s: Lock acquired for collisions.\n", actorPIDStr) // Reduce noise
		a.detectCollisions(ctx)
		// fmt.Printf("GameActor %s: Lock released after collisions.\n", actorPIDStr) // Reduce noise
		a.mu.Unlock()

		// fmt.Printf("GameActor %s: Calling broadcastGameState after GameTick.\n", actorPIDStr) // Reduce noise
		a.broadcastGameState()  // Broadcast reads state (uses RLock internally)
		a.updateGameStateJSON() // Update JSON reads state (uses RLock internally)

	case PlayerConnectRequest:
		fmt.Printf("GameActor %s: Processing PlayerConnectRequest.\n", actorPIDStr)
		a.handlePlayerConnect(ctx, m.WsConn)

	case PlayerDisconnect:
		fmt.Printf("GameActor %s: Processing PlayerDisconnect.\n", actorPIDStr)
		a.handlePlayerDisconnect(ctx, m.PlayerIndex, m.WsConn)

	case ForwardedPaddleDirection:
		// fmt.Printf("GameActor %s: Processing ForwardedPaddleDirection.\n", actorPIDStr) // Reduce noise
		a.handlePaddleDirection(ctx, m.WsConn, m.Direction)

	case PaddlePositionMessage:
		// fmt.Printf("GameActor %s: Processing PaddlePositionMessage.\n", actorPIDStr) // Reduce noise
		a.handlePaddlePositionUpdate(ctx, m.Paddle)

	case BallPositionMessage:
		// fmt.Printf("GameActor %s: Processing BallPositionMessage.\n", actorPIDStr) // Reduce noise
		a.handleBallPositionUpdate(ctx, m.Ball)

	case SpawnBallCommand:
		fmt.Printf("GameActor %s: Processing SpawnBallCommand.\n", actorPIDStr)
		a.spawnBall(ctx, m.OwnerIndex, m.X, m.Y, m.ExpireIn)

	case DestroyExpiredBall:
		fmt.Printf("GameActor %s: Processing DestroyExpiredBall.\n", actorPIDStr)
		a.handleDestroyExpiredBall(ctx, m.BallID)

	case bollywood.Stopping:
		fmt.Printf("GameActor %s: Processing Stopping message.\n", actorPIDStr)
		fmt.Printf("GameActor %s: Stopping ticker...\n", actorPIDStr)
		if a.ticker != nil {
			a.ticker.Stop()
			select {
			case <-a.stopTickerCh:
				fmt.Printf("GameActor %s: stopTickerCh already closed.\n", actorPIDStr)
			default:
				close(a.stopTickerCh) // Ensure ticker loop stops
				fmt.Printf("GameActor %s: stopTickerCh closed.\n", actorPIDStr)
			}
		} else {
			fmt.Printf("GameActor %s: Ticker was nil during Stopping.\n", actorPIDStr)
		}
		fmt.Printf("GameActor %s: Starting cleanup...\n", actorPIDStr)
		a.cleanupChildActorsAndConnections()
		fmt.Printf("GameActor %s: Cleanup finished.\n", actorPIDStr)

	case bollywood.Stopped:
		// This message is sent *after* the actor's goroutine has exited.
		// We might not see this log if the test times out before the goroutine fully exits.
		fmt.Printf("GameActor %s: Processing Stopped message (goroutine should be exiting).\n", actorPIDStr)

	default:
		fmt.Printf("GameActor %s: Processing unknown message type: %T\n", actorPIDStr, m)
	}
	// fmt.Printf("GameActor %s: Receive finished for Message Type: %s\n", actorPIDStr, reflect.TypeOf(msg)) // Optional: log exit
}

// runTickerLoop sends GameTick messages to the actor's own mailbox at regular intervals.
func (a *GameActor) runTickerLoop() {
	// Wait briefly for selfPID to be set in Receive
	time.Sleep(15 * time.Millisecond) // Slightly longer wait
	actorPID := a.selfPID
	if actorPID == nil {
		fmt.Println("ERROR: GameActor ticker loop cannot start, self PID not set after wait.")
		return
	}
	actorPIDStr := actorPID.String()
	fmt.Printf("GameActor %s: Ticker loop started.\n", actorPIDStr)
	defer fmt.Printf("GameActor %s: Ticker loop stopped.\n", actorPIDStr)

	tickCount := 0
	tickMsg := &GameTick{} // Create message once

	for {
		select {
		case <-a.stopTickerCh: // Check if stopped first
			fmt.Printf("GameActor %s: Ticker loop detected stop signal. Exiting.\n", actorPIDStr)
			return
		case tickTime, ok := <-a.ticker.C:
			if !ok {
				fmt.Printf("GameActor %s: Ticker channel closed. Exiting loop.\n", actorPIDStr)
				return // Exit if ticker channel is closed
			}
			_ = tickTime // Use tickTime if needed for logging
			tickCount++
			// Check again before sending, in case stop happened between ticks
			select {
			case <-a.stopTickerCh:
				fmt.Printf("GameActor %s: Ticker loop detected stop signal after tick %d. Exiting.\n", actorPIDStr, tickCount)
				return
			default:
				// Send the tick message using the standard engine Send method
				// Use the captured actorPID which is guaranteed non-nil here
				// fmt.Printf("GameActor %s: Attempting to send GameTick %d\n", actorPIDStr, tickCount) // Reduce noise
				a.engine.Send(actorPID, tickMsg, nil)
				// fmt.Printf("GameActor %s: Sent GameTick %d\n", actorPIDStr, tickCount) // Log *after* send attempt
			}
		}
	}
}

// cleanupChildActorsAndConnections handles stopping child actors and closing connections during shutdown.
func (a *GameActor) cleanupChildActorsAndConnections() {
	pidStr := "unknown"
	if a.selfPID != nil {
		pidStr = a.selfPID.String()
	}
	fmt.Printf("GameActor %s: Acquiring lock for cleanup...\n", pidStr)
	// Collect PIDs and connections to stop/close while holding lock
	a.mu.Lock()
	fmt.Printf("GameActor %s: Lock acquired for cleanup.\n", pidStr)
	paddlesToStop := make([]*bollywood.PID, 0, MaxPlayers) // Use exported constant
	ballsToStop := make([]*bollywood.PID, 0, len(a.ballActors))
	connectionsToClose := []PlayerConnection{}

	for i := 0; i < MaxPlayers; i++ { // Use exported constant
		if pid := a.paddleActors[i]; pid != nil {
			paddlesToStop = append(paddlesToStop, pid)
			a.paddleActors[i] = nil // Clear PID immediately
		}
		// Also collect connections associated with active players
		if pInfo := a.players[i]; pInfo != nil && pInfo.Ws != nil {
			// Check if it's still in the connToIndex map before adding
			if _, exists := a.connToIndex[pInfo.Ws]; exists {
				connectionsToClose = append(connectionsToClose, pInfo.Ws)
				delete(a.connToIndex, pInfo.Ws) // Remove mapping
			} else {
				// Log if connection exists in playerInfo but not connToIndex during cleanup
				fmt.Printf("GameActor %s Cleanup WARN: Connection for player %d not found in connToIndex.\n", pidStr, i)
			}
			pInfo.Ws = nil // Clear reference
			pInfo.IsConnected = false
		}
		a.players[i] = nil // Clear player info
	}

	for ballID, pid := range a.ballActors {
		if pid != nil {
			ballsToStop = append(ballsToStop, pid)
		}
		delete(a.ballActors, ballID) // Clear PID reference
		delete(a.balls, ballID)      // Clear ball state
	}

	// Clear any remaining entries in connToIndex (should be empty now)
	if len(a.connToIndex) > 0 {
		fmt.Printf("GameActor %s Cleanup WARN: connToIndex not empty after player cleanup (%d entries remain).\n", pidStr, len(a.connToIndex))
		a.connToIndex = make(map[PlayerConnection]int)
	}

	a.mu.Unlock() // Release lock before stopping children/closing connections
	fmt.Printf("GameActor %s: Lock released for cleanup.\n", pidStr)

	fmt.Printf("GameActor %s Cleanup: Stopping %d paddles, %d balls. Closing %d connections.\n",
		pidStr, len(paddlesToStop), len(ballsToStop), len(connectionsToClose))

	// Stop child actors
	for _, pid := range paddlesToStop {
		if pid != nil {
			// fmt.Printf("GameActor %s Cleanup: Stopping paddle actor %s\n", pidStr, pid) // Reduce noise
			a.engine.Stop(pid)
		}
	}
	for _, pid := range ballsToStop {
		if pid != nil {
			// fmt.Printf("GameActor %s Cleanup: Stopping ball actor %s\n", pidStr, pid) // Reduce noise
			a.engine.Stop(pid)
		}
	}

	// Close connections
	for _, ws := range connectionsToClose {
		if ws != nil {
			// fmt.Printf("GameActor %s Cleanup: Closing connection %s\n", pidStr, ws.RemoteAddr()) // Reduce noise
			_ = ws.Close() // Attempt to close
		}
	}
	fmt.Printf("GameActor %s: Cleanup actions finished.\n", pidStr)
}

// GetGameStateJSON retrieves the latest marshalled game state for HTTP handlers.
// NOTE: This is still a placeholder as direct access isn't ideal.
// A better approach might involve the engine providing a way to query state,
// or the GameActor periodically writing to a shared resource.
func (a *GameActor) GetGameStateJSON() []byte {
	val := a.gameStateJSON.Load()
	if val == nil {
		// Attempt to generate current state if nil (e.g., called before first tick)
		a.updateGameStateJSON()
		val = a.gameStateJSON.Load()
		if val == nil {
			return []byte(`{"error": "failed to initialize game state"}`)
		}
	}
	jsonBytes, ok := val.([]byte)
	if !ok {
		fmt.Println("ERROR: GameActor gameStateJSON is not []byte")
		return []byte(`{"error": "invalid internal state type"}`)
	}
	return jsonBytes
}
