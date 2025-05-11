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

	// Collision Tracking
	activeCollisions *CollisionTracker // Tracks ongoing collisions (ball-brick, ball-paddle)

	// Phasing Timers (Managed by GameActor)
	phasingTimers   map[int]*time.Timer // Map ball ID to its phasing timer
	phasingTimersMu sync.Mutex          // Protects phasingTimers map

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
	Ws          *websocket.Conn // Can be nil in tests
	IsConnected bool
}

// NewGameActorProducer creates a producer for the GameActor.
func NewGameActorProducer(engine *bollywood.Engine, cfg utils.Config, roomManagerPID *bollywood.PID) bollywood.Producer {
	return func() bollywood.Actor {
		canvas := NewCanvas(cfg.CanvasSize, cfg.GridSize)
		// Grid generation happens when first player joins now

		ga := &GameActor{
			cfg:              cfg,
			canvas:           canvas, // Canvas exists, but grid is empty initially
			players:          [utils.MaxPlayers]*playerInfo{},
			paddles:          [utils.MaxPlayers]*Paddle{}, // Initialize cache map
			paddleActors:     [utils.MaxPlayers]*bollywood.PID{},
			balls:            make(map[int]*Ball), // Initialize cache map
			ballActors:       make(map[int]*bollywood.PID),
			engine:           engine,
			stopPhysicsCh:    make(chan struct{}), // Initialize channels here
			stopBroadcastCh:  make(chan struct{}),
			connToIndex:      make(map[*websocket.Conn]int),
			playerConns:      [utils.MaxPlayers]*websocket.Conn{},
			roomManagerPID:   roomManagerPID,
			pendingUpdates:   make([]interface{}, 0, 128), // Pre-allocate some capacity
			activeCollisions: NewCollisionTracker(),       // Initialize collision tracker
			phasingTimers:    make(map[int]*time.Timer),   // Initialize phasing timers map
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
			// Reply with error if it was an Ask request
			if ctx.RequestID() != "" {
				ctx.Reply(fmt.Errorf("game actor panicked: %v", r))
			}
		}
	}()

	// Set self PID if not already set
	if a.selfPID == nil {
		a.selfPID = ctx.Self()
		if a.selfPID == nil {
			fmt.Printf("ERROR: GameActor ???: Failed to set self PID on first Receive.")
			if ctx.RequestID() != "" {
				ctx.Reply(fmt.Errorf("failed to initialize game actor"))
			}
			return
		}
	}

	// Ignore messages if game is already over or stopping, except for system messages
	if a.gameOver.Load() || a.isStopping.Load() {
		switch ctx.Message().(type) {
		case bollywood.Stopping, bollywood.Stopped, PlayerDisconnect, stopPhasingTimerMsg: // Allow phasing timer msg during cleanup
			// Allow these messages during game over/stopping for cleanup
		default:
			// If it's an Ask request during shutdown, reply with an error
			if ctx.RequestID() != "" {
				ctx.Reply(fmt.Errorf("game actor is shutting down or game over"))
			}
			return // Ignore other messages
		}
	}

	// Main message switch
	switch m := ctx.Message().(type) {
	case bollywood.Started:
		a.handleStart(ctx)

	case GameTick: // Message from physicsTicker
		start := time.Now()

		// 1. Move entities based on current velocity/direction (updates cache)
		a.moveEntities()

		// 2. Detect and resolve collisions (updates cache: positions, velocities, Collided flags; adds score/event updates to pending)
		a.detectCollisions(ctx)

		// 3. Generate position updates from final cached state and add to pending
		a.generatePositionUpdates()

		// 4. Reset Collided flags in cache for the next tick
		a.resetPerTickCollisionFlags()

		// 5. Check for game over
		a.checkGameOver(ctx)

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
			ball.Vx = m.Vx // Update velocity from BallActor
			ball.Vy = m.Vy
			ball.Radius = m.Radius
			ball.Mass = m.Mass
			ball.Phasing = m.Phasing // Crucially update phasing state from BallActor
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
		a.spawnBall(ctx, m.OwnerIndex, m.X, m.Y, m.ExpireIn, m.IsPermanent, m.SetInitialPhasing)
	case DestroyExpiredBall:
		a.handleDestroyExpiredBall(ctx, m.BallID)
	case stopPhasingTimerMsg: // Handle internal timer expiry
		a.handleStopPhasingTimerMsg(ctx, m.BallID)
	// --- End Delegation ---

	// --- Internal Test Messages ---
	case internalAddBallTestMsg: // Handle internal message for adding ball in tests
		if m.Ball != nil && m.PID != nil {
			a.balls[m.Ball.Id] = m.Ball
			a.ballActors[m.Ball.Id] = m.PID
		}
	case internalStartTickersTestMsg: // Handle internal message for starting tickers in tests
		a.startTickers(ctx)
	case internalTestingAddPlayerAndStart: // Handle internal message for adding player and starting game in tests
		a.handleInternalTestPlayerAdd(ctx, m.PlayerIndex)
	case internalGetBallRequest: // Handle Ask request for ball state
		ball, exists := a.balls[m.BallID]
		var ballCopy *Ball
		if exists && ball != nil {
			// Create a copy to send back, avoid sending pointer to internal state
			temp := *ball
			ballCopy = &temp
		}
		ctx.Reply(internalGetBallResponse{Ball: ballCopy, Exists: exists})
	case internalGetBrickRequest: // Handle Ask request for brick state
		resp := internalGetBrickResponse{Exists: false}
		if a.canvas != nil && a.canvas.Grid != nil &&
			m.Row >= 0 && m.Row < len(a.canvas.Grid) &&
			m.Col >= 0 && m.Col < len(a.canvas.Grid[m.Row]) {
			cell := a.canvas.Grid[m.Row][m.Col]
			resp.Exists = true
			if cell.Data != nil {
				resp.Life = cell.Data.Life
				resp.Type = cell.Data.Type
				resp.IsBrick = (cell.Data.Type == utils.Cells.Brick)
			} else {
				resp.Type = utils.Cells.Empty // Assume empty if data is nil
			}
		}
		ctx.Reply(resp)
	case internalTriggerStartPhasingPowerUp:
		ball, ballExists := a.balls[m.BallID]
		ballActorPID, actorExists := a.ballActors[m.BallID]
		if ballExists && actorExists && ball != nil && ballActorPID != nil {
			// Apply phasing regardless of current state to reset timer if already phasing
			ball.Phasing = true
			a.startPhasingTimer(ball.Id) // This will stop existing timer and start new one
			a.engine.Send(ballActorPID, SetPhasingCommand{}, a.selfPID)
		}
	case internalConfirmPhasingRequest:
		ball, exists := a.balls[m.BallID]
		isPhasing := false
		if exists && ball != nil {
			isPhasing = ball.Phasing
		}
		ctx.Reply(internalConfirmPhasingResponse{IsPhasing: isPhasing, Exists: exists})
	// --- End Internal Test Messages ---

	case bollywood.Stopping:
		a.handleStopping(ctx)

	case bollywood.Stopped:
		a.handleStopped(ctx)

	default:
		if ctx.RequestID() != "" {
			ctx.Reply(fmt.Errorf("unknown message type: %T", m))
		}
	}
}

// handleInternalTestPlayerAdd sets up a player and starts the game for testing purposes.
func (a *GameActor) handleInternalTestPlayerAdd(ctx bollywood.Context, playerIndex int) {
	if playerIndex < 0 || playerIndex >= utils.MaxPlayers {
		fmt.Printf("ERROR: GameActor %s: Received internalTestingAddPlayerAndStart with invalid index %d\n", a.selfPID, playerIndex)
		return
	}
	if a.players[playerIndex] != nil {
		fmt.Printf("WARN: GameActor %s: Received internalTestingAddPlayerAndStart for already occupied index %d\n", a.selfPID, playerIndex)
		return
	}

	// Check if this is the first player (to initialize grid/tickers)
	isFirstPlayerInRoom := true
	for i, p := range a.players {
		if p != nil && i != playerIndex {
			isFirstPlayerInRoom = false
			break
		}
	}
	if isFirstPlayerInRoom {
		if a.canvas == nil {
			a.canvas = NewCanvas(a.cfg.CanvasSize, a.cfg.GridSize)
		}
		a.canvas.Grid.FillSymmetrical(a.cfg)
		// Tickers are now started by internalStartTickersTestMsg or by actual player connect
		// a.startTickers(ctx) // Do not start tickers here automatically for this test message
	} else if a.canvas == nil || a.canvas.Grid == nil {
		fmt.Printf("ERROR: GameActor %s: Adding test player %d but grid/canvas not initialized!\n", a.selfPID, playerIndex)
		return
	}

	// Create player info (without WsConn)
	playerDataPtr := NewPlayer(a.canvas, playerIndex)
	player := &playerInfo{
		Index:       playerIndex,
		ID:          playerDataPtr.Id,
		Color:       playerDataPtr.Color,
		Ws:          nil,  // Explicitly nil for test player
		IsConnected: true, // Mark as connected for game logic
	}
	player.Score.Store(playerDataPtr.Score)
	a.players[playerIndex] = player

	// Create paddle data and actor
	paddleDataPtr := NewPaddle(a.cfg, playerIndex)
	a.paddles[playerIndex] = paddleDataPtr
	paddleProducer := NewPaddleActorProducer(*paddleDataPtr, a.selfPID, a.cfg)
	paddlePID := a.engine.Spawn(bollywood.NewProps(paddleProducer))
	if paddlePID == nil {
		fmt.Printf("ERROR: GameActor %s failed to spawn PaddleActor for test player %d\n", a.selfPID, playerIndex)
		a.players[playerIndex] = nil
		a.paddles[playerIndex] = nil
		return
	}
	a.paddleActors[playerIndex] = paddlePID

	// Do not spawn ball here automatically for this test message, let tests control ball spawning
}

// --- Game Tick Processing Methods ---

// moveEntities updates positions of paddles and balls in cache based on their current velocities/directions.
func (a *GameActor) moveEntities() {
	// Update paddles
	for _, paddle := range a.paddles {
		if paddle != nil {
			paddle.Move() // Updates internal X, Y, Vx, Vy, IsMoving
		}
	}

	// Update balls
	for _, ball := range a.balls {
		if ball != nil {
			ball.Move() // Updates internal X, Y
		}
	}
}

// generatePositionUpdates creates BallPositionUpdate and PaddlePositionUpdate messages
// using the current state from the cache (after movement and collision resolution)
// and adds them to the pendingUpdates buffer.
func (a *GameActor) generatePositionUpdates() {
	canvasSize := a.cfg.CanvasSize

	// Paddle position updates
	for i, paddle := range a.paddles {
		if paddle != nil {
			// Calculate R3F coords for the paddle center
			r3fX, r3fY := mapToR3FCoords(paddle.X+paddle.Width/2, paddle.Y+paddle.Height/2, canvasSize)
			update := &PaddlePositionUpdate{
				MessageType: "paddlePositionUpdate", Index: i,
				X: paddle.X, Y: paddle.Y, // Original coords
				R3fX: r3fX, R3fY: r3fY, // R3F coords
				Width: paddle.Width, Height: paddle.Height, // Dimensions for frontend geometry
				Vx: paddle.Vx, Vy: paddle.Vy, IsMoving: paddle.IsMoving,
				Collided: paddle.Collided, // Use Collided flag set by detectCollisions
			}
			a.addUpdate(update)
		}
	}

	// Ball position updates
	for id, ball := range a.balls {
		if ball != nil {
			// Calculate R3F coords
			r3fX, r3fY := mapToR3FCoords(ball.X, ball.Y, canvasSize)
			update := &BallPositionUpdate{
				MessageType: "ballPositionUpdate", ID: id,
				X: ball.X, Y: ball.Y, // Original coords
				R3fX: r3fX, R3fY: r3fY, // R3F coords
				Vx: ball.Vx, Vy: ball.Vy,
				Collided: ball.Collided, // Use Collided flag set by detectCollisions
				Phasing:  ball.Phasing,  // Include current phasing state
			}
			a.addUpdate(update)
		}
	}
}

// resetPerTickCollisionFlags resets the Collided flag for all paddles and balls in the cache.
// This is done at the end of a tick, after updates have been generated.
func (a *GameActor) resetPerTickCollisionFlags() {
	for _, paddle := range a.paddles {
		if paddle != nil {
			paddle.Collided = false
		}
	}
	for _, ball := range a.balls {
		if ball != nil {
			ball.Collided = false
		}
	}
}
