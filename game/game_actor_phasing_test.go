// File: game/game_actor_phasing_test.go
package game

import (
	"testing"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
)

// TestGameActor_PhasingBall_DamagesBrickOnceNoReflect verifies the core phasing logic.
func TestGameActor_PhasingBall_DamagesBrickOnceNoReflect(t *testing.T) {
	// 1. Setup Engine and Config
	engine := bollywood.NewEngine()
	defer engine.Shutdown(testShutdownTimeout) // Use existing constant
	cfg := utils.DefaultConfig()
	cfg.GameTickPeriod = 10 * time.Millisecond // Faster ticks
	cfg.BallPhasingTime = 200 * time.Millisecond // Ensure phasing is active
	cfg.PowerUpChance = 0.0                    // <<< DISABLE POWERUPS >>>

	gridSize := cfg.GridSize
	cellSize := cfg.CellSize
	brickCol := 9 // Target col 9
	brickRow := 9 // Target row 9
	initialLife := 3

	// 2. Spawn MockBroadcaster
	mockBroadcaster := &MockBroadcasterActor{}
	mockBroadcasterPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockBroadcaster }))
	assert.NotNil(t, mockBroadcasterPID)

	// 3. Create initial GameActor state instance
	gameActorInstance := &GameActor{
		canvas:     NewCanvas(cfg.CanvasSize, gridSize),
		players:    [utils.MaxPlayers]*playerInfo{}, // No players initially
		paddles:    [utils.MaxPlayers]*Paddle{},
		balls:      make(map[int]*Ball),
		ballActors: make(map[int]*bollywood.PID),
	}
	// Place the target brick
	gameActorInstance.canvas.Grid[brickRow][brickCol] = NewCell(brickCol, brickRow, initialLife, utils.Cells.Brick)
	// Fill grid (needed for GameActor initialization)
	gameActorInstance.canvas.Grid.FillSymmetrical(cfg)

	// Ball parameters - Positioned directly above the target brick, moving straight down
	ballX := brickCol*cellSize + cellSize/2 // Center X = 9*50 + 25 = 475
	ballY := (brickRow-1)*cellSize + cellSize/2 // Center Y of cell above = 8*50 + 25 = 425
	initialVx := 0
	initialVy := 11 // Move downwards (use default max vel)

	// 4. Create the custom producer
	testProducer := &TestGameActorProducer{
		engine:             engine,
		cfg:                cfg,
		roomManagerPID:     nil,
		mockBroadcasterPID: mockBroadcasterPID, // Pass mock broadcaster
		initialState:       gameActorInstance,
	}

	// 5. Spawn GameActor
	gameActorPID := engine.Spawn(bollywood.NewProps(testProducer.Produce))
	assert.NotNil(t, gameActorPID)
	assert.True(t, waitForGameActorReady(t, engine, gameActorPID, 500*time.Millisecond), "GameActor did not become ready")

	// 6. Add ONLY the test ball using internal message
	ballID := 12345
	ballData := NewBall(cfg, ballX, ballY, -1, ballID, true) // Owner -1 (ownerless)
	ballData.Vx = initialVx
	ballData.Vy = initialVy
	ballData.Phasing = true // <<< START PHASING >>>

	mockBallActor := &MockSimpleActor{}
	mockBallActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockBallActor }))

	engine.Send(gameActorPID, internalAddBallTestMsg{Ball: ballData, PID: mockBallActorPID}, nil)
	// Start the phasing timer within GameActor
	engine.Send(gameActorPID, SetPhasingCommand{}, mockBallActorPID) // Send command to trigger timer start
	// Start tickers manually
	engine.Send(gameActorPID, internalStartTickersTestMsg{}, nil)
	time.Sleep(50 * time.Millisecond) // Allow messages to process and tickers to start

	// 8. Run Game Ticks manually (fewer ticks to ensure phasing is active)
	numTicksToRun := 5 // Should be enough to hit the brick once while phasing
	for i := 0; i < numTicksToRun; i++ {
		engine.Send(gameActorPID, GameTick{}, nil)
		time.Sleep(cfg.GameTickPeriod + 2*time.Millisecond) // Wait slightly longer than tick period
	}

	// 9. Query Final State using Ask
	askTimeout := 500 * time.Millisecond
	ballReply, ballErr := engine.Ask(gameActorPID, internalGetBallRequest{BallID: ballID}, askTimeout)
	brickReply, brickErr := engine.Ask(gameActorPID, internalGetBrickRequest{Row: brickRow, Col: brickCol}, askTimeout)

	// 10. Assertions
	assert.NoError(t, ballErr, "Ask for ball state should not error")
	assert.NoError(t, brickErr, "Ask for brick state should not error")

	// Assert Ball State
	assert.NotNil(t, ballReply, "Ball reply should not be nil")
	if ballReply != nil {
		ballResponse, ok := ballReply.(internalGetBallResponse)
		assert.True(t, ok, "Ball reply should be internalGetBallResponse")
		if ok {
			assert.True(t, ballResponse.Exists, "Ball should still exist")
			assert.NotNil(t, ballResponse.Ball, "Ball data in response should not be nil")
			if ballResponse.Ball != nil {
				assert.Equal(t, initialVx, ballResponse.Ball.Vx, "Phasing ball Vx should not change (no reflection)")
				assert.Equal(t, initialVy, ballResponse.Ball.Vy, "Phasing ball Vy should not change (no reflection)")
				assert.True(t, ballResponse.Ball.Phasing, "Ball should still be phasing") // Check phasing state
			}
		}
	}

	// Assert Brick State
	assert.NotNil(t, brickReply, "Brick reply should not be nil")
	if brickReply != nil {
		brickResponse, ok := brickReply.(internalGetBrickResponse)
		assert.True(t, ok, "Brick reply should be internalGetBrickResponse")
		if ok {
			assert.True(t, brickResponse.Exists, "Brick cell should exist")
			expectedLife := initialLife - 1 // Expect life to be decremented by exactly 1
			assert.Equal(t, utils.Cells.Brick, brickResponse.Type, "Brick cell should still be a brick")
			assert.Equal(t, expectedLife, brickResponse.Life, "Brick life should be decremented by exactly 1 by phasing ball")
		}
	}
}

// TestGameActor_PhasingBall_ReflectsWall verifies phasing balls reflect off walls.
func TestGameActor_PhasingBall_ReflectsWall(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(testShutdownTimeout)
	cfg := utils.DefaultConfig()
	cfg.GameTickPeriod = 10 * time.Millisecond
	cfg.BallPhasingTime = 500 * time.Millisecond // Long phasing time

	mockBroadcaster := &MockBroadcasterActor{}
	mockBroadcasterPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockBroadcaster }))

	gameActorInstance := &GameActor{
		canvas:     NewCanvas(cfg.CanvasSize, cfg.GridSize),
		players:    [utils.MaxPlayers]*playerInfo{},
		paddles:    [utils.MaxPlayers]*Paddle{},
		balls:      make(map[int]*Ball),
		ballActors: make(map[int]*bollywood.PID),
	}
	gameActorInstance.canvas.Grid.FillSymmetrical(cfg) // Fill grid

	// Ball aimed at the right wall (wall index 0)
	ballX := cfg.CanvasSize - cfg.BallRadius*2 // Start close to right wall
	ballY := cfg.CanvasSize / 2
	initialVx := 10 // Moving right
	initialVy := 0  // Ensure Vy starts at 0

	testProducer := &TestGameActorProducer{
		engine: engine, cfg: cfg, mockBroadcasterPID: mockBroadcasterPID, initialState: gameActorInstance,
	}
	gameActorPID := engine.Spawn(bollywood.NewProps(testProducer.Produce))
	assert.True(t, waitForGameActorReady(t, engine, gameActorPID, 500*time.Millisecond))

	// Add ONLY the test ball using internal message
	ballID := 54321
	ballData := NewBall(cfg, ballX, ballY, -1, ballID, true) // Ownerless
	ballData.Vx = initialVx
	ballData.Vy = initialVy // Explicitly set Vy to 0
	ballData.Phasing = true // <<< START PHASING >>>

	mockBallActor := &MockSimpleActor{}
	mockBallActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockBallActor }))
	engine.Send(gameActorPID, internalAddBallTestMsg{Ball: ballData, PID: mockBallActorPID}, nil)
	engine.Send(gameActorPID, SetPhasingCommand{}, mockBallActorPID) // Start timer
	// Start tickers manually
	engine.Send(gameActorPID, internalStartTickersTestMsg{}, nil)
	time.Sleep(50 * time.Millisecond)

	// Run ticks until collision and reflection
	reflected := false
	for i := 0; i < 20; i++ { // Limit ticks
		engine.Send(gameActorPID, GameTick{}, nil)
		time.Sleep(cfg.GameTickPeriod + 2*time.Millisecond)

		// Query ball state
		reply, err := engine.Ask(gameActorPID, internalGetBallRequest{BallID: ballID}, 100*time.Millisecond)
		if err == nil && reply != nil {
			if resp, ok := reply.(internalGetBallResponse); ok && resp.Exists && resp.Ball != nil {
				// Check if reflected (Vx should be negative)
				if resp.Ball.Vx < 0 {
					reflected = true
					assert.Equal(t, -initialVx, resp.Ball.Vx, "Vx should be reflected")
					assert.Equal(t, initialVy, resp.Ball.Vy, "Vy should remain unchanged") // Vy was 0
					assert.True(t, resp.Ball.Phasing, "Ball should still be phasing after wall reflection")
					break
				}
			}
		}
	}

	assert.True(t, reflected, "Phasing ball did not reflect off the wall")
	// We don't explicitly check score here, but the handleWallCollision logic skips it for phasing balls.
}

// TestGameActor_PhasingBall_ReflectsPaddle verifies phasing balls reflect off paddles and change owner.
func TestGameActor_PhasingBall_ReflectsPaddle(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(testShutdownTimeout)
	cfg := utils.DefaultConfig()
	cfg.GameTickPeriod = 10 * time.Millisecond
	cfg.BallPhasingTime = 500 * time.Millisecond // Long phasing time

	mockBroadcaster := &MockBroadcasterActor{}
	mockBroadcasterPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockBroadcaster }))

	gameActorInstance := &GameActor{
		canvas:     NewCanvas(cfg.CanvasSize, cfg.GridSize),
		players:    [utils.MaxPlayers]*playerInfo{},
		paddles:    [utils.MaxPlayers]*Paddle{}, // Initialize paddles map
		balls:      make(map[int]*Ball),
		ballActors: make(map[int]*bollywood.PID),
	}
	gameActorInstance.canvas.Grid.FillSymmetrical(cfg) // Fill grid

	// Paddle 0 (Right wall)
	paddle0 := NewPaddle(cfg, 0)
	paddle0.Y = cfg.CanvasSize/2 - paddle0.Height/2 // Center it vertically
	// Set paddle state in the initial instance BEFORE spawning
	gameActorInstance.paddles[0] = paddle0

	// Ball aimed at Paddle 0
	ballX := paddle0.X - cfg.BallRadius*2 // Start left of paddle
	ballY := paddle0.Y + paddle0.Height/2 // Aim for center Y
	initialVx := 10                       // Moving right
	initialVy := 0
	initialOwner := 1 // Start owned by player 1

	testProducer := &TestGameActorProducer{
		engine: engine, cfg: cfg, mockBroadcasterPID: mockBroadcasterPID, initialState: gameActorInstance,
	}
	gameActorPID := engine.Spawn(bollywood.NewProps(testProducer.Produce))
	assert.True(t, waitForGameActorReady(t, engine, gameActorPID, 500*time.Millisecond))

	// Add players 0 and 1 (this will also spawn their default paddles/balls, but we set paddle 0 above)
	// Crucially, this happens AFTER the actor is spawned, using messages.
	engine.Send(gameActorPID, internalTestingAddPlayerAndStart{PlayerIndex: 0}, nil)
	engine.Send(gameActorPID, internalTestingAddPlayerAndStart{PlayerIndex: 1}, nil)
	time.Sleep(100 * time.Millisecond) // Allow setup

	ballID := 65432
	ballData := NewBall(cfg, ballX, ballY, initialOwner, ballID, true)
	ballData.Vx = initialVx
	ballData.Vy = initialVy
	ballData.Phasing = true // <<< START PHASING >>>

	mockBallActor := &MockSimpleActor{}
	mockBallActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockBallActor }))
	engine.Send(gameActorPID, internalAddBallTestMsg{Ball: ballData, PID: mockBallActorPID}, nil)
	engine.Send(gameActorPID, SetPhasingCommand{}, mockBallActorPID) // Start timer
	time.Sleep(50 * time.Millisecond)

	// Run ticks until collision and reflection/owner change
	reflected := false
	ownerChanged := false
	for i := 0; i < 20; i++ { // Limit ticks
		engine.Send(gameActorPID, GameTick{}, nil)
		time.Sleep(cfg.GameTickPeriod + 2*time.Millisecond)

		// Query ball state
		reply, err := engine.Ask(gameActorPID, internalGetBallRequest{BallID: ballID}, 100*time.Millisecond)
		if err == nil && reply != nil {
			if resp, ok := reply.(internalGetBallResponse); ok && resp.Exists && resp.Ball != nil {
				// Check if reflected (Vx should be negative)
				if resp.Ball.Vx < 0 {
					reflected = true
				}
				// Check if owner changed to paddle owner (index 0)
				if resp.Ball.OwnerIndex == 0 {
					ownerChanged = true
				}
				if reflected && ownerChanged {
					assert.True(t, resp.Ball.Phasing, "Ball should still be phasing after paddle reflection")
					break
				}
			}
		}
	}

	assert.True(t, reflected, "Phasing ball did not reflect off the paddle")
	assert.True(t, ownerChanged, "Phasing ball owner did not change after hitting paddle")
}

// MockSimpleActor is used when the actor's response isn't critical for the test.
type MockSimpleActor struct{}

func (a *MockSimpleActor) Receive(ctx bollywood.Context) {
	// Does nothing, just acknowledges messages
}