// File: game/game_actor.go
package game

import (
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// GameActor manages the overall game state and coordinates child actors for a single room.
type GameActor struct {
	cfg             utils.Config
	canvas          *Canvas
	players         [utils.MaxPlayers]*playerInfo // State managed serially by actor
	paddles         [utils.MaxPlayers]*Paddle     // Local cache, authoritative state for simulation
	paddleActors    [utils.MaxPlayers]*bollywood.PID
	balls           map[int]*Ball // Local cache, authoritative state for simulation
	ballActors      map[int]*bollywood.PID
	engine          *bollywood.Engine
	physicsTicker   *time.Ticker // Ticker for physics/game logic
	stopPhysicsCh   chan struct{}
	broadcastTicker *time.Ticker // Ticker for broadcasting state
	stopBroadcastCh chan struct{}
	tickerMu        sync.Mutex // Mutex to protect ticker fields and channels
	selfPID         *bollywood.PID
	roomManagerPID  *bollywood.PID
	broadcasterPID  *bollywood.PID // PID of the dedicated broadcaster actor
	connToIndex     map[*websocket.Conn]int
	playerConns     [utils.MaxPlayers]*websocket.Conn
	gameOver        atomic.Bool // Flag to prevent multiple game over triggers

	// Buffer for pending updates to broadcast
	pendingUpdates []interface{} // Holds pointers to newly allocated update messages
	updatesMu      sync.Mutex    // Protects pendingUpdates slice

	// Removed phasingBallIntersections map

	// Performance Metrics
	tickDurationSum time.Duration
	tickCount       int64
	metricsMu       sync.Mutex // Protect metrics during updates

	// Cleanup control
	cleanupOnce sync.Once   // Ensures cleanup happens only once
	isStopping  atomic.Bool // Indicates if Stopping message has been received
}

// playerInfo holds state associated with a connected player/websocket.
type playerInfo struct {
	Index       int
	ID          string
	Score       atomic.Int32 // Use atomic Int32 for score
	Color       [3]int
	Ws          *websocket.Conn
	IsConnected bool
}

// NewGameActorProducer creates a producer for the GameActor.
func NewGameActorProducer(engine *bollywood.Engine, cfg utils.Config, roomManagerPID *bollywood.PID) bollywood.Producer {
	return func() bollywood.Actor {
		canvas := NewCanvas(cfg.CanvasSize, cfg.GridSize)
		// Grid generation happens when first player joins now

		ga := &GameActor{
			cfg:             cfg,
			canvas:          canvas, // Canvas exists, but grid is empty initially
			players:         [utils.MaxPlayers]*playerInfo{},
			paddles:         [utils.MaxPlayers]*Paddle{}, // Initialize cache map
			paddleActors:    [utils.MaxPlayers]*bollywood.PID{},
			balls:           make(map[int]*Ball), // Initialize cache map
			ballActors:      make(map[int]*bollywood.PID),
			engine:          engine,
			stopPhysicsCh:   make(chan struct{}), // Initialize channels here
			stopBroadcastCh: make(chan struct{}),
			connToIndex:     make(map[*websocket.Conn]int),
			playerConns:     [utils.MaxPlayers]*websocket.Conn{},
			roomManagerPID:  roomManagerPID,
			pendingUpdates:  make([]interface{}, 0, 128), // Pre-allocate some capacity
			// Removed phasingBallIntersections initialization
			// Initialize metrics
			tickDurationSum: 0,
			tickCount:       0,
		}
		ga.gameOver.Store(false) // Initialize game over flag
		ga.isStopping.Store(false)
		return ga
	}
}

// Receive is the main message handler for the GameActor.
func (a *GameActor) Receive(ctx bollywood.Context) {
	// Defer panic recovery
	defer func() {
		if r := recover(); r != nil {
			pidStr := "unknown"
			if a.selfPID != nil {
				pidStr = a.selfPID.String()
			}
			fmt.Printf("PANIC recovered in GameActor %s Receive: %v\nStack trace:\n%s\n", pidStr, r, string(debug.Stack()))
			// Ensure cleanup happens exactly once, even on panic
			a.performCleanup()
			// Notify room manager that this room is now defunct due to panic
			if a.roomManagerPID != nil && a.engine != nil && a.selfPID != nil {
				fmt.Printf("GameActor %s: Notifying RoomManager %s of panic exit.\n", a.selfPID, a.roomManagerPID)
				a.engine.Send(a.roomManagerPID, GameRoomEmpty{RoomPID: a.selfPID}, nil)
			}
			// Explicitly stop self if panic occurred before normal shutdown sequence
			if !a.isStopping.Load() && a.engine != nil && a.selfPID != nil {
				a.engine.Stop(a.selfPID)
			}
		}
	}()

	// Set self PID if not already set
	if a.selfPID == nil {
		a.selfPID = ctx.Self()
		if a.selfPID == nil {
			fmt.Println("ERROR: GameActor ???: Failed to set self PID on first Receive.")
			return
		}
	}

	// Ignore messages if game is already over or stopping, except for system messages
	if a.gameOver.Load() || a.isStopping.Load() {
		switch ctx.Message().(type) {
		case bollywood.Stopping, bollywood.Stopped, PlayerDisconnect:
			// Allow these messages during game over/stopping for cleanup
		default:
			return // Ignore other messages
		}
	}

	// Main message switch
	switch m := ctx.Message().(type) {
	case bollywood.Started:
		a.handleStart(ctx)

	case GameTick: // Message from physicsTicker
		start := time.Now()
		a.updateInternalState() // Generates position updates
		a.detectCollisions(ctx) // Generates collision/score/etc updates
		a.checkGameOver(ctx)    // Checks game end condition
		duration := time.Since(start)
		a.metricsMu.Lock()
		a.tickDurationSum += duration
		a.tickCount++
		a.metricsMu.Unlock()

	case PaddleStateUpdate: // Update cache with state from PaddleActor
		if paddle := a.paddles[m.Index]; paddle != nil {
			paddle.Direction = m.Direction
		}

	case BallStateUpdate: // Update cache with state from BallActor
		if ball := a.balls[m.ID]; ball != nil {
			// Check if phasing state is ending
			if ball.Phasing && !m.Phasing {
				// Clear the intersection tracker for this ball when it stops phasing
				// No longer needed here, handled by BallActor
				// delete(a.phasingBallIntersections, m.ID)
			}
			// Update cached state
			ball.Vx = m.Vx
			ball.Vy = m.Vy
			ball.Radius = m.Radius
			ball.Mass = m.Mass
			ball.Phasing = m.Phasing
		}

	case BroadcastTick: // Message from broadcastTicker
		a.handleBroadcastTick(ctx)

	// --- Delegate to handlers defined in game_actor_handlers.go ---
	case AssignPlayerToRoom:
		a.handlePlayerConnect(ctx, m.WsConn)
	case PlayerDisconnect:
		a.handlePlayerDisconnect(ctx, m.WsConn)
	case ForwardedPaddleDirection:
		a.handlePaddleDirection(ctx, m.WsConn, m.Direction)
	case SpawnBallCommand:
		a.spawnBall(ctx, m.OwnerIndex, m.X, m.Y, m.ExpireIn, m.IsPermanent)
	case DestroyExpiredBall:
		a.handleDestroyExpiredBall(ctx, m.BallID)
	case ApplyBrickDamage: // Handle damage application instruction from BallActor
		a.handleApplyBrickDamage(ctx, m)
	// --- End Delegation ---

	case bollywood.Stopping:
		a.handleStopping(ctx)

	case bollywood.Stopped:
		a.handleStopped(ctx)

	default:
		fmt.Printf("GameActor %s: Received unknown message type: %T\n", a.selfPID, m)
	}
}