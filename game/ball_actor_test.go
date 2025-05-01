// File: game/ball_actor_test.go
package game

import (
	// Import errors
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
)

// Using MockGameActor from paddle_actor_test.go

// Helper function to Ask for position
func askBallPosition(t *testing.T, engine *bollywood.Engine, pid *bollywood.PID) (*PositionResponse, error) {
	t.Helper()
	// Increased timeout for Ask in tests
	reply, err := engine.Ask(pid, GetPositionRequest{}, 500*time.Millisecond) // Increased timeout
	if err != nil {
		return nil, err
	}
	if posResp, ok := reply.(PositionResponse); ok {
		return &posResp, nil
	}
	return nil, fmt.Errorf("unexpected reply type: %T", reply)
}

func TestBallActor_SpawnsAndMoves(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second)

	mockGameActor := &MockGameActor{} // Not strictly needed, but keeps setup consistent
	mockGameActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockGameActor }))
	cfg := utils.DefaultConfig()
	time.Sleep(cfg.GameTickPeriod * 4)

	initialBall := NewBall(cfg, 100, 100, 0, 123, false)
	initialX, initialY := initialBall.X, initialBall.Y

	ballProducer := NewBallActorProducer(*initialBall, mockGameActorPID, cfg)
	ballPID := engine.Spawn(bollywood.NewProps(ballProducer))
	assert.NotNil(t, ballPID)
	// Add a slightly longer delay after spawn before first Ask
	time.Sleep(cfg.GameTickPeriod * 4)

	// Ask for initial position
	pos1, err1 := askBallPosition(t, engine, ballPID)
	assert.NoError(t, err1)
	if err1 != nil { // Fail fast if Ask failed
		t.FailNow()
	}
	assert.NotNil(t, pos1)
	assert.Equal(t, initialX, pos1.X, "Initial X should match")
	assert.Equal(t, initialY, pos1.Y, "Initial Y should match")

	// Send UpdatePosition command
	engine.Send(ballPID, UpdatePositionCommand{}, nil)
	time.Sleep(cfg.GameTickPeriod * 2) // Allow processing

	// Ask for position again
	pos2, err2 := askBallPosition(t, engine, ballPID)
	assert.NoError(t, err2)
	if err2 != nil { // Fail fast if Ask failed
		t.FailNow()
	}
	assert.NotNil(t, pos2)

	// Verify movement
	assert.NotEqual(t, pos1.X, pos2.X, "Ball X should change after UpdatePositionCommand")
	assert.NotEqual(t, pos1.Y, pos2.Y, "Ball Y should change after UpdatePositionCommand")
	fmt.Printf("Ball moved from (%d,%d) to (%d,%d)\n", pos1.X, pos1.Y, pos2.X, pos2.Y)
}

func TestBallActor_ReceivesCommands(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second)

	mockGameActor := &MockGameActor{}
	mockGameActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockGameActor }))
	cfg := utils.DefaultConfig()
	time.Sleep(cfg.GameTickPeriod * 2)

	initialBall := NewBall(cfg, 100, 100, 0, 456, false)
	initialVx, initialVy := initialBall.Vx, initialBall.Vy
	// Corrected: Remove initialMass from declaration
	initialRadius := initialBall.Radius

	ballProducer := NewBallActorProducer(*initialBall, mockGameActorPID, cfg)
	ballPID := engine.Spawn(bollywood.NewProps(ballProducer))
	time.Sleep(cfg.GameTickPeriod * 2)

	// --- Test Velocity Increase ---
	velRatio := cfg.PowerUpIncreaseVelRatio
	engine.Send(ballPID, IncreaseVelocityCommand{Ratio: velRatio}, nil)
	time.Sleep(cfg.GameTickPeriod) // Allow command processing

	posVel, errVel := askBallPosition(t, engine, ballPID)
	assert.NoError(t, errVel)
	assert.NotNil(t, posVel)

	expectedVx := int(math.Floor(float64(initialVx) * velRatio))
	expectedVy := int(math.Floor(float64(initialVy) * velRatio))
	if initialVx != 0 && expectedVx == 0 {
		expectedVx = int(math.Copysign(1, float64(initialVx)))
	}
	if initialVy != 0 && expectedVy == 0 {
		expectedVy = int(math.Copysign(1, float64(initialVy)))
	}
	assert.Equal(t, expectedVx, posVel.Vx, "Vx should be increased")
	assert.Equal(t, expectedVy, posVel.Vy, "Vy should be increased")
	initialVx, initialVy = posVel.Vx, posVel.Vy // Update for next check

	// --- Test Mass Increase ---
	massAdd := cfg.PowerUpIncreaseMassAdd
	engine.Send(ballPID, IncreaseMassCommand{Additional: massAdd}, nil)
	time.Sleep(cfg.GameTickPeriod) // Allow command processing

	posMass, errMass := askBallPosition(t, engine, ballPID)
	assert.NoError(t, errMass)
	assert.NotNil(t, posMass)

	// expectedMass := initialMass + massAdd // Cannot check Mass via Ask currently
	expectedRadius := initialRadius + massAdd*cfg.PowerUpIncreaseMassSize
	assert.Equal(t, expectedRadius, posMass.Radius, "Radius should be increased")
	initialRadius = posMass.Radius // Update for next check

	// --- Test Phasing ---
	phasingDuration := cfg.BallPhasingTime
	engine.Send(ballPID, SetPhasingCommand{}, nil)
	time.Sleep(cfg.GameTickPeriod) // Allow command processing

	posPhase1, errPhase1 := askBallPosition(t, engine, ballPID)
	assert.NoError(t, errPhase1)
	assert.NotNil(t, posPhase1)
	assert.True(t, posPhase1.Phasing, "Ball should be phasing after SetPhasingCommand")

	// Wait for phasing to expire
	time.Sleep(phasingDuration + cfg.GameTickPeriod*2)

	posPhase2, errPhase2 := askBallPosition(t, engine, ballPID)
	assert.NoError(t, errPhase2)
	assert.NotNil(t, posPhase2)
	assert.False(t, posPhase2.Phasing, "Ball should not be phasing after timer expires")

	// --- Test Reflect Velocity ---
	engine.Send(ballPID, ReflectVelocityCommand{Axis: "X"}, nil)
	time.Sleep(cfg.GameTickPeriod) // Allow command processing

	posReflect, errReflect := askBallPosition(t, engine, ballPID)
	assert.NoError(t, errReflect)
	assert.NotNil(t, posReflect)

	assert.Equal(t, -initialVx, posReflect.Vx, "Vx should be reflected")
	assert.Equal(t, initialVy, posReflect.Vy, "Vy should be unchanged")
}
