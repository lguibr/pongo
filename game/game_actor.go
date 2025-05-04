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
	cfg            utils.Config
	canvas         *Canvas
	players        [utils.MaxPlayers]*playerInfo // State managed serially by actor
	paddles        [utils.MaxPlayers]*Paddle     // Local cache, authoritative state for simulation
	paddleActors   [utils.MaxPlayers]*bollywood.PID
	balls          map[int]*Ball // Local cache, authoritative state for simulation
	ballActors     map[int]*bollywood.PID
	engine         *bollywood.Engine
	physicsTicker  *time.Ticker // Ticker for physics/game logic
	stopPhysicsCh  chan struct{}
	broadcastTicker *time.Ticker // Ticker for broadcasting state
	stopBroadcastCh chan struct{}
	tickerMu        sync.Mutex // Mutex to protect ticker fields and channels
	selfPID         *bollywood.PID
	roomManagerPID  *bollywood.PID
	broadcasterPID  *bollywood.PID // PID of the dedicated broadcaster actor
	connToIndex     map[*websocket.Conn]int
	playerConns     [utils.MaxPlayers]*websocket.Conn
	gameOver        atomic.Bool // Flag to prevent multiple game over triggers

	// Performance Metrics
	tickDurationSum time.Duration
	tickCount       int64
	metricsMu       sync.Mutex // Protect metrics during updates
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
			// Initialize metrics
			tickDurationSum: 0,
			tickCount:       0,
		}
		ga.gameOver.Store(false) // Initialize game over flag
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
			// Ensure cleanup even on panic
			if !a.gameOver.Load() { // Avoid double cleanup if panic happens during game over sequence
				if a.roomManagerPID != nil && a.engine != nil && a.selfPID != nil {
					a.engine.Send(a.roomManagerPID, GameRoomEmpty{RoomPID: a.selfPID}, nil)
				}
				if a.broadcasterPID != nil {
					a.engine.Stop(a.broadcasterPID)
				}
				a.stopTickers()
				a.logPerformanceMetrics() // Log metrics on panic exit
			}
		}
	}()

	// Set self PID if not already set
	if a.selfPID == nil {
		a.selfPID = ctx.Self()
		if a.selfPID == nil {
			fmt.Println("ERROR: GameActor ???: Failed to set self PID on first Receive.")
			return // Cannot proceed without PID
		}
	}

	// Ignore messages if game is already over, except for system messages
	if a.gameOver.Load() {
		switch ctx.Message().(type) {
		case bollywood.Stopping, bollywood.Stopped, PlayerDisconnect:
			// Allow these messages during game over for cleanup
		default:
			return // Ignore other messages
		}
	}

	// Main message switch
	switch m := ctx.Message().(type) {
	case bollywood.Started:
		broadcasterProps := bollywood.NewProps(NewBroadcasterProducer(a.selfPID))
		a.broadcasterPID = a.engine.Spawn(broadcasterProps)
		if a.broadcasterPID == nil {
			fmt.Printf("FATAL: GameActor %s failed to spawn BroadcasterActor. Stopping self.\n", a.selfPID)
			a.engine.Stop(a.selfPID)
			return
		}
		fmt.Printf("GameActor %s: Started. Broadcaster: %s.\n", a.selfPID, a.broadcasterPID)
		// Tickers are started when the first player joins

	case GameTick: // Message from physicsTicker
		start := time.Now()

		// 1. Update internal state cache (move paddles, balls based on current velocity/direction)
		// This uses the latest state received via PaddleStateUpdate/BallStateUpdate
		a.updateInternalState()

		// 2. Detect collisions using the updated cache and send commands to children
		a.detectCollisions(ctx)

		// 3. Check for Game Over condition
		a.checkGameOver(ctx) // Check if game ended after collisions

		// Update metrics
		duration := time.Since(start)
		a.metricsMu.Lock()
		a.tickDurationSum += duration
		a.tickCount++
		a.metricsMu.Unlock()

	case PaddleStateUpdate: // Update cache with state from PaddleActor
		if paddle := a.paddles[m.Index]; paddle != nil {
			paddle.Direction = m.Direction
		} else {
			// Log if trying to update a non-existent paddle cache entry
			// fmt.Printf("WARN: GameActor %s received PaddleStateUpdate for nil cache index %d from PID %s\n", a.selfPID, m.Index, m.PID)
		}

	case BallStateUpdate: // Update cache with state from BallActor
		if ball := a.balls[m.ID]; ball != nil {
			ball.Vx = m.Vx
			ball.Vy = m.Vy
			ball.Radius = m.Radius
			ball.Mass = m.Mass
			ball.Phasing = m.Phasing
		} else {
			// Log if trying to update a non-existent ball cache entry
			// fmt.Printf("WARN: GameActor %s received BallStateUpdate for nil cache ID %d from PID %s\n", a.selfPID, m.ID, m.PID)
		}

	case BroadcastTick: // Message from broadcastTicker
		if a.broadcasterPID != nil {
			snapshot := a.createGameStateSnapshot() // Reads cache, resets Collided flags
			a.engine.Send(a.broadcasterPID, BroadcastStateCommand{State: snapshot}, a.selfPID)
		}

	// --- Delegate to handlers in game_actor_handlers.go ---
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
	// --- End Delegation ---

	case bollywood.Stopping:
		fmt.Printf("GameActor %s: Stopping.\n", a.selfPID)
		a.gameOver.Store(true) // Ensure flag is set on stop
		a.stopTickers()
		a.cleanupChildActorsAndConnections()
		a.logPerformanceMetrics() // Log metrics on graceful exit

	case bollywood.Stopped:
		fmt.Printf("GameActor %s: Stopped.\n", a.selfPID)

	default:
		fmt.Printf("GameActor %s: Received unknown message type: %T\n", a.selfPID, m)
	}
}

// updateInternalState applies velocity/direction to update positions in the local cache.
// This is called during the GameTick.
func (a *GameActor) updateInternalState() {
	// Update paddles based on their Direction (set by PaddleStateUpdate)
	for _, paddle := range a.paddles {
		if paddle != nil {
			paddle.Move() // Paddle.Move updates its own X, Y, Vx, Vy, IsMoving based on Direction
		}
	}
	// Update balls based on their Vx, Vy (set by BallStateUpdate)
	for _, ball := range a.balls {
		if ball != nil {
			ball.Move() // Ball.Move updates its own X, Y based on Vx, Vy
		}
	}
}

// checkGameOver checks if all bricks are destroyed and triggers the end sequence.
func (a *GameActor) checkGameOver(ctx bollywood.Context) {
	if a.gameOver.Load() || a.canvas == nil || a.canvas.Grid == nil {
		return // Already over or grid not initialized
	}

	allBricksGone := true
	for _, row := range a.canvas.Grid {
		for _, cell := range row {
			if cell.Data != nil && cell.Data.Type == utils.Cells.Brick {
				allBricksGone = false
				break
			}
		}
		if !allBricksGone {
			break
		}
	}

	if allBricksGone {
		// --- Game Over Sequence ---
		if !a.gameOver.CompareAndSwap(false, true) {
			return // Already handled
		}

		fmt.Printf("GameActor %s: GAME_OVER - All bricks destroyed.\n", a.selfPID) // Clear Game Over Log

		// 1. Stop Tickers
		a.stopTickers()

		// 2. Determine Winner
		winnerIndex := -1
		highestScore := int32(-999999) // Use a very low initial value
		var finalScores [utils.MaxPlayers]int32
		tie := false

		for i, p := range a.players {
			if p != nil && p.IsConnected {
				score := p.Score.Load()
				finalScores[i] = score
				if score > highestScore {
					highestScore = score
					winnerIndex = i
					tie = false
				} else if score == highestScore && score > -999999 { // Check score is not the initial low value
					tie = true
				}
			} else if p != nil { // Player exists but disconnected
				finalScores[i] = p.Score.Load() // Record score even if disconnected
			} else {
				finalScores[i] = 0
			}
		}
		if tie {
			winnerIndex = -1 // Mark as tie
		}

		fmt.Printf("GameActor %s: GAME_OVER - Winner Index: %d (Score: %d)\n", a.selfPID, winnerIndex, highestScore)

		// 3. Send GameOverMessage via Broadcaster
		if a.broadcasterPID != nil {
			gameOverMsg := GameOverMessage{
				WinnerIndex: winnerIndex,
				FinalScores: finalScores,
				Reason:      "All bricks destroyed",
				RoomPID:     a.selfPID.String(),
				MessageType: "gameOver", // Add message type
			}
			a.engine.Send(a.broadcasterPID, gameOverMsg, a.selfPID)
		}

		// 4. Notify RoomManager
		if a.roomManagerPID != nil {
			fmt.Printf("GameActor %s: GAME_OVER - Notifying RoomManager %s.\n", a.selfPID, a.roomManagerPID)
			a.engine.Send(a.roomManagerPID, GameRoomEmpty{RoomPID: a.selfPID}, nil)
		}

		// 5. Initiate Self Stop
		if a.engine != nil && a.selfPID != nil {
			a.engine.Stop(a.selfPID)
		}
	}
}

// logPerformanceMetrics calculates and prints the average tick duration.
func (a *GameActor) logPerformanceMetrics() {
	a.metricsMu.Lock()
	defer a.metricsMu.Unlock()

	if a.tickCount > 0 {
		avgDuration := a.tickDurationSum / time.Duration(a.tickCount)
		// Use a distinct prefix for easy filtering
		fmt.Printf("PERF_METRIC GameActor %s: AvgPhysicsTick=%v Ticks=%d\n", a.selfPID, avgDuration, a.tickCount)
	} else {
		fmt.Printf("PERF_METRIC GameActor %s: No physics ticks processed.\n", a.selfPID)
	}
}

// startTickers starts the physics and broadcast tickers.
func (a *GameActor) startTickers(ctx bollywood.Context) {
	a.tickerMu.Lock() // Lock before accessing ticker fields/channels

	// Physics Ticker
	if a.physicsTicker == nil {
		a.physicsTicker = time.NewTicker(a.cfg.GameTickPeriod)
		select {
		case <-a.stopPhysicsCh: // Channel might be closed if restarted quickly
			a.stopPhysicsCh = make(chan struct{}) // Recreate if closed
		default:
		}
		stopCh := a.stopPhysicsCh
		tickerCh := a.physicsTicker.C

		// Unlock before starting goroutine
		a.tickerMu.Unlock()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("PANIC recovered in GameActor %s Physics Ticker: %v\n", a.selfPID, r)
				}
			}()
			for {
				select {
				case <-stopCh:
					return
				case _, ok := <-tickerCh:
					if !ok {
						return // Ticker channel closed
					}
					if a.gameOver.Load() {
						return // Stop ticker if game is over
					}
					// Send tick message to self
					currentEngine := a.engine
					currentSelfPID := a.selfPID
					if currentEngine != nil && currentSelfPID != nil {
						currentEngine.Send(currentSelfPID, GameTick{}, nil)
					} else {
						return // Engine or PID gone, stop ticker
					}
				}
			}
		}()
	} else {
		// Already running, unlock
		a.tickerMu.Unlock()
	}

	a.tickerMu.Lock() // Lock again for broadcast ticker

	// Broadcast Ticker
	if a.broadcastTicker == nil {
		broadcastInterval := time.Second / time.Duration(a.cfg.BroadcastRateHz)
		if broadcastInterval <= 0 {
			broadcastInterval = 33 * time.Millisecond // Default fallback if rate is invalid
		}
		a.broadcastTicker = time.NewTicker(broadcastInterval)
		select {
		case <-a.stopBroadcastCh: // Channel might be closed if restarted quickly
			a.stopBroadcastCh = make(chan struct{}) // Recreate if closed
		default:
		}
		stopCh := a.stopBroadcastCh
		tickerCh := a.broadcastTicker.C

		// Unlock before starting goroutine
		a.tickerMu.Unlock()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("PANIC recovered in GameActor %s Broadcast Ticker: %v\n", a.selfPID, r)
				}
			}()
			for {
				select {
				case <-stopCh:
					return
				case _, ok := <-tickerCh:
					if !ok {
						return // Ticker channel closed
					}
					if a.gameOver.Load() {
						return // Stop ticker if game is over
					}
					// Send tick message to self
					currentEngine := a.engine
					currentSelfPID := a.selfPID
					if currentEngine != nil && currentSelfPID != nil {
						currentEngine.Send(currentSelfPID, BroadcastTick{}, nil)
					} else {
						return // Engine or PID gone, stop ticker
					}
				}
			}
		}()
	} else {
		// Already running, unlock
		a.tickerMu.Unlock()
	}
}

// stopTickers stops the physics and broadcast tickers safely using the mutex.
func (a *GameActor) stopTickers() {
	a.tickerMu.Lock()
	defer a.tickerMu.Unlock()

	// Stop Physics Ticker
	if a.physicsTicker != nil {
		a.physicsTicker.Stop()
		select {
		case <-a.stopPhysicsCh: // Already closed
		default:
			close(a.stopPhysicsCh)
		}
		a.physicsTicker = nil
	}

	// Stop Broadcast Ticker
	if a.broadcastTicker != nil {
		a.broadcastTicker.Stop()
		select {
		case <-a.stopBroadcastCh: // Already closed
		default:
			close(a.stopBroadcastCh)
		}
		a.broadcastTicker = nil
	}
}

// cleanupChildActorsAndConnections stops all managed actors and cleans caches.
func (a *GameActor) cleanupChildActorsAndConnections() {
	paddlesToStop := make([]*bollywood.PID, 0, utils.MaxPlayers)
	ballsToStop := make([]*bollywood.PID, 0, len(a.ballActors))
	broadcasterToStop := a.broadcasterPID // Capture PID before potentially nil-ing

	for i := 0; i < utils.MaxPlayers; i++ {
		if pid := a.paddleActors[i]; pid != nil {
			paddlesToStop = append(paddlesToStop, pid)
			a.paddleActors[i] = nil
			a.paddles[i] = nil // Clear paddle state cache
		}
		if pInfo := a.players[i]; pInfo != nil {
			if pInfo.Ws != nil {
				delete(a.connToIndex, pInfo.Ws)
			}
			pInfo.Ws = nil // Nil out reference
			pInfo.IsConnected = false
		}
		a.players[i] = nil
		a.playerConns[i] = nil
	}
	for ballID, pid := range a.ballActors {
		if pid != nil {
			ballsToStop = append(ballsToStop, pid)
		}
		delete(a.ballActors, ballID)
		delete(a.balls, ballID) // Clear ball state cache
	}
	if len(a.connToIndex) > 0 {
		a.connToIndex = make(map[*websocket.Conn]int) // Clear map
	}

	// Stop actors outside the loop
	currentEngine := a.engine
	if currentEngine != nil {
		if broadcasterToStop != nil && a.broadcasterPID != nil { // Check if not already nilled
			// Stop broadcaster during general cleanup or if game didn't end normally
			currentEngine.Stop(broadcasterToStop)
			a.broadcasterPID = nil // Mark as stopped
		}
		for _, pid := range paddlesToStop {
			if pid != nil {
				currentEngine.Stop(pid)
			}
		}
		for _, pid := range ballsToStop {
			if pid != nil {
				currentEngine.Stop(pid)
			}
		}
	}
}