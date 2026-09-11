package game

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/internal/transport"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket"
)

// Phase represents the current state of the game room.
type Phase int

const (
	PhaseLobby Phase = iota
	PhaseCountingDown
	PhasePlaying
)

// GameActor owns all simulation state for a single room.
type GameActor struct {
	finalDeliveryHandedOff bool
	reconnectGeneration    uint64
	expiryTimers           map[int]*time.Timer
	countdownGeneration    uint64
	cleanupGeneration      uint64
	phasingGeneration      map[int]uint64
	gridDirty              bool
	gridInitialized        bool
	forceStartPending      bool
	nextBallID             int
	physicsPending         actor.PendingTick
	broadcastPending       actor.PendingTick
	lastHeartbeat          time.Time
	cfg                    utils.Config
	canvas                 *Canvas
	players                [utils.MaxPlayers]*playerInfo // State managed serially by actor
	paddles                [utils.MaxPlayers]*Paddle     // Local cache, authoritative state for simulation
	balls                  map[int]*Ball                 // Local cache, authoritative state for simulation
	engine                 *actor.Engine
	physicsTicker          *time.Ticker // Ticker for physics/game logic
	stopPhysicsCh          chan struct{}
	broadcastTicker        *time.Ticker // Ticker for broadcasting state
	stopBroadcastCh        chan struct{}
	selfPID                *actor.PID
	roomManagerPID         *actor.PID
	broadcasterPID         *actor.PID // PID of the dedicated broadcaster actor
	connToIndex            map[*websocket.Conn]int
	playerConns            [utils.MaxPlayers]*websocket.Conn
	gameOver               bool  // Set once the game has ended or the room is stopping
	phase                  Phase // Current phase of the room

	// Buffer for pending updates to broadcast
	pendingUpdates []interface{} // Holds pointers to newly allocated update messages

	// Collision Tracking
	activeCollisions *CollisionTracker // Tracks ongoing collisions (ball-brick, ball-paddle)

	// Phasing Timers (Managed by GameActor)
	phasingTimers map[int]*time.Timer // Map ball ID to its phasing timer

	// Countdown Timer
	countdownTimer *time.Timer

	// Room Cleanup Timer (for grace period when empty)
	roomCleanupTimer *time.Timer

	// Reconnection Timers
	reconnectTimers map[int]*time.Timer // Map player index to timer

	// Performance Metrics
	tickDurationSum time.Duration
	tickCount       int64
	maxQueueLen     int // Largest backlog seen at the end of a physics tick

	// Cleanup control
	cleanupOnce sync.Once // Ensures cleanup happens only once
	isStopping  bool      // Set when the room starts stopping
}

// playerInfo holds state associated with a connected player/websocket.
type playerInfo struct {
	DisconnectGeneration uint64
	Index                int
	ID                   string
	SessionID            string // Unique session ID for reconnection
	Score                int32
	Color                [3]int
	Client               *transport.Client
	Ws                   *websocket.Conn // Can be nil in tests
	IsConnected          bool
	IsReady              bool // Lobby readiness
}

// NewGameActorProducer creates a producer for the GameActor.
func NewGameActorProducer(engine *actor.Engine, cfg utils.Config, roomManagerPID *actor.PID) actor.Producer {
	return func() actor.Actor {
		canvas := NewCanvas(cfg.CanvasSize, cfg.GridSize)
		// Grid generation happens when first player joins now

		ga := &GameActor{
			cfg:              cfg,
			canvas:           canvas, // Canvas exists, but grid is empty initially
			players:          [utils.MaxPlayers]*playerInfo{},
			paddles:          [utils.MaxPlayers]*Paddle{}, // Initialize cache map
			balls:            make(map[int]*Ball),         // Initialize cache map
			engine:           engine,
			connToIndex:      make(map[*websocket.Conn]int),
			playerConns:      [utils.MaxPlayers]*websocket.Conn{},
			roomManagerPID:   roomManagerPID,
			pendingUpdates:   make([]interface{}, 0, 128), // Pre-allocate some capacity
			activeCollisions: NewCollisionTracker(),       // Initialize collision tracker
			phasingTimers:    make(map[int]*time.Timer),   // Initialize phasing timers map
			reconnectTimers:  make(map[int]*time.Timer),   // Initialize reconnect timers map
			// Initialize metrics
			tickDurationSum: 0,
			tickCount:       0,
			phase:           PhaseLobby,
		}
		return ga
	}
}

// Receive is the main message handler for the GameActor.
func (a *GameActor) Receive(ctx actor.Context) {
	// Defer panic recovery
	defer func() {
		if r := recover(); r != nil {
			pidStr := "unknown"
			if a.selfPID != nil {
				pidStr = a.selfPID.String()
			}
			slog.Error("game actor panic", "room", pidStr, "panic", r, "stack", string(debug.Stack()))
			// Ensure cleanup happens exactly once, even on panic
			a.performCleanup()
			// Notify room manager that this room is now defunct due to panic
			if a.roomManagerPID != nil && a.engine != nil && a.selfPID != nil {
				slog.Debug("notifying room manager of panic exit", "room", a.selfPID)
				a.engine.Send(a.roomManagerPID, GameRoomEmpty{RoomPID: a.selfPID}, nil)
			}
			// Explicitly stop self if panic occurred before normal shutdown sequence
			if !a.isStopping && a.engine != nil && a.selfPID != nil {
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
			slog.Error("game actor has no PID")
			if ctx.RequestID() != "" {
				ctx.Reply(fmt.Errorf("failed to initialize game actor"))
			}
			return
		}
	}

	// Ignore messages if game is already over or stopping, except for system messages
	if a.gameOver || a.isStopping {
		switch ctx.Message().(type) {
		case AssignPlayerToRoom, actor.Stopping, actor.Stopped, PlayerDisconnect, stopPhasingTimerMsg, stopReconnectTimerMsg: // Allow timers during cleanup
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
	case actor.Started:
		a.handleStart(ctx)

	case GameTick: // Message from physicsTicker
		a.physicsPending.Clear()
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
		a.tickDurationSum += duration
		a.tickCount++
		if n := a.engine.QueueLen(a.selfPID); n > a.maxQueueLen {
			a.maxQueueLen = n
		}

	case BroadcastTick: // Message from broadcastTicker
		a.broadcastPending.Clear()
		a.handleBroadcastTick(ctx)

	// --- Handlers live in game_actor_admission.go, _disconnect.go, _entities.go and _lobby.go ---
	case AssignPlayerToRoom:
		a.handleAdmission(ctx, m)
	case PlayerDisconnect:
		a.handlePlayerDisconnect(ctx, m.WsConn)
	case ForwardedPaddleDirection:
		a.handlePaddleDirection(ctx, m.WsConn, m.Direction)
	case DestroyExpiredBall:
		a.handleDestroyExpiredBall(ctx, m.BallID)
	case stopPhasingTimerMsg: // Handle internal timer expiry
		if m.Generation == a.phasingGeneration[m.BallID] {
			a.handleStopPhasingTimerMsg(ctx, m.BallID)
		}
	case stopReconnectTimerMsg:
		if m.PlayerIndex >= 0 && m.PlayerIndex < utils.MaxPlayers && a.players[m.PlayerIndex] != nil && a.players[m.PlayerIndex].DisconnectGeneration == m.Generation {
			a.handleStopReconnectTimerMsg(ctx, m.PlayerIndex)
		}
	case ForwardedPlayerReady:
		a.handlePlayerReady(ctx, m.WsConn, m.IsReady)
	case startCountdownMsg:
		a.startCountdown(ctx)
	case startGameMsg:
		if m.Generation == a.countdownGeneration {
			a.startGame(ctx)
		}
	case ForceStartGame:
		a.handleForceStartGame(ctx)
	case CountdownTick:
		if m.Generation == a.countdownGeneration {
			a.handleCountdownTick(ctx, m.SecondsRemaining)
		}
	case RoomCleanupTimeout:
		if m.Generation == a.cleanupGeneration {
			a.handleRoomCleanupTimeout(ctx)
		}
	// --- End Delegation ---

	// --- Internal Test Messages ---
	case internalAddBallTestMsg: // Handle internal message for adding ball in tests
		if m.Ball != nil {
			a.balls[m.Ball.Id] = m.Ball
		}
	case internalStartTickersTestMsg: // Handle internal message for starting tickers in tests
		a.startPhysicsTicker(ctx)
		a.startBroadcastTicker(ctx)
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
		if ballExists && ball != nil {
			ball.Phasing = true
			a.startPhasingTimer(ball.Id)
		}
	case internalConfirmPhasingRequest:
		ball, exists := a.balls[m.BallID]
		isPhasing := false
		if exists && ball != nil {
			isPhasing = ball.Phasing
		}
		ctx.Reply(internalConfirmPhasingResponse{IsPhasing: isPhasing, Exists: exists})
	// --- End Internal Test Messages ---

	case actor.Stopping:
		a.handleStopping(ctx)

	case actor.Stopped:
		a.handleStopped(ctx)

	default:
		if ctx.RequestID() != "" {
			ctx.Reply(fmt.Errorf("unknown message type: %T", m))
		}
	}
}

// handleInternalTestPlayerAdd sets up a player and starts the game for testing purposes.
func (a *GameActor) handleInternalTestPlayerAdd(ctx actor.Context, playerIndex int) {
	if playerIndex < 0 || playerIndex >= utils.MaxPlayers {
		slog.Error("test player index out of range", "room", a.selfPID, "index", playerIndex)
		return
	}
	if a.players[playerIndex] != nil {
		slog.Warn("test player slot occupied", "room", a.selfPID, "index", playerIndex)
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
		slog.Error("test player added before grid initialization", "room", a.selfPID, "index", playerIndex)
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
		SessionID:   "test-session",
	}
	player.Score = playerDataPtr.Score
	a.players[playerIndex] = player

	a.paddles[playerIndex] = NewPaddle(a.cfg, playerIndex)

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
