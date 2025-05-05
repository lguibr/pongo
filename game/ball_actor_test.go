// File: game/ball_actor_test.go
package game

import (
	"math"
	"testing"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
)

// Using MockGameActor from paddle_actor_test.go

// Helper to find the last message of a specific type sent to the mock GameActor
func findLastSentBallUpdate(t *testing.T, mockGameActor *MockGameActor) (*BallStateUpdate, bool) {
	t.Helper()
	var lastMsg *BallStateUpdate
	found := false
	messages := mockGameActor.GetMessages() // Get a copy
	for _, msg := range messages {
		if typedMsg, ok := msg.(BallStateUpdate); ok {
			// Need to capture the value correctly in the loop
			msgCopy := typedMsg
			lastMsg = &msgCopy
			found = true
		}
	}
	return lastMsg, found
}

func TestBallActor_ReceivesCommandsAndSendsUpdate(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second)

	mockGameActor := &MockGameActor{}
	mockGameActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockGameActor }))
	cfg := utils.DefaultConfig()

	initialBallValue := NewBall(cfg, 100, 100, 0, 456, false)
	initialVx, initialVy := initialBallValue.Vx, initialBallValue.Vy
	initialMass, initialRadius := initialBallValue.Mass, initialBallValue.Radius

	ballProducer := NewBallActorProducer(*initialBallValue, mockGameActorPID, cfg)
	ballPID := engine.Spawn(bollywood.NewProps(ballProducer))
	time.Sleep(50 * time.Millisecond) // Allow actor to start

	// --- Test Sending Commands (Verify BallStateUpdate sent to GameActor) ---
	mockGameActor.ClearMessages() // Clear any startup messages

	// Send Velocity Increase Command
	velRatio := cfg.PowerUpIncreaseVelRatio
	engine.Send(ballPID, IncreaseVelocityCommand{Ratio: velRatio}, nil)
	time.Sleep(cfg.GameTickPeriod) // Allow command processing
	updateMsg, found := findLastSentBallUpdate(t, mockGameActor)
	assert.True(t, found, "GameActor should receive BallStateUpdate after IncreaseVelocityCommand")
	if found {
		expectedVx := int(math.Floor(float64(initialVx) * velRatio))
		expectedVy := int(math.Floor(float64(initialVy) * velRatio))
		if initialVx != 0 && expectedVx == 0 {
			expectedVx = int(math.Copysign(1.0, float64(initialVx)))
		}
		if initialVy != 0 && expectedVy == 0 {
			expectedVy = int(math.Copysign(1.0, float64(initialVy)))
		}
		assert.Equal(t, expectedVx, updateMsg.Vx, "Update Vx mismatch")
		assert.Equal(t, expectedVy, updateMsg.Vy, "Update Vy mismatch")
		assert.Equal(t, ballPID, updateMsg.PID, "Update PID mismatch")
		assert.Equal(t, initialBallValue.Id, updateMsg.ID, "Update ID mismatch")
	}
	mockGameActor.ClearMessages()
	// if found { // Update initial values only if the update was found
	// initialVx, initialVy = updateMsg.Vx, updateMsg.Vy // Update for next check if found // REMOVED ineffassign
	// }

	// Send Mass Increase Command
	massAdd := cfg.PowerUpIncreaseMassAdd
	engine.Send(ballPID, IncreaseMassCommand{Additional: massAdd}, nil)
	time.Sleep(cfg.GameTickPeriod) // Allow command processing
	updateMsg, found = findLastSentBallUpdate(t, mockGameActor)
	assert.True(t, found, "GameActor should receive BallStateUpdate after IncreaseMassCommand")
	if found {
		expectedMass := initialMass + massAdd
		expectedRadius := initialRadius + massAdd*cfg.PowerUpIncreaseMassSize
		assert.Equal(t, expectedMass, updateMsg.Mass, "Update Mass mismatch")
		assert.Equal(t, expectedRadius, updateMsg.Radius, "Update Radius mismatch")
	}
	mockGameActor.ClearMessages()
	// if found { // Update initial values only if the update was found
	// initialMass, initialRadius = updateMsg.Mass, updateMsg.Radius // Update for next check if found // REMOVED ineffassign
	// }

	// Send SetPhasing Command
	engine.Send(ballPID, SetPhasingCommand{}, nil)
	time.Sleep(cfg.GameTickPeriod) // Allow command processing
	updateMsg, found = findLastSentBallUpdate(t, mockGameActor)
	assert.True(t, found, "GameActor should receive BallStateUpdate after SetPhasingCommand")
	if found {
		assert.True(t, updateMsg.Phasing, "Update Phasing should be true")
	}
	mockGameActor.ClearMessages()

	// Send StopPhasing Command
	engine.Send(ballPID, StopPhasingCommand{}, nil)
	time.Sleep(cfg.GameTickPeriod) // Allow command processing
	updateMsg, found = findLastSentBallUpdate(t, mockGameActor)
	assert.True(t, found, "GameActor should receive BallStateUpdate after StopPhasingCommand")
	if found {
		assert.False(t, updateMsg.Phasing, "Update Phasing should be false after StopPhasingCommand")
	}
	mockGameActor.ClearMessages()

	// Send Reflect Velocity Command (Reflect X)
	vxBeforeReflect := 0 // Need to get the current Vx from the last update if possible
	// Re-query the state or use the last known value if reliable
	// For simplicity, let's assume we know the Vx before reflect based on previous steps
	// If the IncreaseVelocity step ran, Vx would be updated. If not, it's initialVx.
	// This highlights a potential fragility in the test. A better approach might involve Ask.
	// Let's assume the IncreaseVelocity step worked and use its expected value.
	expectedVxAfterIncrease := int(math.Floor(float64(initialBallValue.Vx) * velRatio))
	if initialBallValue.Vx != 0 && expectedVxAfterIncrease == 0 {
		expectedVxAfterIncrease = int(math.Copysign(1.0, float64(initialBallValue.Vx)))
	}
	vxBeforeReflect = expectedVxAfterIncrease

	engine.Send(ballPID, ReflectVelocityCommand{Axis: "X"}, nil)
	time.Sleep(cfg.GameTickPeriod) // Allow command processing
	updateMsg, found = findLastSentBallUpdate(t, mockGameActor)
	assert.True(t, found, "GameActor should receive BallStateUpdate after ReflectVelocityCommand")
	if found {
		expectedReflectedVx := -vxBeforeReflect
		if vxBeforeReflect != 0 && expectedReflectedVx == 0 {
			expectedReflectedVx = int(math.Copysign(1.0, float64(-vxBeforeReflect)))
		}
		assert.Equal(t, expectedReflectedVx, updateMsg.Vx, "Update Vx mismatch after reflect")
	}
	mockGameActor.ClearMessages()
	// if found { // Update initial values only if the update was found
	// initialVx = updateMsg.Vx // Update for next check if found // REMOVED ineffassign
	// }

	// Send Set Velocity Command
	newVx, newVy := 5, -5
	engine.Send(ballPID, SetVelocityCommand{Vx: newVx, Vy: newVy}, nil)
	time.Sleep(cfg.GameTickPeriod) // Allow command processing
	updateMsg, found = findLastSentBallUpdate(t, mockGameActor)
	assert.True(t, found, "GameActor should receive BallStateUpdate after SetVelocityCommand")
	if found {
		assert.Equal(t, newVx, updateMsg.Vx, "Update Vx mismatch after set")
		assert.Equal(t, newVy, updateMsg.Vy, "Update Vy mismatch after set")
	}
}