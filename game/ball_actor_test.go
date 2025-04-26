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

// Using TestReceiver from paddle_actor_test.go - ensure it's available or redefine it here.
// Assuming TestReceiver is available in the package.

func TestBallActor_SpawnsAndMoves(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second)

	receiver := &TestReceiver{}
	receiverPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return receiver }))
	time.Sleep(10 * time.Millisecond) // Wait for receiver

	initialBall := NewBall(100, 100, 10, utils.CanvasSize, 0, 123)
	initialX, initialY := initialBall.X, initialBall.Y
	// initialVx, initialVy := initialBall.Vx, initialBall.Vy // No longer needed for assertion

	ballProducer := NewBallActorProducer(*initialBall, receiverPID)
	ballPID := engine.Spawn(bollywood.NewProps(ballProducer))
	assert.NotNil(t, ballPID)

	time.Sleep(utils.Period * 3) // Wait for actor to start and potentially send initial position + tick

	// Check if initial position was sent
	received := receiver.GetMessages()
	initialPosFound := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok {
			if posMsg.Ball.Id == initialBall.Id && posMsg.Ball.X == initialX && posMsg.Ball.Y == initialY {
				initialPosFound = true
				break
			}
		}
	}
	assert.True(t, initialPosFound, "Should have received initial position message")

	// Force a tick and check for movement
	receiver.received = nil // Clear previous messages
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(utils.Period * 2) // Wait for processing

	received = receiver.GetMessages()
	movedPosFound := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok {
			if posMsg.Ball.Id == initialBall.Id {
				movedPosFound = true
				// Check if position changed from initial state (cannot predict exact change due to random V)
				assert.NotEqual(t, initialX, posMsg.Ball.X, "Ball X should change")
				assert.NotEqual(t, initialY, posMsg.Ball.Y, "Ball Y should change")
				fmt.Printf("Ball moved to X=%d, Y=%d\n", posMsg.Ball.X, posMsg.Ball.Y)
				break
			}
		}
	}
	assert.True(t, movedPosFound, "Should have received position message after tick")
}

func TestBallActor_Commands(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second)

	receiver := &TestReceiver{} // GameActor mock
	receiverPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return receiver }))
	time.Sleep(10 * time.Millisecond)

	initialBall := NewBall(100, 100, 10, utils.CanvasSize, 0, 456)
	initialVx, initialVy := initialBall.Vx, initialBall.Vy
    initialMass, initialRadius := initialBall.Mass, initialBall.Radius


	ballProducer := NewBallActorProducer(*initialBall, receiverPID)
	ballPID := engine.Spawn(bollywood.NewProps(ballProducer))
	time.Sleep(utils.Period) // Wait for start

	// --- Test Velocity Increase ---
	velRatio := 1.5
	engine.Send(ballPID, IncreaseVelocityCommand{Ratio: velRatio}, nil)
	time.Sleep(utils.Period) // Wait for processing

	// Force tick to get updated state
	receiver.received = nil
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(utils.Period * 2)

	received := receiver.GetMessages()
	velUpdated := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok && posMsg.Ball.Id == initialBall.Id {
            expectedVx := int(math.Floor(float64(initialVx) * velRatio))
            expectedVy := int(math.Floor(float64(initialVy) * velRatio))
			assert.Equal(t, expectedVx, posMsg.Ball.Vx, "Vx should be increased")
			assert.Equal(t, expectedVy, posMsg.Ball.Vy, "Vy should be increased")
			velUpdated = true
			break
		}
	}
	assert.True(t, velUpdated, "Position message with updated velocity not received")


    // --- Test Mass Increase ---
    massAdd := 5
	engine.Send(ballPID, IncreaseMassCommand{Additional: massAdd}, nil)
	time.Sleep(utils.Period) // Wait for processing

    // Force tick
	receiver.received = nil
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(utils.Period * 2)

	received = receiver.GetMessages()
	massUpdated := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok && posMsg.Ball.Id == initialBall.Id {
            expectedMass := initialMass + massAdd
            expectedRadius := initialRadius + massAdd * 2 // Based on current simple formula
			assert.Equal(t, expectedMass, posMsg.Ball.Mass, "Mass should be increased")
            assert.Equal(t, expectedRadius, posMsg.Ball.Radius, "Radius should be increased")
			massUpdated = true
			break
		}
	}
	assert.True(t, massUpdated, "Position message with updated mass/radius not received")


    // --- Test Phasing ---
    phasingDuration := 50 * time.Millisecond
	engine.Send(ballPID, SetPhasingCommand{ExpireIn: phasingDuration}, nil)
	time.Sleep(utils.Period) // Wait for processing SetPhasingCommand

    // Force tick and check if phasing is true
    receiver.received = nil
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(utils.Period * 2)

    received = receiver.GetMessages()
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
    time.Sleep(phasingDuration + utils.Period) // Wait longer than duration

    // Force tick and check if phasing is false
    receiver.received = nil
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(utils.Period * 2)

    received = receiver.GetMessages()
	phasingEnded := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok && posMsg.Ball.Id == initialBall.Id {
            assert.False(t, posMsg.Ball.Phasing, "Ball should not be phasing after timer expires")
			phasingEnded = true
			break
		}
	}
    assert.True(t, phasingEnded, "Position message with phasing=false not received after expiry")

}

