// File: game/ball_actor_test.go
package game

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/lguibr/pongo/bollywood"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
)

// Using MockGameActor from paddle_actor_test.go

func TestBallActor_SpawnsAndSendsPosition(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second)

	mockGameActor := &MockGameActor{}
	mockGameActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockGameActor }))
	time.Sleep(50 * time.Millisecond) // Wait for receiver

	initialBall := NewBall(100, 100, 10, utils.CanvasSize, 0, 123)
	initialX, initialY := initialBall.X, initialBall.Y

	ballProducer := NewBallActorProducer(*initialBall, mockGameActorPID) // Pass mock PID
	ballPID := engine.Spawn(bollywood.NewProps(ballProducer))
	assert.NotNil(t, ballPID)

	time.Sleep(utils.Period * 5) // Wait longer for actor to start and send initial position + tick

	// Check if initial position was sent
	received := mockGameActor.GetMessages()
	initialPosFound := false
	var lastBallState *Ball
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok {
			// Make a copy before assigning to lastBallState
			ballCopy := *posMsg.Ball
			lastBallState = &ballCopy
			if posMsg.Ball.Id == initialBall.Id && posMsg.Ball.X == initialX && posMsg.Ball.Y == initialY {
				initialPosFound = true
				// Don't break, let it process ticks too
			}
		}
	}
	assert.True(t, initialPosFound, "Should have received initial position message")
	assert.NotNil(t, lastBallState, "Should have received at least one position message")

	// Check if movement occurred after ticks
	assert.NotEqual(t, initialX, lastBallState.X, "Ball X should change after ticks")
	assert.NotEqual(t, initialY, lastBallState.Y, "Ball Y should change after ticks")
	fmt.Printf("Ball moved to X=%d, Y=%d\n", lastBallState.X, lastBallState.Y)
}

func TestBallActor_ReceivesCommands(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second)

	mockGameActor := &MockGameActor{}
	mockGameActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockGameActor }))
	time.Sleep(50 * time.Millisecond) // Wait longer

	initialBall := NewBall(100, 100, 10, utils.CanvasSize, 0, 456)
	initialVx, initialVy := initialBall.Vx, initialBall.Vy
	initialMass, initialRadius := initialBall.Mass, initialBall.Radius

	ballProducer := NewBallActorProducer(*initialBall, mockGameActorPID)
	ballPID := engine.Spawn(bollywood.NewProps(ballProducer))
	time.Sleep(utils.Period * 2) // Wait longer for start

	// --- Test Velocity Increase ---
	velRatio := 1.5
	engine.Send(ballPID, IncreaseVelocityCommand{Ratio: velRatio}, nil)
	time.Sleep(utils.Period) // Wait for processing

	// Force tick to get updated state sent to mock GameActor
	mockGameActor.ClearMessages() // Use safe method
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(utils.Period * 2) // Wait longer

	received := mockGameActor.GetMessages()
	velUpdated := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok && posMsg.Ball.Id == initialBall.Id {
			expectedVx := int(math.Floor(float64(initialVx) * velRatio))
			expectedVy := int(math.Floor(float64(initialVy) * velRatio))
			// Handle potential zeroing due to floor
			if initialVx != 0 && expectedVx == 0 {
				expectedVx = int(math.Copysign(1, float64(initialVx)))
			}
			if initialVy != 0 && expectedVy == 0 {
				expectedVy = int(math.Copysign(1, float64(initialVy)))
			}

			assert.Equal(t, expectedVx, posMsg.Ball.Vx, "Vx should be increased")
			assert.Equal(t, expectedVy, posMsg.Ball.Vy, "Vy should be increased")
			velUpdated = true
			initialVx, initialVy = posMsg.Ball.Vx, posMsg.Ball.Vy // Update for next test
			break
		}
	}
	assert.True(t, velUpdated, "Position message with updated velocity not received")

	// --- Test Mass Increase ---
	massAdd := 5
	engine.Send(ballPID, IncreaseMassCommand{Additional: massAdd}, nil)
	time.Sleep(utils.Period) // Wait

	mockGameActor.ClearMessages() // Use safe method
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(utils.Period * 2) // Wait longer

	received = mockGameActor.GetMessages()
	massUpdated := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok && posMsg.Ball.Id == initialBall.Id {
			expectedMass := initialMass + massAdd
			expectedRadius := initialRadius + massAdd*2 // Based on current simple formula
			assert.Equal(t, expectedMass, posMsg.Ball.Mass, "Mass should be increased")
			assert.Equal(t, expectedRadius, posMsg.Ball.Radius, "Radius should be increased")
			massUpdated = true
			initialMass, initialRadius = posMsg.Ball.Mass, posMsg.Ball.Radius // Update
			break
		}
	}
	assert.True(t, massUpdated, "Position message with updated mass/radius not received")

	// --- Test Phasing ---
	phasingDuration := 100 * time.Millisecond // Increase duration slightly
	engine.Send(ballPID, SetPhasingCommand{ExpireIn: phasingDuration}, nil)
	time.Sleep(utils.Period) // Wait

	mockGameActor.ClearMessages() // Use safe method
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(utils.Period * 2) // Wait longer

	received = mockGameActor.GetMessages()
	phasingStarted := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok && posMsg.Ball.Id == initialBall.Id {
			assert.True(t, posMsg.Ball.Phasing, "Ball should be phasing after SetPhasingCommand")
			phasingStarted = true
			break
		}
	}
	assert.True(t, phasingStarted, "Position message with phasing=true not received")

	// Wait for phasing to expire
	time.Sleep(phasingDuration + utils.Period*2) // Wait longer than duration

	mockGameActor.ClearMessages()              // Use safe method
	engine.Send(ballPID, &internalTick{}, nil) // Force another tick
	time.Sleep(utils.Period * 2)               // Wait longer

	received = mockGameActor.GetMessages()
	phasingEnded := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok && posMsg.Ball.Id == initialBall.Id {
			assert.False(t, posMsg.Ball.Phasing, "Ball should not be phasing after timer expires")
			phasingEnded = true
			break
		}
	}
	assert.True(t, phasingEnded, "Position message with phasing=false not received after expiry")

	// --- Test Reflect Velocity ---
	engine.Send(ballPID, ReflectVelocityCommand{Axis: "X"}, nil)
	time.Sleep(utils.Period) // Wait

	mockGameActor.ClearMessages() // Use safe method
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(utils.Period * 2) // Wait longer

	received = mockGameActor.GetMessages()
	reflectXDone := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok && posMsg.Ball.Id == initialBall.Id {
			assert.Equal(t, -initialVx, posMsg.Ball.Vx, "Vx should be reflected")
			assert.Equal(t, initialVy, posMsg.Ball.Vy, "Vy should be unchanged") // Check Vy didn't change
			reflectXDone = true
			initialVx = posMsg.Ball.Vx // Update for next check
			break
		}
	}
	assert.True(t, reflectXDone, "Position message with reflected X velocity not received")

}
