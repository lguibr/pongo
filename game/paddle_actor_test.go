package game

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/lguibr/pongo/bollywood"
	"github.com/lguibr/pongo/utils"
	"github.com/stretchr/testify/assert" // Using testify for assertions
)

// --- Test Receiver Actor ---

// TestReceiver captures messages sent to it.
type TestReceiver struct {
	mu       sync.Mutex
	received []interface{}
}

func (tr *TestReceiver) Receive(ctx bollywood.Context) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	// fmt.Printf("TestReceiver received: %T\n", ctx.Message()) // Debug
	tr.received = append(tr.received, ctx.Message())
}

func (tr *TestReceiver) GetMessages() []interface{} {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	// Return a copy
	msgs := make([]interface{}, len(tr.received))
	copy(msgs, tr.received)
	return msgs
}

// --- Paddle Actor Test ---

func TestPaddleActor_ReceivesDirectionAndMoves(t *testing.T) {
	// 1. Setup Engine and Test Receiver
	engine := bollywood.NewEngine()
	defer engine.Shutdown(1 * time.Second) // Ensure engine shutdown

	receiver := &TestReceiver{}
	receiverProps := bollywood.NewProps(func() bollywood.Actor { return receiver })
	receiverPID := engine.Spawn(receiverProps)
	assert.NotNil(t, receiverPID, "Receiver PID should not be nil")

	// Wait briefly for receiver to start (important!)
	time.Sleep(10 * time.Millisecond) 

	// 2. Setup Paddle Actor
	initialPaddleState := NewPaddle(utils.CanvasSize, 0) // Create initial data for paddle 0
    initialY := initialPaddleState.Y // Store initial Y
	paddleProducer := NewPaddleActorProducer(*initialPaddleState, receiverPID) // Create the producer func
	paddleProps := bollywood.NewProps(paddleProducer) // Wrap producer in Props
	paddlePID := engine.Spawn(paddleProps)
	assert.NotNil(t, paddlePID, "Paddle PID should not be nil")

	// Wait briefly for paddle actor and its ticker goroutine to start
	time.Sleep(utils.Period * 3) // Wait longer to ensure ticker starts sending

    // Retrieve the spawned actor instance to check its state (requires type assertion)
    // NOTE: Accessing actor state directly is generally discouraged in actor systems,
    // but necessary here for verifying internal state change without specific query messages.
    // A better approach might be to add a specific query message handled by the actor.
	// var paddleActorInstance *PaddleActor
    // This part is tricky - bollywood engine doesn't expose spawned actors directly.
    // We might need to modify the engine for testing or rely solely on message output.
    // For now, let's focus on message output verification.

	// 3. Send Direction Message ("ArrowRight" -> "right")
	directionMsgPayload, _ := json.Marshal(Direction{Direction: "ArrowRight"})
	directionMsg := PaddleDirectionMessage{Direction: directionMsgPayload}
	engine.Send(paddlePID, directionMsg, nil) // Send direction change

	// Wait for the actor to process the direction message and potentially a tick
	time.Sleep(utils.Period * 2) // Wait a bit longer than one tick period

    // --- Verification (State - requires access, skipped for now) ---
    // TODO: Need a way to get the actor's current state. 
    // If we could get paddleActorInstance:
    // assert.Equal(t, "right", paddleActorInstance.state.Direction, "Paddle direction should be updated to 'right'")


	// 4. Simulate Ticks and Check Position Message Output
    // Clear previous messages (like initial position updates before direction change)
    receiver.received = nil 

    // Manually send a tick message to force movement check immediately
    // (Alternatively, wait for the internal ticker)
	engine.Send(paddlePID, &internalTick{}, nil)
    time.Sleep(utils.Period) // Wait for tick processing and message sending
	engine.Send(paddlePID, &internalTick{}, nil)
    time.Sleep(utils.Period) // Wait for tick processing and message sending


	// 5. Verify Position Message Received
	receivedMessages := receiver.GetMessages()

	foundPositionMsg := false
	var lastPaddlePos *Paddle
	for _, msg := range receivedMessages {
		if posMsg, ok := msg.(PaddlePositionMessage); ok {
			foundPositionMsg = true
			lastPaddlePos = posMsg.Paddle // Get the state sent
			fmt.Printf("Received PaddlePositionMessage: X=%d, Y=%d, Dir=%s\n", lastPaddlePos.X, lastPaddlePos.Y, lastPaddlePos.Direction)
            // Check if movement occurred (Y should increase for index 0 moving right)
            assert.Greater(t, lastPaddlePos.Y, initialY, "Paddle Y position should have increased from initial Y")
            assert.Equal(t, "right", lastPaddlePos.Direction, "Paddle direction in position message should be 'right'")
		}
	}
	assert.True(t, foundPositionMsg, "TestReceiver should have received at least one PaddlePositionMessage after direction change and ticks")
    assert.NotNil(t, lastPaddlePos, "Last paddle position should not be nil")


	// 6. Stop Actor (Optional here as engine shutdown handles it)
	// engine.Stop(paddlePID)
	// time.Sleep(50 * time.Millisecond) // Allow time for stop processing
}


// TODO: Add more tests:
// - Test sending "ArrowLeft"
// - Test stopping the actor explicitly
// - Test initial state (before any messages)
// - Test boundary conditions for movement
