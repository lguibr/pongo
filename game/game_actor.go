// File: game/game_actor.go
package game

import (
	// Keep json import for potential future use or debugging
	"fmt"
	"runtime/debug"
	"sync" // Import sync
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
	paddles        [utils.MaxPlayers]*Paddle     // State managed serially by actor
	paddleActors   [utils.MaxPlayers]*bollywood.PID
	balls          map[int]*Ball // State managed serially by actor
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
		canvas.Grid.Fill(cfg.GridFillVectors, cfg.GridFillVectorSize, cfg.GridFillWalkers, cfg.GridFillSteps)

		ga := &GameActor{
			cfg:            cfg,
			canvas:         canvas,
			players:        [utils.MaxPlayers]*playerInfo{},
			paddles:        [utils.MaxPlayers]*Paddle{},
			paddleActors:   [utils.MaxPlayers]*bollywood.PID{},
			balls:          make(map[int]*Ball),
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
		return ga
	}
}

// Receive is the main message handler for the GameActor.
func (a *GameActor) Receive(ctx bollywood.Context) {
	defer func() {
		if r := recover(); r != nil {
			pidStr := "unknown"
			if a.selfPID != nil {
				pidStr = a.selfPID.String()
			}
			fmt.Printf("PANIC recovered in GameActor %s Receive: %v\nStack trace:\n%s\n", pidStr, r, string(debug.Stack()))
			if a.roomManagerPID != nil && a.engine != nil && a.selfPID != nil {
				a.engine.Send(a.roomManagerPID, GameRoomEmpty{RoomPID: a.selfPID}, nil)
			}
			if a.broadcasterPID != nil {
				a.engine.Stop(a.broadcasterPID)
			}
			// Ensure tickers are stopped on panic
			a.stopTickers()
			// Log metrics on panic as well
			a.logPerformanceMetrics()
		}
	}()

	if a.selfPID == nil {
		a.selfPID = ctx.Self()
		if a.selfPID == nil {
			fmt.Println("ERROR: GameActor ???: Failed to set self PID on first Receive.")
			return
		}
	}

	switch m := ctx.Message().(type) {
	case bollywood.Started:
		broadcasterProps := bollywood.NewProps(NewBroadcasterProducer(a.selfPID))
		a.broadcasterPID = a.engine.Spawn(broadcasterProps)
		if a.broadcasterPID == nil {
			fmt.Printf("FATAL: GameActor %s failed to spawn BroadcasterActor. Stopping self.\n", a.selfPID)
			a.engine.Stop(a.selfPID) // This will trigger Stopping case below
			return
		}
		fmt.Printf("GameActor %s: Started. Broadcaster: %s.\n", a.selfPID, a.broadcasterPID)
		a.startTickers(ctx) // Start tickers after actor is fully started

	case GameTick: // Physics tick - Process directly
		start := time.Now() // Start timer

		// fmt.Printf("GameActor %s: Received GameTick\n", a.selfPID) // Optional: Add log
		updateCmd := UpdatePositionCommand{}
		// Collect PIDs without lock
		pidsToUpdate := make([]*bollywood.PID, 0, len(a.paddleActors)+len(a.ballActors))
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
		// Send updates
		for _, pid := range pidsToUpdate {
			a.engine.Send(pid, updateCmd, a.selfPID)
		}
		// Detect collisions (operates on actor state directly now)
		a.detectCollisions(ctx)

		// Update metrics
		duration := time.Since(start)
		a.metricsMu.Lock()
		a.tickDurationSum += duration
		a.tickCount++
		a.metricsMu.Unlock()

	case BroadcastTick: // Broadcast tick - Process directly
		// fmt.Printf("GameActor %s: Received BroadcastTick\n", a.selfPID) // Optional: Add log
		if a.broadcasterPID != nil {
			// Create snapshot (operates on actor state directly now)
			snapshot := a.createGameStateSnapshot()
			// Send the GameState struct directly
			a.engine.Send(a.broadcasterPID, BroadcastStateCommand{State: snapshot}, a.selfPID)
		}

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

	case bollywood.Stopping:
		fmt.Printf("GameActor %s: Stopping.\n", a.selfPID)
		a.stopTickers()
		a.cleanupChildActorsAndConnections()
		// Log metrics before fully stopped
		a.logPerformanceMetrics()

	case bollywood.Stopped:
		fmt.Printf("GameActor %s: Stopped.\n", a.selfPID)
		// Metrics are logged in Stopping phase

	default:
		fmt.Printf("GameActor %s: Received unknown message type: %T\n", a.selfPID, m)
	}
}

// logPerformanceMetrics calculates and prints the average tick duration.
func (a *GameActor) logPerformanceMetrics() {
	a.metricsMu.Lock()
	defer a.metricsMu.Unlock()

	if a.tickCount > 0 {
		avgDuration := a.tickDurationSum / time.Duration(a.tickCount)
		fmt.Printf("PERF_METRIC GameActor %s: Avg Tick Duration: %v (%d ticks)\n", a.selfPID, avgDuration, a.tickCount)
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
		// Ensure stop channel is fresh if restarting
		select {
		case <-a.stopTickerCh: // If already closed, make a new one
			a.stopTickerCh = make(chan struct{})
		default:
		}
		stopCh := a.stopTickerCh // Capture channel for goroutine
		tickerCh := a.ticker.C   // Capture ticker channel

		a.tickerMu.Unlock() // Unlock before starting goroutine

		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("PANIC recovered in GameActor %s Physics Ticker: %v\n", a.selfPID, r)
				}
				fmt.Printf("GameActor %s: Physics Ticker stopped.\n", a.selfPID)
			}()
			for {
				select {
				case <-stopCh: // Check stop channel first
					return
				case _, ok := <-tickerCh: // Read from ticker channel
					if !ok {
						return // Ticker channel closed
					}
					// Send tick message to self only if engine and PID are valid
					currentEngine := a.engine
					currentSelfPID := a.selfPID
					if currentEngine != nil && currentSelfPID != nil {
						currentEngine.Send(currentSelfPID, GameTick{}, nil)
					} else {
						return // Engine or PID gone, stop goroutine
					}
				}
			}
		}()
	} else {
		a.tickerMu.Unlock() // Unlock if ticker already exists
	}

	a.tickerMu.Lock() // Lock again for broadcast ticker

	// Broadcast Ticker
	if a.bcastTicker == nil {
		// Increase broadcast interval slightly
		broadcastInterval := 50 * time.Millisecond // Increased from 30ms
		if broadcastInterval < a.cfg.GameTickPeriod {
			broadcastInterval = a.cfg.GameTickPeriod // Ensure it's not faster than physics tick
		}
		a.bcastTicker = time.NewTicker(broadcastInterval)
		// Ensure stop channel is fresh if restarting
		select {
		case <-a.stopBcastCh: // If already closed, make a new one
			a.stopBcastCh = make(chan struct{})
		default:
		}
		stopCh := a.stopBcastCh     // Capture channel for goroutine
		tickerCh := a.bcastTicker.C // Capture ticker channel

		a.tickerMu.Unlock() // Unlock before starting goroutine

		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("PANIC recovered in GameActor %s Broadcast Ticker: %v\n", a.selfPID, r)
				}
				fmt.Printf("GameActor %s: Broadcast Ticker stopped.\n", a.selfPID)
			}()
			for {
				select {
				case <-stopCh: // Check stop channel first
					return
				case _, ok := <-tickerCh: // Read from ticker channel
					if !ok {
						return // Ticker channel closed
					}
					// Send tick message to self only if engine and PID are valid
					currentEngine := a.engine
					currentSelfPID := a.selfPID
					if currentEngine != nil && currentSelfPID != nil {
						currentEngine.Send(currentSelfPID, BroadcastTick{}, nil)
					} else {
						return // Engine or PID gone, stop goroutine
					}
				}
			}
		}()
	} else {
		a.tickerMu.Unlock() // Unlock if ticker already exists
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

// cleanupChildActorsAndConnections stops all managed actors. No lock needed.
func (a *GameActor) cleanupChildActorsAndConnections() {
	paddlesToStop := make([]*bollywood.PID, 0, utils.MaxPlayers)
	ballsToStop := make([]*bollywood.PID, 0, len(a.ballActors))
	broadcasterToStop := a.broadcasterPID
	a.broadcasterPID = nil // Clear immediately

	for i := 0; i < utils.MaxPlayers; i++ {
		if pid := a.paddleActors[i]; pid != nil {
			paddlesToStop = append(paddlesToStop, pid)
			a.paddleActors[i] = nil
		}
		if pInfo := a.players[i]; pInfo != nil {
			if pInfo.Ws != nil {
				delete(a.connToIndex, pInfo.Ws) // connToIndex still needs management
			}
			pInfo.Ws = nil
			pInfo.IsConnected = false
		}
		a.players[i] = nil
		a.playerConns[i] = nil
	}
	for ballID, pid := range a.ballActors {
		if pid != nil {
			ballsToStop = append(ballsToStop, pid)
		}
		delete(a.ballActors, ballID) // Modify map directly
		delete(a.balls, ballID)      // Modify map directly
	}
	if len(a.connToIndex) > 0 {
		a.connToIndex = make(map[*websocket.Conn]int) // Clear map
	}

	// Stop actors outside the loop
	currentEngine := a.engine
	if currentEngine != nil {
		if broadcasterToStop != nil {
			currentEngine.Stop(broadcasterToStop)
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
