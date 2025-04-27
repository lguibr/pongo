// File: game/paddle_actor_test.go
package game

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lguibr/pongo/bollywood"
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
	// fmt.Printf("MockGameActor received: %T from %s\n", ctx.Message(), ctx.Sender()) // Debug
	// Make a copy of the message if it's a pointer type we might modify elsewhere,
	// though for simple structs/messages it might not be strictly necessary.
	// For simplicity here, we append directly. Be cautious if messages are complex pointers.
	tr.received = append(tr.received, ctx.Message())
}

func (tr *MockGameActor) GetMessages() []interface{} {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	// Return a copy to avoid race conditions if the caller modifies the slice
	msgs := make([]interface{}, len(tr.received))
	copy(msgs, tr.received)
	return msgs
}

// ClearMessages safely clears the received messages slice.
func (tr *MockGameActor) ClearMessages() {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.received = nil // Reset the slice
}

// --- Paddle Actor Test ---

func TestPaddleActor_ReceivesDirectionAndSendsPosition(t *testing.T) {
	// 1. Setup Engine and Mock GameActor
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second)

	mockGameActor := &MockGameActor{}
	mockGameActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockGameActor }))
	assert.NotNil(t, mockGameActorPID, "Mock GameActor PID should not be nil")
	time.Sleep(50 * time.Millisecond) // Wait for mock actor start

	// 2. Setup Paddle Actor
	initialPaddleState := NewPaddle(utils.CanvasSize, 0) // Paddle 0
	initialY := initialPaddleState.Y
	paddleProducer := NewPaddleActorProducer(*initialPaddleState, mockGameActorPID) // Pass mock PID
	paddlePID := engine.Spawn(bollywood.NewProps(paddleProducer))
	assert.NotNil(t, paddlePID, "Paddle PID should not be nil")
	time.Sleep(utils.Period * 5) // Wait longer for actor start and initial position send

	// 3. Verify Initial Position Sent
	receivedByGame := mockGameActor.GetMessages()
	initialPosFound := false
	for _, msg := range receivedByGame {
		if posMsg, ok := msg.(PaddlePositionMessage); ok {
			assert.Equal(t, initialPaddleState.X, posMsg.Paddle.X)
			assert.Equal(t, initialPaddleState.Y, posMsg.Paddle.Y)
			initialPosFound = true
			break // Assuming only one initial message
		}
	}
	assert.True(t, initialPosFound, "Mock GameActor should have received initial position")
	mockGameActor.ClearMessages() // Use the safe method

	// 4. Send Direction Message ("ArrowRight" -> "right")
	directionMsgPayload, _ := json.Marshal(Direction{Direction: "ArrowRight"})
	directionMsg := PaddleDirectionMessage{Direction: directionMsgPayload}
	engine.Send(paddlePID, directionMsg, nil)
	time.Sleep(utils.Period * 2) // Wait for direction processing

	// 5. Force Ticks and Check Position Message Output
	engine.Send(paddlePID, &internalTick{}, nil) // Force tick 1
	time.Sleep(utils.Period * 2)
	engine.Send(paddlePID, &internalTick{}, nil) // Force tick 2
	time.Sleep(utils.Period * 2)

	// 6. Verify Position Messages Received by Mock GameActor
	receivedByGame = mockGameActor.GetMessages()
	assert.GreaterOrEqual(t, len(receivedByGame), 2, "Should receive at least two position updates after ticks")

	foundMovedPositionMsg := false
	var lastPaddlePos *Paddle
	for _, msg := range receivedByGame {
		if posMsg, ok := msg.(PaddlePositionMessage); ok {
			// Make a copy of the paddle state from the message before using it
			paddleCopy := *posMsg.Paddle
			lastPaddlePos = &paddleCopy // Use the copy

			fmt.Printf("MockGameActor received PaddlePosition: X=%d, Y=%d, Dir=%s\n", lastPaddlePos.X, lastPaddlePos.Y, lastPaddlePos.Direction)
			if lastPaddlePos.Y > initialY { // Check if movement occurred (Paddle 0 moves Y)
				assert.Equal(t, "right", lastPaddlePos.Direction, "Paddle direction in position message should be 'right'")
				foundMovedPositionMsg = true
			}
		}
	}
	assert.True(t, foundMovedPositionMsg, "Mock GameActor should have received PaddlePositionMessage showing movement")
	assert.NotNil(t, lastPaddlePos, "Last paddle position should not be nil")
}
