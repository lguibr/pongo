// File: game/paddle_actor_test.go
package game

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lguibr/bollywood"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert"
)

// --- Test Receiver Actor (Mock GameActor) ---
type MockGameActor struct {
	mu       sync.Mutex
	received []interface{}
}

func (tr *MockGameActor) Receive(ctx bollywood.Context) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.received = append(tr.received, ctx.Message())
}

func (tr *MockGameActor) GetMessages() []interface{} {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	msgs := make([]interface{}, len(tr.received))
	copy(msgs, tr.received)
	return msgs
}

func (tr *MockGameActor) ClearMessages() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.received = nil
}

// --- Paddle Actor Test ---

func TestPaddleActor_ReceivesDirectionAndSendsPosition(t *testing.T) {
	// 1. Setup Engine and Mock GameActor
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second)

	mockGameActor := &MockGameActor{}
	mockGameActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockGameActor }))
	assert.NotNil(t, mockGameActorPID, "Mock GameActor PID should not be nil")
	time.Sleep(50 * time.Millisecond)

	// 2. Setup Paddle Actor
	cfg := utils.DefaultConfig() // Create default config
	// Pass config to NewPaddle
	initialPaddleState := NewPaddle(cfg, 0) // Paddle 0
	initialY := initialPaddleState.Y
	// Pass config to producer
	paddleProducer := NewPaddleActorProducer(*initialPaddleState, mockGameActorPID, cfg)
	paddlePID := engine.Spawn(bollywood.NewProps(paddleProducer))
	assert.NotNil(t, paddlePID, "Paddle PID should not be nil")
	time.Sleep(cfg.GameTickPeriod * 5) // Use config tick period

	// 3. Verify Initial Position Sent
	receivedByGame := mockGameActor.GetMessages()
	initialPosFound := false
	for _, msg := range receivedByGame {
		if posMsg, ok := msg.(PaddlePositionMessage); ok {
			assert.Equal(t, initialPaddleState.X, posMsg.Paddle.X)
			assert.Equal(t, initialPaddleState.Y, posMsg.Paddle.Y)
			initialPosFound = true
			break
		}
	}
	assert.True(t, initialPosFound, "Mock GameActor should have received initial position")
	mockGameActor.ClearMessages()

	// 4. Send Direction Message ("ArrowRight" -> "right")
	directionMsgPayload, _ := json.Marshal(Direction{Direction: "ArrowRight"})
	directionMsg := PaddleDirectionMessage{Direction: directionMsgPayload}
	engine.Send(paddlePID, directionMsg, nil)
	time.Sleep(cfg.GameTickPeriod * 2) // Use config tick period

	// 5. Force Ticks and Check Position Message Output
	engine.Send(paddlePID, &internalTick{}, nil)
	time.Sleep(cfg.GameTickPeriod * 2)
	engine.Send(paddlePID, &internalTick{}, nil)
	time.Sleep(cfg.GameTickPeriod * 2)

	// 6. Verify Position Messages Received by Mock GameActor
	receivedByGame = mockGameActor.GetMessages()
	assert.GreaterOrEqual(t, len(receivedByGame), 2, "Should receive at least two position updates after ticks")

	foundMovedPositionMsg := false
	var lastPaddlePos *Paddle
	for _, msg := range receivedByGame {
		if posMsg, ok := msg.(PaddlePositionMessage); ok {
			paddleCopy := *posMsg.Paddle
			lastPaddlePos = &paddleCopy

			fmt.Printf("MockGameActor received PaddlePosition: X=%d, Y=%d, Dir=%s, Vx=%d, Vy=%d\n",
				lastPaddlePos.X, lastPaddlePos.Y, lastPaddlePos.Direction, lastPaddlePos.Vx, lastPaddlePos.Vy)
			if lastPaddlePos.Y > initialY { // Check if movement occurred (Paddle 0 moves Y)
				assert.Equal(t, "right", lastPaddlePos.Direction, "Paddle direction in position message should be 'right'")
				assert.Equal(t, cfg.PaddleVelocity, lastPaddlePos.Vy, "Paddle Vy should match config velocity") // Check Vy
				assert.Equal(t, 0, lastPaddlePos.Vx, "Paddle Vx should be 0")                                   // Check Vx
				foundMovedPositionMsg = true
			}
		}
	}
	assert.True(t, foundMovedPositionMsg, "Mock GameActor should have received PaddlePositionMessage showing movement")
	assert.NotNil(t, lastPaddlePos, "Last paddle position should not be nil")

	// 7. Send Stop message
	stopMsgPayload, _ := json.Marshal(Direction{Direction: "Stop"})
	stopMsg := PaddleDirectionMessage{Direction: stopMsgPayload}
	engine.Send(paddlePID, stopMsg, nil)
	time.Sleep(cfg.GameTickPeriod * 2) // Wait for stop processing

	// 8. Force Tick and Check Position Message Output
	mockGameActor.ClearMessages()
	engine.Send(paddlePID, &internalTick{}, nil)
	time.Sleep(cfg.GameTickPeriod * 2)

	// 9. Verify Position Message shows stopped state
	receivedByGame = mockGameActor.GetMessages()
	assert.GreaterOrEqual(t, len(receivedByGame), 1, "Should receive at least one position update after stop")

	foundStoppedPositionMsg := false
	stoppedY := lastPaddlePos.Y // Y position should remain the same after stop
	for _, msg := range receivedByGame {
		if posMsg, ok := msg.(PaddlePositionMessage); ok {
			paddleCopy := *posMsg.Paddle
			lastPaddlePos = &paddleCopy

			fmt.Printf("MockGameActor received PaddlePosition after Stop: X=%d, Y=%d, Dir=%s, Vx=%d, Vy=%d\n",
				lastPaddlePos.X, lastPaddlePos.Y, lastPaddlePos.Direction, lastPaddlePos.Vx, lastPaddlePos.Vy)
			if lastPaddlePos.Direction == "" { // Check if direction is empty (stopped)
				assert.Equal(t, stoppedY, lastPaddlePos.Y, "Paddle Y should not change after stop")
				assert.Equal(t, 0, lastPaddlePos.Vx, "Paddle Vx should be 0 after stop")
				assert.Equal(t, 0, lastPaddlePos.Vy, "Paddle Vy should be 0 after stop")
				foundStoppedPositionMsg = true
				break
			}
		}
	}
	assert.True(t, foundStoppedPositionMsg, "Mock GameActor should have received PaddlePositionMessage showing stopped state")
}
