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
	paddles        [utils.MaxPlayers]*Paddle     // Local cache of paddle state
	paddleActors   [utils.MaxPlayers]*bollywood.PID
	balls          map[int]*Ball // Local cache of ball state
	ballActors     map[int]*bollywood.PID
	engine         *bollywood.Engine
	ticker         *time.Ticker // Ticker for physics/game logic
	stopTickerCh   chan struct{}
	bcastTicker    *time.Ticker // Ticker for broadcasting state
	stopBcastCh    chan struct{}
	tickerMu       sync.Mutex // Mutex to protect ticker fields and channels
	selfPID        *bollywood.PID
	roomManagerPID *bollywood.PID
	broadcasterPID *bollywood.PID // PID of the dedicated broadcaster actor
	connToIndex    map[*websocket.Conn]int
	playerConns    [utils.MaxPlayers]*websocket.Conn
	gameOver       atomic.Bool // Flag to prevent multiple game over triggers

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
			cfg:            cfg,
			canvas:         canvas, // Canvas exists, but grid is empty initially
			players:        [utils.MaxPlayers]*playerInfo{},
			paddles:        [utils.MaxPlayers]*Paddle{}, // Initialize cache map
			paddleActors:   [utils.MaxPlayers]*bollywood.PID{},
			balls:          make(map[int]*Ball), // Initialize cache map
			ballActors:     make(map[int]*bollywood.PID),
			engine:         engine,
			stopTickerCh:   make(chan struct{}), // Initialize channels here
			stopBcastCh:    make(chan struct{}),
			connToIndex:    make(map[*websocket.Conn]int),
			playerConns:    [utils.MaxPlayers]*websocket.Conn{},
			roomManagerPID: roomManagerPID,
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

	case GameTick:
		start := time.Now()

		// 1. Send UpdatePositionCommand to all children
		updateCmd := UpdatePositionCommand{}
		pidsToUpdate := make([]*bollywood.PID, 0, utils.MaxPlayers+len(a.ballActors))
		for _, pid := range a.paddleActors {
			if pid != nil {
				pidsToUpdate = append(pidsToUpdate, pid)
			}
		}
		for _, pid := range a.ballActors {
			if pid != nil {
				pidsToUpdate = append(pidsToUpdate, pid)
			}
		}
		for _, pid := range pidsToUpdate {
			a.engine.Send(pid, updateCmd, a.selfPID)
		}

		// 2. Detect collisions using the locally cached state
		a.detectCollisions(ctx)

		// 3. Check for Game Over condition
		a.checkGameOver(ctx) // Check if game ended after collisions

		// Update metrics
		duration := time.Since(start)
		a.metricsMu.Lock()
		a.tickDurationSum += duration
		a.tickCount++
		a.metricsMu.Unlock()

	case PositionUpdateMessage: // Handle state updates from children
		if m.IsPaddle {
			paddleIndex := m.ActorID
			if paddleIndex >= 0 && paddleIndex < len(a.paddles) {
				if a.paddles[paddleIndex] == nil {
					// This case should ideally not happen if connect initializes cache
					// fmt.Printf("WARN: GameActor %s received paddle update for nil cache index %d from PID %s\n", a.selfPID, paddleIndex, m.PID) // Reduce noise
					a.paddles[paddleIndex] = &Paddle{Index: paddleIndex}
					a.paddles[paddleIndex].Velocity = a.cfg.PaddleVelocity
					if a.canvas != nil {
						a.paddles[paddleIndex].canvasSize = a.canvas.CanvasSize
					}
				}
				// Update state
				paddle := a.paddles[paddleIndex]
				paddle.X, paddle.Y = m.X, m.Y
				paddle.Vx, paddle.Vy = m.Vx, m.Vy
				paddle.Width, paddle.Height = m.Width, m.Height
				paddle.IsMoving = m.IsMoving
			} else {
				// Index out of bounds, log warning
				// fmt.Printf("WARN: GameActor %s received paddle update for invalid index %d from PID %s\n", a.selfPID, paddleIndex, m.PID) // Reduce noise
			}
		} else { // Is Ball
			ballID := m.ActorID
			if _, actorOk := a.ballActors[ballID]; actorOk { // Check if actor still exists
				if a.balls[ballID] == nil { // Initialize if nil
					// fmt.Printf("WARN: GameActor %s received ball update for nil cache ID %d from PID %s\n", a.selfPID, ballID, m.PID) // Reduce noise
					a.balls[ballID] = &Ball{Id: ballID}
					a.balls[ballID].Mass = a.cfg.BallMass
					if a.canvas != nil {
						a.balls[ballID].canvasSize = a.canvas.CanvasSize
					}
					// OwnerIndex and IsPermanent are harder to set here, rely on initial spawn
				}
				// Update state
				ball := a.balls[ballID]
				ball.X, ball.Y = m.X, m.Y
				ball.Vx, ball.Vy = m.Vx, m.Vy
				ball.Radius = m.Radius
				ball.Phasing = m.Phasing
			} else {
				// Actor doesn't exist, ignore update
			}
		}

	case BroadcastTick:
		if a.broadcasterPID != nil {
			snapshot := a.createGameStateSnapshot()
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
		fmt.Printf("PERF_METRIC GameActor %s: AvgTick=%v Ticks=%d\n", a.selfPID, avgDuration, a.tickCount)
	} else {
		fmt.Printf("PERF_METRIC GameActor %s: No ticks processed.\n", a.selfPID)
	}
}

// startTickers starts the physics and broadcast tickers.
func (a *GameActor) startTickers(ctx bollywood.Context) {
	a.tickerMu.Lock() // Lock before accessing ticker fields/channels

	// Physics Ticker
	if a.ticker == nil {
		a.ticker = time.NewTicker(a.cfg.GameTickPeriod)
		select {
		case <-a.stopTickerCh:
			a.stopTickerCh = make(chan struct{})
		default:
		}
		stopCh := a.stopTickerCh
		tickerCh := a.ticker.C

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
						return
					}
					if a.gameOver.Load() {
						return
					}
					currentEngine := a.engine
					currentSelfPID := a.selfPID
					if currentEngine != nil && currentSelfPID != nil {
						currentEngine.Send(currentSelfPID, GameTick{}, nil)
					} else {
						return
					}
				}
			}
		}()
	} else {
		a.tickerMu.Unlock()
	}

	a.tickerMu.Lock() // Lock again for broadcast ticker

	// Broadcast Ticker
	if a.bcastTicker == nil {
		broadcastInterval := 100 * time.Millisecond
		if broadcastInterval < a.cfg.GameTickPeriod {
			broadcastInterval = a.cfg.GameTickPeriod
		}
		a.bcastTicker = time.NewTicker(broadcastInterval)
		select {
		case <-a.stopBcastCh:
			a.stopBcastCh = make(chan struct{})
		default:
		}
		stopCh := a.stopBcastCh
		tickerCh := a.bcastTicker.C

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
						return
					}
					if a.gameOver.Load() {
						return
					}
					currentEngine := a.engine
					currentSelfPID := a.selfPID
					if currentEngine != nil && currentSelfPID != nil {
						currentEngine.Send(currentSelfPID, BroadcastTick{}, nil)
					} else {
						return
					}
				}
			}
		}()
	} else {
		a.tickerMu.Unlock()
	}
}

// stopTickers stops the physics and broadcast tickers safely using the mutex.
func (a *GameActor) stopTickers() {
	a.tickerMu.Lock()
	defer a.tickerMu.Unlock()

	// Stop Physics Ticker
	if a.ticker != nil {
		a.ticker.Stop()
		select {
		case <-a.stopTickerCh: // Already closed
		default:
			close(a.stopTickerCh)
		}
		a.ticker = nil
	}

	// Stop Broadcast Ticker
	if a.bcastTicker != nil {
		a.bcastTicker.Stop()
		select {
		case <-a.stopBcastCh: // Already closed
		default:
			close(a.stopBcastCh)
		}
		a.bcastTicker = nil
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
			// Don't stop broadcaster here if game over, let it send message first
			// It will be stopped when GameActor stops.
			// If stopping for other reasons, stop it now.
			if !a.gameOver.Load() {
				currentEngine.Stop(broadcasterToStop)
				a.broadcasterPID = nil // Mark as stopped
			}
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

// --- Handler method bodies removed from this file ---
// Implementations are now solely in game_actor_handlers.go
