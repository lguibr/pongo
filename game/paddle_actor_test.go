package game

import (
	"encoding/json"
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

// Helper to find the last message of a specific type sent to the mock GameActor
func findLastSentMessage[T any](t *testing.T, mockGameActor *MockGameActor) (*T, bool) {
	t.Helper()
	var lastMsg *T
	found := false
	messages := mockGameActor.GetMessages() // Get a copy
	for _, msg := range messages {
		if typedMsg, ok := msg.(T); ok {
			// Need to capture the value correctly in the loop
			msgCopy := typedMsg
			lastMsg = &msgCopy
			found = true
		}
	}
	return lastMsg, found
}

// --- Paddle Actor Test ---

// TestPaddleActor_ReceivesDirectionAndSendsUpdate verifies the actor sends state update on direction change.
func TestPaddleActor_ReceivesDirectionAndSendsUpdate(t *testing.T) {
	// 1. Setup Engine and Mock GameActor
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second)

	mockGameActor := &MockGameActor{}
	mockGameActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockGameActor }))
	assert.NotNil(t, mockGameActorPID, "Mock GameActor PID should not be nil")
	time.Sleep(50 * time.Millisecond)

	// 2. Setup Paddle Actor
	cfg := utils.DefaultConfig()
	initialPaddleStateValue := NewPaddle(cfg, 0) // Paddle 0
	initialPaddleStateValue.Direction = ""       // Start stopped

	paddleProducer := NewPaddleActorProducer(*initialPaddleStateValue, mockGameActorPID, cfg)
	paddlePID := engine.Spawn(bollywood.NewProps(paddleProducer))
	assert.NotNil(t, paddlePID, "Paddle PID should not be nil")
	time.Sleep(50 * time.Millisecond) // Allow actor to start

	// 3. Send Direction Message ("ArrowRight" -> "right")
	directionMsgPayload, _ := json.Marshal(Direction{Direction: "ArrowRight"})
	directionMsg := PaddleDirectionMessage{Direction: directionMsgPayload}
	engine.Send(paddlePID, directionMsg, nil)
	time.Sleep(cfg.GameTickPeriod * 2) // Allow direction processing and message sending

	// 4. Verify PaddleStateUpdate was sent to GameActor
	updateMsg, found := findLastSentMessage[PaddleStateUpdate](t, mockGameActor)
	assert.True(t, found, "GameActor should have received PaddleStateUpdate")
	if found {
		assert.Equal(t, 0, updateMsg.Index, "Update message index mismatch")
		assert.Equal(t, "right", updateMsg.Direction, "Update message direction mismatch")
		assert.Equal(t, paddlePID, updateMsg.PID, "Update message PID mismatch")
	}
	mockGameActor.ClearMessages() // Clear for next check

	// 5. Send Stop message
	stopMsgPayload, _ := json.Marshal(Direction{Direction: "Stop"})
	stopMsg := PaddleDirectionMessage{Direction: stopMsgPayload}
	engine.Send(paddlePID, stopMsg, nil)
	time.Sleep(cfg.GameTickPeriod * 2) // Allow stop processing and message sending

	// 6. Verify PaddleStateUpdate was sent for stop
	updateMsg, found = findLastSentMessage[PaddleStateUpdate](t, mockGameActor)
	assert.True(t, found, "GameActor should have received PaddleStateUpdate for stop")
	if found {
		assert.Equal(t, 0, updateMsg.Index, "Stop update message index mismatch")
		assert.Equal(t, "", updateMsg.Direction, "Stop update message direction mismatch")
		assert.Equal(t, paddlePID, updateMsg.PID, "Stop update message PID mismatch")
	}
	mockGameActor.ClearMessages()

	// 7. Send Same Stop message again (should not send update)
	engine.Send(paddlePID, stopMsg, nil)
	time.Sleep(cfg.GameTickPeriod * 2)
	_, found = findLastSentMessage[PaddleStateUpdate](t, mockGameActor) // Ignore updateMsg with _
	assert.False(t, found, "GameActor should NOT have received PaddleStateUpdate for same direction")

}
