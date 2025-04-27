// File: game/ball_actor_test.go
package game

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/lguibr/bollywood"
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

	cfg := utils.DefaultConfig() // Create default config

	// Pass config and isPermanent=false (default for tests)
	initialBall := NewBall(cfg, 100, 100, 0, 123, false)
	initialX, initialY := initialBall.X, initialBall.Y

	// Pass config to producer
	ballProducer := NewBallActorProducer(*initialBall, mockGameActorPID, cfg)
	ballPID := engine.Spawn(bollywood.NewProps(ballProducer))
	assert.NotNil(t, ballPID)

	// Use config tick period for waiting
	time.Sleep(cfg.GameTickPeriod * 5)

	received := mockGameActor.GetMessages()
	initialPosFound := false
	var lastBallState *Ball
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok {
			ballCopy := *posMsg.Ball
			lastBallState = &ballCopy
			if posMsg.Ball.Id == initialBall.Id && posMsg.Ball.X == initialX && posMsg.Ball.Y == initialY {
				initialPosFound = true
			}
		}
	}
	assert.True(t, initialPosFound, "Should have received initial position message")
	assert.NotNil(t, lastBallState, "Should have received at least one position message")

	assert.NotEqual(t, initialX, lastBallState.X, "Ball X should change after ticks")
	assert.NotEqual(t, initialY, lastBallState.Y, "Ball Y should change after ticks")
	fmt.Printf("Ball moved to X=%d, Y=%d\n", lastBallState.X, lastBallState.Y)
}

func TestBallActor_ReceivesCommands(t *testing.T) {
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second)

	mockGameActor := &MockGameActor{}
	mockGameActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockGameActor }))
	time.Sleep(50 * time.Millisecond)

	cfg := utils.DefaultConfig() // Create default config

	// Pass config and isPermanent=false
	initialBall := NewBall(cfg, 100, 100, 0, 456, false)
	initialVx, initialVy := initialBall.Vx, initialBall.Vy
	initialMass, initialRadius := initialBall.Mass, initialBall.Radius

	// Pass config to producer
	ballProducer := NewBallActorProducer(*initialBall, mockGameActorPID, cfg)
	ballPID := engine.Spawn(bollywood.NewProps(ballProducer))
	time.Sleep(cfg.GameTickPeriod * 2) // Use config tick period

	// --- Test Velocity Increase ---
	velRatio := cfg.PowerUpIncreaseVelRatio // Use config ratio
	engine.Send(ballPID, IncreaseVelocityCommand{Ratio: velRatio}, nil)
	time.Sleep(cfg.GameTickPeriod)

	mockGameActor.ClearMessages()
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(cfg.GameTickPeriod * 2)

	received := mockGameActor.GetMessages()
	velUpdated := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok && posMsg.Ball.Id == initialBall.Id {
			expectedVx := int(math.Floor(float64(initialVx) * velRatio))
			expectedVy := int(math.Floor(float64(initialVy) * velRatio))
			if initialVx != 0 && expectedVx == 0 {
				expectedVx = int(math.Copysign(1, float64(initialVx)))
			}
			if initialVy != 0 && expectedVy == 0 {
				expectedVy = int(math.Copysign(1, float64(initialVy)))
			}

			assert.Equal(t, expectedVx, posMsg.Ball.Vx, "Vx should be increased")
			assert.Equal(t, expectedVy, posMsg.Ball.Vy, "Vy should be increased")
			velUpdated = true
			initialVx, initialVy = posMsg.Ball.Vx, posMsg.Ball.Vy
			break
		}
	}
	assert.True(t, velUpdated, "Position message with updated velocity not received")

	// --- Test Mass Increase ---
	massAdd := cfg.PowerUpIncreaseMassAdd // Use config value
	engine.Send(ballPID, IncreaseMassCommand{Additional: massAdd}, nil)
	time.Sleep(cfg.GameTickPeriod)

	mockGameActor.ClearMessages()
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(cfg.GameTickPeriod * 2)

	received = mockGameActor.GetMessages()
	massUpdated := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok && posMsg.Ball.Id == initialBall.Id {
			expectedMass := initialMass + massAdd
			// Use config for radius increase calculation
			expectedRadius := initialRadius + massAdd*cfg.PowerUpIncreaseMassSize
			assert.Equal(t, expectedMass, posMsg.Ball.Mass, "Mass should be increased")
			assert.Equal(t, expectedRadius, posMsg.Ball.Radius, "Radius should be increased")
			massUpdated = true
			initialMass, initialRadius = posMsg.Ball.Mass, posMsg.Ball.Radius
			break
		}
	}
	assert.True(t, massUpdated, "Position message with updated mass/radius not received")

	// --- Test Phasing ---
	phasingDuration := cfg.BallPhasingTime         // Use config value
	engine.Send(ballPID, SetPhasingCommand{}, nil) // ExpireIn is now handled by GameActor physics
	time.Sleep(cfg.GameTickPeriod)

	mockGameActor.ClearMessages()
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(cfg.GameTickPeriod * 2)

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

	time.Sleep(phasingDuration + cfg.GameTickPeriod*2) // Wait longer than duration

	mockGameActor.ClearMessages()
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(cfg.GameTickPeriod * 2)

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
	time.Sleep(cfg.GameTickPeriod)

	mockGameActor.ClearMessages()
	engine.Send(ballPID, &internalTick{}, nil)
	time.Sleep(cfg.GameTickPeriod * 2)

	received = mockGameActor.GetMessages()
	reflectXDone := false
	for _, msg := range received {
		if posMsg, ok := msg.(BallPositionMessage); ok && posMsg.Ball.Id == initialBall.Id {
			assert.Equal(t, -initialVx, posMsg.Ball.Vx, "Vx should be reflected")
			assert.Equal(t, initialVy, posMsg.Ball.Vy, "Vy should be unchanged")
			reflectXDone = true
			initialVx = posMsg.Ball.Vx
			break
		}
	}
	assert.True(t, reflectXDone, "Position message with reflected X velocity not received")
}
