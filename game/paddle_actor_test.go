// File: game/paddle_actor_test.go
package game

import (
	"encoding/json" // Import errors
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

// Helper function to Ask for position
func askPaddlePosition(t *testing.T, engine *bollywood.Engine, pid *bollywood.PID) (*PositionResponse, error) {
	t.Helper()
	reply, err := engine.Ask(pid, GetPositionRequest{}, 100*time.Millisecond) // Short timeout for tests
	if err != nil {
		return nil, err
	}
	if posResp, ok := reply.(PositionResponse); ok {
		return &posResp, nil
	}
	return nil, fmt.Errorf("unexpected reply type: %T", reply)
}

// --- Paddle Actor Test ---

func TestPaddleActor_ReceivesDirectionAndMoves(t *testing.T) {
	// 1. Setup Engine and Mock GameActor
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second)

	mockGameActor := &MockGameActor{} // Not strictly needed, but keeps setup consistent
	mockGameActorPID := engine.Spawn(bollywood.NewProps(func() bollywood.Actor { return mockGameActor }))
	assert.NotNil(t, mockGameActorPID, "Mock GameActor PID should not be nil")
	time.Sleep(50 * time.Millisecond)

	// 2. Setup Paddle Actor
	cfg := utils.DefaultConfig()
	initialPaddleState := NewPaddle(cfg, 0) // Paddle 0 (Right edge, moves Up/Down)
	initialY := initialPaddleState.Y

	paddleProducer := NewPaddleActorProducer(*initialPaddleState, mockGameActorPID, cfg)
	paddlePID := engine.Spawn(bollywood.NewProps(paddleProducer))
	assert.NotNil(t, paddlePID, "Paddle PID should not be nil")
	time.Sleep(cfg.GameTickPeriod * 2) // Allow actor to start

	// 3. Verify Initial Position via Ask
	pos1, err1 := askPaddlePosition(t, engine, paddlePID)
	assert.NoError(t, err1)
	assert.NotNil(t, pos1)
	assert.Equal(t, initialPaddleState.X, pos1.X)
	assert.Equal(t, initialPaddleState.Y, pos1.Y)
	assert.False(t, pos1.IsMoving, "Paddle should initially not be moving")

	// 4. Send Direction Message ("ArrowRight" -> "right" -> Move Down for Paddle 0)
	directionMsgPayload, _ := json.Marshal(Direction{Direction: "ArrowRight"})
	directionMsg := PaddleDirectionMessage{Direction: directionMsgPayload}
	engine.Send(paddlePID, directionMsg, nil)
	time.Sleep(cfg.GameTickPeriod * 2) // Allow direction processing

	// 5. Send UpdatePosition command to trigger movement
	engine.Send(paddlePID, UpdatePositionCommand{}, nil)
	time.Sleep(cfg.GameTickPeriod * 2) // Allow movement processing

	// 6. Verify Position After Move via Ask
	pos2, err2 := askPaddlePosition(t, engine, paddlePID)
	assert.NoError(t, err2)
	assert.NotNil(t, pos2)
	assert.Equal(t, initialPaddleState.X, pos2.X, "X should not change for paddle 0")
	assert.Greater(t, pos2.Y, initialY, "Y should increase (move down) for paddle 0")
	assert.True(t, pos2.IsMoving, "Paddle should be moving")
	assert.Equal(t, cfg.PaddleVelocity, pos2.Vy, "Vy should match config velocity")
	assert.Equal(t, 0, pos2.Vx, "Vx should be 0")
	movedY := pos2.Y // Store position after move

	// 7. Send Stop message
	stopMsgPayload, _ := json.Marshal(Direction{Direction: "Stop"})
	stopMsg := PaddleDirectionMessage{Direction: stopMsgPayload}
	engine.Send(paddlePID, stopMsg, nil)
	time.Sleep(cfg.GameTickPeriod * 2) // Allow stop processing

	// 8. Send UpdatePosition command (should not move)
	engine.Send(paddlePID, UpdatePositionCommand{}, nil)
	time.Sleep(cfg.GameTickPeriod * 2) // Allow potential movement processing

	// 9. Verify Position After Stop via Ask
	pos3, err3 := askPaddlePosition(t, engine, paddlePID)
	assert.NoError(t, err3)
	assert.NotNil(t, pos3)
	assert.Equal(t, initialPaddleState.X, pos3.X, "X should remain unchanged after stop")
	assert.Equal(t, movedY, pos3.Y, "Y should remain unchanged after stop")
	assert.False(t, pos3.IsMoving, "Paddle should not be moving after stop")
	assert.Equal(t, 0, pos3.Vx, "Vx should be 0 after stop")
	assert.Equal(t, 0, pos3.Vy, "Vy should be 0 after stop")
}
