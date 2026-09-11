// File: game/game_actor_test.go

package game

import (
	"sync"
	"testing"
	"time"

	"github.com/lguibr/pongo/internal/actor"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert" // Use testify for assertions
	"golang.org/x/net/websocket"         // Import websocket
)

// --- Mock Broadcaster Actor ---
// Simple actor to capture messages sent to it
type MockBroadcasterActor struct {
	mu       sync.Mutex
	Received []interface{}
	PID      *actor.PID
}

func (a *MockBroadcasterActor) Receive(ctx actor.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.PID == nil {
		a.PID = ctx.Self()
	}
	a.Received = append(a.Received, ctx.Message())
}

func (a *MockBroadcasterActor) GetMessages() []interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	// Return a copy
	msgs := make([]interface{}, len(a.Received))
	copy(msgs, a.Received)
	return msgs
}

func (a *MockBroadcasterActor) ClearMessages() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.Received = nil
}

// --- Tests ---

// Increased shutdown timeout for all tests
const testShutdownTimeout = 8 * time.Second // Increased shutdown timeout

// --- TestGameActorProducer ---
// TestGameActorProducer allows injecting dependencies for testing.
type TestGameActorProducer struct {
	engine             *actor.Engine
	cfg                utils.Config
	roomManagerPID     *actor.PID
	mockBroadcasterPID *actor.PID // Inject mock broadcaster PID
	initialState       *GameActor // Inject initial state
}

func (p *TestGameActorProducer) Produce() actor.Actor {
	// Use the injected initial state
	ga := p.initialState
	// Set dependencies BEFORE the actor starts receiving messages
	ga.engine = p.engine
	ga.cfg = p.cfg
	ga.roomManagerPID = p.roomManagerPID
	ga.broadcasterPID = p.mockBroadcasterPID // Set the mock broadcaster PID here
	ga.stopPhysicsCh = make(chan struct{})   // Initialize channels
	ga.stopBroadcastCh = make(chan struct{})
	ga.pendingUpdates = make([]interface{}, 0, 128)
	ga.activeCollisions = NewCollisionTracker()  // Initialize collision tracker
	ga.phasingTimers = make(map[int]*time.Timer) // Initialize phasing timers map
	// The GameActor's Started handler will now skip spawning its own broadcaster
	return ga
}

// Helper to wait for GameActor to be ready (e.g., after Started message)
func waitForGameActorReady(t *testing.T, engine *actor.Engine, pid *actor.PID, timeout time.Duration) bool {
	t.Helper()
	// Simple approach: wait a fixed short duration.
	// A more robust way would involve an Ask/Reply or a dedicated Ready message.
	// For now, a short sleep is often sufficient in tests.
	time.Sleep(100 * time.Millisecond) // Give actor time to process Started
	// We could add an Ask here if needed:
	// _, err := engine.Ask(pid, IsReadyCheck{}, timeout) // Assuming IsReadyCheck message exists
	// return err == nil
	return true // Assume ready after sleep for now
}

// --- REFACTORED TEST CASE ---
func TestGameActor_BrickCollisionAndGridUpdate(t *testing.T) {
	// 1. Setup Engine and Config
	engine := actor.NewEngine()
	defer engine.Shutdown(testShutdownTimeout)
	cfg := utils.DefaultConfig()
	cfg.GameTickPeriod = 10 * time.Millisecond // Faster ticks for testing
	cfg.BroadcastRateHz = 100                  // Faster broadcast for testing
	cfg.BallPhasingTime = 2 * time.Second      // Longer phasing to avoid accidental re-hits in test
	gridSize := cfg.GridSize
	targetBrickRow, targetBrickCol := 9, 9 // Target brick [9,9]
	initialLife := 2                       // Start with 2 life

	// 2. Spawn MockBroadcaster FIRST
	mockBroadcaster := &MockBroadcasterActor{}
	mockBroadcasterPID := engine.Spawn(actor.NewProps(func() actor.Actor { return mockBroadcaster }))
	assert.NotNil(t, mockBroadcasterPID)

	// 3. Create the initial GameActor state instance
	gameActorInstance := &GameActor{
		// Set initial state fields directly
		canvas:      NewCanvas(cfg.CanvasSize, gridSize),
		players:     [utils.MaxPlayers]*playerInfo{},
		paddles:     [utils.MaxPlayers]*Paddle{},
		balls:       make(map[int]*Ball),
		connToIndex: make(map[*websocket.Conn]int),
		playerConns: [utils.MaxPlayers]*websocket.Conn{},
		// Metrics, etc. will be initialized by producer
	}
	// --- Pre-Spawn State Setup (apply to gameActorInstance) ---
	// Place the target brick
	gameActorInstance.canvas.Grid[targetBrickRow][targetBrickCol] = NewCell(targetBrickCol, targetBrickRow, initialLife, utils.Cells.Brick)
	// Ball parameters - Positioned directly above the target brick, moving down
	ballX := targetBrickCol*cfg.CellSize + cfg.CellSize/2     // Center X = 9*50 + 25 = 475
	ballY := (targetBrickRow-1)*cfg.CellSize + cfg.CellSize/2 // Center Y of cell above = 8*50 + 25 = 425
	ballVx := 0
	ballVy := 8 // Move straight down
	// --- End Pre-Spawn State Setup ---

	// 4. Create the custom producer
	testProducer := &TestGameActorProducer{
		engine:             engine,
		cfg:                cfg,
		roomManagerPID:     nil, // Not needed for this test
		mockBroadcasterPID: mockBroadcasterPID,
		initialState:       gameActorInstance,
	}

	// 5. Spawn GameActor using the custom producer
	gameActorPID := engine.Spawn(actor.NewProps(testProducer.Produce)) // Call Produce()
	assert.NotNil(t, gameActorPID)
	// Wait for GameActor to be ready (process Started message)
	assert.True(t, waitForGameActorReady(t, engine, gameActorPID, 500*time.Millisecond), "GameActor did not become ready")

	// 6. Add ONLY the test ball using internal message (NO player/default balls)
	ballID := 99999                                          // Use a fixed ID for the test ball
	testBall := NewBall(cfg, ballX, ballY, -1, ballID, true) // Owner -1 (ownerless)
	testBall.Vx = ballVx
	testBall.Vy = ballVy
	testBall.Phasing = false // Start non-phasing
	engine.Send(gameActorPID, internalAddBallTestMsg{Ball: testBall}, nil)
	// Start tickers manually for this isolated test
	engine.Send(gameActorPID, internalStartTickersTestMsg{}, nil)
	time.Sleep(50 * time.Millisecond) // Allow message processing and ticker start

	// 8. Run Game Ticks manually to ensure collision happens
	numTicksToRun := 15 // Should be enough for the ball to travel one cell down and hit
	for i := 0; i < numTicksToRun; i++ {
		engine.Send(gameActorPID, GameTick{}, nil)
		time.Sleep(cfg.GameTickPeriod + 2*time.Millisecond) // Wait slightly longer than tick period
	}

	// 9. Query the final brick state using Ask
	askTimeout := 500 * time.Millisecond
	reply, err := engine.Ask(gameActorPID, internalGetBrickRequest{Row: targetBrickRow, Col: targetBrickCol}, askTimeout)

	// 10. Assertions
	assert.NoError(t, err, "Ask for brick state should not error")
	assert.NotNil(t, reply, "Reply from Ask should not be nil")

	if reply != nil {
		brickResponse, ok := reply.(internalGetBrickResponse)
		assert.True(t, ok, "Reply should be of type internalGetBrickResponse")
		if ok {
			assert.True(t, brickResponse.Exists, "Target brick cell should exist")
			// The brick should have been hit once
			expectedLife := initialLife - 1
			finalLife := brickResponse.Life
			t.Logf("Final Queried State Check: Target Brick [%d,%d] Life=%d, Type=%v", targetBrickRow, targetBrickCol, finalLife, brickResponse.Type)
			assert.Equal(t, expectedLife, finalLife, "Queried brick life mismatch - Expected life %d, Got %d", expectedLife, finalLife)

			// Check if the brick type is correct based on expected life
			if expectedLife <= 0 {
				assert.Equal(t, utils.Cells.Empty, brickResponse.Type, "Brick should be Empty after life reached 0")
			} else {
				assert.Equal(t, utils.Cells.Brick, brickResponse.Type, "Brick should still be of Type Brick")
			}
		}
	}
}
