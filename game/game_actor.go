// File: game/game_actor.go
package game

import (
	"fmt"
	"reflect" // Import reflect for logging message type
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"golang.org/x/net/websocket" // Import websocket
)

// MaxPlayers constant moved to utils/constants.go

// GameActor manages the overall game state and coordinates child actors.
type GameActor struct {
	cfg           utils.Config // Add config field
	canvas        *Canvas
	players       [utils.MaxPlayers]*playerInfo    // Use constant from utils
	paddles       [utils.MaxPlayers]*Paddle        // Use constant from utils
	paddleActors  [utils.MaxPlayers]*bollywood.PID // Use constant from utils
	balls         map[int]*Ball                    // Live state of balls (updated by messages) - Keyed by Ball ID
	ballActors    map[int]*bollywood.PID
	engine        *bollywood.Engine // Reference to the engine
	ticker        *time.Ticker
	stopTickerCh  chan struct{}
	gameStateJSON atomic.Value   // Stores marshalled JSON for HTTP endpoint
	selfPID       *bollywood.PID // Store self PID for internal use
	mu            sync.RWMutex   // Protects shared maps/slices

	connToIndex map[*websocket.Conn]int // Use concrete type
}

// playerInfo holds state associated with a connected player/websocket.
type playerInfo struct {
	Index       int
	ID          string
	Score       int
	Color       [3]int
	Ws          *websocket.Conn // Use concrete type
	IsConnected bool            // Tracks if the connection is considered active by GameActor
}

// NewGameActorProducer creates a producer for the GameActor.
func NewGameActorProducer(engine *bollywood.Engine, cfg utils.Config) bollywood.Producer { // Accept config
	return func() bollywood.Actor {
		// Use config for canvas creation
		canvas := NewCanvas(cfg.CanvasSize, cfg.GridSize)
		// Use config for grid fill parameters
		canvas.Grid.Fill(cfg.GridFillVectors, cfg.GridFillVectorSize, cfg.GridFillWalkers, cfg.GridFillSteps)

		ga := &GameActor{
			cfg:          cfg, // Store config
			canvas:       canvas,
			players:      [utils.MaxPlayers]*playerInfo{},    // Use constant from utils
			paddles:      [utils.MaxPlayers]*Paddle{},        // Use constant from utils
			paddleActors: [utils.MaxPlayers]*bollywood.PID{}, // Use constant from utils
			balls:        make(map[int]*Ball),
			ballActors:   make(map[int]*bollywood.PID),
			engine:       engine,
			stopTickerCh: make(chan struct{}),
			connToIndex:  make(map[*websocket.Conn]int), // Use concrete type
		}
		ga.updateGameStateJSON() // Initialize JSON state
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
		}
	}()

	actorPIDStr := "nil"
	if a.selfPID == nil {
		a.selfPID = ctx.Self()
		if a.selfPID != nil {
			actorPIDStr = a.selfPID.String()
		} else {
			fmt.Println("GameActor ???: Failed to set self PID on first Receive.")
			return
		}
	} else {
		actorPIDStr = a.selfPID.String()
	}

	msg := ctx.Message()
	msgType := reflect.TypeOf(msg)
	if msgType.String() != "*game.GameTick" && msgType.String() != "game.BallPositionMessage" && msgType.String() != "game.PaddlePositionMessage" {
		// fmt.Printf("GameActor %s: Receive entered. Message Type: %s\n", actorPIDStr, msgType) // Reduce noise
	}

	switch m := msg.(type) {
	case bollywood.Started:
		fmt.Printf("GameActor %s: Processing Started message.\n", actorPIDStr)
		if a.selfPID == nil {
			a.selfPID = ctx.Self()
			if a.selfPID != nil {
				actorPIDStr = a.selfPID.String()
				fmt.Printf("GameActor %s: Self PID set during Started.\n", actorPIDStr)
			} else {
				fmt.Printf("GameActor ???: Failed to set self PID during Started.\n")
				return
			}
		}
		a.ticker = time.NewTicker(a.cfg.GameTickPeriod) // Use config for period
		go a.runTickerLoop()

	case *GameTick:
		a.mu.Lock()
		a.detectCollisions(ctx) // Pass context
		a.mu.Unlock()
		a.broadcastGameState()
		a.updateGameStateJSON()

	case PlayerConnectRequest:
		fmt.Printf("GameActor %s: Processing PlayerConnectRequest.\n", actorPIDStr)
		a.handlePlayerConnect(ctx, m.WsConn)

	case PlayerDisconnect:
		fmt.Printf("GameActor %s: Processing PlayerDisconnect.\n", actorPIDStr)
		a.handlePlayerDisconnect(ctx, m.PlayerIndex, m.WsConn)

	case ForwardedPaddleDirection:
		a.handlePaddleDirection(ctx, m.WsConn, m.Direction)

	case PaddlePositionMessage:
		a.handlePaddlePositionUpdate(ctx, m.Paddle)

	case BallPositionMessage:
		a.handleBallPositionUpdate(ctx, m.Ball)

	case SpawnBallCommand:
		fmt.Printf("GameActor %s: Processing SpawnBallCommand.\n", actorPIDStr)
		// Pass config to spawnBall
		a.spawnBall(ctx, m.OwnerIndex, m.X, m.Y, m.ExpireIn, m.IsPermanent)

	case DestroyExpiredBall:
		fmt.Printf("GameActor %s: Processing DestroyExpiredBall.\n", actorPIDStr)
		a.handleDestroyExpiredBall(ctx, m.BallID)

	case bollywood.Stopping:
		fmt.Printf("GameActor %s: Processing Stopping message.\n", actorPIDStr)
		if a.ticker != nil {
			a.ticker.Stop()
			select {
			case <-a.stopTickerCh:
			default:
				close(a.stopTickerCh)
			}
		}
		a.cleanupChildActorsAndConnections()

	case bollywood.Stopped:
		fmt.Printf("GameActor %s: Processing Stopped message.\n", actorPIDStr)

	default:
		fmt.Printf("GameActor %s: Processing unknown message type: %T\n", actorPIDStr, m)
	}
}

// runTickerLoop sends GameTick messages to the actor's own mailbox at regular intervals.
func (a *GameActor) runTickerLoop() {
	defer func() {
		if r := recover(); r != nil {
			pidStr := "unknown"
			if a.selfPID != nil {
				pidStr = a.selfPID.String()
			}
			fmt.Printf("PANIC recovered in GameActor %s Ticker Loop: %v\nStack trace:\n%s\n", pidStr, r, string(debug.Stack()))
			select {
			case <-a.stopTickerCh:
			default:
				close(a.stopTickerCh)
			}
		}
	}()

	time.Sleep(15 * time.Millisecond)
	actorPID := a.selfPID
	if actorPID == nil {
		fmt.Println("ERROR: GameActor ticker loop cannot start, self PID not set after wait.")
		return
	}
	actorPIDStr := actorPID.String()
	fmt.Printf("GameActor %s: Ticker loop started.\n", actorPIDStr)
	defer fmt.Printf("GameActor %s: Ticker loop stopped.\n", actorPIDStr)

	tickCount := 0
	tickMsg := &GameTick{}

	for {
		select {
		case <-a.stopTickerCh:
			fmt.Printf("GameActor %s: Ticker loop detected stop signal. Exiting.\n", actorPIDStr)
			return
		case _, ok := <-a.ticker.C:
			if !ok {
				fmt.Printf("GameActor %s: Ticker channel closed. Exiting loop.\n", actorPIDStr)
				return
			}
			tickCount++
			select {
			case <-a.stopTickerCh:
				fmt.Printf("GameActor %s: Ticker loop detected stop signal after tick %d. Exiting.\n", actorPIDStr, tickCount)
				return
			default:
				a.engine.Send(actorPID, tickMsg, nil)
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
	a.mu.Lock()
	fmt.Printf("GameActor %s: Lock acquired for cleanup.\n", pidStr)
	paddlesToStop := make([]*bollywood.PID, 0, utils.MaxPlayers) // Use constant from utils
	ballsToStop := make([]*bollywood.PID, 0, len(a.ballActors))
	connectionsToClose := []*websocket.Conn{} // Use concrete type

	for i := 0; i < utils.MaxPlayers; i++ { // Use constant from utils
		if pid := a.paddleActors[i]; pid != nil {
			paddlesToStop = append(paddlesToStop, pid)
			a.paddleActors[i] = nil
		}
		if pInfo := a.players[i]; pInfo != nil && pInfo.Ws != nil {
			if _, exists := a.connToIndex[pInfo.Ws]; exists {
				connectionsToClose = append(connectionsToClose, pInfo.Ws)
				delete(a.connToIndex, pInfo.Ws)
			} else {
				fmt.Printf("GameActor %s Cleanup WARN: Connection for player %d not found in connToIndex.\n", pidStr, i)
			}
			pInfo.Ws = nil
			pInfo.IsConnected = false
		}
		a.players[i] = nil
	}

	for ballID, pid := range a.ballActors {
		if pid != nil {
			ballsToStop = append(ballsToStop, pid)
		}
		delete(a.ballActors, ballID)
		delete(a.balls, ballID)
	}

	if len(a.connToIndex) > 0 {
		fmt.Printf("GameActor %s Cleanup WARN: connToIndex not empty after player cleanup (%d entries remain).\n", pidStr, len(a.connToIndex))
		a.connToIndex = make(map[*websocket.Conn]int) // Use concrete type
	}

	a.mu.Unlock()
	fmt.Printf("GameActor %s: Lock released for cleanup.\n", pidStr)

	fmt.Printf("GameActor %s Cleanup: Stopping %d paddles, %d balls. Closing %d connections.\n",
		pidStr, len(paddlesToStop), len(ballsToStop), len(connectionsToClose))

	for _, pid := range paddlesToStop {
		if pid != nil {
			a.engine.Stop(pid)
		}
	}
	for _, pid := range ballsToStop {
		if pid != nil {
			a.engine.Stop(pid)
		}
	}

	for _, ws := range connectionsToClose {
		if ws != nil {
			_ = ws.Close()
		}
	}
	fmt.Printf("GameActor %s: Cleanup actions finished.\n", pidStr)
}

// GetGameStateJSON retrieves the latest marshalled game state for HTTP handlers.
func (a *GameActor) GetGameStateJSON() []byte {
	val := a.gameStateJSON.Load()
	if val == nil {
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
